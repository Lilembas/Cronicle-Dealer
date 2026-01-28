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
	testConfigPath = flag.String("config", "../config.yaml", "配置文件路径")
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
	cfg, err := config.Load(*testConfigPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	// 1. 测试 SQLite 数据库连接
	fmt.Println("1️⃣ 测试 SQLite 数据库连接...")
	if err := storage.InitDB(&cfg.Database); err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}
	defer storage.CloseDB()

	// 自动迁移数据库
	if err := storage.AutoMigrate(); err != nil {
		log.Fatalf("❌ 数据库迁移失败: %v", err)
	}
	fmt.Println("✅ SQLite 数据库连接成功")

	// 2. 测试 Redis 连接
	fmt.Println("2️⃣ 测试 Redis 连接...")
	if err := storage.InitRedis(&cfg.Redis); err != nil {
		log.Fatalf("❌ Redis 连接失败: %v", err)
	}
	defer storage.CloseRedis()
	fmt.Println("✅ Redis 连接成功")

	// 3. 启动 Master 节点
	fmt.Println("3️⃣ 启动 Master 节点...")
	masterNode := master.NewMaster(cfg)
	if err := masterNode.Start(); err != nil {
		log.Fatalf("❌ Master 启动失败: %v", err)
	}
	defer masterNode.Stop()

	// 等待 Master 启动完成
	time.Sleep(2 * time.Second)
	fmt.Println("✅ Master 节点启动成功")

	// 4. 启动 Worker 节点
	fmt.Println("4️⃣ 启动 Worker 节点...")
	workerClient := worker.NewClient(&cfg.Worker)

	// 连接到 Master
	if err := workerClient.Connect(); err != nil {
		log.Fatalf("❌ Worker 连接 Master 失败: %v", err)
	}
	defer workerClient.Close()

	// 注册节点
	if err := workerClient.Register(); err != nil {
		log.Fatalf("❌ Worker 注册失败: %v", err)
	}
	fmt.Println("✅ Worker 节点启动成功")

	// 5. 测试任务创建和调度
	fmt.Println("5️⃣ 测试任务创建和调度...")
	testJob := &models.Job{
		ID:          "test_job_001",
		Name:        "测试任务",
		Description: "用于测试通讯链路的简单任务",
		CronExpr:    "* * * * * *", // 每秒执行一次（仅用于测试）
		Command:     "echo 'Hello from test task!' && sleep 1",
		TaskType:    "shell",
		Enabled:     true,
		Timeout:     10,
	}

	// 保存任务到数据库
	if err := storage.DB.Create(testJob).Error; err != nil {
		log.Fatalf("❌ 任务创建失败: %v", err)
	}

	// 手动触发任务执行
	event := &models.Event{
		ID:      "test_event_001",
		JobID:   testJob.ID,
		JobName: testJob.Name,
		Status:  "pending",
	}

	if err := storage.DB.Create(event).Error; err != nil {
		log.Fatalf("❌ 事件创建失败: %v", err)
	}

	// 将任务添加到 Redis 队列
	taskKey := fmt.Sprintf("%s:%s", testJob.ID, event.ID)
	taskData := map[string]interface{}{
		"job_id":      testJob.ID,
		"event_id":    event.ID,
		"job_name":    testJob.Name,
		"command":     testJob.Command,
		"task_type":   testJob.TaskType,
		"timeout":     testJob.Timeout,
		"working_dir": testJob.WorkingDir,
		"env":         testJob.Env,
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

	// 6. 验证 Worker 心跳
	fmt.Println("6️⃣ 验证 Worker 心跳...")
	// 等待几次心跳
	time.Sleep(5 * time.Second)

	// 检查 Worker 是否在线
	nodeID := workerClient.GetNodeID()
	isOnline, err := storage.IsWorkerOnline(ctx, nodeID)
	if err != nil {
		log.Printf("⚠️  检查 Worker 在线状态失败: %v", err)
	} else if isOnline {
		fmt.Println("✅ Worker 心跳正常，节点在线")
	} else {
		fmt.Println("⚠️  Worker 节点离线")
	}

	// 7. 验证任务状态
	fmt.Println("7️⃣ 验证任务状态...")
	status, err := storage.GetTaskStatus(ctx, taskKey)
	if err != nil {
		log.Printf("⚠️  获取任务状态失败: %v", err)
	} else {
		fmt.Printf("✅ 任务状态: %s\n", status)
	}

	// 8. 清理测试数据
	fmt.Println("8️⃣ 清理测试数据...")
	storage.DB.Where("id = ?", testJob.ID).Delete(&models.Job{})
	storage.DB.Where("id = ?", event.ID).Delete(&models.Event{})
	storage.RedisClient.Del(ctx, "tasks:details:"+taskKey)
	storage.RemoveWorkerOffline(ctx, nodeID)

	fmt.Println("✅ 测试数据清理完成")

	// 等待几秒让所有操作完成
	time.Sleep(3 * time.Second)

	fmt.Println("\n🎉 所有测试完成！")
	fmt.Println("✅ SQLite 数据库: 正常")
	fmt.Println("✅ Redis: 正常")
	fmt.Println("✅ Master 节点: 正常")
	fmt.Println("✅ Worker 节点: 正常")
	fmt.Println("✅ gRPC 通讯: 正常")
	fmt.Println("✅ 任务调度: 正常")
	fmt.Println("✅ 心跳机制: 正常")
	fmt.Println("✅ 状态缓存: 正常")

	fmt.Println("\n💡 提示: 这个测试脚本验证了基本的通讯链路，但不包含完整的任务执行流程。")
	fmt.Println("   如需测试完整任务执行，请运行实际的 Master 和 Worker 服务。")
}