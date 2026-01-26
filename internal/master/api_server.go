package master

import (
	"net/http"
	"strconv"
	
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// APIServer REST API 服务器
type APIServer struct {
	cfg        *config.Config
	router     *gin.Engine
	scheduler  *Scheduler
	dispatcher *Dispatcher
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
	
	// 任务管理
	jobs := api.Group("/jobs")
	{
		jobs.GET("", s.listJobs)           // 获取任务列表
		jobs.POST("", s.createJob)         // 创建任务
		jobs.GET("/:id", s.getJob)         // 获取任务详情
		jobs.PUT("/:id", s.updateJob)      // 更新任务
		jobs.DELETE("/:id", s.deleteJob)   // 删除任务
		jobs.POST("/:id/trigger", s.triggerJob) // 手动触发任务
	}
	
	// 执行记录
	events := api.Group("/events")
	{
		events.GET("", s.listEvents)       // 获取执行记录列表
		events.GET("/:id", s.getEvent)     // 获取执行记录详情
		events.POST("/:id/abort", s.abortEvent) // 中止执行
	}
	
	// 节点管理
	nodes := api.Group("/nodes")
	{
		nodes.GET("", s.listNodes)         // 获取节点列表
		nodes.GET("/:id", s.getNode)       // 获取节点详情
		nodes.DELETE("/:id", s.deleteNode) // 删除节点
	}
	
	// 统计信息
	api.GET("/stats", s.getStats)
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
		"time":   c.Request.Context().Value("server_time"),
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
	
	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"data":  jobs,
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
		job.ID = "job_" + strconv.FormatInt(c.Request.Context().Value("server_time").(int64), 10)
	}
	
	// 保存到数据库
	if err := storage.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 如果任务已启用，添加到调度器
	if job.Enabled {
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
	var job models.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	job.ID = c.Param("id")
	
	if err := storage.DB.Save(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 更新调度器
	if err := s.scheduler.UpdateJob(&job); err != nil {
		logger.Error("更新调度器任务失败", zap.Error(err))
	}
	
	c.JSON(http.StatusOK, job)
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
	
	// TODO: 手动创建任务执行记录并分发
	
	c.JSON(http.StatusOK, gin.H{"message": "任务已触发"})
}

// listEvents 获取执行记录列表
func (s *APIServer) listEvents(c *gin.Context) {
	var events []models.Event
	
	query := storage.DB
	
	// 过滤条件
	if jobID := c.Query("job_id"); jobID != "" {
		query = query.Where("job_id = ?", jobID)
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
	
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	// TODO: 实现任务中止逻辑
	c.JSON(http.StatusOK, gin.H{"message": "功能开发中"})
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

// getNode 获取节点详情
func (s *APIServer) getNode(c *gin.Context) {
	var node models.Node
	if err := storage.DB.Where("id = ?", c.Param("id")).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}
	
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
	}
	
	storage.DB.Model(&models.Job{}).Count(&stats.TotalJobs)
	storage.DB.Model(&models.Job{}).Where("enabled = ?", true).Count(&stats.EnabledJobs)
	storage.DB.Model(&models.Event{}).Count(&stats.TotalEvents)
	storage.DB.Model(&models.Event{}).Where("status = ?", "running").Count(&stats.RunningEvents)
	storage.DB.Model(&models.Event{}).Where("status = ?", "success").Count(&stats.SuccessEvents)
	storage.DB.Model(&models.Event{}).Where("status = ?", "failed").Count(&stats.FailedEvents)
	storage.DB.Model(&models.Node{}).Where("status = ?", "online").Count(&stats.OnlineNodes)
	storage.DB.Model(&models.Node{}).Where("status = ?", "offline").Count(&stats.OfflineNodes)
	
	c.JSON(http.StatusOK, stats)
}
