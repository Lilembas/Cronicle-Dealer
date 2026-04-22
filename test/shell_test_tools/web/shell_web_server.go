package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/utils"
)

func main() {
	cfg, err := config.Load("../../../config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v\n", err)
	}

	// 初始化日志
	if err := logger.InitLogger(&config.LoggingConfig{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	}); err != nil {
		log.Fatalf("初始化日志失败: %v\n", err)
	}
	defer logger.Sync()

	// 初始化存储
	fmt.Println("🔧 初始化存储...")
	if err := storage.InitDB(&cfg.Database); err != nil {
		logger.Fatal("数据库初始化失败", zap.Error(err))
	}
	defer storage.CloseDB()

	if err := storage.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败", zap.Error(err))
	}

	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 初始化失败", zap.Error(err))
	}
	defer storage.CloseRedis()
	fmt.Println("✅ 存储初始化成功")

	// 设置Gin模式
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// 静态文件
	router.StaticFile("/", "./shell_test.html")
	router.Static("/static", "./static")

	// API路由
	api := router.Group("/api/v1")
	{
		api.POST("/shell/execute", executeShellHandler)
		api.GET("/health", healthCheckHandler)
		api.GET("/stats", statsHandler)
		api.GET("/nodes", getNodesHandler)
		api.GET("/logs/:event_id", getLogsHandler)
	}

	// 启动Web服务器
	addr := fmt.Sprintf("%s:%d", cfg.Manager.Host, 8888)

	fmt.Println("\n========================================")
	fmt.Println("🌐 Shell测试Web服务器启动成功！")
	fmt.Println("========================================")
	fmt.Printf("📝 测试页面: http://%s\n", addr)
	fmt.Printf("🔧 API地址: http://%s/api/v1\n", addr)
	fmt.Println("========================================")
	fmt.Println("📝 按 Ctrl+C 停止服务")

	logger.Info("Web服务器启动", zap.String("address", addr))

	if err := router.Run(addr); err != nil {
		logger.Fatal("Web服务器启动失败", zap.Error(err))
	}
}

// executeShellHandler 执行Shell命令的处理函数
func executeShellHandler(c *gin.Context) {
	var req struct {
		Command string `json:"command" binding:"required"`
		NodeID  string `json:"node_id"` // 可选：指定执行节点
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 检查Worker是否在线（检查最近60秒有心跳的节点）
	heartbeatTimeout := time.Now().Add(-60 * time.Second)
	var nodes []models.Node

	query := storage.DB.Where("status = ? AND last_heartbeat > ?", "online", heartbeatTimeout)

	// 如果指定了节点ID，只查询该节点
	if req.NodeID != "" {
		query = query.Where("id = ?", req.NodeID)
	}

	if err := query.Find(&nodes).Error; err != nil || len(nodes) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "没有可用的Worker节点",
		})
		return
	}

	// 创建临时任务和事件
	jobID := utils.GenerateID("job")
	eventID := utils.GenerateID("event")

	ctx := context.Background()
	taskKey := fmt.Sprintf("%s:%s", jobID, eventID)

	// 创建Job记录（Dispatcher需要从jobs表查询任务配置）
	now := time.Now()
	job := &models.Job{
		ID:          jobID,
		Name:        "Shell测试命令",
		Description: "通过Web界面执行的临时Shell命令测试",
		Category:    "test",
		CronExpr:    "* * * * *", // 临时任务，使用假的cron表达式
		Enabled:     false,       // 禁用，避免被调度器重复执行
		TaskType:    "shell",
		Command:     req.Command,
		TargetType:  "any",
		Timeout:     30,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := storage.DB.Create(job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建任务失败: " + err.Error(),
		})
		return
	}

	// 创建事件
	event := &models.Event{
		ID:        eventID,
		JobID:     jobID,
		JobName:   "Shell测试命令",
		Status:    "pending",
		CreatedAt: now,
	}

	if err := storage.DB.Create(event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建事件失败: " + err.Error(),
		})
		return
	}

	// 保存任务详情到Redis
	taskData := map[string]interface{}{
		"job_id":         jobID,
		"event_id":       eventID,
		"job_name":       "Shell测试命令",
		"command":        req.Command,
		"task_type":      "shell",
		"timeout":        30,
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

	// 立即返回event_id，让前端通过轮询获取实时输出
	c.JSON(http.StatusOK, gin.H{
		"event_id": eventID,
		"job_id":   jobID,
		"command":  req.Command,
		"status":   "queued",
		"message":  "任务已提交，正在执行中",
	})
}

// healthCheckHandler 健康检查
func healthCheckHandler(c *gin.Context) {
	// 检查最近60秒有心跳的节点
	heartbeatTimeout := time.Now().Add(-60 * time.Second)
	var onlineNodes int64
	storage.DB.Model(&models.Node{}).
		Where("status = ? AND last_heartbeat > ?", "online", heartbeatTimeout).
		Count(&onlineNodes)

	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"online_nodes": onlineNodes,
		"timestamp":    time.Now().Unix(),
	})
}

// getNodesHandler 获取所有在线节点列表
func getNodesHandler(c *gin.Context) {
	heartbeatTimeout := time.Now().Add(-60 * time.Second)
	var nodes []models.Node
	if err := storage.DB.Where("status = ? AND last_heartbeat > ?", "online", heartbeatTimeout).
		Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询节点失败: " + err.Error(),
		})
		return
	}

	// 转换为简化的节点信息
	nodeList := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		nodeList = append(nodeList, gin.H{
			"id":       node.ID,
			"hostname": node.Hostname,
			"ip":       node.IP,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodeList,
		"count": len(nodeList),
	})
}

// getLogsHandler 获取任务日志（支持流式输出）
func getLogsHandler(c *gin.Context) {
	eventID := c.Param("event_id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "event_id参数缺失",
		})
		return
	}

	ctx := context.Background()
	logKey := fmt.Sprintf("task_logs:%s", eventID)
	logs, _ := storage.RedisClient.Get(ctx, logKey).Result()

	// 检查任务是否完成
	var event models.Event
	complete := false
	exitCode := 0
	status := "unknown"

	if err := storage.DB.Where("id = ?", eventID).First(&event).Error; err == nil {
		status = event.Status
		if event.Status == "success" || event.Status == "failed" {
			complete = true
			exitCode = event.ExitCode
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"event_id":  eventID,
		"logs":      logs,
		"complete":  complete,
		"exit_code": exitCode,
		"status":    status,
	})
}

// statsHandler 获取统计信息
func statsHandler(c *gin.Context) {
	var stats struct {
		OnlineNodes int64 `json:"online_nodes"`
		RunningJobs int64 `json:"running_jobs"`
	}

	// 只统计最近60秒有心跳的节点
	heartbeatTimeout := time.Now().Add(-60 * time.Second)
	storage.DB.Model(&models.Node{}).
		Where("status = ? AND last_heartbeat > ?", "online", heartbeatTimeout).
		Count(&stats.OnlineNodes)
	storage.DB.Model(&models.Event{}).Where("status = ?", "running").Count(&stats.RunningJobs)

	c.JSON(http.StatusOK, stats)
}
