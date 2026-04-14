package main

import (
	"fmt"

	"github.com/cronicle/cronicle-next/internal/config"
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

	// 删除所有节点记录
	result := storage.DB.Exec("DELETE FROM nodes")
	if result.Error != nil {
		fmt.Printf("删除节点失败: %v\n", result.Error)
		return
	}

	fmt.Printf("✅ 已删除 %d 个节点记录\n", result.RowsAffected)
	fmt.Println("请重启 Master 和 Worker 让它们重新注册")
}
