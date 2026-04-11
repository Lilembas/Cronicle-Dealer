package master

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

const (
	defaultWorkerPort = 9090
)

// Dispatcher 任务分发器
type Dispatcher struct {
	grpcClients map[string]pb.CronicleServiceClient
	conns       map[string]*grpc.ClientConn
}

// NewDispatcher 创建分发器
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		grpcClients: make(map[string]pb.CronicleServiceClient),
		conns:       make(map[string]*grpc.ClientConn),
	}
}

// DispatchEvent 分发任务
func (d *Dispatcher) DispatchEvent(event *models.Event) error {
	logger.Info("开始分发任务",
		zap.String("event_id", event.ID),
		zap.String("job_id", event.JobID))

	job, err := d.getJob(event.JobID)
	if err != nil {
		return fmt.Errorf("获取任务配置失败: %w", err)
	}

	node, err := d.selectNode(job)
	if err != nil {
		return fmt.Errorf("选择节点失败: %w", err)
	}

	if err := d.updateEventAndDispatch(event, node, job); err != nil {
		return err
	}

	logger.Info("任务分发成功",
		zap.String("event_id", event.ID),
		zap.String("node_id", node.ID),
		zap.String("node_name", node.Hostname))

	return nil
}

// AbortTask 中止正在运行的任务
func (d *Dispatcher) AbortTask(event *models.Event, reason string) error {
	if event == nil {
		return fmt.Errorf("event 不能为空")
	}
	if event.NodeID == "" {
		return fmt.Errorf("任务未分配执行节点")
	}

	var node models.Node
	if err := storage.DB.Where("id = ?", event.NodeID).First(&node).Error; err != nil {
		return fmt.Errorf("查询执行节点失败: %w", err)
	}

	client, err := d.getGRPCClient(&node)
	if err != nil {
		return fmt.Errorf("获取 gRPC 客户端失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.AbortTask(ctx, &pb.AbortTaskRequest{
		JobId:   event.JobID,
		EventId: event.ID,
		Reason:  reason,
	})
	if err != nil {
		return fmt.Errorf("调用 Worker AbortTask 失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("Worker 拒绝中止请求: %s", resp.Message)
	}

	logger.Info("任务中止请求已下发",
		zap.String("event_id", event.ID),
		zap.String("node_id", event.NodeID))

	return nil
}

// Close 关闭所有 gRPC 连接
func (d *Dispatcher) Close() {
	for nodeID, conn := range d.conns {
		logger.Info("关闭 gRPC 客户端", zap.String("node_id", nodeID))
		conn.Close()
	}
	d.grpcClients = make(map[string]pb.CronicleServiceClient)
	d.conns = make(map[string]*grpc.ClientConn)
}

// getJob 获取任务配置
func (d *Dispatcher) getJob(jobID string) (*models.Job, error) {
	var job models.Job
	if err := storage.DB.Where("id = ?", jobID).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// selectNode 选择执行节点
func (d *Dispatcher) selectNode(job *models.Job) (*models.Node, error) {
	var nodes []models.Node

	query := storage.DB.Where("status = ?", "online")

	switch job.TargetType {
	case "node_id":
		query = query.Where("id = ?", job.TargetValue)
	case "tags":
		query = query.Where("tags LIKE ?", "%"+job.TargetValue+"%")
	case "any":
		// 不添加额外条件
	}

	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("没有可用的节点")
	}

	return d.selectLeastBusyNode(nodes)
}

// selectLeastBusyNode 选择最空闲的节点
func (d *Dispatcher) selectLeastBusyNode(nodes []models.Node) (*models.Node, error) {
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

// updateEventAndDispatch 更新事件并分发任务
func (d *Dispatcher) updateEventAndDispatch(event *models.Event, node *models.Node, job *models.Job) error {
	event.NodeID = node.ID
	event.NodeName = node.Hostname
	event.Status = "running"

	if err := storage.DB.Save(event).Error; err != nil {
		return fmt.Errorf("更新任务记录失败: %w", err)
	}

	client, err := d.getGRPCClient(node)
	if err != nil {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("获取 gRPC 客户端失败: %v", err)
		storage.DB.Save(event)
		return fmt.Errorf("获取 gRPC 客户端失败: %w", err)
	}

	env, err := parseEnv(job.Env)
	if err != nil {
		logger.Warn("解析环境变量失败", zap.Error(err))
		env = make(map[string]string)
	}

	taskReq := &pb.TaskRequest{
		JobId:         job.ID,
		EventId:       event.ID,
		Type:          parseTaskType(job.TaskType),
		Command:       job.Command,
		Env:           env,
		Timeout:       int32(job.Timeout),
		WorkingDir:    job.WorkingDir,
		ScheduledTime: event.ScheduledTime.Unix(),
	}

	ctx := context.Background()
	resp, err := client.SubmitTask(ctx, taskReq)
	if err != nil {
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

	return nil
}

// getGRPCClient 获取或创建 gRPC 客户端
func (d *Dispatcher) getGRPCClient(node *models.Node) (pb.CronicleServiceClient, error) {
	if client, ok := d.grpcClients[node.ID]; ok {
		return client, nil
	}

	// 优先使用 Worker 注册时提供的 gRPC 地址
	var addr string
	if node.GRPCAddress != "" && !strings.HasSuffix(node.GRPCAddress, ":0") {
		// 验证grpc_address有效（端口不为0）
		addr = node.GRPCAddress
	} else {
		// 回退到使用 IP 和默认端口
		addr = fmt.Sprintf("%s:%d", node.IP, defaultWorkerPort)
		logger.Warn("Worker grpc_address无效，使用IP和默认端口",
			zap.String("node_id", node.ID),
			zap.String("grpc_address", node.GRPCAddress),
			zap.String("fallback_addr", addr))
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接节点失败: %w", err)
	}

	client := pb.NewCronicleServiceClient(conn)
	d.grpcClients[node.ID] = client
	d.conns[node.ID] = conn

	logger.Info("创建 gRPC 客户端", zap.String("node_id", node.ID), zap.String("address", addr))

	return client, nil
}

// parseEnv 解析环境变量
func parseEnv(envStr string) (map[string]string, error) {
	if envStr == "" {
		return make(map[string]string), nil
	}

	var env map[string]string
	if err := json.Unmarshal([]byte(envStr), &env); err != nil {
		return nil, err
	}

	if env == nil {
		env = make(map[string]string)
	}

	return env, nil
}

// parseTaskType 解析任务类型
func parseTaskType(taskType string) pb.TaskType {
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
