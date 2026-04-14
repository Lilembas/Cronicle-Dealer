package main

import (
	"fmt"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		return
	}

	if err := logger.InitLogger(&cfg.Logging); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		return
	}
	defer logger.Sync()

	if err := storage.InitDB(&cfg.Database); err != nil {
		fmt.Printf("数据库连接失败: %v\n", err)
		return
	}
	defer storage.CloseDB()

	var nodes []models.Node
	if err := storage.DB.Find(&nodes).Error; err != nil {
		fmt.Printf("查询节点失败: %v\n", err)
		return
	}

	fmt.Printf("当前有 %d 个节点:\n", len(nodes))
	for i, node := range nodes {
		fmt.Printf("\n[%d] ID: %s\n", i+1, node.ID)
		fmt.Printf("    Hostname: %s\n", node.Hostname)
		fmt.Printf("    IP: %s\n", node.IP)
		fmt.Printf("    Tags: %s\n", node.Tags)
		fmt.Printf("    Status: %s\n", node.Status)
		fmt.Printf("    PID: %d\n", node.PID)
		fmt.Printf("    RegisteredAt: %s\n", node.RegisteredAt)
		fmt.Printf("    CreatedAt: %s\n", node.CreatedAt)
	}
}
