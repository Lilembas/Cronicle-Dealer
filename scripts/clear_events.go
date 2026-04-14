package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"gorm.io/gorm"
)

func main() {
	// 初始化数据库
	if err := storage.InitializeDatabase("sqlite", "/codespace/developers/linnan/claudeProjects/cronicle-next/cronicle.db"); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 确认操作
	fmt.Print("⚠️  警告：此操作将删除所有历史执行记录（events 表）\n")
	fmt.Print("是否继续？(yes/no): ")

	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" && confirm != "y" {
		fmt.Println("❌ 操作已取消")
		os.Exit(0)
	}

	// 开始事务
	tx := storage.DB.Begin()
	if tx.Error != nil {
		log.Fatalf("开始事务失败: %v", tx.Error)
	}

	// 查询要删除的记录数
	var count int64
	if err := tx.Model(&models.Event{}).Count(&count).Error; err != nil {
		log.Fatalf("查询记录数失败: %v", err)
	}

	fmt.Printf("📊 找到 %d 条历史记录\n", count)

	if count == 0 {
		fmt.Println("✅ 没有需要删除的记录")
		tx.Rollback()
		os.Exit(0)
	}

	// 删除所有记录
	if err := tx.Exec("DELETE FROM events").Error; err != nil {
		log.Fatalf("删除记录失败: %v", err)
		tx.Rollback()
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Fatalf("提交事务失败: %v", err)
	}

	fmt.Printf("✅ 成功删除 %d 条历史记录\n", count)

	// 清理日志文件
	fmt.Println("\n🧹 清理日志文件...")
	cleanupLogFiles()
}

func cleanupLogFiles() {
	logDir := "/var/log/cronicle/events"

	// 检查目录是否存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		fmt.Printf("⚠️  日志目录不存在: %s\n", logDir)
		return
	}

	// 读取目录内容
	files, err := os.ReadDir(logDir)
	if err != nil {
		fmt.Printf("⚠️  读取日志目录失败: %v\n", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("✅ 日志目录为空")
		return
	}

	// 删除所有日志文件
	deletedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := fmt.Sprintf("%s/%s", logDir, file.Name())
		if err := os.Remove(filePath); err != nil {
			fmt.Printf("⚠️  删除文件失败: %s - %v\n", file.Name(), err)
		} else {
			deletedCount++
		}
	}

	fmt.Printf("✅ 成功删除 %d 个日志文件\n", deletedCount)
}
