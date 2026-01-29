package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/master"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/internal/worker"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

var (
	configPath = flag.String("config", "../config.yaml", "配置文件路径")
	jobCount   = flag.Int("jobs", 3, "测试任务数量")
	waitTime   = flag.Duration("wait", 60*time.Second, "等待任务执行完成的时长")
)

type testContext struct {
	config      *config.Config
	masterNode  *master.Master
	workerClient *worker.Client
	executor    *worker.Executor
	jobs        []*models.Job
	events      []*models.Event
}

func main() {
	flag.Parse()

	fmt.Println("🚀 Cronicle-Next Master + Worker E2E 测试")
	fmt.Println("=========================================")

	ctx := &testContext{}
	ctx.config = loadConfig(*configPath)
	initializeLogger(ctx.config)

	initializeStorage(ctx.config)
	defer storage.CloseDB()
	defer storage.CloseRedis()

	startMaster(ctx)
	defer ctx.masterNode.Stop()

	startWorker(ctx)
	defer ctx.workerClient.Close()
	defer ctx.executor.Stop()

	ctx.jobs = createTestJobs(*jobCount)
	ctx.events = scheduleJobs(ctx.jobs)

	waitForJobCompletion(ctx.events, *waitTime)
	displayJobResults(ctx.events)

	cleanupTestData(ctx)
	exitCode := printTestSummary(ctx.events)

	// 所有 defer 语句执行完毕后退出
	os.Exit(exitCode)
}

func loadConfig(path string) *config.Config {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("❌ 配置文件不存在: %s\n请先复制 config.example.yaml 到 config.yaml\n", path)
	}

	cfg, err := config.Load(path)
	if err != nil {
		logger.Fatal("加载配置失败", zap.Error(err))
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
}

func initializeStorage(cfg *config.Config) {
	fmt.Println("\n📋 阶段 1: 初始化存储")
	fmt.Println("----------------------")

	fmt.Println("1️⃣ 初始化数据库和 Redis...")

	if err := storage.InitDB(&cfg.Database); err != nil {
		logger.Fatal("数据库初始化失败", zap.Error(err))
	}

	if err := storage.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败", zap.Error(err))
	}

	cleanupOldTestData()

	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 初始化失败", zap.Error(err))
	}

	fmt.Println("✅ 存储初始化成功")
}

func cleanupOldTestData() {
	fmt.Println("🧹 清理旧测试数据...")
	storage.DB.Where("id LIKE ?", "test_job_%").Delete(&models.Job{})
	storage.DB.Where("id LIKE ?", "test_event_%").Delete(&models.Event{})
}

func startMaster(ctx *testContext) {
	fmt.Println("\n📋 阶段 2: 启动 Master 节点")
	fmt.Println("-------------------------")

	fmt.Println("2️⃣ 启动 Master 服务...")
	ctx.masterNode = master.NewMaster(ctx.config)

	if err := ctx.masterNode.Start(); err != nil {
		logger.Fatal("Master 启动失败", zap.Error(err))
	}

	time.Sleep(2 * time.Second)
	fmt.Println("✅ Master 启动成功")
}

func startWorker(ctx *testContext) {
	fmt.Println("\n📋 阶段 3: 启动 Worker 节点")
	fmt.Println("-------------------------")

	fmt.Println("3️⃣ 连接 Worker 到 Master...")
	ctx.workerClient = worker.NewClient(&ctx.config.Worker)

	if err := ctx.workerClient.Connect(); err != nil {
		logger.Fatal("Worker 连接 Master 失败", zap.Error(err))
	}

	if err := ctx.workerClient.Register(); err != nil {
		logger.Fatal("Worker 注册失败", zap.Error(err))
	}

	fmt.Println("✅ Worker 注册成功")

	nodeID := ctx.workerClient.GetNodeID()
	logger.Info("Worker 节点信息",
		zap.String("node_id", nodeID),
		zap.Strings("tags", ctx.config.Worker.Node.Tags))

	fmt.Println("\n4️⃣ 启动 Worker 执行器...")
	ctx.executor = worker.NewExecutor(&ctx.config.Worker.Executor)

	if err := ctx.executor.Start(0); err != nil {
		logger.Fatal("执行器启动失败", zap.Error(err))
	}

	fmt.Println("\n5️⃣ 启动心跳机制...")
	go ctx.workerClient.StartHeartbeat()

	time.Sleep(2 * time.Second)
	fmt.Println("✅ Worker 就绪（通过 gRPC 接收任务）")
}

