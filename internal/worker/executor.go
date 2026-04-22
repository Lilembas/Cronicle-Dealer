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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"runtime"

	"google.golang.org/grpc"
	"go.uber.org/zap"

	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/sysmetrics"
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
	managerClient pb.CronicleServiceClient // Manager客户端，用于报告任务结果
	runningJobs       sync.Map // map[eventID]*exec.Cmd
	runningCancelFuncs sync.Map // map[eventID]context.CancelFunc
	abortedJobs       sync.Map // map[eventID]string
	jobCount     int
	mu           sync.Mutex
}

// NewExecutor 创建执行器
func NewExecutor(cfg *config.ExecutorConfig) *Executor {
	return &Executor{
		cfg: cfg,
	}
}

// SetManagerClient 设置Manager客户端（用于报告任务结果）
func (e *Executor) SetManagerClient(client pb.CronicleServiceClient) {
	e.managerClient = client
}

// GetRunningJobIDs 获取当前运行的任务 ID 列表
func (e *Executor) GetRunningJobIDs() []string {
	var ids []string
	e.runningJobs.Range(func(key, _ interface{}) bool {
		if id, ok := key.(string); ok {
			ids = append(ids, id)
		}
		return true
	})
	return ids
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


	e.incrementJobCount()
	go e.executeTask(req)

	return &pb.TaskResponse{
		Accepted: true,
		Message:  "任务已接受",
	}, nil
}

// AbortTask 中止任务（幂等：对已结束或重复调用的任务均返回成功）
func (e *Executor) AbortTask(ctx context.Context, req *pb.AbortTaskRequest) (*pb.AbortTaskResponse, error) {
	logger.Info("收到中止任务请求", zap.String("event_id", req.EventId))

	reason := req.Reason
	if reason == "" {
		reason = "aborted by user"
	}
	e.abortedJobs.Store(req.EventId, reason)

	// 取消 context，触发 CommandContext 的取消逻辑
	if cancelVal, ok := e.runningCancelFuncs.Load(req.EventId); ok {
		if cancel, ok := cancelVal.(context.CancelFunc); ok {
			cancel()
		}
		e.runningCancelFuncs.Delete(req.EventId)
	}

	val, ok := e.runningJobs.Load(req.EventId)
	if !ok {
		logger.Info("任务已结束，无需中止", zap.String("event_id", req.EventId))
		return &pb.AbortTaskResponse{Success: true, Message: "任务已结束"}, nil
	}

	cmd, ok := val.(*exec.Cmd)
	if !ok || cmd == nil || cmd.Process == nil {
		return &pb.AbortTaskResponse{Success: true, Message: "任务已结束"}, nil
	}

	pid := cmd.Process.Pid

	// 检查进程是否已退出（竞态窗口：任务可能在毫秒级前自然结束）
	if !isProcessAlive(pid) {
		logger.Info("进程已退出，无需中止", zap.String("event_id", req.EventId), zap.Int("pid", pid))
		return &pb.AbortTaskResponse{Success: true, Message: "进程已退出"}, nil
	}

	// 1) SIGTERM 整个进程组
	logger.Info("发送 SIGTERM 到进程组", zap.String("event_id", req.EventId), zap.Int("pid", pid))
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		logger.Warn("发送 SIGTERM 失败，直接 SIGKILL",
			zap.String("event_id", req.EventId), zap.Int("pid", pid), zap.Error(err))
		if killErr := syscall.Kill(-pid, syscall.SIGKILL); killErr != nil {
			logger.Error("SIGKILL 也失败", zap.String("event_id", req.EventId), zap.Int("pid", pid), zap.Error(killErr))
		}
		return &pb.AbortTaskResponse{Success: true, Message: "任务中止请求已执行"}, nil
	}

	// 2) 轮询等待进程组退出（每 200ms 检查，最多 5 秒）
	const sigtermGracePeriod = 5 * time.Second
	const pollInterval = 200 * time.Millisecond
	timer := time.NewTimer(sigtermGracePeriod)
	defer timer.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			// 超时，进程组未退出，SIGKILL 兜底
			logger.Warn("SIGTERM 超时，发送 SIGKILL 到进程组",
				zap.String("event_id", req.EventId), zap.Int("pid", pid))
			if killErr := syscall.Kill(-pid, syscall.SIGKILL); killErr != nil {
				logger.Error("SIGKILL 失败",
					zap.String("event_id", req.EventId), zap.Int("pid", pid), zap.Error(killErr))
			}
			return &pb.AbortTaskResponse{Success: true, Message: "任务中止请求已执行（SIGKILL）"}, nil
		case <-ticker.C:
			if !isProcessGroupAlive(pid) {
				logger.Info("进程组已正常退出（SIGTERM 生效）",
					zap.String("event_id", req.EventId), zap.Int("pid", pid))
				return &pb.AbortTaskResponse{Success: true, Message: "任务中止请求已执行（SIGTERM）"}, nil
			}
		}
	}
}

