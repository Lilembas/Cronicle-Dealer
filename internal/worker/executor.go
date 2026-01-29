package worker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
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
)

// Executor 任务执行器
type Executor struct {
	pb.UnimplementedCronicleServiceServer

	cfg          *config.ExecutorConfig
	grpcServer   *grpc.Server
	masterClient pb.CronicleServiceClient // Master客户端，用于报告任务结果
	runningJobs  sync.Map                 // map[eventID]*exec.Cmd
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

	// TODO: 实现任务中止逻辑

	return &pb.AbortTaskResponse{
		Success: true,
		Message: "任务已中止",
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

	storage.SetTaskStatus(ctx, taskKey, "running")

	defer func() {
		e.decrementJobCount()
	}()

	logger.Info("开始执行任务", zap.String("event_id", req.EventId))

	exitCode, output, err := e.executeByType(req)
	endTime := time.Now()

	status := "success"
	if exitCode != 0 {
		status = "failed"
	}

	storage.SetTaskStatus(ctx, taskKey, status)
	e.recordTaskResult(ctx, taskKey, req, startTime, endTime, exitCode, output, err)

	logger.Info("任务执行完成",
		zap.String("event_id", req.EventId),
		zap.Int("exit_code", exitCode),
		zap.Duration("duration", endTime.Sub(startTime)))
}

// executeByType 根据任务类型执行
func (e *Executor) executeByType(req *pb.TaskRequest) (int, string, error) {
	switch req.Type {
	case pb.TaskType_SHELL:
		return e.executeShell(req)
	case pb.TaskType_HTTP:
		return e.executeHTTP(req)
	case pb.TaskType_DOCKER:
		return e.executeDocker(req)
	default:
		return 1, "", fmt.Errorf("不支持的任务类型: %v", req.Type)
	}
}

// executeShell 执行 Shell 脚本（支持流式输出）
func (e *Executor) executeShell(req *pb.TaskRequest) (int, string, error) {
	ctx := context.Background()
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", req.Command)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	if len(req.Env) > 0 {
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// 创建管道来捕获实时输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, "", fmt.Errorf("创建stdout管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, "", fmt.Errorf("创建stderr管道失败: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return 1, "", fmt.Errorf("启动命令失败: %w", err)
	}

	// 用于收集完整输出
	var fullOutput bytes.Buffer
	outputChan := make(chan []byte, 100) // 缓冲通道避免阻塞

	// 创建日志流（如果Master客户端可用）
	var logStream pb.CronicleService_StreamLogsClient
	if e.masterClient != nil {
		logStreamCtx, logStreamCancel := context.WithCancel(context.Background())
		defer logStreamCancel()

		logStream, err = e.masterClient.StreamLogs(logStreamCtx)
		if err != nil {
			logger.Warn("创建日志流失败，将只存储到内存", zap.String("event_id", req.EventId), zap.Error(err))
			logStream = nil
		}
	}

	// 启动goroutine读取并发送日志
	done := make(chan struct{})
	go func() {
		defer close(done)
		for chunk := range outputChan {
			fullOutput.Write(chunk)

			// 实时发送到Master
			if logStream != nil {
				chunk := &pb.LogChunk{
					EventId:    req.EventId,
					Content:    chunk,
					Timestamp:  time.Now().Unix(),
					StreamType: pb.StreamType_STDOUT,
				}

				if err := logStream.Send(chunk); err != nil {
					logger.Warn("发送日志失败", zap.String("event_id", req.EventId), zap.Error(err))
					break
				}
			}
		}
	}()

	// 启动goroutine读取stdout
	go e.readStream(stdout, req.EventId, "stdout", outputChan)
	// 启动goroutine读取stderr
	go e.readStream(stderr, req.EventId, "stderr", outputChan)

	// 等待命令执行完成
	err = cmd.Wait()
	close(outputChan) // 关闭输出通道
	<-done             // 等待日志发送完成

	// 关闭日志流
	if logStream != nil {
		if _, err := logStream.CloseAndRecv(); err != nil {
			logger.Warn("关闭日志流失败", zap.Error(err))
		}
	}

	exitCode, _ := extractExitCode(err)
	return exitCode, fullOutput.String(), err
}

// readStream 读取输出流
func (e *Executor) readStream(reader io.Reader, eventID, streamType string, outputChan chan<- []byte) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 1024) // 1KB缓冲区
	scanner.Buffer(buf, 10*1024*1024) // 最大10MB行

	for scanner.Scan() {
		line := scanner.Bytes()
		// 添加换行符
		lineWithNewline := append(line, '\n')
		outputChan <- lineWithNewline

		logger.Debug("实时输出",
			zap.String("event_id", eventID),
			zap.String("stream", streamType),
			zap.String("line", string(line)))
	}

	if err := scanner.Err(); err != nil {
		logger.Error("读取输出流失败",
			zap.String("event_id", eventID),
			zap.String("stream", streamType),
			zap.Error(err))
	}
}

// executeHTTP 执行 HTTP 请求
func (e *Executor) executeHTTP(req *pb.TaskRequest) (int, string, error) {
	logger.Warn("HTTP 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, "", fmt.Errorf("HTTP 任务执行器未实现")
}

// executeDocker 执行 Docker 容器任务
func (e *Executor) executeDocker(req *pb.TaskRequest) (int, string, error) {
	logger.Warn("Docker 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, "", fmt.Errorf("Docker 任务执行器未实现")
}

// recordTaskResult 记录任务结果
func (e *Executor) recordTaskResult(ctx context.Context, taskKey string, req *pb.TaskRequest, startTime, endTime time.Time, exitCode int, output string, execErr error) {
	result := map[string]interface{}{
		"job_id":       req.JobId,
		"event_id":     req.EventId,
		"exit_code":    exitCode,
		"output":       output,
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
	status := "completed"
	if exitCode != 0 || execErr != nil {
		status = "failed"
	}
	storage.SetTaskStatus(ctx, taskKey, status)

	// 向Master报告任务结果
	if e.masterClient != nil {
		go e.reportToMaster(req, startTime, endTime, exitCode, execErr)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