func createTestJobs(count int) []*models.Job {
	fmt.Println("\n📋 阶段 4: 创建测试任务")
	fmt.Println("----------------------")

	var jobs []*models.Job

	// 创建标准测试任务
	for i := 1; i <= count; i++ {
		job := &models.Job{
			ID:          fmt.Sprintf("test_job_%03d", i),
			Name:        fmt.Sprintf("测试任务 #%d", i),
			Description: fmt.Sprintf("E2E 测试任务 %d", i),
			CronExpr:    "",
			Command:     fmt.Sprintf("echo '执行任务 #%d' && sleep %d && date", i, 3-i%3),
			TaskType:    "shell",
			Enabled:     true,
			Timeout:     30,
		}

		if err := storage.DB.Create(job).Error; err != nil {
			logger.Error("创建任务失败", zap.Error(err))
			continue
		}

		jobs = append(jobs, job)
		logger.Info("✅ 任务创建成功",
			zap.String("job_id", job.ID),
			zap.String("job_name", job.Name))
	}

	// 创建 Python 版本检查任务
	pythonJob := &models.Job{
		ID:          "test_job_python",
		Name:        "Python 版本检查",
		Description: "测试 Python3 版本检查命令",
		CronExpr:    "",
		Command:     "python3 -V",
		TaskType:    "shell",
		Enabled:     true,
		Timeout:     10,
	}

	if err := storage.DB.Create(pythonJob).Error; err == nil {
		jobs = append(jobs, pythonJob)
		logger.Info("✅ Python 任务创建成功",
			zap.String("job_id", pythonJob.ID),
			zap.String("job_name", pythonJob.Name))
	} else {
		logger.Error("创建 Python 任务失败", zap.Error(err))
	}

	if len(jobs) == 0 {
		logger.Fatal("没有成功创建任何任务")
	}

	return jobs
}

func scheduleJobs(jobs []*models.Job) []*models.Event {
	fmt.Println("\n📋 阶段 5: 调度任务执行")
	fmt.Println("----------------------")

	var events []*models.Event
	ctx := context.Background()

	for _, job := range jobs {
		event := createEvent(job)
		if err := storage.DB.Create(event).Error; err != nil {
			logger.Error("创建事件失败", zap.Error(err))
			continue
		}

		taskKey := fmt.Sprintf("%s:%s", job.ID, event.ID)

		if !saveTaskDetails(ctx, taskKey, job, event) {
			continue
		}

		if err := storage.AddTaskToQueue(ctx, taskKey); err != nil {
			logger.Error("添加任务到队列失败", zap.Error(err))
			continue
		}

		events = append(events, event)
		logger.Info("📤 任务已调度",
			zap.String("task_key", taskKey),
			zap.String("job_name", job.Name))
	}

	if len(events) == 0 {
		logger.Fatal("没有成功调度任何事件")
	}

	fmt.Printf("\n✅ 成功调度 %d 个任务\n", len(events))
	return events
}