// canAcceptJob 检查是否可以接受新任务

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
		e.runningCancelFuncs.Delete(req.EventId)
		e.abortedJobs.Delete(req.EventId)
	}()

	logger.Info("开始执行任务", zap.String("event_id", req.EventId))

	exitCode, output, stderr, cpuPercent, memoryBytes, err := e.executeByType(req)
	endTime := time.Now()

	status := taskStatusSuccess
	if exitCode != 0 {
		status = taskStatusFailed
	}
	if _, aborted := e.abortedJobs.Load(req.EventId); aborted {
		status = "aborted"
	}

	storage.SetTaskStatus(ctx, taskKey, status)
	e.recordTaskResult(ctx, taskKey, req, startTime, endTime, exitCode, output, stderr, cpuPercent, memoryBytes, err)

	logger.Info("任务执行完成",
		zap.String("event_id", req.EventId),
		zap.Int("exit_code", exitCode),
		zap.Float64("cpu_percent", cpuPercent),
		zap.Int64("memory_bytes", memoryBytes),
		zap.Duration("duration", endTime.Sub(startTime)))
}

// executeByType 根据任务类型执行
func (e *Executor) executeByType(req *pb.TaskRequest) (int, string, string, float64, int64, error) {
	switch req.Type {
	case pb.TaskType_SHELL:
		return e.executeShell(req)
	case pb.TaskType_HTTP:
		return e.executeHTTP(req)
	case pb.TaskType_DOCKER:
		return e.executeDocker(req)
	default:
		return 1, "", "", 0, 0, fmt.Errorf("不支持的任务类型: %v", req.Type)
	}
}

