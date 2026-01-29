package main

import (
	"fmt"
	"log"

	"github.com/cronicle/cronicle-next/internal/config"
)

func main() {
	fmt.Println("📋 测试配置加载")
	fmt.Println("================")

	cfg, err := config.Load("../../../config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v\n", err)
	}

	fmt.Printf("Server.Host: %s\n", cfg.Server.Host)
	fmt.Printf("Server.GRPCPort: %d\n", cfg.Server.GRPCPort)
	fmt.Printf("Worker.MasterAddress: %s\n", cfg.Worker.MasterAddress)
	fmt.Printf("Worker.Executor.GRPCPort: %d\n", cfg.Worker.Executor.GRPCPort)
	fmt.Printf("Worker.Node.Tags: %v\n", cfg.Worker.Node.Tags)

	fmt.Println("\n✅ 配置加载完成")
}
