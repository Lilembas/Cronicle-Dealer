package master

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
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
	mu          sync.Mutex
	grpcClients map[string]pb.CronicleServiceClient
	conns       map[string]*grpc.ClientConn
	wsServer    *WebSocketServer
}

// NewDispatcher 创建分发器
func NewDispatcher(wsServer *WebSocketServer) *Dispatcher {
	return &Dispatcher{
		grpcClients: make(map[string]pb.CronicleServiceClient),
		conns:       make(map[string]*grpc.ClientConn),
		wsServer:    wsServer,
	}
}

// DispatchEvent 分发任务（支持从 Redis 获取 ad-hoc 任务详情）
func (d *Dispatcher) DispatchEvent(event *models.Event, taskDetails map[string]string) error {
	logger.Info("开始分发任务",
		zap.String("event_id", event.ID),
		zap.String("job_id", event.JobID))

	// 关键改进：检查 Event 是否已经处理过（避免重试时重复处理）
	var existingEvent models.Event
	err := storage.DB.Where("id = ?", event.ID).First(&existingEvent).Error

	if err == nil {
		// Event 已存在，检查状态
		if existingEvent.Status == "running" {
			logger.Warn("任务已在运行中，跳过重复调度",
				zap.String("event_id", event.ID),
				zap.String("current_status", existingEvent.Status))
			return nil
		}
		if existingEvent.Status == "failed" || existingEvent.Status == "success" ||
		   existingEvent.Status == "aborted" || existingEvent.Status == "timeout" {
			logger.Warn("任务已完成，跳过重复调度",
				zap.String("event_id", event.ID),
				zap.String("current_status", existingEvent.Status))
			return nil
		}
		// 如果是 pending 状态，继续处理
			// 复用 DB 中已有的 StartTime，避免重试时重复初始化日志
			if existingEvent.StartTime != nil && !existingEvent.StartTime.IsZero() {
				event.StartTime = existingEvent.StartTime
				event.LogPath = existingEvent.LogPath
			}
	}

	// 只在第一次调度时创建日志文件路径
	if event.LogPath == "" {
		event.LogPath = fmt.Sprintf("/var/log/cronicle/events/%s.log", event.ID)

		// 同步写入数据库
		if err := storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).
			Update("log_path", event.LogPath).Error; err != nil {
			logger.Warn("更新事件 log_path 失败",
				zap.String("event_id", event.ID),
				zap.Error(err))
		}

		// 记录调度开始日志
		now := time.Now()
		dispatchLog := fmt.Sprintf("[%s] [Master] 任务开始调度\n", now.Format("2006-01-02 15:04:05"))
		dispatchLog += fmt.Sprintf("[%s] [Master] 任务ID: %s, 作业ID: %s\n", now.Format("2006-01-02 15:04:05"), event.ID, event.JobID)

		if logErr := storage.SaveLogChunk(context.Background(), event.ID, dispatchLog); logErr != nil {
			logger.Warn("写入调度日志失败", zap.Error(logErr))
		}
	}

	var job *models.Job

	// 如果提供了 taskDetails（从 Redis），优先使用；否则从数据库查询
	if len(taskDetails) > 0 {
		job = &models.Job{
			ID:         taskDetails["job_id"],
			Name:       taskDetails["job_name"],
			Command:    taskDetails["command"],
			TaskType:   taskDetails["task_type"],
			Timeout:    parseIntDefault(taskDetails["timeout"], 30),
			TargetType: taskDetails["target_type"],
			TargetValue: taskDetails["target_value"],
			StrictMode: parseBoolDefault(taskDetails["strict_mode"], false),
			Env:        taskDetails["env"],
			WorkingDir: taskDetails["working_dir"],
		}
		
		// 诊断日志：打印完整的 Redis 任务详情
		logger.Info("【核心诊断】Redis 原始详情内容",
			zap.String("event_id", event.ID),
			zap.Any("all_details", taskDetails))
			
		logger.Info("从 Redis 解析任务详情结果",
			zap.String("job_id", job.ID),
			zap.String("raw_from_redis", taskDetails["strict_mode"]),
			zap.Bool("parsed_value", job.StrictMode))
	} else {
		job, err = d.getJob(event.JobID)
		if err != nil {
			// 记录获取任务配置失败
			errorLog := fmt.Sprintf("[%s] [Master] ❌ 获取任务配置失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
			storage.SaveLogChunk(context.Background(), event.ID, errorLog)

			event.Status = "failed"
			event.ErrorMessage = fmt.Sprintf("获取任务配置失败: %v", err)
			storage.DB.Save(event)
			return fmt.Errorf("获取任务配置失败: %w", err)
		}
	}

	// 设置 retryCount
	retryCount := 0
	if rcStr, ok := taskDetails["dispatch_retry_count"]; ok {
		if rc, err := strconv.Atoi(rcStr); err == nil {
			retryCount = rc
		}
	}

	// 如果是重试调用，在日志中追加提示
	if retryCount > 0 {
		retryLog := fmt.Sprintf("[%s] [Master] 🔄 开始第 %d 次重试调度...\n", time.Now().Format("2006-01-02 15:04:05"), retryCount)
		storage.SaveLogChunk(context.Background(), event.ID, retryLog)
	}

	// 记录任务分发详情
	logger.Info("任务分发详情",
		zap.String("event_id", event.ID),
		zap.String("command", job.Command),
		zap.Bool("strict_mode", job.StrictMode),
		zap.Int("retry_count", retryCount))

	node, err := d.selectNode(job)
	if err != nil {
		// 记录选择节点失败（仅写日志，不标记 failed — 临时性错误，可重试）
		errorLog := fmt.Sprintf("[%s] [Master] ❌ 选择节点失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		errorLog += fmt.Sprintf("[%s] [Master] 可能原因：\n", time.Now().Format("2006-01-02 15:04:05"))
		errorLog += fmt.Sprintf("[%s] [Master] - 没有在线的 Worker 节点\n", time.Now().Format("2006-01-02 15:04:05"))
		errorLog += fmt.Sprintf("[%s] [Master] - 所有 Worker 节点都已满载\n", time.Now().Format("2006-01-02 15:04:05"))
		errorLog += fmt.Sprintf("[%s] [Master] - 没有符合标签条件的节点\n", time.Now().Format("2006-01-02 15:04:05"))
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

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

func parseIntDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return defaultVal
	}
	return v
}

func parseBoolDefault(s interface{}, defaultVal bool) bool {
	if s == nil {
		return defaultVal
	}
	switch v := s.(type) {
	case bool:
		return s.(bool)
	case string:
		str := s.(string)
		if str == "" {
			return defaultVal
		}
		// 解析字符串 "1", "true", "True", "TRUE" 为 true
		if str == "1" || str == "true" || str == "True" || str == "TRUE" {
			return true
		}
		// 解析字符串 "0", "false", "False", "FALSE" 为 false
		if str == "0" || str == "false" || str == "False" || str == "FALSE" {
			return false
		}
		// 默认返回
		return defaultVal
	case int, int32, int64, float32, float64:
		// 数字类型：非零为 true
		return fmt.Sprintf("%v", v) != "0"
	default:
		return defaultVal
	}
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
	d.mu.Lock()
	defer d.mu.Unlock()

	for nodeID, conn := range d.conns {
		logger.Info("关闭 gRPC 客户端", zap.String("node_id", nodeID))
		conn.Close()
	}
	d.grpcClients = make(map[string]pb.CronicleServiceClient)
	d.conns = make(map[string]*grpc.ClientConn)
}

// RemoveNodeClient 关闭并移除指定节点的 gRPC 连接
func (d *Dispatcher) RemoveNodeClient(nodeID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if conn, ok := d.conns[nodeID]; ok {
		conn.Close()
		delete(d.conns, nodeID)
		delete(d.grpcClients, nodeID)
		logger.Info("已清理离线节点的 gRPC 连接",
			zap.String("node_id", nodeID))
	}
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

	// 只选择心跳新鲜的在线节点（排除心跳超时的僵尸节点）
	heartbeatThreshold := time.Now().Add(-90 * time.Second)

	query := storage.DB.Where("status = ?", "online").
		Where("last_heartbeat > ?", heartbeatThreshold).
		// 排除 master 节点（master 节点不执行任务）
		Where("(tags NOT LIKE '%master%' OR tags IS NULL OR tags = '')")

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
	// 设置节点信息和状态
	event.NodeID = node.ID
	event.NodeName = node.Hostname
	event.Status = "running"

	// 记录节点选择日志（在 DispatchEvent 已经记录了调度开始日志）
	nodeLog := fmt.Sprintf("[%s] [Master] 目标节点: %s (%s)\n", time.Now().Format("2006-01-02 15:04:05"), node.Hostname, node.ID)
	nodeLog += fmt.Sprintf("[%s] [Master] 节点地址: %s\n", time.Now().Format("2006-01-02 15:04:05"), node.IP)
	if node.GRPCAddress != "" {
		nodeLog += fmt.Sprintf("[%s] [Master] gRPC 地址: %s\n", time.Now().Format("2006-01-02 15:04:05"), node.GRPCAddress)
	}
	storage.SaveLogChunk(context.Background(), event.ID, nodeLog)

	if err := storage.DB.Save(event).Error; err != nil {
		// 记录数据库更新失败
		errorLog := fmt.Sprintf("[%s] [Master] ❌ 更新任务记录失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)
		return fmt.Errorf("更新任务记录失败: %w", err)
	}

	client, err := d.getGRPCClient(node)
	if err != nil {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("获取 gRPC 客户端失败: %v", err)
		storage.DB.Save(event)

		// 记录连接失败日志
		errorLog := fmt.Sprintf("[%s] [Master] ❌ 获取 gRPC 客户端失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		errorLog += fmt.Sprintf("[%s] [Master] 节点地址: %s, gRPC地址: %s\n", time.Now().Format("2006-01-02 15:04:05"), node.IP, node.GRPCAddress)
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

		return fmt.Errorf("获取 gRPC 客户端失败: %w", err)
	}

	env, err := parseEnv(job.Env)
	if err != nil {
		logger.Warn("解析环境变量失败", zap.Error(err))
		env = make(map[string]string)
	}

	// 【协议隧道】通过环境变量兜底传输严格模式
	if job.StrictMode {
		if env == nil {
			env = make(map[string]string)
		}
		env["CRONICLE_STRICT_MODE"] = "true"
	}

	scheduledTime := event.ScheduledTime.Unix()
	if scheduledTime < 0 {
		scheduledTime = time.Now().Unix()
	}

	taskReq := &pb.TaskRequest{
		JobId:         job.ID,
		EventId:       event.ID,
		Type:          parseTaskType(job.TaskType),
		Command:       job.Command,
		Env:           env,
		Timeout:       int32(job.Timeout),
		WorkingDir:    job.WorkingDir,
		ScheduledTime: scheduledTime,
		StrictMode:    job.StrictMode, // 传递严格模式配置
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 记录任务发送日志
	sendLog := fmt.Sprintf("[%s] [Master] 发送任务到 Worker...\n", time.Now().Format("2006-01-02 15:04:05"))
	if err := storage.SaveLogChunk(context.Background(), event.ID, sendLog); err != nil {
		logger.Warn("写入发送日志失败", zap.Error(err))
	}

	resp, err := client.SubmitTask(ctx, taskReq)
	if err != nil {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("发送任务失败: %v", err)
		storage.DB.Save(event)

		// 记录发送失败日志
		errorLog := fmt.Sprintf("[%s] [Master] ❌ 发送任务到 Worker 失败: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		errorLog += fmt.Sprintf("[%s] [Master] 任务已标记为失败状态\n", time.Now().Format("2006-01-02 15:04:05"))
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

		return fmt.Errorf("发送任务失败: %w", err)
	}

	if !resp.Accepted {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("Worker 拒绝任务: %s", resp.Message)
		storage.DB.Save(event)

		// 记录Worker拒绝日志
		errorLog := fmt.Sprintf("[%s] [Master] ❌ Worker 拒绝任务: %s\n", time.Now().Format("2006-01-02 15:04:05"), resp.Message)
		errorLog += fmt.Sprintf("[%s] [Master] 可能原因: Worker 已达到最大并发数或其他限制\n", time.Now().Format("2006-01-02 15:04:05"))
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

		return fmt.Errorf("Worker 拒绝任务: %s", resp.Message)
	}

	// 任务被接受，正式设置开始时间
	now := time.Now()
	event.StartTime = &now
	storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).Update("start_time", now)

	// 记录任务接受成功日志
	successLog := fmt.Sprintf("[%s] [Master] ✅ Worker 已接受任务并开始执行\n", now.Format("2006-01-02 15:04:05"))
	storage.SaveLogChunk(context.Background(), event.ID, successLog)

	// 通过WebSocket推送任务状态变化（pending → running）
	if d.wsServer != nil {
		d.wsServer.BroadcastTaskStatus(event.ID, event.JobID, "running", 0)
	}

	return nil
}

// getGRPCClient 获取或创建 gRPC 客户端
func (d *Dispatcher) getGRPCClient(node *models.Node) (pb.CronicleServiceClient, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

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
