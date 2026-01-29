package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/internal/worker"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

func main() {
	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Worker panic",
				zap.Any("panic", r),
				zap.Stack("stack"))
		}
	}()

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

	// 调试：打印实际读取的配置
	fmt.Printf("📋 Master地址配置: %s\n", cfg.Worker.MasterAddress)
	fmt.Printf("📋 Server.Host: %s\n", cfg.Server.Host)
	fmt.Printf("📋 Worker.GRPCPort: %d\n", cfg.Worker.Executor.GRPCPort)

	workerClient := worker.NewClient(&cfg.Worker)
	if err := workerClient.Connect(); err != nil {
		logger.Fatal("Worker 连接 Master 失败", zap.Error(err))
	}
	defer workerClient.Close()

	// 设置 executor gRPC 地址（用于Master连接Worker）
	// 传入"0.0.0.0"让Worker自动使用检测到的真实IP地址
	workerClient.SetGRPCAddress(cfg.Server.Host, cfg.Worker.Executor.GRPCPort)

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
	executor.SetMasterClient(workerClient.GetMasterClient()) // 设置Master客户端用于报告结果
	if err := executor.Start(cfg.Worker.Executor.GRPCPort); err != nil {
		logger.Fatal("执行器启动失败", zap.Error(err))
	}
	defer executor.Stop()

	fmt.Println("✅ 执行器启动成功")
	fmt.Printf("   📡 gRPC 端口: %d\n", cfg.Worker.Executor.GRPCPort)

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

	// 优雅关闭：监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	logger.Info("收到关闭信号，正在优雅关闭...", zap.String("signal", sig.String()))

	fmt.Println("\n🛑 正在关闭 Worker...")
	// defer会自动调用workerClient.Close()和executor.Stop()
}
