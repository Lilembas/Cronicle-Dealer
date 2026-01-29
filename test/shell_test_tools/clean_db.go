package main

import (
	"fmt"
	"log"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
)

func main() {
	fmt.Println("🧹 清理数据库中的旧Worker节点...")

	// 加载配置
	cfg, err := config.Load("../../../config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v\n", err)
	}

	// 初始化数据库
	if err := storage.InitDB(&cfg.Database); err != nil {
		log.Fatalf("数据库初始化失败: %v\n", err)
	}
	defer storage.CloseDB()

	// 查看当前节点
	var nodes []models.Node
	if err := storage.DB.Find(&nodes).Error; err != nil {
		fmt.Printf("查询节点失败: %v\n", err)
	} else {
		fmt.Printf("\n📋 当前数据库中的节点 (%d 个):\n", len(nodes))
		for _, node := range nodes {
			fmt.Printf("  - ID: %s\n", node.ID)
			fmt.Printf("    主机名: %s\n", node.Hostname)
			fmt.Printf("    IP: %s\n", node.IP)
			fmt.Printf("    状态: %s\n", node.Status)
			fmt.Printf("    最后心跳: %s\n", node.LastHeartbeat)
			fmt.Printf("    注册时间: %s\n", node.RegisteredAt)
			fmt.Println()
		}
	}

	// 删除所有节点
	result := storage.DB.Where("1 = 1").Delete(&models.Node{})
	if result.Error != nil {
		fmt.Printf("❌ 删除节点失败: %v\n", result.Error)
		return
	}

	fmt.Printf("✅ 已删除 %d 个节点记录\n", result.RowsAffected)

	// 删除所有事件
	var eventCount int64
	storage.DB.Model(&models.Event{}).Count(&eventCount)
	if eventCount > 0 {
		result = storage.DB.Where("1 = 1").Delete(&models.Event{})
		if result.Error != nil {
			fmt.Printf("❌ 删除事件失败: %v\n", result.Error)
			return
		}
		fmt.Printf("✅ 已删除 %d 个事件记录\n", result.RowsAffected)
	}

	fmt.Println("\n✅ 数据库清理完成！")
	fmt.Println("💡 现在可以重启Worker节点，它会作为新节点注册。")
}
