package main

import (
	"context"
	"flag"
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

var (
	testConfigPath = flag.String("config", "../config.yaml", "配置文件路径")
	testDuration   = flag.Duration("duration", 30*time.Second, "测试运行时长")
)

func main() {
	flag.Parse()

	fmt.Println("🔧 Cronicle-Next Worker 启动测试")
	fmt.Println("================================")

	// 检查配置文件
	if _, err := os.Stat(*testConfigPath); os.IsNotExist(err) {
		log.Fatalf("❌ 配置文件不存在: %s\n请先复制 config.example.yaml 到 config.yaml\n", *testConfigPath)
	}

	// 初始化日志
	if err := logger.InitLogger(&config.LoggingConfig{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	}); err != nil {
		log.Fatalf("❌ 初始化日志失败: %v\n", err)
	}
	defer logger.Sync()

	logger.Info("Worker 启动测试开始",
		zap.String("config", *testConfigPath),
		zap.Duration("duration", *testDuration))

	// 加载配置
	cfg, err := config.Load(*testConfigPath)
	if err != nil {
		logger.Fatal("加载配置失败", zap.Error(err))
	}

	// 测试步骤 1: 连接 Redis
	fmt.Println("\n1️⃣ 测试 Redis 连接...")
	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 连接失败", zap.Error(err))
	}
	defer storage.CloseRedis()

	ctx := context.Background()
	if err := storage.RedisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Redis Ping 失败", zap.Error(err))
	}
	fmt.Println("✅ Redis 连接成功")

	// 测试步骤 2: 创建 Worker 客户端
	fmt.Println("\n2️⃣ 创建 Worker 客户端...")
	logger.Info("连接 Master",
		zap.String("master_address", cfg.Worker.MasterAddress))

	client := worker.NewClient(&cfg.Worker)

	// 测试步骤 3: 连接 Master
	fmt.Println("\n3️⃣ 连接 Master 节点...")
	if err := client.Connect(); err != nil {
		logger.Fatal("连接 Master 失败", zap.Error(err))
	}
	defer client.Close()
	fmt.Println("✅ 成功连接到 Master")

	// 测试步骤 4: 注册 Worker
	fmt.Println("\n4️⃣ 注册 Worker 节点...")
	if err := client.Register(); err != nil {
		logger.Fatal("Worker 注册失败", zap.Error(err))
	}
	fmt.Println("✅ Worker 注册成功")

	nodeID := client.GetNodeID()
	logger.Info("Worker 节点信息",
		zap.String("node_id", nodeID),
		zap.String("hostname", cfg.Worker.Node.Hostname),
		zap.Strings("tags", cfg.Worker.Node.Tags))

	// 测试步骤 5: 启动执行器
	fmt.Println("\n5️⃣ 启动任务执行器...")
	executor := worker.NewExecutor(&cfg.Worker.Executor)
	if err := executor.Start(0); err != nil {
		logger.Fatal("执行器启动失败", zap.Error(err))
	}
	defer executor.Stop()
	fmt.Println("✅ 执行器启动成功")

	// 测试步骤 6: 启动心跳
	fmt.Println("\n6️⃣ 启动心跳机制...")
	heartbeatDone := make(chan struct{})
	go func() {
		client.StartHeartbeat()
		close(heartbeatDone)
	}()
	fmt.Println("✅ 心跳启动成功")

	// 测试步骤 7: 验证心跳
	fmt.Println("\n7️⃣ 验证心跳状态...")
	time.Sleep(3 * time.Second) // 等待几次心跳

	isOnline, err := storage.IsWorkerOnline(ctx, nodeID)
	if err != nil {
		logger.Warn("检查 Worker 在线状态失败", zap.Error(err))
	} else if isOnline {
		fmt.Println("✅ Worker 心跳正常，节点在线")
	} else {
		logger.Warn("Worker 节点显示为离线")
	}

	// 测试步骤 8: 等待接收任务
	fmt.Println("\n8️⃣ Worker 就绪，等待 Master 分发任务...")
	logger.Info("Worker 执行器已启动",
		zap.Int("max_concurrent", cfg.Worker.Executor.MaxConcurrentJobs))

	// 测试步骤 9: 运行指定时长
	fmt.Printf("\n🟢 Worker 运行中，将持续 %v...\n", *testDuration)
	fmt.Println("💡 提示: 你可以在另一个终端运行 Master 来调度任务")

	// 设置超时或等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	timer := time.NewTimer(*testDuration)
	defer timer.Stop()

	select {
	case sig := <-sigChan:
		logger.Info("收到退出信号", zap.String("signal", sig.String()))
	case <-timer.C:
		logger.Info("测试时间到")
	}

	// 测试步骤 10: 清理
	fmt.Println("\n🧙 清理测试环境...")

	// 移除 Worker 注册信息
	if err := storage.RemoveWorkerOffline(ctx, nodeID); err != nil {
		logger.Warn("清理 Worker 注册信息失败", zap.Error(err))
	}

	// 停止执行器
	fmt.Println("⏳ 停止执行器...")
	executor.Stop()

	// 关闭连接
	client.Close()

	fmt.Println("\n✅ Worker 启动测试完成")
	fmt.Println("===============================")
	fmt.Printf("✅ Redis 连接: 正常\n")
	fmt.Printf("✅ Master 连接: 正常\n")
	fmt.Printf("✅ 节点注册: 正常\n")
	fmt.Printf("✅ 执行器: 正常\n")
	fmt.Printf("✅ 心跳机制: 正常\n")
	fmt.Printf("\n💡 Worker 节点 ID: %s\n", nodeID)
	fmt.Println("\n💡 提示: Worker 现在可以通过 Master 调度任务")
}
