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
	"github.com/cronicle/cronicle-next/pkg/logger"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	version    = "0.1.0"
)

func main() {
	flag.Parse()
	
	fmt.Printf("🚀 Cronicle-Next Master 节点 v%s\n", version)
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
	
	logger.Info("Master 节点启动中...",
		zap.String("version", version),
		zap.String("mode", cfg.Server.Mode))
	
	// 初始化数据库
	logger.Info("连接数据库...")
	if err := storage.InitDB(&cfg.Database); err != nil {
		logger.Fatal("数据库连接失败", zap.Error(err))
	}
	defer storage.CloseDB()
	
	// 自动迁移数据库
	logger.Info("执行数据库迁移...")
	if err := storage.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败", zap.Error(err))
	}
	
	// 初始化 Redis
	logger.Info("连接 Redis...")
	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 连接失败", zap.Error(err))
	}
	defer storage.CloseRedis()
	
	// TODO: 启动 Master 核心服务
	// - gRPC 服务器
	// - REST API 服务器
	// - WebSocket 服务器
	// - 调度引擎
	// - Master 选举（如果启用）
	
	// 启动 Master 选举
	election := master.NewElection(&cfg.Master.Election)
	if err := election.Start(); err != nil {
		logger.Fatal("Master 选举失败", zap.Error(err))
	}
	defer election.Stop()
	
	// 只有 Master 才启动核心服务
	if election.IsMaster() {
		logger.Info("当前节点为 Master，启动核心服务...")
		
		// 启动 gRPC 服务器
		grpcServer := master.NewGRPCServer(cfg)
		if err := grpcServer.Start(); err != nil {
			logger.Fatal("gRPC 服务器启动失败", zap.Error(err))
		}
		defer grpcServer.Stop()
		
		// 创建任务分发器
		dispatcher := master.NewDispatcher()
		defer dispatcher.Close()
		
		// 启动任务调度器
		scheduler := master.NewScheduler(&cfg.Master.Scheduler)
		if err := scheduler.Start(); err != nil {
			logger.Fatal("调度器启动失败", zap.Error(err))
		}
		defer scheduler.Stop()
		
		// 启动 REST API 服务器
		apiServer := master.NewAPIServer(cfg, scheduler, dispatcher)
		if err := apiServer.Start(); err != nil {
			logger.Fatal("API 服务器启动失败", zap.Error(err))
		}
		
		// TODO: 启动 WebSocket 服务器（日志流）
		
	} else {
		logger.Info("当前节点为 Backup，等待成为 Master...")
	}
	
	logger.Info("Master 节点启动成功",
		zap.Int("http_port", cfg.Server.HTTPPort),
		zap.Int("grpc_port", cfg.Server.GRPCPort))
	
	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	sig := <-sigChan
	logger.Info("收到退出信号，正在关闭...", zap.String("signal", sig.String()))
	
	// TODO: 优雅关闭
	// - 停止接受新任务
	// - 等待现有任务完成
	// - 关闭服务器
	
	logger.Info("Master 节点已关闭")
}
