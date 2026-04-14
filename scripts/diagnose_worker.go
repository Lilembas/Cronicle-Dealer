package main

import (
	"fmt"
	"time"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/utils"
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

	// 查看当前节点
	var nodes []models.Node
	storage.DB.Find(&nodes)

	fmt.Printf("当前有 %d 个节点:\n", len(nodes))
	for _, node := range nodes {
		fmt.Printf("- ID: %s, Host: %s, IP: %s, Tags: %s, Status: %s, PID: %d\n",
			node.ID, node.Hostname, node.IP, node.Tags, node.Status, node.PID)
	}

	if len(nodes) == 0 {
		fmt.Println("\n没有节点，创建测试节点...")

		// 创建一个测试 Worker 节点
		nodeID := utils.GenerateID("node")
		testNode := &models.Node{
			ID:            nodeID,
			Hostname:      "ubuntu-developer-offline-host",
			IP:            "10.2.131.171",
			GRPCAddress:   "10.2.131.171:9090",
			Tags:          "[default]",
			PID:           1234,
			Status:        "offline",
			CPUCores:      4,
			CPUUsage:      10.5,
			MemoryTotal:   16.0,
			MemoryUsage:   8.0,
			MemoryPercent: 50.0,
			DiskTotal:     500.0,
			DiskUsage:     200.0,
			DiskPercent:   40.0,
			Version:       "0.1.0",
			RunningJobs:   0,
			MaxConcurrent: 10,
			LastHeartbeat: time.Now(),
		}

		if err := storage.DB.Create(testNode).Error; err != nil {
			fmt.Printf("创建测试节点失败: %v\n", err)
			return
		}
		fmt.Printf("✅ 创建测试节点成功，ID: %s\n", nodeID)

		// 现在测试更新
		fmt.Println("\n测试更新节点...")
		updates := map[string]interface{}{
			"grpc_address":   "10.2.131.171:9090",
			"tags":           "[default]",
			"pid":            int32(5678),
			"status":         "online",
			"cpu_cores":      4,
			"cpu_usage":      15.5,
			"memory_total":   16.0,
			"memory_usage":    9.0,
			"memory_percent":  56.25,
			"disk_total":     500.0,
			"disk_usage":     200.0,
			"disk_percent":   40.0,
			"version":        "0.1.0",
			"last_heartbeat": time.Now(),
		}

		fmt.Printf("更新内容: %+v\n", updates)
		if err := storage.DB.Model(&models.Node{}).Where("id = ?", nodeID).Updates(updates).Error; err != nil {
			fmt.Printf("❌ 更新失败: %v\n", err)
		} else {
			fmt.Printf("✅ 更新成功\n")
		}
	} else {
		// 测试更新第一个节点
		node := nodes[0]
		fmt.Printf("\n测试更新节点 %s...\n", node.ID)

		updates := map[string]interface{}{
			"grpc_address":   "10.2.131.171:9090",
			"tags":           "[default]",
			"pid":            int32(9999),
			"status":         "online",
			"cpu_cores":      4,
			"cpu_usage":      20.5,
			"memory_total":   16.0,
			"memory_usage":    10.0,
			"memory_percent":  62.5,
			"disk_total":     500.0,
			"disk_usage":     200.0,
			"disk_percent":   40.0,
			"version":        "0.1.0",
			"last_heartbeat": time.Now(),
		}

		fmt.Printf("更新内容: %+v\n", updates)
		if err := storage.DB.Model(&models.Node{}).Where("id = ?", node.ID).Updates(updates).Error; err != nil {
			fmt.Printf("❌ 更新失败: %v\n", err)
		} else {
			fmt.Printf("✅ 更新成功\n")
		}

		// 查看更新后的节点
		var updatedNode models.Node
		storage.DB.Where("id = ?", node.ID).First(&updatedNode)
		fmt.Printf("更新后节点状态: Status=%s, PID=%d\n", updatedNode.Status, updatedNode.PID)
	}
}
