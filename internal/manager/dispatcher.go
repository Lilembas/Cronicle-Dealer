package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/cronicle/cronicle-dealer/pkg/grpc/pb"
	"github.com/cronicle/cronicle-dealer/internal/models"
	"github.com/cronicle/cronicle-dealer/internal/storage"
	"github.com/cronicle/cronicle-dealer/pkg/logger"
	"github.com/cronicle/cronicle-dealer/pkg/utils"
)

const (
	defaultWorkerPort = 9090
	logTimeFormat     = "2006-01-02 15:04:05"
)

type Dispatcher struct {
	mu           sync.Mutex
	grpcClients  map[string]pb.CronicleServiceClient
	conns        map[string]*grpc.ClientConn
	wsServer     *WebSocketServer
	strategyCache sync.Map // 策略缓存，key=策略ID，value=*models.LoadBalanceStrategy
}

func NewDispatcher(wsServer *WebSocketServer) *Dispatcher {
	return &Dispatcher{
		grpcClients: make(map[string]pb.CronicleServiceClient),
		conns:       make(map[string]*grpc.ClientConn),
		wsServer:    wsServer,
	}
}

func (d *Dispatcher) DispatchEvent(event *models.Event, taskDetails map[string]string) error {
	logger.Info("开始分发任务",
		zap.String("event_id", event.ID),
		zap.String("job_id", event.JobID))

	var existingEvent models.Event
	err := storage.DB.Where("id = ?", event.ID).First(&existingEvent).Error

	if err == nil {
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
				if existingEvent.StartTime != nil && !existingEvent.StartTime.IsZero() {
				event.StartTime = existingEvent.StartTime
				event.LogPath = existingEvent.LogPath
			}
	}

	if event.LogPath == "" {
		event.LogPath = fmt.Sprintf("/var/log/cronicle/events/%s.log", event.ID)

			if err := storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).
			Update("log_path", event.LogPath).Error; err != nil {
			logger.Warn("更新事件 log_path 失败",
				zap.String("event_id", event.ID),
				zap.Error(err))
		}

		now := time.Now()
		dispatchLog := fmt.Sprintf("[%s] [Manager] 任务开始调度\n", now.Format(logTimeFormat))
		dispatchLog += fmt.Sprintf("[%s] [Manager] 任务ID: %s, 作业ID: %s\n", now.Format(logTimeFormat), event.ID, event.JobID)

		if logErr := storage.SaveLogChunk(context.Background(), event.ID, dispatchLog); logErr != nil {
			logger.Warn("写入调度日志失败", zap.Error(logErr))
		}
	}

	var job *models.Job

	if len(taskDetails) > 0 {
		job = &models.Job{
			ID:          taskDetails["job_id"],
			Name:        taskDetails["job_name"],
			Command:     taskDetails["command"],
			TaskType:    taskDetails["task_type"],
			Timeout:     parseIntDefault(taskDetails["timeout"], 30),
			TargetType:  taskDetails["target_type"],
			TargetValue: taskDetails["target_value"],
			StrictMode:  parseBoolDefault(taskDetails["strict_mode"], false),
			Env:         taskDetails["env"],
			WorkingDir:  taskDetails["working_dir"],
			StrategyID:  taskDetails["strategy_id"],
		}
		
	} else {
		job, err = d.getJob(event.JobID)
		if err != nil {
					errorLog := fmt.Sprintf("[%s] [Manager] ❌ 获取任务配置失败: %v\n", time.Now().Format(logTimeFormat), err)
			storage.SaveLogChunk(context.Background(), event.ID, errorLog)

			event.Status = "failed"
			event.ErrorMessage = fmt.Sprintf("获取任务配置失败: %v", err)
			storage.DB.Save(event)
			return fmt.Errorf("获取任务配置失败: %w", err)
		}
	}

	retryCount := 0
	if rcStr, ok := taskDetails["dispatch_retry_count"]; ok {
		if rc, err := strconv.Atoi(rcStr); err == nil {
			retryCount = rc
		}
	}

	if retryCount > 0 {
		retryLog := fmt.Sprintf("[%s] [Manager] 🔄 开始第 %d 次重试调度...\n", time.Now().Format(logTimeFormat), retryCount)
		storage.SaveLogChunk(context.Background(), event.ID, retryLog)
	}

	logger.Info("任务分发详情",
		zap.String("event_id", event.ID),
		zap.String("job_name", job.Name),
		zap.String("task_type", job.TaskType),
		zap.Bool("strict_mode", job.StrictMode),
		zap.Int("retry_count", retryCount))

	candidates, err := d.selectCandidates(job)
	if err != nil {
			errorLog := fmt.Sprintf("[%s] [Manager] ❌ 获取候选节点失败: %v\n", time.Now().Format(logTimeFormat), err)
		errorLog += fmt.Sprintf("[%s] [Manager] 可能原因：\n", time.Now().Format(logTimeFormat))
		errorLog += fmt.Sprintf("[%s] [Manager] - 没有在线的 Worker 节点\n", time.Now().Format(logTimeFormat))
		errorLog += fmt.Sprintf("[%s] [Manager] - 没有符合查询条件的节点\n", time.Now().Format(logTimeFormat))
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

		return fmt.Errorf("选择节点失败: %w", err)
	}

	if job.StrategyID == "broadcast" {
		return d.dispatchBroadcast(event, candidates, job)
	}

	node, err := d.pickOneNode(candidates, job.StrategyID)
	if err != nil {
		errorLog := fmt.Sprintf("[%s] [Manager] ❌ 负载均衡筛选失败: %v\n", time.Now().Format(logTimeFormat), err)
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)
		return fmt.Errorf("筛选节点失败: %w", err)
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
		return v
	case string:
		if v == "" {
			return defaultVal
		}
		lower := strings.ToLower(v)
		switch lower {
		case "1", "true":
			return true
		case "0", "false":
			return false
		default:
			return defaultVal
		}
	default:
		return defaultVal
	}
}

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
		logger.Warn("获取 gRPC 客户端失败，跳过 Worker 中止，直接更新 DB 状态",
			zap.String("event_id", event.ID),
			zap.String("node_id", event.NodeID),
			zap.Error(err))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.AbortTask(ctx, &pb.AbortTaskRequest{
		JobId:   event.JobID,
		EventId: event.ID,
		Reason:  reason,
	})
	if err != nil {
		logger.Warn("调用 Worker AbortTask 失败，跳过 Worker 中止，直接更新 DB 状态",
			zap.String("event_id", event.ID),
			zap.String("node_id", event.NodeID),
			zap.Error(err))
		return nil
	}

	if !resp.Success {
		logger.Warn("Worker 拒绝中止请求，直接更新 DB 状态",
			zap.String("event_id", event.ID),
			zap.String("node_id", event.NodeID),
			zap.String("message", resp.Message))
		return nil
	}

	logger.Info("任务中止请求已下发",
		zap.String("event_id", event.ID),
		zap.String("node_id", event.NodeID))

	return nil
}

