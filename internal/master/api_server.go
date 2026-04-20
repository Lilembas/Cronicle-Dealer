package master

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/utils"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// APIServer REST API 服务器
type APIServer struct {
	cfg        *config.Config
	router     *gin.Engine
	scheduler  *Scheduler
	dispatcher *Dispatcher
	wsServer   *WebSocketServer
}

// SetWebSocketServer 设置WebSocket服务器
func (s *APIServer) SetWebSocketServer(wsServer *WebSocketServer) {
	s.wsServer = wsServer
}

// NewAPIServer 创建 API 服务器
func NewAPIServer(cfg *config.Config, scheduler *Scheduler, dispatcher *Dispatcher) *APIServer {
	// 设置 Gin 模式
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	
	router := gin.Default()
	
	server := &APIServer{
		cfg:        cfg,
		router:     router,
		scheduler:  scheduler,
		dispatcher: dispatcher,
	}
	
	server.setupRoutes()
	
	return server
}

// setupRoutes 设置路由
func (s *APIServer) setupRoutes() {
	// API 根路径
	api := s.router.Group("/api/v1")
	
	// 健康检查
	s.router.GET("/health", s.healthCheck)

	// 认证接口（公开）
	auth := api.Group("/auth")
	{
		auth.POST("/login", s.login)
		auth.POST("/refresh", s.refreshToken)
	}

	// 受保护接口
	protected := api.Group("")
	protected.Use(s.authMiddleware())
	
	// 任务管理
	jobs := protected.Group("/jobs")
	{
		jobs.GET("", s.listJobs)           // 获取任务列表
		jobs.POST("", s.createJob)         // 创建任务
		jobs.GET("/:id", s.getJob)         // 获取任务详情
		jobs.PUT("/:id", s.updateJob)      // 更新任务
		jobs.DELETE("/:id", s.deleteJob)   // 删除任务
		jobs.POST("/:id/trigger", s.triggerJob) // 手动触发任务
	}
	
	// 执行记录
	events := protected.Group("/events")
	{
		events.GET("", s.listEvents)       // 获取执行记录列表
		events.GET("/:id", s.getEvent)     // 获取执行记录详情
		events.POST("/:id/abort", s.abortEvent) // 中止执行
		events.GET("/:id/download", s.downloadLog) // 下载日志
	}
	
	// 节点管理
	nodes := protected.Group("/nodes")
	{
		nodes.GET("", s.listNodes)         // 获取节点列表
		nodes.GET("/tags", s.listNodeTags) // 获取所有节点标签
		nodes.GET("/:id", s.getNode)       // 获取节点详情
		nodes.PUT("/:id", s.updateNode)    // 更新节点
		nodes.DELETE("/:id", s.deleteNode) // 删除节点
	}
	
	// 统计信息
	protected.GET("/stats", s.getStats)

	// Shell 命令执行（ad-hoc）
	shell := protected.Group("/shell")
	{
		shell.POST("/execute", s.executeShell)       // 执行 Shell 命令
		shell.GET("/logs/:event_id", s.getShellLogs) // 获取实时日志
	}

	// 负载均衡策略
	strategies := protected.Group("/strategies")
	{
		strategies.GET("", s.listStrategies)
		strategies.GET("/parameters", s.getFormulaParameters)
		strategies.POST("", s.createStrategy)
		strategies.POST("/validate", s.validateFormula)
		strategies.GET("/:id", s.getStrategy)
		strategies.PUT("/:id", s.updateStrategy)
		strategies.DELETE("/:id", s.deleteStrategy)
	}
}

// Start 启动 API 服务器
func (s *APIServer) Start() error {
	addr := s.cfg.Server.Host + ":" + strconv.Itoa(s.cfg.Server.HTTPPort)
	
	logger.Info("API 服务器启动", zap.String("address", addr))
	
	go func() {
		if err := s.router.Run(addr); err != nil {
			logger.Fatal("API 服务器启动失败", zap.Error(err))
		}
	}()
	
	return nil
}

// ========== 处理函数 ==========

