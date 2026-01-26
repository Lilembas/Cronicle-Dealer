package worker

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"
	
	"google.golang.org/grpc"
	"go.uber.org/zap"
	
	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// Executor 任务执行器
type Executor struct {
	pb.UnimplementedCronicleServiceServer
	
	cfg         *config.ExecutorConfig
	grpcServer  *grpc.Server
	runningJobs sync.Map // map[eventID]*exec.Cmd
	jobCount    int
	mu          sync.Mutex
}

// NewExecutor 创建执行器
func NewExecutor(cfg *config.ExecutorConfig) *Executor {
	return &Executor{
		cfg: cfg,
	}
}

// Start 启动 gRPC 服务器（接收任务）
func (e *Executor) Start(port int) error {
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
			logger.Error("Worker gRPC 服务器启动失败", zap.Error(err))
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
	
	// 检查是否达到最大并发数
	e.mu.Lock()
	if e.jobCount >= e.cfg.MaxConcurrentJobs {
		e.mu.Unlock()
		return &pb.TaskResponse{
			Accepted: false,
			Message:  "已达到最大并发任务数",
		}, nil
	}
	e.jobCount++
	e.mu.Unlock()
	
	// 异步执行任务
	go e.executeTask(req)
	
	return &pb.TaskResponse{
		Accepted: true,
		Message:  "任务已接受",
	}, nil
}

// executeTask 执行任务
func (e *Executor) executeTask(req *pb.TaskRequest) {
	startTime := time.Now()
	
	defer func() {
		e.mu.Lock()
		e.jobCount--
		e.mu.Unlock()
	}()
	
	logger.Info("开始执行任务", zap.String("event_id", req.EventId))
	
	// 根据任务类型执行
	var err error
	var exitCode int
	
	switch req.Type {
	case pb.TaskType_SHELL:
		exitCode, err = e.executeShell(req)
	case pb.TaskType_HTTP:
		exitCode, err = e.executeHTTP(req)
	case pb.TaskType_DOCKER:
		exitCode, err = e.executeDocker(req)
	default:
		exitCode = 1
		err = fmt.Errorf("不支持的任务类型: %v", req.Type)
	}
	
	endTime := time.Now()
	
	// 上报执行结果
	result := &pb.TaskResult{
		JobId:     req.JobId,
		EventId:   req.EventId,
		ExitCode:  int32(exitCode),
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
		ResourceUsage: &pb.ResourceUsage{
			CpuPercent:      0, // TODO: 实际测量
			MemoryBytes:     0, // TODO: 实际测量
			ElapsedSeconds:  endTime.Sub(startTime).Seconds(),
		},
	}
	
	if err != nil {
		result.ErrorMessage = err.Error()
	}
	
	// TODO: 发送结果到 Master
	logger.Info("任务执行完成",
		zap.String("event_id", req.EventId),
		zap.Int("exit_code", exitCode),
		zap.Duration("duration", endTime.Sub(startTime)))
}

// executeShell 执行 Shell 脚本
func (e *Executor) executeShell(req *pb.TaskRequest) (int, error) {
	// 创建上下文，支持超时
	ctx := context.Background()
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
		defer cancel()
	}
	
	// 创建命令
	var cmd *exec.Cmd
	if req.WorkingDir != "" {
		cmd = exec.CommandContext(ctx, "sh", "-c", req.Command)
		cmd.Dir = req.WorkingDir
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", req.Command)
	}
	
	// 设置环境变量
	if len(req.Env) > 0 {
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	
	// 捕获输出
	output, err := cmd.CombinedOutput()
	
	logger.Debug("命令输出",
		zap.String("event_id", req.EventId),
		zap.String("output", string(output)))
	
	// TODO: 实时流式发送日志到 Master
	
	// 获取退出码
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	
	return exitCode, err
}

// executeHTTP 执行 HTTP 请求
func (e *Executor) executeHTTP(req *pb.TaskRequest) (int, error) {
	// TODO: 实现 HTTP 请求执行
	logger.Warn("HTTP 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, fmt.Errorf("HTTP 任务执行器未实现")
}

// executeDocker 执行 Docker 容器任务
func (e *Executor) executeDocker(req *pb.TaskRequest) (int, error) {
	// TODO: 实现 Docker 任务执行
	logger.Warn("Docker 任务执行器未实现", zap.String("event_id", req.EventId))
	return 1, fmt.Errorf("Docker 任务执行器未实现")
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
