#!/bin/bash

echo "🔍 Redis 连接测试"
echo "================="

REDIS_HOST="localhost:6379"
REDIS_PASSWORD="6677095"

echo "Host: $REDIS_HOST"
echo "Password: $REDIS_PASSWORD"
echo ""

# 创建临时 Go 程序测试
cat > /tmp/redis_check.go << 'GOEOF'
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	host := os.Args[1]
	password := os.Args[2]

	client := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pong, err := client.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("❌ Redis 连接失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Redis 连接成功: %s\n", pong)

	// 获取 Redis 版本
	info, err := client.Info(ctx, "server").Result()
	if err == nil {
		fmt.Printf("📊 Redis 信息:\n")
		for i, line := range fmt.Sprintf("%s", info) {
			if i > 200 { break }
			fmt.Printf("%c", line)
		}
	}

	client.Close()
}
GOEOF

/usr/local/go/bin/go run /tmp/redis_check.go "$REDIS_HOST" "$REDIS_PASSWORD"

echo ""
echo "✅ 测试完成"
