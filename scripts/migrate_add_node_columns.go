package main

import (
	"fmt"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		return
	}

	// 初始化日志
	if err := logger.InitLogger(&cfg.Logging); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		return
	}
	defer logger.Sync()

	// 连接数据库
	if err := storage.InitDB(&cfg.Database); err != nil {
		fmt.Printf("数据库连接失败: %v\n", err)
		return
	}
	defer storage.CloseDB()

	fmt.Println("开始数据库迁移...")

	// 检查并添加 pid 列
	// SQLite 不支持直接检查列是否存在，我们尝试添加，如果失败就跳过
	pidSQL := `ALTER TABLE nodes ADD COLUMN pid INTEGER DEFAULT 0`
	if err := storage.DB.Exec(pidSQL).Error; err != nil {
		fmt.Printf("✓ pid 列可能已存在或添加失败: %v\n", err)
	} else {
		fmt.Println("✅ 成功添加 pid 列")
	}

	// 检查并添加 created_at 列
	// SQLite 不支持非常量默认值，所以先添加列，然后更新数据
	createdSQL := `ALTER TABLE nodes ADD COLUMN created_at DATETIME`
	if err := storage.DB.Exec(createdSQL).Error; err != nil {
		fmt.Printf("✓ created_at 列可能已存在或添加失败: %v\n", err)
	} else {
		fmt.Println("✅ 成功添加 created_at 列")

		// 为现有记录设置 created_at
		if err := storage.DB.Exec(`UPDATE nodes SET created_at = registered_at WHERE created_at IS NULL`).Error; err != nil {
			fmt.Printf("更新 created_at 值失败: %v\n", err)
		}
	}

	fmt.Println("数据库迁移完成")
}


