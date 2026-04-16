package worker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"go.uber.org/zap"

	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

const (
	defaultWorkerPort = 9090
	// 任务状态常量
	taskStatusSuccess = "success"
	taskStatusFailed  = "failed"
	taskStatusRunning = "running"
	// STDERR缓冲区大小限制
	maxStderrBufferSize = 32 * 1024 // 32KB
)

// Executor 任务执行器
type Executor struct {
	pb.UnimplementedCronicleServiceServer

	cfg          *config.ExecutorConfig
	grpcServer   *grpc.Server
	masterClient pb.CronicleServiceClient // Master客户端，用于报告任务结果
	runningJobs  sync.Map                 // map[eventID]*exec.Cmd
	abortedJobs  sync.Map                 // map[eventID]string
	jobCount     int
	mu           sync.Mutex
}

// NewExecutor 创建执行器
func NewExecutor(cfg *config.ExecutorConfig) *Executor {
	return &Executor{
		cfg: cfg,
	}
}

// SetMasterClient 设置Master客户端（用于报告任务结果）
func (e *Executor) SetMasterClient(client pb.CronicleServiceClient) {
	e.masterClient = client
}

// Start 启动 gRPC 服务器（接收任务）
func (e *Executor) Start(port int) error {
	if port <= 0 {
		port = defaultWorkerPort
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	e.grpcServer = grpc.NewServer()
	pb.RegisterCronicleServiceServer(e.grpcServer, e)

	logger.Info("Worker gRPC 服务器启动", zap.String("address", addr))

	go func() {
		if err := e.grpcServer.Serve(listener); err != nil {
			logger.Error("Worker gRPC 服务器运行失败", zap.Error(err))
		}
	}()

	return nil
}

// Stop 停止执行器
func (e *Executor) Stop() {
	if e.grpcServer != nil {
		e.grpcServer.GracefulStop()
	}
}

// SubmitTask 接收任务
func (e *Executor) SubmitTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	logger.Info("收到任务",
		zap.String("job_id", req.JobId),
		zap.String("event_id", req.EventId),
		zap.String("command", req.Command))

	if !e.canAcceptJob() {
		return &pb.TaskResponse{
			Accepted: false,
			Message:  "已达到最大并发任务数",
		}, nil
	}

	e.incrementJobCount()
	go e.executeTask(req)

	return &pb.TaskResponse{
		Accepted: true,
		Message:  "任务已接受",
	}, nil
}

// AbortTask 中止任务
func (e *Executor) AbortTask(ctx context.Context, req *pb.AbortTaskRequest) (*pb.AbortTaskResponse, error) {
	logger.Info("收到中止任务请求", zap.String("event_id", req.EventId))
	val, ok := e.runningJobs.Load(req.EventId)
	if !ok {
		return &pb.AbortTaskResponse{
			Success: false,
			Message: "任务未运行或已结束",
		}, nil
	}

	cmd, ok := val.(*exec.Cmd)
	if !ok || cmd == nil || cmd.Process == nil {
		return &pb.AbortTaskResponse{
			Success: false,
			Message: "任务进程不可用",
		}, nil
	}

	reason := req.Reason
	if reason == "" {
		reason = "aborted by user"
	}
	e.abortedJobs.Store(req.EventId, reason)

	if err := cmd.Process.Kill(); err != nil {
		return &pb.AbortTaskResponse{
			Success: false,
			Message: "终止进程失败: " + err.Error(),
		}, nil
	}

	return &pb.AbortTaskResponse{
		Success: true,
		Message: "任务中止请求已执行",
	}, nil
}

// canAcceptJob 检查是否可以接受新任务
func (e *Executor) canAcceptJob() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.jobCount < e.cfg.MaxConcurrentJobs
}

// incrementJobCount 增加任务计数
func (e *Executor) incrementJobCount() {
	e.mu.Lock()
	e.jobCount++
	e.mu.Unlock()
}

// decrementJobCount 减少任务计数
func (e *Executor) decrementJobCount() {
	e.mu.Lock()
	e.jobCount--
	e.mu.Unlock()
}