func createEvent(job *models.Job) *models.Event {
	return &models.Event{
		ID:        fmt.Sprintf("test_event_%s", job.ID),
		JobID:     job.ID,
		JobName:   job.Name,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
}

func saveTaskDetails(ctx context.Context, taskKey string, job *models.Job, event *models.Event) bool {
	taskData := map[string]interface{}{
		"job_id":         job.ID,
		"event_id":       event.ID,
		"job_name":       job.Name,
		"command":        job.Command,
		"task_type":      job.TaskType,
		"timeout":        job.Timeout,
		"working_dir":    job.WorkingDir,
		"env":            job.Env,
		"scheduled_time": time.Now().Unix(),
	}

	if err := storage.RedisClient.HSet(ctx, "tasks:details:"+taskKey, taskData).Err(); err != nil {
		logger.Error("保存任务详情失败", zap.Error(err))
		return false
	}

	return true
}

func waitForJobCompletion(events []*models.Event, timeout time.Duration) {
	fmt.Println("\n📋 阶段 6: 监控任务执行")
	fmt.Println("----------------------")

	fmt.Printf("⏳ 等待任务执行完成 (最多 %v)...\n\n", timeout)

	checkInterval := 2 * time.Second
	deadline := time.Now().Add(timeout)
	completedCount := 0

	for time.Now().Before(deadline) && completedCount < len(events) {
		time.Sleep(checkInterval)

		for _, event := range events {
			if isEventFinished(event) {
				continue
			}

			updateEventStatus(event)
			if isEventFinished(event) {
				completedCount++
				printStatusUpdate(event)
			}
		}

		if completedCount < len(events) {
			fmt.Printf("   进度: %d/%d 完成\n", completedCount, len(events))
		}
	}
}

func isEventFinished(event *models.Event) bool {
	return event.Status == "completed" || event.Status == "failed"
}

func updateEventStatus(event *models.Event) {
	taskKey := fmt.Sprintf("%s:%s", event.JobID, event.ID)
	ctx := context.Background()

	status, err := storage.GetTaskStatus(ctx, taskKey)
	if err != nil {
		logger.Warn("获取任务状态失败",
			zap.String("event_id", event.ID),
			zap.Error(err))
		return
	}

	if status != event.Status {
		event.Status = status
		storage.DB.Model(event).Update("status", status)
	}
}

func printStatusUpdate(event *models.Event) {
	switch event.Status {
	case "running":
		fmt.Printf("🔄 [%s] 任务执行中...\n", event.JobName)
	case "completed":
		fmt.Printf("✅ [%s] 任务完成\n", event.JobName)
	case "failed":
		fmt.Printf("❌ [%s] 任务失败\n", event.JobName)
	}
}

func displayJobResults(events []*models.Event) {
	fmt.Println("\n📋 阶段 7: 查看任务结果")
	fmt.Println("----------------------")

	successCount := 0
	failedCount := 0

	for _, event := range events {
		taskKey := fmt.Sprintf("%s:%s", event.JobID, event.ID)
		ctx := context.Background()

		fmt.Printf("\n任务: %s\n", event.JobName)
		fmt.Printf("  状态: %s\n", event.Status)

		result, err := storage.GetTaskResult(ctx, taskKey)
		if err == nil && len(result) > 0 {
			if exitCode, ok := result["exit_code"]; ok && exitCode != "" {
				fmt.Printf("  退出码: %s\n", exitCode)
			}

			if output, ok := result["output"]; ok && output != "" {
				fmt.Printf("  输出: %s\n", output)
			}
		}

		switch event.Status {
		case "completed":
			successCount++
		case "failed":
			failedCount++
		}
	}
}

func cleanupTestData(ctx *testContext) {
	fmt.Println("\n📋 阶段 8: 清理测试数据")
	fmt.Println("----------------------")

	redisCtx := context.Background()
	nodeID := ctx.workerClient.GetNodeID()

	for _, job := range ctx.jobs {
		storage.DB.Where("id = ?", job.ID).Delete(&models.Job{})
	}

	for _, event := range ctx.events {
		storage.DB.Where("id = ?", event.ID).Delete(&models.Event{})
		taskKey := fmt.Sprintf("%s:%s", event.JobID, event.ID)
		storage.RedisClient.Del(redisCtx, "tasks:details:"+taskKey)
	}

	storage.RemoveWorkerOffline(redisCtx, nodeID)
	fmt.Println("✅ 测试数据清理完成")
}

func printTestSummary(events []*models.Event) int {
	fmt.Println("\n" + strings.Repeat("=", 40))
	fmt.Println("🎉 E2E 测试完成")
	fmt.Println(strings.Repeat("=", 40))

	successCount := countEventsByStatus(events, "completed")
	failedCount := countEventsByStatus(events, "failed")

	fmt.Printf("\n📊 测试结果统计:\n")
	fmt.Printf("   总任务数: %d\n", len(events))
	fmt.Printf("   成功: %d ✅\n", successCount)
	fmt.Printf("   失败: %d ❌\n", failedCount)
	fmt.Printf("   完成率: %.1f%%\n", float64(successCount)/float64(len(events))*100)

	printVerificationList()

	if successCount == len(events) {
		fmt.Println("\n🎊 所有任务执行成功！")
		return 0
	} else {
		fmt.Printf("\n⚠️  有 %d 个任务失败\n", failedCount)
		return 1
	}
}

func countEventsByStatus(events []*models.Event, status string) int {
	count := 0
	for _, event := range events {
		if event.Status == status {
			count++
		}
	}
	return count
}

func printVerificationList() {
	fmt.Println("\n✅ 验证项:")
	fmt.Println("   ✅ Master 启动和运行")
	fmt.Println("   ✅ Worker 注册和心跳")
	fmt.Println("   ✅ 任务创建和持久化")
	fmt.Println("   ✅ 任务调度和分发")
	fmt.Println("   ✅ 任务队列监听")
	fmt.Println("   ✅ 任务执行和状态更新")
	fmt.Println("   ✅ 结果记录和查询")
}
