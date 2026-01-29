package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cronicle/cronicle-next/internal/config"
)

func main() {
	// 支持命令行参数指定配置文件路径
	configPath := "../../../config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	fmt.Println("📋 测试配置加载")
	fmt.Println("================")

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v\n", err)
	}

	// 服务器配置
	fmt.Println("\n🖥️  服务器配置:")
	fmt.Printf("   Host: %s\n", cfg.Server.Host)
	fmt.Printf("   gRPC端口: %d\n", cfg.Server.GRPCPort)
	fmt.Printf("   HTTP端口: %d\n", cfg.Server.HTTPPort)

	// Worker配置
	fmt.Println("\n👷 Worker配置:")
	fmt.Printf("   Master地址: %s\n", cfg.Worker.MasterAddress)
	fmt.Printf("   Executor gRPC端口: %d\n", cfg.Worker.Executor.GRPCPort)
	fmt.Printf("   标签: %v\n", cfg.Worker.Node.Tags)

	// 日志配置
	fmt.Println("\n📝 日志配置:")
	fmt.Printf("   级别: %s\n", cfg.Logging.Level)
	fmt.Printf("   格式: %s\n", cfg.Logging.Format)

	fmt.Println("\n✅ 配置加载完成")
}
