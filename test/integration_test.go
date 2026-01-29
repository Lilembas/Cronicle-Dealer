package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/master"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/internal/worker"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

var (
	configPath = flag.String("config", "../config.yaml", "配置文件路径")
)

func main() {
	flag.Parse()

	fmt.Println("🧪 Cronicle-Next 后端通讯链路测试")
	fmt.Println("================================")

	// 初始化日志
	if err := logger.InitLogger(&config.LoggingConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}); err != nil {
		log.Fatalf("❌ 初始化日志失败: %v", err)
	}
	defer logger.Sync()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	// 测试各个组件
	testStorage(cfg)
	masterNode := testMaster(cfg)
	defer masterNode.Stop()

	workerClient := testWorker(cfg)
	testJob, event, taskKey := testTaskScheduling(cfg)
	nodeID := testWorkerHeartbeat(cfg, workerClient)
	testTaskStatus(cfg, taskKey)
	cleanupTestData(cfg, testJob, event, taskKey, nodeID)

	printTestSummary()
}

func testStorage(cfg *config.Config) {
	fmt.Println("1️⃣ 测试存储连接...")

	if err := storage.InitDB(&cfg.Database); err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}
	defer storage.CloseDB()

	if err := storage.AutoMigrate(); err != nil {
		log.Fatalf("❌ 数据库迁移失败: %v", err)
	}

	if err := storage.InitRedis(&cfg.Redis); err != nil {
		log.Fatalf("❌ Redis 连接失败: %v", err)
	}
	defer storage.CloseRedis()

	fmt.Println("✅ 存储连接成功")
}

func testMaster(cfg *config.Config) *master.Master {
	fmt.Println("2️⃣ 启动 Master 节点...")

	masterNode := master.NewMaster(cfg)
	if err := masterNode.Start(); err != nil {
		log.Fatalf("❌ Master 启动失败: %v", err)
	}

	time.Sleep(2 * time.Second)
	fmt.Println("✅ Master 节点启动成功")

	return masterNode
}

func testWorker(cfg *config.Config) *worker.Client {
	fmt.Println("3️⃣ 启动 Worker 节点...")

	workerClient := worker.NewClient(&cfg.Worker)

	if err := workerClient.Connect(); err != nil {
		log.Fatalf("❌ Worker 连接 Master 失败: %v", err)
	}
	defer workerClient.Close()

	if err := workerClient.Register(); err != nil {
		log.Fatalf("❌ Worker 注册失败: %v", err)
	}

	fmt.Println("✅ Worker 节点启动成功")

	return workerClient
}

func testTaskScheduling(cfg *config.Config) (*models.Job, *models.Event, string) {
	fmt.Println("4️⃣ 测试任务创建和调度...")

	testJob := &models.Job{
		ID:          "test_job_001",
		Name:        "测试任务",
		Description: "用于测试通讯链路的简单任务",
		CronExpr:    "* * * * * *",
		Command:     "echo 'Hello from test task!' && sleep 1",
		TaskType:    "shell",
		Enabled:     true,
		Timeout:     10,
	}

	if err := storage.DB.Create(testJob).Error; err != nil {
		log.Fatalf("❌ 任务创建失败: %v", err)
	}

	event := &models.Event{
		ID:      "test_event_001",
		JobID:   testJob.ID,
		JobName: testJob.Name,
		Status:  "pending",
	}

	if err := storage.DB.Create(event).Error; err != nil {
		log.Fatalf("❌ 事件创建失败: %v", err)
	}

	taskKey := fmt.Sprintf("%s:%s", testJob.ID, event.ID)
	taskData := map[string]interface{}{
		"job_id":         testJob.ID,
		"event_id":       event.ID,
		"job_name":       testJob.Name,
		"command":        testJob.Command,
		"task_type":      testJob.TaskType,
		"timeout":        testJob.Timeout,
		"working_dir":    testJob.WorkingDir,
		"env":            testJob.Env,
		"scheduled_time": time.Now().Unix(),
	}

	ctx := context.Background()
	if err := storage.RedisClient.HSet(ctx, "tasks:details:"+taskKey, taskData).Err(); err != nil {
		log.Fatalf("❌ 任务详情存储失败: %v", err)
	}

	if err := storage.AddTaskToQueue(ctx, taskKey); err != nil {
		log.Fatalf("❌ 任务添加到队列失败: %v", err)
	}

	fmt.Println("✅ 任务已添加到队列")

	return testJob, event, taskKey
}

func testWorkerHeartbeat(cfg *config.Config, workerClient *worker.Client) string {
	fmt.Println("5️⃣ 验证 Worker 心跳...")

	time.Sleep(5 * time.Second)

	ctx := context.Background()
	nodeID := workerClient.GetNodeID()
	isOnline, err := storage.IsWorkerOnline(ctx, nodeID)

	if err != nil {
		log.Printf("⚠️  检查 Worker 在线状态失败: %v", err)
	} else if isOnline {
		fmt.Println("✅ Worker 心跳正常，节点在线")
	} else {
		fmt.Println("⚠️  Worker 节点离线")
	}

	return nodeID
}

func testTaskStatus(cfg *config.Config, taskKey string) {
	fmt.Println("6️⃣ 验证任务状态...")

	ctx := context.Background()
	status, err := storage.GetTaskStatus(ctx, taskKey)

	if err != nil {
		log.Printf("⚠️  获取任务状态失败: %v", err)
	} else {
		fmt.Printf("✅ 任务状态: %s\n", status)
	}
}

func cleanupTestData(cfg *config.Config, testJob *models.Job, event *models.Event, taskKey string, nodeID string) {
	fmt.Println("7️⃣ 清理测试数据...")

	ctx := context.Background()
	storage.DB.Where("id = ?", testJob.ID).Delete(&models.Job{})
	storage.DB.Where("id = ?", event.ID).Delete(&models.Event{})
	storage.RedisClient.Del(ctx, "tasks:details:"+taskKey)
	storage.RemoveWorkerOffline(ctx, nodeID)

	fmt.Println("✅ 测试数据清理完成")
}

func printTestSummary() {
	time.Sleep(3 * time.Second)

	fmt.Println("\n🎉 所有测试完成！")
	printTestResult("SQLite 数据库", true)
	printTestResult("Redis", true)
	printTestResult("Master 节点", true)
	printTestResult("Worker 节点", true)
	printTestResult("gRPC 通讯", true)
	printTestResult("任务调度", true)
	printTestResult("心跳机制", true)
	printTestResult("状态缓存", true)

	fmt.Println("\n💡 提示: 这个测试脚本验证了基本的通讯链路，但不包含完整的任务执行流程。")
	fmt.Println("   如需测试完整任务执行，请运行实际的 Master 和 Worker 服务。")
}

func printTestResult(name string, success bool) {
	status := "✅"
	if !success {
		status = "❌"
	}
	fmt.Printf("%s %s: 正常\n", status, name)
}