// executeShell 执行 Shell 脚本（日志直写 Redis + 文件，通过 Pub/Sub 实时推送）
// TODO: 若命令以 docker/podman/nerdctl run 开头，后续 Docker 执行器实现时需用 docker stop --time=5 优雅停止容器，
// 而非直接 kill CLI 进程，因为 SIGTERM 不会传递到容器内 PID 1。
func (e *Executor) executeShell(req *pb.TaskRequest) (int, string, string, float64, int64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.runningCancelFuncs.Store(req.EventId, cancel)
	defer e.runningCancelFuncs.Delete(req.EventId)

	if req.Timeout > 0 {
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
		return 1, "", "", 0, 0, fmt.Errorf("创建stdout管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, "", "", 0, 0, fmt.Errorf("创建stderr管道失败: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return 1, "", "", 0, 0, fmt.Errorf("启动命令失败: %w", err)
	}
	e.runningJobs.Store(req.EventId, cmd)

	// 启动进程资源采样
	var stats processStats
	monitorStop := make(chan struct{})
	go monitorProcess(cmd.Process.Pid, monitorStop, &stats)

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

	// 停止资源采样，等待命令完成
	close(monitorStop)

	// 等待命令执行完成（此时管道已读完，安全地 reap 子进程）
	err = cmd.Wait()

	// 计算平均资源使用
	stats.mu.Lock()
	avgCPU := calcAvgCPU(stats.cpuPercents)
	avgMem := calcAvgMemory(stats.memoryBytes)
	stats.mu.Unlock()

	exitCode, _ := extractExitCode(err)
	if reason, aborted := e.abortedJobs.Load(req.EventId); aborted {
		return 137, fullOutput.String(), stderrBuffer.String(), avgCPU, avgMem, fmt.Errorf("task aborted: %v", reason)
	}

	// 如果有错误且stderr不为空，增强错误消息
	if err != nil && stderrBuffer.Len() > 0 {
		stderrFirstLine := strings.SplitN(stderrBuffer.String(), "\n", 2)[0]
		if len(stderrFirstLine) > 0 && len(stderrFirstLine) < 256 {
			err = fmt.Errorf("%s: %s", err.Error(), stderrFirstLine)
		}
	}

	return exitCode, fullOutput.String(), stderrBuffer.String(), avgCPU, avgMem, err
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
func (e *Executor) executeHTTP(req *pb.TaskRequest) (int, string, string, float64, int64, error) {
	logger.Warn("HTTP 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, "", "", 0, 0, fmt.Errorf("HTTP 任务执行器未实现")
}

// executeDocker 执行 Docker 容器任务
func (e *Executor) executeDocker(req *pb.TaskRequest) (int, string, string, float64, int64, error) {
	logger.Warn("Docker 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, "", "", 0, 0, fmt.Errorf("Docker 任务执行器未实现")
}

// recordTaskResult 记录任务结果
func (e *Executor) recordTaskResult(ctx context.Context, taskKey string, req *pb.TaskRequest, startTime, endTime time.Time, exitCode int, output, stderr string, cpuPercent float64, memoryBytes int64, execErr error) {
	result := map[string]interface{}{
		"job_id":       req.JobId,
		"event_id":     req.EventId,
		"exit_code":    exitCode,
		"output":       output,
		"stderr":       stderr,
		"start_time":   startTime.Unix(),
		"end_time":     endTime.Unix(),
		"duration":     endTime.Sub(startTime).Seconds(),
		"cpu_percent":  cpuPercent,
		"memory_bytes": memoryBytes,
	}

	if execErr != nil {
		result["error_message"] = execErr.Error()
	}

	// 存储到Redis供Manager查询
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

	// 向Manager报告任务结果（同步调用，确保 Manager 收到后才返回）
	if e.managerClient != nil {
		e.reportToManager(req, startTime, endTime, exitCode, cpuPercent, memoryBytes, execErr)
	} else {
		logger.Warn("Manager客户端未设置，无法主动报告任务结果",
			zap.String("event_id", req.EventId))
	}

	logger.Debug("任务执行完成，结果已存储到Redis",
		zap.String("event_id", req.EventId),
		zap.String("status", status),
		zap.Int("exit_code", exitCode))
}

// reportToManager 向Manager报告任务执行结果
func (e *Executor) reportToManager(req *pb.TaskRequest, startTime, endTime time.Time, exitCode int, cpuPercent float64, memoryBytes int64, execErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := &pb.TaskResult{
		JobId:      req.JobId,
		EventId:    req.EventId,
		ExitCode:   int32(exitCode),
		StartTime:  startTime.Unix(),
		EndTime:    endTime.Unix(),
		ResourceUsage: &pb.ResourceUsage{
			CpuPercent:  cpuPercent,
			MemoryBytes: memoryBytes,
		},
	}

	if execErr != nil {
		result.ErrorMessage = execErr.Error()
	}

	ack, err := e.managerClient.ReportTaskResult(ctx, result)
	if err != nil {
		logger.Error("向Manager报告任务结果失败",
			zap.String("event_id", req.EventId),
			zap.Error(err))
		return
	}

	if !ack.Received {
		logger.Warn("Manager未正确接收任务结果",
			zap.String("event_id", req.EventId))
	} else {
		logger.Info("已成功向Manager报告任务结果",
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

// isProcessAlive 检查单个进程是否存活
func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// isProcessGroupAlive 检查进程组是否仍有存活的进程
// 通过向进程组发送信号 0 检测：返回 nil 表示至少有一个进程存活，返回 ESRCH 表示进程组已全部消失
func isProcessGroupAlive(pid int) bool {
	return syscall.Kill(-pid, 0) == nil
}

// processStats 保存进程资源采样数据
type processStats struct {
	cpuPercents []float64
	memoryBytes []int64
	mu          sync.Mutex
}

// monitorProcess 采样进程 CPU 和 RSS：启动后立即采一次，之后每 1 秒采样，停止前再采一次
func monitorProcess(pid int, stop <-chan struct{}, stats *processStats) {
	const sampleInterval = 1 * time.Second
	var prevUTime, prevSTime, prevTotal uint64
	inited := false

	sample := func() {
		cpuPercent, rssBytes, totalDelta, uTime, sTime, ok := readProcessStat(pid, prevUTime, prevSTime, prevTotal, inited)
		if !ok {
			return
		}
		stats.mu.Lock()
		if cpuPercent > 0 {
			stats.cpuPercents = append(stats.cpuPercents, cpuPercent)
		}
		stats.memoryBytes = append(stats.memoryBytes, rssBytes)
		stats.mu.Unlock()
		prevUTime = uTime
		prevSTime = sTime
		prevTotal = totalDelta
		inited = true
	}

	// 立即采样一次（初始化 prev 值 + 获取初始内存）
	sample()

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			// 停止前再采一次（进程可能仍在，捕获最后一次数据）
			sample()
			return
		case <-ticker.C:
			sample()
		}
	}
}

// readProcessStat 读取 /proc/<pid>/stat，返回 CPU% 和 RSS bytes
func readProcessStat(pid int, prevUTime, prevSTime, prevTotal uint64, inited bool) (cpuPercent float64, rssBytes int64, total uint64, uTime, sTime uint64, ok bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, 0, 0, 0, false
	}

	// /proc/<pid>/stat 格式：pid (comm) state ppid pgrp session tty_nr tpgid flags
	//   minflt cminflt majflt cmajflt utime stime cutime cstime priority nice
	//   num_threads itrealvalue starttime vsize rss ...
	// comm 可能包含括号和空格，从最后一个 ')' 之后开始解析
	content := string(data)
	idx := strings.LastIndex(content, ")")
	if idx < 0 {
		return 0, 0, 0, 0, 0, false
	}
	fields := strings.Fields(content[idx+2:])
	if len(fields) < 22 {
		return 0, 0, 0, 0, 0, false
	}

	// fields 索引（从 ')' 之后算起，state=0）：utime=11, stime=12, rss=21
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, false
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, false
	}
	rss, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, false
	}

	// 读取系统总 CPU 时间（同 /proc/stat 第一行）
	sysTotal, _, err := sysmetrics.ReadCPUStat()
	if err != nil {
		return 0, rss * int64(os.Getpagesize()), sysTotal, utime, stime, true
	}

	if !inited || prevTotal == 0 {
		return 0, rss * int64(os.Getpagesize()), sysTotal, utime, stime, true
	}

	totalDelta := sysTotal - prevTotal
	procDelta := (utime - prevUTime) + (stime - prevSTime)
	if totalDelta == 0 {
		return 0, rss * int64(os.Getpagesize()), sysTotal, utime, stime, true
	}

	cpuCores := uint64(runtime.NumCPU())
	usage := float64(procDelta) / float64(totalDelta) * 100.0 * float64(cpuCores)
	if usage < 0 {
		usage = 0
	}

	return usage, rss * int64(os.Getpagesize()), sysTotal, utime, stime, true
}

// calcAvgCPU 计算平均 CPU 使用率
func calcAvgCPU(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	return sum / float64(len(samples))
}

// calcAvgMemory 计算平均内存占用
func calcAvgMemory(samples []int64) int64 {
	if len(samples) == 0 {
		return 0
	}
	sum := int64(0)
	for _, v := range samples {
		sum += v
	}
	return sum / int64(len(samples))
}