// healthCheck 健康检查
func (s *APIServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

// listJobs 获取任务列表
func (s *APIServer) listJobs(c *gin.Context) {
	var jobs []models.Job

	query := storage.DB

	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	offset := (page - 1) * pageSize

	// 过滤条件
	if category := c.Query("category"); category != "" {
		query = query.Where("category = ?", category)
	}
	if enabled := c.Query("enabled"); enabled != "" {
		query = query.Where("enabled = ?", enabled == "true")
	}

	// 查询总数
	var total int64
	query.Model(&models.Job{}).Count(&total)

	// 查询数据
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 批量查询每个 Job 最近一次执行记录的 status
	jobIDs := make([]string, 0, len(jobs))
	for _, j := range jobs {
		jobIDs = append(jobIDs, j.ID)
	}

	type lastEventResult struct {
		JobID  string
		Status string
	}
	var lastEvents []lastEventResult
	if len(jobIDs) > 0 {
		// 使用子查询找到每个 job_id 最近一次执行的 status
		storage.DB.Raw(`
			SELECT e.job_id, e.status
			FROM events e
			INNER JOIN (
				SELECT job_id, MAX(start_time) as max_start_time
				FROM events
				WHERE job_id IN ?
				GROUP BY job_id
			) t ON e.job_id = t.job_id AND e.start_time = t.max_start_time
			WHERE e.job_id IN ?`, jobIDs, jobIDs).
			Scan(&lastEvents)
	}

	lastStatusMap := make(map[string]string)
	for _, le := range lastEvents {
		lastStatusMap[le.JobID] = le.Status
	}

	// 组装返回数据，附加 last_status
	type jobWithStatus struct {
		models.Job
		LastStatus string `json:"last_status"`
	}
	result := make([]jobWithStatus, 0, len(jobs))
	for _, j := range jobs {
		result = append(result, jobWithStatus{
			Job:        j,
			LastStatus: lastStatusMap[j.ID],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"data":  result,
	})
}

// createJob 创建任务
func (s *APIServer) createJob(c *gin.Context) {
	var job models.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// 生成 ID
	if job.ID == "" {
		job.ID = utils.GenerateID("job")
	}
	
	// 保存到数据库
	logger.Info("[API] 创建任务详情",
		zap.String("name", job.Name),
		zap.Bool("strict_mode", job.StrictMode))

	if err := storage.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 如果任务已启用，添加到调度器
	if utils.BoolValue(job.Enabled) {
		if err := s.scheduler.AddJob(&job); err != nil {
			logger.Error("添加任务到调度器失败", zap.Error(err))
		}
	}
	
	c.JSON(http.StatusCreated, job)
}

// getJob 获取任务详情
func (s *APIServer) getJob(c *gin.Context) {
	var job models.Job
	if err := storage.DB.Where("id = ?", c.Param("id")).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	
	c.JSON(http.StatusOK, job)
}

// updateJob 更新任务
func (s *APIServer) updateJob(c *gin.Context) {
	jobID := c.Param("id")

	// 先查出原有记录
	var existing models.Job
	if err := storage.DB.First(&existing, "id = ?", jobID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 解析请求体，只覆盖传入的字段
	var updates models.Job
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("[API] 更新任务详情",
		zap.String("id", jobID),
		zap.String("name", updates.Name),
		zap.Bool("strict_mode", updates.StrictMode))

	// 使用 Updates 只更新非零值字段
	if err := storage.DB.Model(&existing).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 重新查询更新后的记录（确保调度器收到最新数据）
	if err := storage.DB.First(&existing, "id = ?", jobID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新调度器（需要完整记录）
	if err := s.scheduler.UpdateJob(&existing); err != nil {
		logger.Error("更新调度器任务失败", zap.Error(err))
	}

	c.JSON(http.StatusOK, existing)
}

// deleteJob 删除任务
func (s *APIServer) deleteJob(c *gin.Context) {
	jobID := c.Param("id")
	
	if err := storage.DB.Where("id = ?", jobID).Delete(&models.Job{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 从调度器移除
	s.scheduler.RemoveJob(jobID)
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// triggerJob 手动触发任务
func (s *APIServer) triggerJob(c *gin.Context) {
	jobID := c.Param("id")

	var job models.Job
	if err := storage.DB.Where("id = ?", jobID).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	now := time.Now()

	// 生成 Event 记录
	eventID := utils.GenerateID("event")
	event := &models.Event{
		ID:            eventID,
		JobID:         job.ID,
		JobName:       job.Name,
		Status:        eventStatusPending,
		ScheduledTime: now,
		CreatedAt:     now,
	}

	if err := storage.DB.Create(event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建执行记录失败: " + err.Error()})
		return
	}

	// 写入 Redis 任务详情
	ctx := context.Background()
	taskKey := fmt.Sprintf("%s:%s", job.ID, eventID)
	taskData := map[string]interface{}{
		"job_id":          job.ID,
		"event_id":        eventID,
		"job_name":        job.Name,
		"command":         job.Command,
		"task_type":       job.TaskType,
		"timeout":         job.Timeout,
		"working_dir":     job.WorkingDir,
		"env":             job.Env,
		"strict_mode":     fmt.Sprintf("%v", job.StrictMode),
		"scheduled_time":  now.Unix(),
		"manual_trigger":  "true",
	}

	logger.Info("手动触发任务 Redis 详情",
		zap.String("job_id", job.ID),
		zap.String("strict_mode", fmt.Sprintf("%v", job.StrictMode)))

	logger.Info("手动触发任务详情",
		zap.String("job_id", job.ID),
		zap.Bool("strict_mode", job.StrictMode))

	if err := storage.RedisClient.HSet(ctx, "tasks:details:"+taskKey, taskData).Err(); err != nil {
		// 回滚 DB 记录
		storage.DB.Delete(event)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入任务详情失败: " + err.Error()})
		return
	}

	// 推入就绪队列
	if err := storage.AddTaskToQueue(ctx, taskKey); err != nil {
		storage.RedisClient.Del(ctx, "tasks:details:"+taskKey)
		storage.DB.Delete(event)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "任务入队失败: " + err.Error()})
		return
	}

	logger.Info("手动触发任务",
		zap.String("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.String("event_id", eventID))

	// 通过WebSocket推送任务状态变化
	if s.wsServer != nil {
		s.wsServer.BroadcastTaskStatus(eventID, job.ID, eventStatusPending, "", "", 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"event_id":  eventID,
		"job_id":    job.ID,
		"job_name":  job.Name,
		"status":    "queued",
		"queued_at": now.Unix(),
		"message":   "任务已加入队列，等待执行",
	})
}

// listEvents 获取执行记录列表
func (s *APIServer) listEvents(c *gin.Context) {
	var events []models.Event
	
	query := storage.DB
	
	// 过滤条件
	if jobID := c.Query("job_id"); jobID != "" {
		query = query.Where("job_id = ?", jobID)
	}
	if jobName := c.Query("job_name"); jobName != "" {
		query = query.Where("job_name LIKE ?", "%"+jobName+"%")
	}
	if jobCategory := c.Query("job_category"); jobCategory != "" {
		var jobIDs []string
		storage.DB.Model(&models.Job{}).Where("category = ?", jobCategory).Pluck("id", &jobIDs)
		if len(jobIDs) == 0 {
			// 没有匹配的 Job，直接返回空结果
			c.JSON(http.StatusOK, gin.H{"total": 0, "page": 1, "data": []models.Event{}})
			return
		}
		query = query.Where("job_id IN ?", jobIDs)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	
	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	offset := (page - 1) * pageSize
	
	var total int64
	query.Model(&models.Event{}).Count(&total)
	
	// 排序：按开始时间从新往旧，最后按ID
	if err := query.Offset(offset).Limit(pageSize).Order("start_time DESC, id DESC").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 填充 Job 分组信息
	if len(events) > 0 {
		var jobIDs []string
		jobCategoryMap := make(map[string]string)
		for _, e := range events {
			jobIDs = append(jobIDs, e.JobID)
		}
		var jobs []models.Job
		storage.DB.Select("id, category").Where("id IN ?", jobIDs).Find(&jobs)
		for _, j := range jobs {
			jobCategoryMap[j.ID] = j.Category
		}
		for i := range events {
			events[i].JobCategory = jobCategoryMap[events[i].JobID]
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"data":  events,
	})
}

// getEvent 获取执行记录详情
func (s *APIServer) getEvent(c *gin.Context) {
	var event models.Event
	if err := storage.DB.Where("id = ?", c.Param("id")).First(&event).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	
	c.JSON(http.StatusOK, event)
}

// abortEvent 中止执行
func (s *APIServer) abortEvent(c *gin.Context) {
	eventID := c.Param("id")

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req) // 请求体可选

	var event models.Event
	if err := storage.DB.Where("id = ?", eventID).First(&event).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "执行记录不存在"})
		return
	}

	if event.Status == eventStatusSuccess || event.Status == eventStatusFailed || event.Status == eventStatusAborted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已结束，无法中止"})
		return
	}

	abortReason := req.Reason
	if abortReason == "" {
		abortReason = "aborted by user"
	}

	// running 状态：下发到 Worker 进行中止
	if event.Status == eventStatusRunning {
		if err := s.dispatcher.AbortTask(&event, abortReason); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "中止任务失败: " + err.Error()})
			return
		}
	}

	// pending/queued 状态：从队列中移除，避免后续被执行
	if event.Status == eventStatusPending || event.Status == "queued" {
		ctx := context.Background()
		taskKey := fmt.Sprintf("%s:%s", event.JobID, event.ID)
		_ = storage.RemoveTaskFromQueue(ctx, taskKey)
		_ = storage.DeleteTaskDetails(ctx, taskKey)
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":        eventStatusAborted,
		"end_time":      &now,
		"error_message": abortReason,
	}
	if event.StartTime != nil {
		updates["duration"] = now.Unix() - event.StartTime.Unix()
	}

	if err := storage.DB.Model(&event).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新任务状态失败: " + err.Error()})
		return
	}

	// 通过WebSocket推送任务状态变化
	if s.wsServer != nil {
		s.wsServer.BroadcastTaskStatus(eventID, event.JobID, eventStatusAborted, event.NodeID, event.NodeName, -1)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "任务已中止",
		"event_id": eventID,
		"status":   eventStatusAborted,
	})
}

