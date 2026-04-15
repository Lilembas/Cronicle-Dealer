package master

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"go.uber.org/zap"
	"gorm.io/gorm"

	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/utils"
)

const (
	defaultMaxConcurrent = 10
)

// GRPCServer Master 的 gRPC 服务器
type GRPCServer struct {
	pb.UnimplementedCronicleServiceServer

	cfg        *config.Config
	grpcServer *grpc.Server
	nodes      sync.Map
	wsServer   *WebSocketServer // WebSocket服务器
}

// NewGRPCServer 创建 gRPC 服务器
func NewGRPCServer(cfg *config.Config) *GRPCServer {
	return &GRPCServer{
		cfg: cfg,
	}
}

// SetWebSocketServer 设置WebSocket服务器
func (s *GRPCServer) SetWebSocketServer(wsServer *WebSocketServer) {
	s.wsServer = wsServer
}

// Start 启动 gRPC 服务器
func (s *GRPCServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.GRPCPort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterCronicleServiceServer(s.grpcServer, s)

	logger.Info("gRPC 服务器启动", zap.String("address", addr))

	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			logger.Error("gRPC 服务器运行失败", zap.Error(err))
		}
	}()

	return nil
}

// Stop 停止 gRPC 服务器
func (s *GRPCServer) Stop() {
	if s.grpcServer != nil {
		logger.Info("停止 gRPC 服务器...")
		s.grpcServer.GracefulStop()
	}
}

// RegisterNode Worker 注册
func (s *GRPCServer) RegisterNode(ctx context.Context, req *pb.RegisterNodeRequest) (*pb.RegisterNodeResponse, error) {
	logger.Info("收到节点注册请求",
		zap.String("hostname", req.Hostname),
		zap.String("ip", req.Ip),
		zap.Int32("pid", req.Pid))

	// 检查是否已存在相同 hostname + ip + tags != "master" 的节点（不更新 Master 节点）
	var existingNode models.Node
	err := storage.DB.Where("hostname = ? AND ip = ? AND (tags = '' OR tags != 'master')", req.Hostname, req.Ip).
		Order("created_at DESC").
		First(&existingNode).Error

	var nodeID string
	var isNewNode bool

	if err == nil {
		// 节点已存在，复用该节点ID（节点重连）
		nodeID = existingNode.ID
		isNewNode = false
		logger.Info("Worker 节点重新上线",
			zap.String("hostname", req.Hostname),
			zap.String("ip", req.Ip),
			zap.String("existing_node_id", nodeID),
			zap.String("old_status", existingNode.Status),
			zap.Int32("old_pid", existingNode.PID))
	} else {
		// 新节点，生成新ID
		nodeID = utils.GenerateID("node")
		isNewNode = true
		logger.Info("新 Worker 节点注册",
			zap.String("hostname", req.Hostname),
			zap.String("ip", req.Ip),
			zap.Int32("pid", req.Pid))
	}

	node := s.buildNode(nodeID, req)

	if isNewNode {
		// 创建新节点记录
		if err := storage.DB.Create(node).Error; err != nil {
			logger.Error("保存节点信息失败", zap.Error(err))
			return &pb.RegisterNodeResponse{
				Success: false,
				Message: "保存节点信息失败",
			}, nil
		}
	} else {
		// 更新现有节点记录，但保留创建时间
		updates := map[string]interface{}{
			"g_rpc_address": node.GRPCAddress,
			"tags":           node.Tags,
			"pid":            node.PID,
			"status":         "online", // 重新上线时设置为在线
			"cpu_cores":      node.CPUCores,
			"cpu_usage":      node.CPUUsage,
			"memory_total":   node.MemoryTotal,
			"memory_usage":   node.MemoryUsage,
			"memory_percent": node.MemoryPercent,
			"disk_total":     node.DiskTotal,
			"disk_usage":     node.DiskUsage,
			"disk_percent":   node.DiskPercent,
			"version":        node.Version,
			"last_heartbeat": time.Now(),
		}

		if err := storage.DB.Model(&models.Node{}).Where("id = ?", nodeID).Updates(updates).Error; err != nil {
			logger.Error("更新节点信息失败", zap.Error(err))
			return &pb.RegisterNodeResponse{
				Success: false,
				Message: "更新节点信息失败",
			}, nil
		}
	}

	s.nodes.Store(nodeID, node)

	logger.Info("Worker 节点注册成功",
		zap.String("node_id", nodeID),
		zap.Bool("is_new", isNewNode))

	// 通过 WebSocket 推送 Worker 节点上线
	if s.wsServer != nil {
		if err := s.wsServer.BroadcastNodeStatus(nodeID, req.Hostname, "online",
			float64(node.CPUUsage), float64(node.MemoryPercent), 0); err != nil {
			logger.Warn("推送 Worker 节点状态失败", zap.Error(err))
		} else {
			logger.Info("已推送 Worker 节点上线状态", zap.String("node_id", nodeID))
		}
	}

	return &pb.RegisterNodeResponse{
		NodeId:        nodeID,
		Success:       true,
		Message:       "注册成功",
		SecurityToken: s.cfg.Security.WorkerToken,
	}, nil
}

