package storage

import (
	"context"
	"fmt"
	"time"
	
	"github.com/go-redis/redis/v8"
	"github.com/cronicle/cronicle-next/internal/config"
)

// RedisClient 全局 Redis 客户端
var RedisClient *redis.Client

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.RedisConfig) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.Address(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	
	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis 连接测试失败: %w", err)
	}
	
	return nil
}

// CloseRedis 关闭 Redis 连接
func CloseRedis() error {
	if RedisClient == nil {
		return nil
	}
	return RedisClient.Close()
}

// ========== 分布式锁相关 ==========

// AcquireLock 获取分布式锁
func AcquireLock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return RedisClient.SetNX(ctx, key, "locked", expiration).Result()
}

// ReleaseLock 释放分布式锁
func ReleaseLock(ctx context.Context, key string) error {
	return RedisClient.Del(ctx, key).Err()
}

// RenewLock 续期分布式锁
func RenewLock(ctx context.Context, key string, expiration time.Duration) error {
	return RedisClient.Expire(ctx, key, expiration).Err()
}