func (d *Dispatcher) loadStrategy(strategyID string) *models.LoadBalanceStrategy {
	if strategyID == "" {
		logger.Debug("任务未配置负载均衡策略，使用默认最小负载策略")
		return nil
	}
	if cached, ok := d.strategyCache.Load(strategyID); ok {
		return cached.(*models.LoadBalanceStrategy)
	}
	var strategy models.LoadBalanceStrategy
	if err := storage.DB.Where("id = ?", strategyID).First(&strategy).Error; err != nil {
		logger.Warn("负载均衡策略未找到，回退到默认策略", zap.String("strategy_id", strategyID))
		return nil
	}
	logger.Debug("加载负载均衡策略",
		zap.String("strategy_id", strategy.ID),
		zap.String("strategy_name", strategy.Name),
		zap.String("direction", strategy.Direction),
		zap.Int("metrics_count", len(strings.Split(strategy.Metrics, ","))))
	d.strategyCache.Store(strategyID, &strategy)
	return &strategy
}

func (d *Dispatcher) ClearStrategyCache(strategyID string) {
	d.strategyCache.Delete(strategyID)
	logger.Info("已清除策略缓存", zap.String("strategy_id", strategyID))
}

func (d *Dispatcher) selectByStrategy(nodes []models.Node, strategy *models.LoadBalanceStrategy) (*models.Node, error) {
	var metrics []models.LBMetric
	if err := json.Unmarshal([]byte(strategy.Metrics), &metrics); err != nil {
		logger.Error("策略指标解析失败，回退到默认策略", zap.Error(err))
		return d.selectLeastBusyNode(nodes)
	}

	if len(metrics) == 0 {
		return d.selectLeastBusyNode(nodes)
	}

	logger.Debug("开始策略评估",
		zap.String("strategy", strategy.Name),
		zap.String("direction", strategy.Direction),
		zap.Int("candidate_nodes", len(nodes)))

	type scoredNode struct {
		node  *models.Node
		score float64
	}

	var candidates []scoredNode
	for i := range nodes {
		node := &nodes[i]
		if !node.CanAcceptJob() {
			logger.Debug("跳过不可接受任务的节点",
				zap.String("node_id", node.ID),
				zap.String("hostname", node.Hostname),
				zap.Int("running_jobs", node.RunningJobs),
				zap.Int("max_concurrent", node.MaxConcurrent))
			continue
		}
		params := BuildParamsFromNode(node)
		totalScore := 0.0
		valid := true
		for _, m := range metrics {
			val, err := EvaluateFormula(m.Formula, params)
			if err != nil {
				logger.Warn("公式求值失败",
					zap.String("metric", m.Name),
					zap.String("formula", m.Formula),
					zap.String("node_id", node.ID),
					zap.Error(err))
				valid = false
				break
			}
			weighted := val * m.Weight
			totalScore += weighted
			logger.Debug("指标求值完成",
				zap.String("node_id", node.ID),
				zap.String("metric", m.Name),
				zap.String("formula", m.Formula),
				zap.Float64("raw_value", val),
				zap.Float64("weight", m.Weight),
				zap.Float64("weighted_score", weighted))
		}
		if valid {
			candidates = append(candidates, scoredNode{node: node, score: totalScore})
			logger.Debug("节点评估完成",
				zap.String("node_id", node.ID),
				zap.String("hostname", node.Hostname),
				zap.Float64("total_score", totalScore))
		}
	}

	if len(candidates) == 0 {
		logger.Warn("所有节点策略评估失败，回退到默认策略")
		return d.selectLeastBusyNode(nodes)
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if strategy.Direction == "desc" {
			if c.score > best.score {
				best = c
			}
		} else {
			if c.score < best.score {
				best = c
			}
		}
	}

	logger.Info("策略选节点",
		zap.String("strategy", strategy.Name),
		zap.String("direction", strategy.Direction),
		zap.String("node_id", best.node.ID),
		zap.String("hostname", best.node.Hostname),
		zap.Float64("score", best.score))

	return best.node, nil
}

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