// executeTask 执行任务
func (e *Executor) executeTask(req *pb.TaskRequest) {
	startTime := time.Now()
	taskKey := fmt.Sprintf("%s:%s", req.JobId, req.EventId)
	ctx := context.Background()

	storage.SetTaskStatus(ctx, taskKey, taskStatusRunning)

	defer func() {
		e.decrementJobCount()
		e.runningJobs.Delete(req.EventId)
		e.abortedJobs.Delete(req.EventId)
	}()

	logger.Info("开始执行任务", zap.String("event_id", req.EventId))

	exitCode, output, stderr, err := e.executeByType(req)
	endTime := time.Now()

	status := taskStatusSuccess
	if exitCode != 0 {
		status = taskStatusFailed
	}
	if _, aborted := e.abortedJobs.Load(req.EventId); aborted {
		status = "aborted"
	}

	storage.SetTaskStatus(ctx, taskKey, status)
	e.recordTaskResult(ctx, taskKey, req, startTime, endTime, exitCode, output, stderr, err)

	logger.Info("任务执行完成",
		zap.String("event_id", req.EventId),
		zap.Int("exit_code", exitCode),
		zap.Duration("duration", endTime.Sub(startTime)))
}

// executeByType 根据任务类型执行
func (e *Executor) executeByType(req *pb.TaskRequest) (int, string, string, error) {
	switch req.Type {
	case pb.TaskType_SHELL:
		return e.executeShell(req)
	case pb.TaskType_HTTP:
		return e.executeHTTP(req)
	case pb.TaskType_DOCKER:
		return e.executeDocker(req)
	default:
		return 1, "", "", fmt.Errorf("不支持的任务类型: %v", req.Type)
	}
}

// executeShell 执行 Shell 脚本（日志直写 Redis + 文件，通过 Pub/Sub 实时推送）
func (e *Executor) executeShell(req *pb.TaskRequest) (int, string, string, error) {
	ctx := context.Background()
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
		defer cancel()
	}

	// 【协议恢复】如果 gRPC 字段丢失，尝试从环境变量隧道恢复
	if req.Env != nil && req.Env["CRONICLE_STRICT_MODE"] == "true" {
		if !req.StrictMode {
			req.StrictMode = true
		}
	}

	// 安全地截取命令预览（最多100字符）
	commandPreview := req.Command
	if len(commandPreview) > 100 {
		commandPreview = commandPreview[:100] + "..."
	}

	logger.Info("执行任务参数",
		zap.String("event_id", req.EventId),
		zap.Bool("strict_mode", req.StrictMode),
		zap.String("command_preview", commandPreview))

	var cmd *exec.Cmd
	if req.StrictMode {
		logger.Info("使用严格模式执行任务", zap.String("event_id", req.EventId))
		cmd = exec.CommandContext(ctx, "/bin/bash", "-e", "-c", req.Command)
	} else {
		logger.Info("使用标准模式执行任务", zap.String("event_id", req.EventId))
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", req.Command)
	}

	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	cmd.Env = os.Environ()
	if len(req.Env) > 0 {
		envList := make([]string, 0, len(req.Env))
		for k, v := range req.Env {
			envList = append(envList, k+"="+v)
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		logger.Info("设置环境变量", zap.String("event_id", req.EventId), zap.Strings("keys", envList))
	}

	// 创建管道来捕获实时输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, "", "", fmt.Errorf("创建stdout管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, "", "", fmt.Errorf("创建stderr管道失败: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return 1, "", "", fmt.Errorf("启动命令失败: %w", err)
	}
	e.runningJobs.Store(req.EventId, cmd)

	// 用于收集完整输出和 stderr
	var fullOutput bytes.Buffer
	var stderrBuffer bytes.Buffer
	var wg sync.WaitGroup

	// 启动goroutine读取stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.readStreamAndStore(stdout, req.EventId, "stdout", &fullOutput, nil)
	}()

	// 启动goroutine读取stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.readStreamAndStore(stderr, req.EventId, "stderr", &fullOutput, &stderrBuffer)
	}()

	// 先等待所有读取goroutine完成（必须先于 cmd.Wait，否则管道 fd 会被关闭导致 "file already closed"）
	wg.Wait()

	// 关闭日志文件句柄，flush 到磁盘
	storage.CloseLogHandle(req.EventId)

	// 等待命令执行完成（此时管道已读完，安全地 reap 子进程）
	err = cmd.Wait()

	exitCode, _ := extractExitCode(err)
	if reason, aborted := e.abortedJobs.Load(req.EventId); aborted {
		return 137, fullOutput.String(), stderrBuffer.String(), fmt.Errorf("task aborted: %v", reason)
	}

	// 如果有错误且stderr不为空，增强错误消息
	if err != nil && stderrBuffer.Len() > 0 {
		stderrFirstLine := strings.SplitN(stderrBuffer.String(), "\n", 2)[0]
		if len(stderrFirstLine) > 0 && len(stderrFirstLine) < 256 {
			err = fmt.Errorf("%s: %s", err.Error(), stderrFirstLine)
		}
	}

	return exitCode, fullOutput.String(), stderrBuffer.String(), err
}

