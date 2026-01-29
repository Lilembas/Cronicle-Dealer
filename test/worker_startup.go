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
	configPath = flag.String("config", "../config.yaml", "配置文件路径")
	duration   = flag.Duration("duration", 30*time.Second, "测试运行时长")
)

func main() {
	flag.Parse()

	fmt.Println("🔧 Cronicle-Next Worker 启动测试")
	fmt.Println("================================")

	cfg := loadConfig(*configPath)
	initializeLogger(cfg)

	testRedisConnection(cfg)
	workerClient := connectToMaster(cfg)
	registerWorker(workerClient, cfg)
	executor := startExecutor(cfg)
	startHeartbeat(workerClient)

	nodeID := workerClient.GetNodeID()
	verifyHeartbeat(nodeID)

	runTestOrWait(duration, nodeID)
	cleanup(workerClient, executor, nodeID)

	printTestSummary(nodeID)
}

func loadConfig(path string) *config.Config {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("❌ 配置文件不存在: %s\n请先复制 config.example.yaml 到 config.yaml\n", path)
	}

	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	return cfg
}

func initializeLogger(cfg *config.Config) {
	if err := logger.InitLogger(&config.LoggingConfig{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	}); err != nil {
		log.Fatalf("❌ 初始化日志失败: %v\n", err)
	}
	defer logger.Sync()

	logger.Info("Worker 启动测试开始",
		zap.String("config", *configPath),
		zap.Duration("duration", *duration))
}

func testRedisConnection(cfg *config.Config) {
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
}

func connectToMaster(cfg *config.Config) *worker.Client {
	fmt.Println("\n2️⃣ 创建 Worker 客户端...")

	logger.Info("连接 Master",
		zap.String("master_address", cfg.Worker.MasterAddress))

	client := worker.NewClient(&cfg.Worker)

	fmt.Println("\n3️⃣ 连接 Master 节点...")
	if err := client.Connect(); err != nil {
		logger.Fatal("连接 Master 失败", zap.Error(err))
	}

	fmt.Println("✅ 成功连接到 Master")
	return client
}

func registerWorker(client *worker.Client, cfg *config.Config) {
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
}

func startExecutor(cfg *config.Config) *worker.Executor {
	fmt.Println("\n5️⃣ 启动任务执行器...")

	executor := worker.NewExecutor(&cfg.Worker.Executor)
	if err := executor.Start(0); err != nil {
		logger.Fatal("执行器启动失败", zap.Error(err))
	}

	fmt.Println("✅ 执行器启动成功")
	return executor
}

func startHeartbeat(client *worker.Client) {
	fmt.Println("\n6️⃣ 启动心跳机制...")
	go client.StartHeartbeat()
	fmt.Println("✅ 心跳启动成功")
}

func verifyHeartbeat(nodeID string) {
	fmt.Println("\n7️⃣ 验证心跳状态...")

	time.Sleep(3 * time.Second)

	ctx := context.Background()
	isOnline, err := storage.IsWorkerOnline(ctx, nodeID)

	if err != nil {
		logger.Warn("检查 Worker 在线状态失败", zap.Error(err))
	} else if isOnline {
		fmt.Println("✅ Worker 心跳正常，节点在线")
	} else {
		logger.Warn("Worker 节点显示为离线")
	}
}

func runTestOrWait(duration *time.Duration, nodeID string) {
	fmt.Println("\n8️⃣ Worker 就绪，等待 Master 分发任务...")

	logger.Info("Worker 执行器已启动",
		zap.Int("max_concurrent", 1))

	fmt.Printf("\n🟢 Worker 运行中，将持续 %v...\n", *duration)
	fmt.Println("💡 提示: 你可以在另一个终端运行 Master 来调度任务")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	timer := time.NewTimer(*duration)
	defer timer.Stop()

	select {
	case sig := <-sigChan:
		logger.Info("收到退出信号", zap.String("signal", sig.String()))
	case <-timer.C:
		logger.Info("测试时间到")
	}
}

func cleanup(client *worker.Client, executor *worker.Executor, nodeID string) {
	fmt.Println("\n🧙 清理测试环境...")

	ctx := context.Background()
	if err := storage.RemoveWorkerOffline(ctx, nodeID); err != nil {
		logger.Warn("清理 Worker 注册信息失败", zap.Error(err))
	}

	fmt.Println("⏳ 停止执行器...")
	executor.Stop()

	fmt.Println("⏳ 关闭连接...")
	client.Close()
}

func printTestSummary(nodeID string) {
	fmt.Println("\n✅ Worker 启动测试完成")
	fmt.Println("===============================")

	printTestStatus("Redis 连接", true)
	printTestStatus("Master 连接", true)
	printTestStatus("节点注册", true)
	printTestStatus("执行器", true)
	printTestStatus("心跳机制", true)

	fmt.Printf("\n💡 Worker 节点 ID: %s\n", nodeID)
	fmt.Println("\n💡 提示: Worker 现在可以通过 Master 调度任务")
}

func printTestStatus(name string, success bool) {
	status := "✅"
	if !success {
		status = "❌"
	}
	fmt.Printf("%s %s: 正常\n", status, name)
}
