package master

import (
	"context"
	"encoding/json"
	"fmt"
	
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	
	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// Dispatcher 任务分发器
type Dispatcher struct {
	grpcClients map[string]pb.CronicleServiceClient // nodeID -> gRPC 客户端
}

// NewDispatcher 创建分发器
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		grpcClients: make(map[string]pb.CronicleServiceClient),
	}
}

// DispatchEvent 分发任务
func (d *Dispatcher) DispatchEvent(event *models.Event) error {
	logger.Info("开始分发任务",
		zap.String("event_id", event.ID),
		zap.String("job_id", event.JobID))
	
	// 获取任务配置
	var job models.Job
	if err := storage.DB.Where("id = ?", event.JobID).First(&job).Error; err != nil {
		return fmt.Errorf("获取任务配置失败: %w", err)
	}
	
	// 选择目标节点
	node, err := d.selectNode(&job)
	if err != nil {
		return fmt.Errorf("选择节点失败: %w", err)
	}
	
	// 更新任务记录
	event.NodeID = node.ID
	event.NodeName = node.Hostname
	event.Status = "running"
	
	if err := storage.DB.Save(event).Error; err != nil {
		return fmt.Errorf("更新任务记录失败: %w", err)
	}
	
	// 获取或创建 gRPC 客户端
	client, err := d.getGRPCClient(node)
	if err != nil {
		return fmt.Errorf("获取 gRPC 客户端失败: %w", err)
	}
	
	// 解析环境变量
	env := make(map[string]string)
	if job.Env != "" {
		json.Unmarshal([]byte(job.Env), &env)
	}
	
	// 构建任务请求
	taskReq := &pb.TaskRequest{
		JobId:         job.ID,
		EventId:       event.ID,
		Type:          d.parseTaskType(job.TaskType),
		Command:       job.Command,
		Env:           env,
		Timeout:       int32(job.Timeout),
		WorkingDir:    job.WorkingDir,
		ScheduledTime: event.ScheduledTime.Unix(),
	}
	
	// 发送任务到 Worker
	ctx := context.Background()
	resp, err := client.SubmitTask(ctx, taskReq)
	if err != nil {
		// 任务发送失败，更新状态
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("发送任务失败: %v", err)
		storage.DB.Save(event)
		
		return fmt.Errorf("发送任务失败: %w", err)
	}
	
	if !resp.Accepted {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("Worker 拒绝任务: %s", resp.Message)
		storage.DB.Save(event)
		
		return fmt.Errorf("Worker 拒绝任务: %s", resp.Message)
	}
	
	logger.Info("任务分发成功",
		zap.String("event_id", event.ID),
		zap.String("node_id", node.ID),
		zap.String("node_name", node.Hostname))
	
	return nil
}

// selectNode 选择执行节点
func (d *Dispatcher) selectNode(job *models.Job) (*models.Node, error) {
	var nodes []models.Node
	
	// 根据目标类型查询节点
	query := storage.DB.Where("status = ?", "online")
	
	switch job.TargetType {
	case "node_id":
		// 指定节点 ID
		query = query.Where("id = ?", job.TargetValue)
		
	case "tags":
		// 匹配标签
		// NOTE: 这里简化处理，实际应该用 JSON 查询
		query = query.Where("tags LIKE ?", "%"+job.TargetValue+"%")
		
	case "any":
		// 任意节点，不添加额外条件
	}
	
	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}
	
	if len(nodes) == 0 {
		return nil, fmt.Errorf("没有可用的节点")
	}
	
	// 选择最空闲的节点（运行任务最少的）
	var selectedNode *models.Node
	minJobs := 999999
	
	for i := range nodes {
		node := &nodes[i]
		if node.CanAcceptJob() && node.RunningJobs < minJobs {
			minJobs = node.RunningJobs
			selectedNode = node
		}
	}
	
	if selectedNode == nil {
		return nil, fmt.Errorf("所有节点都已满载")
	}
	
	return selectedNode, nil
}

// getGRPCClient 获取或创建 gRPC 客户端
func (d *Dispatcher) getGRPCClient(node *models.Node) (pb.CronicleServiceClient, error) {
	// 检查是否已有客户端
	if client, ok := d.grpcClients[node.ID]; ok {
		return client, nil
	}
	
	// 创建新的 gRPC 连接
	// NOTE: 这里假设 Worker 的 gRPC 端口与 Master 相同（9090）
	addr := fmt.Sprintf("%s:9090", node.IP)
	
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接节点失败: %w", err)
	}
	
	client := pb.NewCronicleServiceClient(conn)
	d.grpcClients[node.ID] = client
	
	logger.Info("创建 gRPC 客户端", zap.String("node_id", node.ID), zap.String("address", addr))
	
	return client, nil
}

// parseTaskType 解析任务类型
func (d *Dispatcher) parseTaskType(taskType string) pb.TaskType {
	switch taskType {
	case "shell":
		return pb.TaskType_SHELL
	case "http":
		return pb.TaskType_HTTP
	case "docker":
		return pb.TaskType_DOCKER
	default:
		return pb.TaskType_SHELL
	}
}

// Close 关闭所有 gRPC 连接
func (d *Dispatcher) Close() {
	for nodeID := range d.grpcClients {
		logger.Info("关闭 gRPC 客户端", zap.String("node_id", nodeID))
		// NOTE: 需要保存连接对象才能关闭
	}
}