// listNodes 获取节点列表
func (s *APIServer) listNodes(c *gin.Context) {
	var nodes []models.Node
	
	query := storage.DB
	
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	
	if err := query.Order("last_heartbeat DESC").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, nodes)
}

// listNodeTags 获取所有节点标签（排除 master 节点）
func (s *APIServer) listNodeTags(c *gin.Context) {
	var nodes []models.Node
	if err := storage.DB.Select("tags").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 解析并合并所有标签，排除 master 节点
	tagSet := make(map[string]bool)
	for _, node := range nodes {
		if node.Tags == "" {
			continue
		}
		// 跳过 master 节点
		if node.Tags == "master" || strings.Contains(node.Tags, "\"master\"") || strings.Contains(node.Tags, "master") {
			continue
		}
		var tags []string
		if err := json.Unmarshal([]byte(node.Tags), &tags); err != nil {
			// 如果解析失败，作为单个标签处理
			tagSet[node.Tags] = true
		} else {
			for _, tag := range tags {
				if tag != "" && tag != "master" {
					tagSet[tag] = true
				}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	c.JSON(http.StatusOK, tags)
}

// getNode 获取节点详情
func (s *APIServer) getNode(c *gin.Context) {
	var node models.Node
	if err := storage.DB.Where("id = ?", c.Param("id")).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}
	
	c.JSON(http.StatusOK, node)
}

// updateNode 更新节点
func (s *APIServer) updateNode(c *gin.Context) {
	var node models.Node
	if err := storage.DB.Where("id = ?", c.Param("id")).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	var req struct {
		Tags string `json:"tags"` // JSON 存储标签数组
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Tags != "" {
		updates["tags"] = req.Tags
	}

	if len(updates) > 0 {
		if err := storage.DB.Model(&node).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 重新查询返回最新数据
	storage.DB.Where("id = ?", c.Param("id")).First(&node)
	c.JSON(http.StatusOK, node)
}

// deleteNode 删除节点
func (s *APIServer) deleteNode(c *gin.Context) {
	if err := storage.DB.Where("id = ?", c.Param("id")).Delete(&models.Node{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// getStats 获取统计信息
func (s *APIServer) getStats(c *gin.Context) {
	var stats struct {
		TotalJobs      int64 `json:"total_jobs"`
		EnabledJobs    int64 `json:"enabled_jobs"`
		TotalEvents    int64 `json:"total_events"`
		RunningEvents  int64 `json:"running_events"`
		SuccessEvents  int64 `json:"success_events"`
		FailedEvents   int64 `json:"failed_events"`
		OnlineNodes    int64 `json:"online_nodes"`
		OfflineNodes   int64 `json:"offline_nodes"`
		ServerTime     int64 `json:"server_time"`
	}
	
	storage.DB.Model(&models.Job{}).Count(&stats.TotalJobs)
	storage.DB.Model(&models.Job{}).Where("enabled = ?", true).Count(&stats.EnabledJobs)
	storage.DB.Model(&models.Event{}).Count(&stats.TotalEvents)
	storage.DB.Model(&models.Event{}).Where("status = ?", eventStatusRunning).Count(&stats.RunningEvents)
	storage.DB.Model(&models.Event{}).Where("status = ?", eventStatusSuccess).Count(&stats.SuccessEvents)
	storage.DB.Model(&models.Event{}).Where("status = ?", eventStatusFailed).Count(&stats.FailedEvents)
	storage.DB.Model(&models.Node{}).Where("status = ?", nodeStatusOnline).Count(&stats.OnlineNodes)
	storage.DB.Model(&models.Node{}).Where("status = ?", nodeStatusOffline).Count(&stats.OfflineNodes)
	stats.ServerTime = time.Now().UnixNano() / 1e6
	
	c.JSON(http.StatusOK, stats)
}

// ========== Shell 命令执行 ==========

// executeShell 执行 Shell 命令（ad-hoc）
func (s *APIServer) executeShell(c *gin.Context) {
	var req struct {
		Command    string `json:"command" binding:"required"`
		NodeID     string `json:"node_id"`    // 可选：指定执行节点
		Timeout    int    `json:"timeout"`    // 可选：超时时间（秒），默认 30
		StrictMode bool   `json:"strict_mode"` // 可选：严格模式
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 设置默认超时
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	// 检查 Worker 是否在线（检查最近 60 秒有心跳的节点）
	heartbeatTimeout := time.Now().Add(-60 * time.Second)
	var nodes []models.Node

	query := storage.DB.Where("status = ? AND last_heartbeat > ?", nodeStatusOnline, heartbeatTimeout)

	// 如果指定了节点 ID，只查询该节点
	if req.NodeID != "" {
		query = query.Where("id = ?", req.NodeID)
	}

	if err := query.Find(&nodes).Error; err != nil || len(nodes) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "没有可用的 Worker 节点",
		})
		return
	}

	// 创建临时任务和事件
	jobID := utils.GenerateID("job")
	eventID := utils.GenerateID("event")

	ctx := context.Background()
	taskKey := fmt.Sprintf("%s:%s", jobID, eventID)

	// 确定目标服务器配置
	targetType := "any"
	targetValue := ""
	if req.NodeID != "" {
		targetType = "node_id"
		targetValue = req.NodeID
	}

	// 创建事件
	now := time.Now()
	event := &models.Event{
		ID:        eventID,
		JobID:     jobID,
		JobName:   "Ad-hoc Shell 命令",
		Status:    eventStatusPending,
		NodeID:    nodes[0].ID, // 分配给第一个可用节点
		CreatedAt: now,
	}

	if err := storage.DB.Create(event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建事件失败: " + err.Error(),
		})
		return
	}

	// 保存任务详情到 Redis（包含完整的任务配置，Dispatcher 会直接从这里获取而不是查数据库）
	taskData := map[string]interface{}{
		"job_id":         jobID,
		"event_id":       eventID,
		"job_name":       "Ad-hoc Shell 命令",
		"command":        req.Command,
		"task_type":      "shell",
		"timeout":        req.Timeout,
		"target_type":    targetType,
		"target_value":   targetValue,
		"strict_mode":    fmt.Sprintf("%v", req.StrictMode),
		"scheduled_time": now.Unix(),
	}

	if err := storage.RedisClient.HSet(ctx, "tasks:details:"+taskKey, taskData).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存任务详情失败: " + err.Error(),
		})
		return
	}

	// 添加到任务队列
	if err := storage.AddTaskToQueue(ctx, taskKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "添加任务到队列失败: " + err.Error(),
		})
		return
	}

	logger.Info("Shell 命令已提交",
		zap.String("event_id", eventID),
		zap.String("command", req.Command),
		zap.String("node_id", nodes[0].ID))

	// 立即返回 event_id，让前端通过轮询获取实时输出
	c.JSON(http.StatusOK, gin.H{
		"event_id": eventID,
		"job_id":   jobID,
		"command":  req.Command,
		"status":   "queued",
		"message":  "任务已提交，正在执行中",
		"node_id":  nodes[0].ID,
	})
}