func (d *Dispatcher) getJob(jobID string) (*models.Job, error) {
	var job models.Job
	if err := storage.DB.Where("id = ?", jobID).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (d *Dispatcher) selectCandidates(job *models.Job) ([]models.Node, error) {
	var nodes []models.Node

	heartbeatThreshold := time.Now().Add(-90 * time.Second)

	query := storage.DB.Where("status = ?", "online").
		Where("last_heartbeat > ?", heartbeatThreshold).
			Where("(tags NOT LIKE '%manager%' OR tags IS NULL OR tags = '')")

	switch job.TargetType {
	case "node_id":
		query = query.Where("id = ?", job.TargetValue)
	case "tags":
			var targetTags []string
		if err := json.Unmarshal([]byte(job.TargetValue), &targetTags); err != nil {
					targetTags = []string{job.TargetValue}
		}

		if len(targetTags) > 0 {
			tagSubQuery := storage.DB
			for i, tag := range targetTags {
							cond := "%\"" + tag + "\"%"
				if i == 0 {
					tagSubQuery = tagSubQuery.Where("tags LIKE ?", cond)
				} else {
					tagSubQuery = tagSubQuery.Or("tags LIKE ?", cond)
				}
			}
			query = query.Where(tagSubQuery)
		}
	case "any":
		}

	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("没有符合条件的可用节点")
	}

	return nodes, nil
}

func (d *Dispatcher) pickOneNode(nodes []models.Node, strategyID string) (*models.Node, error) {
	strategy := d.loadStrategy(strategyID)
	if strategy != nil {
		return d.selectByStrategy(nodes, strategy)
	}

	return d.selectLeastBusyNode(nodes)
}