// Heartbeat 心跳检测
func (s *GRPCServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	logger.Debug("收到心跳", zap.String("node_id", req.NodeId))

	var node models.Node
	if err := storage.DB.Where("id = ?", req.NodeId).First(&node).Error; err != nil {
		logger.Warn("节点不存在", zap.String("node_id", req.NodeId))
		return &pb.HeartbeatResponse{Success: false}, nil
	}

	s.updateNodeHeartbeat(&node, req)
	s.nodes.Store(req.NodeId, &node)

	return &pb.HeartbeatResponse{
		Success:    true,
		ServerTime: time.Now().Unix(),
	}, nil
}

// UnregisterNode Worker下线通知
func (s *GRPCServer) UnregisterNode(ctx context.Context, req *pb.UnregisterNodeRequest) (*pb.UnregisterNodeResponse, error) {
	logger.Info("收到Worker下线通知", zap.String("node_id", req.NodeId))

	var node models.Node
	if err := storage.DB.Where("id = ?", req.NodeId).First(&node).Error; err != nil {
		logger.Warn("节点不存在", zap.String("node_id", req.NodeId))
		return &pb.UnregisterNodeResponse{
			Success: false,
			Message: "节点不存在",
		}, nil
	}

	// 更新节点状态为offline
	if err := storage.DB.Model(&node).Updates(map[string]interface{}{
		"status": "offline",
	}).Error; err != nil {
		logger.Error("更新节点状态失败", zap.Error(err))
		return &pb.UnregisterNodeResponse{
			Success: false,
			Message: "更新节点状态失败",
		}, nil
	}

	// 从内存缓存中移除
	s.nodes.Delete(req.NodeId)

	logger.Info("Worker已下线", zap.String("node_id", req.NodeId), zap.String("hostname", node.Hostname))

	// 通过 WebSocket 推送 Worker 节点下线
	if s.wsServer != nil {
		if err := s.wsServer.BroadcastNodeStatus(node.ID, node.Hostname, "offline", 0, 0, 0); err != nil {
			logger.Warn("推送 Worker 下线状态失败", zap.Error(err))
		} else {
			logger.Info("已推送 Worker 下线状态", zap.String("node_id", node.ID))
		}
	}

	return &pb.UnregisterNodeResponse{
		Success: true,
		Message: "下线成功",
	}, nil
}

// SubmitTask 提交任务（Master -> Worker）
func (s *GRPCServer) SubmitTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	return &pb.TaskResponse{
		Accepted: false,
		Message:  "此接口由 Worker 实现",
	}, nil
}

// StreamLogs 接收日志流
func (s *GRPCServer) StreamLogs(stream pb.CronicleService_StreamLogsServer) error {
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// 流结束，返回最终确认
			return stream.SendAndClose(&pb.LogAck{Received: true})
		}
		if err != nil {
			logger.Error("接收日志流失败", zap.Error(err))
			return err
		}

		logger.Debug("收到日志",
			zap.String("job_id", chunk.JobId),
			zap.String("event_id", chunk.EventId),
			zap.Int("size", len(chunk.Content)))

		// 1. 将日志存储到Redis+文件供前端查询
		ctx := context.Background()
		if err := storage.SaveLogChunk(ctx, chunk.EventId, string(chunk.Content)); err != nil {
			logger.Error("存储日志失败", zap.Error(err))
		}

		// 2. 通过WebSocket实时推送到前端
		if s.wsServer != nil {
			content := string(chunk.Content)
			if err := s.wsServer.BroadcastLog(chunk.EventId, content); err != nil {
				logger.Error("WebSocket推送日志失败", zap.Error(err))
			}
		}
	}
}

