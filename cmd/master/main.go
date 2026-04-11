package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/master"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

const (
	version  = "0.1.0"
	nodeType = "Master"
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

	if err := master.EnsureDefaultAdmin(); err != nil {
		logger.Fatal("初始化默认管理员失败", zap.Error(err))
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

	// 启动 Master
	logger.Info("启动核心服务...")
	m := master.NewMaster(cfg)
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
// 这可以防止 Master 重启后出现僵尸节点（Worker 已断开但状态仍为 online）
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