func (d *Dispatcher) selectLeastBusyNode(nodes []models.Node) (*models.Node, error) {
	var selectedNode *models.Node
	minJobs := math.MaxInt

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

func (d *Dispatcher) dispatchBroadcast(event *models.Event, candidates []models.Node, job *models.Job) error {
	logger.Info("执行广播分发",
		zap.String("event_id", event.ID),
		zap.Int("candidate_count", len(candidates)))

	for i := range candidates {
		node := &candidates[i]
		targetEvent := event

			if i > 0 {
			newEventID := utils.GenerateID("event")
			targetEvent = &models.Event{
				ID:            newEventID,
				JobID:         event.JobID,
				JobName:       event.JobName,
				Status:        "pending",
				ScheduledTime: event.ScheduledTime,
				IsRetry:       event.IsRetry,
				RetryCount:    event.RetryCount,
				ParentEventID: event.ID, // 关联到主事件
				CreatedAt:     time.Now(),
			}

			if err := storage.DB.Create(targetEvent).Error; err != nil {
				logger.Error("创建广播子事件失败",
					zap.String("parent_event_id", event.ID),
					zap.String("node_id", node.ID),
					zap.Error(err))
				continue
			}
		}

			if err := d.updateEventAndDispatch(targetEvent, node, job); err != nil {
			logger.Error("广播分发到节点失败",
				zap.String("event_id", targetEvent.ID),
				zap.String("node_id", node.ID),
				zap.Error(err))
		}
	}

	return nil
}

func (d *Dispatcher) updateEventAndDispatch(event *models.Event, node *models.Node, job *models.Job) error {
	event.NodeID = node.ID
	event.NodeName = node.Hostname
	event.Status = "running"

	nodeLog := fmt.Sprintf("[%s] [Manager] 目标节点: %s (%s)\n", time.Now().Format(logTimeFormat), node.Hostname, node.ID)
	nodeLog += fmt.Sprintf("[%s] [Manager] 节点地址: %s\n", time.Now().Format(logTimeFormat), node.IP)
	if node.GRPCAddress != "" {
		nodeLog += fmt.Sprintf("[%s] [Manager] gRPC 地址: %s\n", time.Now().Format(logTimeFormat), node.GRPCAddress)
	}
	storage.SaveLogChunk(context.Background(), event.ID, nodeLog)

	if err := storage.DB.Save(event).Error; err != nil {
			errorLog := fmt.Sprintf("[%s] [Manager] ❌ 更新任务记录失败: %v\n", time.Now().Format(logTimeFormat), err)
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)
		return fmt.Errorf("更新任务记录失败: %w", err)
	}

	client, err := d.getGRPCClient(node)
	if err != nil {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("获取 gRPC 客户端失败: %v", err)
		storage.DB.Save(event)

			errorLog := fmt.Sprintf("[%s] [Manager] ❌ 获取 gRPC 客户端失败: %v\n", time.Now().Format(logTimeFormat), err)
		errorLog += fmt.Sprintf("[%s] [Manager] 节点地址: %s, gRPC地址: %s\n", time.Now().Format(logTimeFormat), node.IP, node.GRPCAddress)
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

		return fmt.Errorf("获取 gRPC 客户端失败: %w", err)
	}

	env, err := parseEnv(job.Env)
	if err != nil {
		logger.Warn("解析环境变量失败", zap.Error(err))
		env = make(map[string]string)
	}

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

	sendLog := fmt.Sprintf("[%s] [Manager] 发送任务到 Worker...\n", time.Now().Format(logTimeFormat))
	if err := storage.SaveLogChunk(context.Background(), event.ID, sendLog); err != nil {
		logger.Warn("写入发送日志失败", zap.Error(err))
	}

	resp, err := client.SubmitTask(ctx, taskReq)
	if err != nil {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("发送任务失败: %v", err)
		storage.DB.Save(event)

			errorLog := fmt.Sprintf("[%s] [Manager] ❌ 发送任务到 Worker 失败: %v\n", time.Now().Format(logTimeFormat), err)
		errorLog += fmt.Sprintf("[%s] [Manager] 任务已标记为失败状态\n", time.Now().Format(logTimeFormat))
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

		return fmt.Errorf("发送任务失败: %w", err)
	}

	if !resp.Accepted {
		event.Status = "failed"
		event.ErrorMessage = fmt.Sprintf("Worker 拒绝任务: %s", resp.Message)
		storage.DB.Save(event)

			errorLog := fmt.Sprintf("[%s] [Manager] ❌ Worker 拒绝任务: %s\n", time.Now().Format(logTimeFormat), resp.Message)
		errorLog += fmt.Sprintf("[%s] [Manager] 可能原因: Worker 已达到最大并发数或其他限制\n", time.Now().Format(logTimeFormat))
		storage.SaveLogChunk(context.Background(), event.ID, errorLog)

		return fmt.Errorf("Worker 拒绝任务: %s", resp.Message)
	}

	now := time.Now()
	event.StartTime = &now
	storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).Update("start_time", now)

	successLog := fmt.Sprintf("[%s] [Manager] ✅ Worker 已接受任务并开始执行\n", now.Format(logTimeFormat))
	storage.SaveLogChunk(context.Background(), event.ID, successLog)

	if d.wsServer != nil {
		d.wsServer.BroadcastTaskStatus(event.ID, event.JobID, "running", event.NodeID, event.NodeName, 0)
	}

	return nil
}

func (d *Dispatcher) getGRPCClient(node *models.Node) (pb.CronicleServiceClient, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if client, ok := d.grpcClients[node.ID]; ok {
		return client, nil
	}

	var addr string
	if node.GRPCAddress != "" && !strings.HasSuffix(node.GRPCAddress, ":0") {
			addr = node.GRPCAddress
	} else {
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
