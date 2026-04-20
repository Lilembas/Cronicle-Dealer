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
		zap.String("manager_address", cfg.Worker.ManagerAddress))

	logger.Info("连接 Redis...")
	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 连接失败", zap.Error(err))
	}
	defer storage.CloseRedis()

	logger.Info("初始化日志存储...")
	if err := storage.InitLogStorage(cfg.Storage.LogDir); err != nil {
		logger.Fatal("日志存储初始化失败", zap.Error(err))
	}

	logger.Info("启动执行器...")
	executor := worker.NewExecutor(&cfg.Worker.Executor)
	if err := executor.Start(cfg.Worker.Executor.GRPCPort); err != nil {
		logger.Fatal("启动执行器失败", zap.Error(err))
	}

	logger.Info("连接 Manager...")
	client := worker.NewClient(&cfg.Worker)
	if err := client.Connect(); err != nil {
		executor.Stop()
		logger.Fatal("连接 Manager 失败", zap.Error(err))
	}

	// 设置 Worker 的 gRPC 地址（必须使用实际 IP，不能用 0.0.0.0）
	client.SetGRPCAddress("0.0.0.0", cfg.Worker.Executor.GRPCPort)

	if err := client.Register(); err != nil {
		client.Close()
		executor.Stop()
		logger.Fatal("注册失败", zap.Error(err))
	}

	executor.SetManagerClient(client.GetManagerClient())
	logger.Info("已设置Manager客户端")

	go client.StartHeartbeat()

	logger.Info("节点启动成功",
		zap.String("manager_address", cfg.Worker.ManagerAddress),
		zap.Strings("tags", cfg.Worker.Node.Tags))

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("收到退出信号，正在关闭...", zap.String("signal", sig.String()))

	// 关闭所有日志文件句柄，flush 到磁盘
	if err := storage.CloseAllLogFiles(); err != nil {
		logger.Warn("关闭日志文件失败", zap.Error(err))
	}

	executor.Stop()
	client.Close()

	logger.Info("节点已关闭")
}

func exitWithError(msg string, err error) {
	fmt.Printf("错误: %s: %v\n", msg, err)
	os.Exit(1)
}