// getShellLogs 获取 Shell 命令执行日志（支持流式输出）
func (s *APIServer) getShellLogs(c *gin.Context) {
	eventID := c.Param("event_id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "event_id 参数缺失",
		})
		return
	}

	ctx := context.Background()

	// 使用 storage.GetLogs 获取日志（优先 Redis，回退到文件）
	logs, err := storage.GetLogs(ctx, eventID)
	if err != nil {
		// 日志不存在时返回空字符串而不是错误
		logs = ""
	}

	// 检查任务是否完成
	var event models.Event
	complete := false
	exitCode := 0
	status := "unknown"

	if err := storage.DB.Where("id = ?", eventID).First(&event).Error; err == nil {
		status = event.Status
		if event.Status == eventStatusSuccess || event.Status == eventStatusFailed || event.Status == eventStatusAborted {
			complete = true
			exitCode = event.ExitCode
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"event_id":      eventID,
		"logs":          logs,
		"complete":      complete,
		"exit_code":     exitCode,
		"status":        status,
		"error_message": event.ErrorMessage,
	})
}

// downloadLog 下载执行日志
func (s *APIServer) downloadLog(c *gin.Context) {
	eventID := c.Param("id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_id 参数缺失"})
		return
	}

	logs, err := storage.GetLogs(context.Background(), eventID)
	if err != nil || logs == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "日志不存在"})
		return
	}

	filename := fmt.Sprintf("%s.log", eventID)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.String(http.StatusOK, logs)
}
