package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/manager"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

const (
	version  = "0.1.0"
	nodeType = "Manager"
)

var configPath = flag.String("config", "config.yaml", "配置文件路径")

func main() {
	flag.Parse()

	fmt.Printf("Cronicle-Next %s 节点 v%s\n", nodeType, version)
	fmt.Printf("加载配置文件: %s\n", *configPath)

	cfg, err := config.Load(*configPath)
	if err != nil {
		exitWithError("加载配置失败", err)
	}

	if err := logger.InitLogger(&cfg.Logging); err != nil {
		exitWithError("初始化日志失败", err)
	}
	defer logger.Sync()

	logger.Info("节点启动中...",
		zap.String("node_type", nodeType),
		zap.String("version", version),
		zap.String("mode", cfg.Server.Mode))

	// 初始化存储
	logger.Info("连接数据库...")
	if err := storage.InitDB(&cfg.Database); err != nil {
		logger.Fatal("数据库连接失败", zap.Error(err))
	}

	logger.Info("执行数据库迁移...")
	if err := storage.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败", zap.Error(err))
	}

	if err := manager.EnsureDefaultAdmin(); err != nil {
		logger.Fatal("初始化默认管理员失败", zap.Error(err))
	}

	logger.Info("初始化日志存储...")
	if err := storage.InitLogStorage(cfg.Storage.LogDir); err != nil {
		logger.Fatal("日志存储初始化失败", zap.Error(err))
	}

	logger.Info("连接 Redis...")
	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 连接失败", zap.Error(err))
	}
	defer func() {
		storage.CloseDB()
		storage.CloseRedis()
	}()

	// 重置 Worker 节点状态（防止僵尸节点）
	if err := resetWorkerNodes(); err != nil {
		logger.Warn("重置 Worker 节点状态失败", zap.Error(err))
	}

	// 清理重复的节点记录（相同 hostname + ip 只保留最新的）
	if err := cleanupDuplicateNodes(); err != nil {
		logger.Warn("清理重复节点失败", zap.Error(err))
	}

	// 启动 Manager
	logger.Info("启动核心服务...")
	m := manager.NewManager(cfg)
	if err := m.Start(); err != nil {
		logger.Fatal("启动失败", zap.Error(err))
	}
	defer m.Stop()

	logger.Info("节点启动成功",
		zap.Int("http_port", cfg.Server.HTTPPort),
		zap.Int("grpc_port", cfg.Server.GRPCPort))

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("收到退出信号，正在关闭...", zap.String("signal", sig.String()))

	logger.Info("节点已关闭")
}

// exitWithError 输出错误并退出
func exitWithError(msg string, err error) {
	fmt.Printf("错误: %s: %v\n", msg, err)
	os.Exit(1)
}

// resetWorkerNodes 将所有在线的 Worker 节点状态重置为离线
// 这可以防止 Manager 重启后出现僵尸节点（Worker 已断开但状态仍为 online）
func resetWorkerNodes() error {
	logger.Info("重置 Worker 节点状态...")
	result := storage.DB.Model(&models.Node{}).
		Where("status = ?", "online").
		Update("status", "offline")
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		logger.Info("已将在线 Worker 标记为离线", zap.Int64("count", result.RowsAffected))
	}
	return nil
}

// cleanupDuplicateNodes 清理重复的节点记录（相同 hostname + ip 只保留 Manager 或最新的）
func cleanupDuplicateNodes() error {
	logger.Info("清理重复的节点记录...")

	// 查找所有重复的 hostname + ip 组
	type NodeGroup struct {
		Hostname string
		IP       string
		Count    int
	}
	var groups []NodeGroup
	if err := storage.DB.Model(&models.Node{}).
		Select("hostname, ip, count(*) as count").
		Where("tags NOT LIKE '%manager%' OR tags IS NULL").
		Group("hostname, ip").
		Having("count > 1").
		Scan(&groups).Error; err != nil {
		return err
	}

	if len(groups) == 0 {
		logger.Info("没有发现重复节点")
		return nil
	}

	logger.Info("发现重复节点组", zap.Int("count", len(groups)))

	// 对每个重复组，优先保留 Manager，其次保留最新的
	totalDeleted := 0
	for _, group := range groups {
		logger.Info("处理重复节点",
			zap.String("hostname", group.Hostname),
			zap.String("ip", group.IP),
			zap.Int("count", group.Count))

		var nodes []models.Node
		if err := storage.DB.Where("hostname = ? AND ip = ?", group.Hostname, group.IP).
			Where("tags NOT LIKE '%manager%' OR tags IS NULL").
			Order("created_at ASC").
			Find(&nodes).Error; err != nil {
			logger.Error("查询重复节点失败", zap.Error(err))
			continue
		}

		if len(nodes) <= 1 {
			continue
		}

		// 找出 Manager 节点（如果有的话）
		var managerNode *models.Node
		var workerNodes []*models.Node

		for i := range nodes {
			if nodes[i].Tags == "manager" {
				managerNode = &nodes[i]
			} else {
				workerNodes = append(workerNodes, &nodes[i])
			}
		}

		// 删除策略：
		// 1. 如果有 Manager 节点，保留 Manager，删除所有 Worker
		// 2. 如果没有 Manager，保留最新的 Worker，删除其他的
		var nodesToKeep []*models.Node
		var nodesToDelete []*models.Node

		if managerNode != nil {
			nodesToKeep = []*models.Node{managerNode}
			nodesToDelete = workerNodes
			logger.Info("保留 Manager 节点，删除 Worker",
				zap.String("hostname", group.Hostname),
				zap.String("manager_id", managerNode.ID),
				zap.Int("worker_count", len(workerNodes)))
		} else {
			// 保留最新的 Worker
			nodesToKeep = []*models.Node{workerNodes[len(workerNodes)-1]}
			nodesToDelete = workerNodes[:len(workerNodes)-1]
			logger.Info("保留最新 Worker，删除旧 Worker",
				zap.String("hostname", group.Hostname),
				zap.String("kept_id", nodesToKeep[0].ID),
				zap.Int("deleted_count", len(nodesToDelete)))
		}

		// 执行删除
		for _, node := range nodesToDelete {
			if err := storage.DB.Delete(node).Error; err != nil {
				logger.Error("删除重复节点失败",
					zap.String("node_id", node.ID),
					zap.Error(err))
			} else {
				totalDeleted++
				logger.Info("删除重复节点",
					zap.String("node_id", node.ID),
					zap.String("hostname", node.Hostname),
					zap.String("tags", node.Tags))
			}
		}
	}

	logger.Info("重复节点清理完成", zap.Int("total_deleted", totalDeleted))
	return nil
}
