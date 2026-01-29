package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

func main() {
	// 加载配置
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

	// 设置Gin
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// 提供静态文件
	router.StaticFile("/", "./shell_test.html")
	router.Static("/static", "./static")

	// API路由
	api := router.Group("/api/v1")
	{
		api.POST("/shell/execute", executeShellHandler)
		api.GET("/health", healthCheckHandler)
		api.GET("/stats", statsHandler)
		api.GET("/nodes", getNodesHandler)
	}

	// 启动Web服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, 8888)

	fmt.Println("\n========================================")
	fmt.Println("🌐 Shell测试Web服务器启动成功！")
	fmt.Println("========================================")
	fmt.Printf("📝 测试页面: http://%s\n", addr)
	fmt.Printf("🔧 API地址: http://%s/api/v1\n", addr)
	fmt.Println("========================================\n")
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
	jobID := fmt.Sprintf("shell_test_%d", time.Now().UnixNano())
	eventID := fmt.Sprintf("shell_event_%d", time.Now().UnixNano())

	ctx := context.Background()
	taskKey := fmt.Sprintf("%s:%s", jobID, eventID)

	// 创建事件
	event := &models.Event{
		ID:        eventID,
		JobID:     jobID,
		JobName:   "Shell测试命令",
		Status:    "pending",
		CreatedAt: time.Now(),
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
		"working_dir":    "",
		"env":            "", // 使用空字符串而不是map
		"scheduled_time": time.Now().Unix(),
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

	// 等待任务执行完成（最多30秒）
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var finalStatus string
	var finalOutput string
	var finalExitCode int

	for {
		select {
		case <-timeout:
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"error": "命令执行超时",
			})
			return

		case <-ticker.C:
			status, err := storage.GetTaskStatus(ctx, taskKey)
			if err != nil {
				continue
			}

			if status == "completed" || status == "failed" {
				finalStatus = status

				// 获取执行结果
				result, err := storage.GetTaskResult(ctx, taskKey)
				if err == nil {
					if output, ok := result["output"]; ok {
						finalOutput = output
					}
					if exitCodeStr, ok := result["exit_code"]; ok && exitCodeStr != "" {
						finalExitCode, _ = strconv.Atoi(exitCodeStr)
					}
				}

				// 清理测试数据
				storage.DB.Where("id = ?", eventID).Delete(&models.Event{})
				storage.RedisClient.Del(ctx, "tasks:details:"+taskKey)
				storage.RedisClient.Del(ctx, "tasks:result:"+taskKey)
				storage.RedisClient.Del(ctx, "tasks:status:"+taskKey)

				// 返回结果
				c.JSON(http.StatusOK, gin.H{
					"command":   req.Command,
					"output":    finalOutput,
					"exit_code": finalExitCode,
					"status":    finalStatus,
				})
				return
			}
		}
	}
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
	// 获取最近60秒有心跳的在线节点
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
	type NodeInfo struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
		IP       string `json:"ip"`
		Tags     string `json:"tags"`
	}

	nodeList := make([]NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		nodeList = append(nodeList, NodeInfo{
			ID:       node.ID,
			Hostname: node.Hostname,
			IP:       node.IP,
			Tags:     string(node.Tags), // 转换JSON字符串
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodeList,
		"count": len(nodeList),
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
