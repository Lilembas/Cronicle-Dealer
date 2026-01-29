package main

import (
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/internal/worker"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load("../../config.yaml")
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

	// 初始化Redis（Worker需要Redis）
	fmt.Println("🔧 初始化Redis连接...")
	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 初始化失败", zap.Error(err))
	}
	defer storage.CloseRedis()

	fmt.Println("✅ Redis 连接成功")

	// 创建Worker客户端
	fmt.Println("\n🚀 启动 Worker 节点")
	fmt.Println("====================")

	workerClient := worker.NewClient(&cfg.Worker)
	if err := workerClient.Connect(); err != nil {
		logger.Fatal("Worker 连接 Master 失败", zap.Error(err))
	}
	defer workerClient.Close()

	if err := workerClient.Register(); err != nil {
		logger.Fatal("Worker 注册失败", zap.Error(err))
	}

	fmt.Println("✅ Worker 注册成功")

	nodeID := workerClient.GetNodeID()
	logger.Info("Worker 节点信息",
		zap.String("node_id", nodeID),
		zap.Strings("tags", cfg.Worker.Node.Tags))

	// 启动Worker执行器
	fmt.Println("\n🔧 启动 Worker 执行器...")
	executor := worker.NewExecutor(&cfg.Worker.Executor)
	if err := executor.Start(0); err != nil {
		logger.Fatal("执行器启动失败", zap.Error(err))
	}
	defer executor.Stop()

	fmt.Println("✅ 执行器启动成功")

	// 启动心跳
	fmt.Println("\n💓 启动心跳机制...")
	go workerClient.StartHeartbeat()

	// 等待Worker完全就绪
	time.Sleep(2 * time.Second)

	fmt.Println("\n========================================")
	fmt.Println("✅ Worker 节点启动成功！")
	fmt.Println("========================================")
	fmt.Printf("🆔 节点ID: %s\n", nodeID)
	fmt.Printf("📡 gRPC 地址: %s:%d\n", cfg.Server.Host, cfg.Worker.Executor.GRPCPort)
	fmt.Printf("🎯 标签: %v\n", cfg.Worker.Node.Tags)
	fmt.Println("========================================\n")
	fmt.Println("📝 按 Ctrl+C 停止服务")

	// 保持运行
	select {}
}
