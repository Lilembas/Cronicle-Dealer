package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cronicle/cronicle-next/internal/config"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	version    = "0.1.0"
)

func main() {
	flag.Parse()

	fmt.Printf("🚀 Cronicle-Next Master 节点 v%s\n", version)
	fmt.Printf("📄 加载配置文件: %s\n", *configPath)

	// 加载配置
	configer, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(configer.Master.Heartbeat.Timeout)
}
