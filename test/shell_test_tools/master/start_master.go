package main

import (
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/master"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

func main() {
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

	// 初始化存储
	fmt.Println("🔧 初始化存储...")
	if err := storage.InitDB(&cfg.Database); err != nil {
		logger.Fatal("数据库初始化失败", zap.Error(err))
	}
	defer storage.CloseDB()

	if err := storage.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败", zap.Error(err))
	}

	if err := storage.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 初始化失败", zap.Error(err))
	}
	defer storage.CloseRedis()

	fmt.Println("✅ 存储初始化成功")

	// 将所有Worker节点状态设置为offline（Master重启时重置）
	fmt.Println("\n🔄 重置Worker节点状态...")
	storage.DB.Model(&models.Node{}).Where("status = ?", "online").Update("status", "offline")
	fmt.Println("✅ 已将所有在线Worker标记为离线")

	// 启动Master
	fmt.Println("\n🚀 启动 Master 节点...")
	fmt.Println("======================")

	masterNode := master.NewMaster(cfg)
	if err := masterNode.Start(); err != nil {
		logger.Fatal("Master 启动失败", zap.Error(err))
	}
	defer masterNode.Stop()

	// 等待Master完全启动
	time.Sleep(2 * time.Second)

	fmt.Println("\n========================================")
	fmt.Println("✅ Master 节点启动成功！")
	fmt.Println("========================================")
	fmt.Printf("📡 gRPC 地址: %s:%d\n", cfg.Server.Host, cfg.Server.GRPCPort)
	fmt.Printf("🌐 API 地址: %s:%d\n", cfg.Server.Host, cfg.Server.HTTPPort)
	fmt.Println("========================================\n")
	fmt.Println("📝 按 Ctrl+C 停止服务")

	// 保持运行
	select {}
}
