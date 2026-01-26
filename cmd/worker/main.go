package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	
	"go.uber.org/zap"
	
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	version    = "0.1.0"
)

func main() {
	flag.Parse()
	
	fmt.Printf("⚙️  Cronicle-Next Worker 节点 v%s\n", version)
	fmt.Printf("📄 加载配置文件: %s\n", *configPath)
	
	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}
	
	// 初始化日志
	if err := logger.InitLogger(&cfg.Logging); err != nil {
		fmt.Printf("❌ 初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	
	logger.Info("Worker 节点启动中...",
		zap.String("version", version),
		zap.String("master", cfg.Worker.MasterAddress))
	
	// 创建 Worker 客户端
	client := worker.NewClient(&cfg.Worker)
	
	// 连接到 Master
	if err := client.Connect(); err != nil {
		logger.Fatal("连接 Master 失败", zap.Error(err))
	}
	defer client.Close()
	
	// 注册节点
	if err := client.Register(); err != nil {
		logger.Fatal("注册节点失败", zap.Error(err))
	}
	
	// 启动任务执行器
	executor := worker.NewExecutor(&cfg.Worker.Executor)
	if err := executor.Start(9090); err != nil {
		logger.Fatal("启动执行器失败", zap.Error(err))
	}
	defer executor.Stop()
	
	// 启动心跳（阻塞）
	go client.StartHeartbeat()
	
	// TODO: 启动资源监控
	
	logger.Info("Worker 节点启动成功",
		zap.String("master_address", cfg.Worker.MasterAddress),
		zap.Strings("tags", cfg.Worker.Node.Tags))
	
	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	sig := <-sigChan
	logger.Info("收到退出信号，正在关闭...", zap.String("signal", sig.String()))
	
	// TODO: 优雅关闭
	// - 停止接受新任务
	// - 等待现有任务完成
	// - 注销节点
	// - 断开连接
	
	logger.Info("Worker 节点已关闭")
}