// ReportTaskResult 接收任务执行结果
func (s *GRPCServer) ReportTaskResult(ctx context.Context, req *pb.TaskResult) (*pb.TaskResultAck, error) {
	logger.Info("收到任务执行结果",
		zap.String("job_id", req.JobId),
		zap.String("event_id", req.EventId),
		zap.Int32("exit_code", req.ExitCode))

	var event models.Event
	if err := storage.DB.Where("id = ?", req.EventId).First(&event).Error; err != nil {
		logger.Error("查询任务记录失败", zap.Error(err))
		return &pb.TaskResultAck{Received: false}, nil
	}

	status := "success"
	if strings.Contains(strings.ToLower(req.ErrorMessage), "abort") {
		status = eventStatusAborted
	} else if req.ExitCode != 0 {
		status = "failed"
	}

	updates := map[string]interface{}{
		"status":       status,
		"start_time":   utils.UnixToTime(req.StartTime),
		"end_time":     utils.UnixToTime(req.EndTime),
		"duration":     req.EndTime - req.StartTime,
		"exit_code":    req.ExitCode,
		"cpu_percent":  req.ResourceUsage.CpuPercent,
		"memory_bytes": req.ResourceUsage.MemoryBytes,
	}

	if req.ErrorMessage != "" {
		updates["error_message"] = req.ErrorMessage
	}

	if err := storage.DB.Model(&event).Updates(updates).Error; err != nil {
		logger.Error("更新任务记录失败", zap.Error(err))
		return &pb.TaskResultAck{Received: false}, nil
	}

	// 设置日志过期时间（任务完成后15分钟自动清理）
	if err := storage.SetLogExpiration(ctx, req.EventId); err != nil {
		logger.Warn("设置日志过期时间失败", zap.Error(err))
		// 不影响任务结果返回
	}

	// 更新 Job 的统计信息
	now := time.Now()
	jobUpdates := map[string]interface{}{
		"last_run_time": now,
	}
	if status == eventStatusSuccess {
		jobUpdates["success_runs"] = gorm.Expr("success_runs + 1")
	} else if status == eventStatusFailed {
		jobUpdates["failed_runs"] = gorm.Expr("failed_runs + 1")
	}
	if err := storage.DB.Model(&models.Job{}).Where("id = ?", req.JobId).Updates(jobUpdates).Error; err != nil {
		logger.Warn("更新任务统计信息失败", zap.String("job_id", req.JobId), zap.Error(err))
	}

	// TODO: 发送通知（Webhook、邮件等）、触发链式任务

	// 通过WebSocket推送任务状态变化
	if s.wsServer != nil {
		if err := s.wsServer.BroadcastTaskStatus(req.EventId, req.JobId, status, int(req.ExitCode)); err != nil {
			logger.Error("WebSocket推送任务状态失败", zap.Error(err))
		}
	}

	return &pb.TaskResultAck{Received: true}, nil
}

// AbortTask 中止任务
func (s *GRPCServer) AbortTask(ctx context.Context, req *pb.AbortTaskRequest) (*pb.AbortTaskResponse, error) {
	return &pb.AbortTaskResponse{
		Success: false,
		Message: "此接口由 Worker 实现",
	}, nil
}

// buildNode 构建节点对象
func (s *GRPCServer) buildNode(nodeID string, req *pb.RegisterNodeRequest) *models.Node {
	return &models.Node{
		ID:            nodeID,
		Hostname:      req.Hostname,
		IP:            req.Ip,
		GRPCAddress:   req.GrpcAddress,
		Tags:          tagsToString(req.Tags),
		PID:           req.Pid,
		Status:        "online",
		CPUCores:      int(req.Resources.CpuCores),
		CPUUsage:      req.Resources.CpuUsage,
		MemoryTotal:   req.Resources.MemoryTotal,
		MemoryUsage:   req.Resources.MemoryUsage,
		MemoryPercent: calculatePercent(req.Resources.MemoryUsage, req.Resources.MemoryTotal),
		DiskTotal:     req.Resources.DiskTotal,
		DiskUsage:     req.Resources.DiskUsage,
		DiskPercent:   calculatePercent(req.Resources.DiskUsage, req.Resources.DiskTotal),
		Version:       req.Version,
		RunningJobs:   0,
		MaxConcurrent: defaultMaxConcurrent,
		LastHeartbeat: time.Now(),
	}
}

// updateNodeHeartbeat 更新节点心跳信息
func (s *GRPCServer) updateNodeHeartbeat(node *models.Node, req *pb.HeartbeatRequest) {
	updates := map[string]interface{}{
		"cpu_usage":       req.Resources.CpuUsage,
		"memory_usage":    req.Resources.MemoryUsage,
		"memory_percent":  calculatePercent(req.Resources.MemoryUsage, req.Resources.MemoryTotal),
		"disk_usage":      req.Resources.DiskUsage,
		"disk_percent":    calculatePercent(req.Resources.DiskUsage, req.Resources.DiskTotal),
		"running_jobs":    len(req.RunningJobs),
		"last_heartbeat":  time.Now(),
		"status":          "online",
	}

	storage.DB.Model(node).Updates(updates)

	node.CPUUsage = req.Resources.CpuUsage
	node.MemoryUsage = req.Resources.MemoryUsage
	node.MemoryPercent = calculatePercent(req.Resources.MemoryUsage, req.Resources.MemoryTotal)
	node.DiskUsage = req.Resources.DiskUsage
	node.DiskPercent = calculatePercent(req.Resources.DiskUsage, req.Resources.DiskTotal)
	node.RunningJobs = len(req.RunningJobs)

	// 通过 WebSocket 推送节点状态更新
	if s.wsServer != nil {
		if err := s.wsServer.BroadcastNodeStatus(node.ID, node.Hostname, "online",
			float64(node.CPUUsage), float64(node.MemoryPercent), node.RunningJobs); err != nil {
			logger.Warn("推送节点状态失败", zap.String("node_id", node.ID), zap.Error(err))
		}
	}
}

// calculatePercent 计算百分比
func calculatePercent(value, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (value / total) * 100
}

// tagsToString 将标签数组转为 JSON 字符串
func tagsToString(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	return fmt.Sprintf("%v", tags)
}
