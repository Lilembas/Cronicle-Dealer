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

	fmt.Println("开始修复 pid 列...")

	// SQLite 不支持直接删除列，需要重建表
	// 1. 创建新表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS nodes_new (
		id VARCHAR(64) PRIMARY KEY,
		hostname VARCHAR(255) NOT NULL,
		ip VARCHAR(50) NOT NULL,
		g_rpc_address VARCHAR(255),
		tags VARCHAR(500),
		status VARCHAR(20) DEFAULT 'online',
		cpu_cores INTEGER,
		cpu_usage REAL,
		memory_total REAL,
		memory_usage REAL,
		memory_percent REAL,
		disk_total REAL,
		disk_usage REAL,
		disk_percent REAL,
		running_jobs INTEGER DEFAULT 0,
		max_concurrent INTEGER DEFAULT 10,
		version VARCHAR(50),
		last_heartbeat DATETIME,
		pid INTEGER DEFAULT 0,
		created_at DATETIME,
		registered_at DATETIME,
		updated_at DATETIME
	)`

	if err := storage.DB.Exec(createTableSQL).Error; err != nil {
		fmt.Printf("创建新表失败: %v\n", err)
		return
	}
	fmt.Println("✅ 创建新表成功")

	// 2. 复制数据
	copyDataSQL := `
	INSERT INTO nodes_new (id, hostname, ip, g_rpc_address, tags, status, cpu_cores, cpu_usage,
		memory_total, memory_usage, memory_percent, disk_total, disk_usage, disk_percent,
		running_jobs, max_concurrent, version, last_heartbeat, pid, created_at,
		registered_at, updated_at)
	SELECT id, hostname, ip, g_rpc_address, tags, status, cpu_cores, cpu_usage,
		memory_total, memory_usage, memory_percent, disk_total, disk_usage, disk_percent,
		running_jobs, max_concurrent, version, last_heartbeat,
		COALESCE(pid, p_id, 0) as pid, created_at, registered_at, updated_at
	FROM nodes`

	if err := storage.DB.Exec(copyDataSQL).Error; err != nil {
		fmt.Printf("复制数据失败: %v\n", err)
		return
	}
	fmt.Println("✅ 复制数据成功")

	// 3. 删除旧表
	if err := storage.DB.Exec("DROP TABLE nodes").Error; err != nil {
		fmt.Printf("删除旧表失败: %v\n", err)
		return
	}
	fmt.Println("✅ 删除旧表成功")

	// 4. 重命名新表
	if err := storage.DB.Exec("ALTER TABLE nodes_new RENAME TO nodes").Error; err != nil {
		fmt.Printf("重命名表失败: %v\n", err)
		return
	}
	fmt.Println("✅ 重命名表成功")

	fmt.Println("数据库修复完成！")
}
