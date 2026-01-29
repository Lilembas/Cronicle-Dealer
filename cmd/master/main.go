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
	version = "0.1.0"
	nodeType = "Master"
)

var configPath = flag.String("config", "config.yaml", "配置文件路径")

func main() {
	flag.Parse()

	fmt.Printf("Cronicle-Next %s 节点 v%s\n", nodeType, version)
	fmt.Printf("加载配置文件: %s\n", *configPath)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		exitWithError("加载配置失败", err)
	}

	if err := initLogger(&cfg.Logging); err != nil {
		exitWithError("初始化日志失败", err)
	}
	defer logger.Sync()

	logger.Info("节点启动中...",
		zap.String("node_type", nodeType),
		zap.String("version", version),
		zap.String("mode", cfg.Server.Mode))

	if err := initStorage(cfg); err != nil {
		logger.Fatal("存储初始化失败", zap.Error(err))
	}
	defer closeStorage()

	if err := resetWorkerNodes(); err != nil {
		logger.Warn("重置 Worker 节点状态失败", zap.Error(err))
	}

	m, err := startMaster(cfg)
	if err != nil {
		logger.Fatal("启动失败", zap.Error(err))
	}
	defer m.Stop()

	logger.Info("节点启动成功",
		zap.Int("http_port", cfg.Server.HTTPPort),
		zap.Int("grpc_port", cfg.Server.GRPCPort))

	waitForShutdown()

	logger.Info("节点已关闭")
}

func loadConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

func initLogger(cfg *config.LoggingConfig) error {
	return logger.InitLogger(cfg)
}

func initStorage(cfg *config.Config) error {
	logger.Info("连接数据库...")
	if err := storage.InitDB(&cfg.Database); err != nil {
		return err
	}

	logger.Info("执行数据库迁移...")
	if err := storage.AutoMigrate(); err != nil {
		return err
	}

	logger.Info("连接 Redis...")
	return storage.InitRedis(&cfg.Redis)
}

func closeStorage() {
	storage.CloseDB()
	storage.CloseRedis()
}

func startMaster(cfg *config.Config) (*master.Master, error) {
	logger.Info("启动核心服务...")
	m := master.NewMaster(cfg)
	return m, m.Start()
}

func exitWithError(msg string, err error) {
	fmt.Printf("错误: %s: %v\n", msg, err)
	os.Exit(1)
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("收到退出信号，正在关闭...", zap.String("signal", sig.String()))
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
