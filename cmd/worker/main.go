package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/internal/worker"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

const (
	version  = "0.1.0"
	nodeType = "Worker"
)

var configPath = flag.String("config", "config.yaml", "配置文件路径")

func main() {
	// Panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Worker panic",
				zap.Any("panic", r),
				zap.Stack("stack"))
		}
	}()

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
		zap.String("master_address", cfg.Worker.MasterAddress))

	if err := initRedis(cfg); err != nil {
		logger.Fatal("Redis 连接失败", zap.Error(err))
	}
	defer storage.CloseRedis()

	client, executor, err := startWorker(cfg)
	if err != nil {
		logger.Fatal("启动失败", zap.Error(err))
	}
	defer cleanupWorker(client, executor)

	go client.StartHeartbeat()

	logger.Info("节点启动成功",
		zap.String("master_address", cfg.Worker.MasterAddress),
		zap.Strings("tags", cfg.Worker.Node.Tags))

	waitForShutdown()

	logger.Info("节点已关闭")
}

func loadConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

func initLogger(cfg *config.LoggingConfig) error {
	return logger.InitLogger(cfg)
}

func initRedis(cfg *config.Config) error {
	logger.Info("连接 Redis...")
	return storage.InitRedis(&cfg.Redis)
}

func startWorker(cfg *config.Config) (*worker.Client, *worker.Executor, error) {
	logger.Info("连接 Master...")

	client := worker.NewClient(&cfg.Worker)
	if err := client.Connect(); err != nil {
		return nil, nil, err
	}

	if err := client.Register(); err != nil {
		client.Close()
		return nil, nil, err
	}

	logger.Info("启动执行器...")
	executor := worker.NewExecutor(&cfg.Worker.Executor)
	if err := executor.Start(0); err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, executor, nil
}

func cleanupWorker(client *worker.Client, executor *worker.Executor) {
	executor.Stop()
	client.Close()
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