// readStreamAndStore 读取输出流，直写 Redis + 本地文件，发布 Pub/Sub 通知
func (e *Executor) readStreamAndStore(reader io.Reader, eventID, streamType string, fullOutput *bytes.Buffer, stderrBuffer *bytes.Buffer) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 1024) // 1KB缓冲区
	scanner.Buffer(buf, 10*1024*1024) // 最大10MB行
	ctx := context.Background()

	for scanner.Scan() {
		line := scanner.Bytes()
		content := string(line) + "\n"

		// 写入完整输出缓冲区
		fullOutput.WriteString(content)

		// 如果是 stderr，写入错误缓冲区
		if stderrBuffer != nil {
			if stderrBuffer.Len() < maxStderrBufferSize {
				stderrBuffer.WriteString(content)
			} else if stderrBuffer.Len() == maxStderrBufferSize {
				stderrBuffer.WriteString("...")
			}
		}

		// 直写 Redis（APPEND）+ 本地文件（Sync），Pub/Sub 实时通知
		storage.SaveLogChunk(ctx, eventID, content)
		storage.PublishLog(ctx, eventID, content)
	}

	if err := scanner.Err(); err != nil {
		logger.Error("读取输出流失败",
			zap.String("event_id", eventID),
			zap.String("stream", streamType),
			zap.Error(err))
	}
}

// executeHTTP 执行 HTTP 请求
func (e *Executor) executeHTTP(req *pb.TaskRequest) (int, string, string, error) {
	logger.Warn("HTTP 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, "", "", fmt.Errorf("HTTP 任务执行器未实现")
}

// executeDocker 执行 Docker 容器任务
func (e *Executor) executeDocker(req *pb.TaskRequest) (int, string, string, error) {
	logger.Warn("Docker 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, "", "", fmt.Errorf("Docker 任务执行器未实现")
}

// recordTaskResult 记录任务结果
func (e *Executor) recordTaskResult(ctx context.Context, taskKey string, req *pb.TaskRequest, startTime, endTime time.Time, exitCode int, output, stderr string, execErr error) {
	result := map[string]interface{}{
		"job_id":       req.JobId,
		"event_id":     req.EventId,
		"exit_code":    exitCode,
		"output":       output,
		"stderr":       stderr,
		"start_time":   startTime.Unix(),
		"end_time":     endTime.Unix(),
		"duration":     endTime.Sub(startTime).Seconds(),
		"cpu_percent":  0.0, // TODO: 实际测量
		"memory_bytes": 0,   // TODO: 实际测量
	}

	if execErr != nil {
		result["error_message"] = execErr.Error()
	}

	// 存储到Redis供Master查询
	storage.SetTaskResult(ctx, taskKey, result)

	// 更新任务状态
	status := taskStatusSuccess
	if exitCode != 0 || execErr != nil {
		status = taskStatusFailed
	}
	if _, aborted := e.abortedJobs.Load(req.EventId); aborted {
		status = "aborted"
	}
	storage.SetTaskStatus(ctx, taskKey, status)

	// 向Master报告任务结果（同步调用，确保 Master 收到后才返回）
	if e.masterClient != nil {
		e.reportToMaster(req, startTime, endTime, exitCode, execErr)
	} else {
		logger.Warn("Master客户端未设置，无法主动报告任务结果",
			zap.String("event_id", req.EventId))
	}

	logger.Debug("任务执行完成，结果已存储到Redis",
		zap.String("event_id", req.EventId),
		zap.String("status", status),
		zap.Int("exit_code", exitCode))
}

// reportToMaster 向Master报告任务执行结果
func (e *Executor) reportToMaster(req *pb.TaskRequest, startTime, endTime time.Time, exitCode int, execErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := &pb.TaskResult{
		JobId:      req.JobId,
		EventId:    req.EventId,
		ExitCode:   int32(exitCode),
		StartTime:  startTime.Unix(),
		EndTime:    endTime.Unix(),
		ResourceUsage: &pb.ResourceUsage{
			CpuPercent:  0.0, // TODO: 实际测量
			MemoryBytes: 0,   // TODO: 实际测量
		},
	}

	if execErr != nil {
		result.ErrorMessage = execErr.Error()
	}

	ack, err := e.masterClient.ReportTaskResult(ctx, result)
	if err != nil {
		logger.Error("向Master报告任务结果失败",
			zap.String("event_id", req.EventId),
			zap.Error(err))
		return
	}

	if !ack.Received {
		logger.Warn("Master未正确接收任务结果",
			zap.String("event_id", req.EventId))
	} else {
		logger.Info("已成功向Master报告任务结果",
			zap.String("event_id", req.EventId))
	}
}

// extractExitCode 从错误中提取退出码
func extractExitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), err
	}

	return 1, err
}
