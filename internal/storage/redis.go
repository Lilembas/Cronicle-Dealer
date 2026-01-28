package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/cronicle/cronicle-next/internal/config"
)

// RedisClient 全局 Redis 客户端
var RedisClient *redis.Client

const (
	// Key prefixes
	keyPrefixWorkersOnline = "workers:online:"
	keyPrefixTasksDetails = "tasks:details:"
	keyPrefixTasksStatus = "tasks:status:"
	keyPrefixTasksResult = "tasks:result:"

	// Queue names
	queueTasksReady = "tasks:ready"
	queueTasksScheduled = "tasks:scheduled"

	// Default values
	defaultPingTimeout = 5 * time.Second
	defaultWorkerExpire = 60 * time.Second
)

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.RedisConfig) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.Address(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), defaultPingTimeout)
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

// ========== 分布式锁 ==========

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

// ========== 任务队列 ==========

// AddTaskToQueue 将任务添加到就绪队列
func AddTaskToQueue(ctx context.Context, taskID string) error {
	return RedisClient.RPush(ctx, queueTasksReady, taskID).Err()
}

// GetTaskFromQueue 从就绪队列获取任务（阻塞）
func GetTaskFromQueue(ctx context.Context, timeout time.Duration) (string, error) {
	result, err := RedisClient.BRPop(ctx, timeout, queueTasksReady).Result()
	if err != nil {
		return "", err
	}
	if len(result) < 2 {
		return "", fmt.Errorf("invalid BRPop result")
	}
	return result[1], nil
}

// AddTaskToScheduled 将任务添加到调度队列（ZSET）
func AddTaskToScheduled(ctx context.Context, taskID string, nextRunTime time.Time) error {
	score := float64(nextRunTime.Unix())
	return RedisClient.ZAdd(ctx, queueTasksScheduled, &redis.Z{
		Score:  score,
		Member: taskID,
	}).Err()
}

// GetDueTasks 获取到期的任务
func GetDueTasks(ctx context.Context, now time.Time) ([]string, error) {
	maxScore := float64(now.Unix())
	taskIDs, err := RedisClient.ZRangeByScore(ctx, queueTasksScheduled, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", maxScore),
	}).Result()
	if err != nil {
		return nil, err
	}

	if len(taskIDs) > 0 {
		if _, err = RedisClient.ZRem(ctx, queueTasksScheduled, taskIDs).Result(); err != nil {
			return nil, err
		}
		for _, taskID := range taskIDs {
			if err := AddTaskToQueue(ctx, taskID); err != nil {
				return nil, err
			}
		}
	}

	return taskIDs, nil
}

// RemoveTaskFromScheduled 从调度队列中移除任务
func RemoveTaskFromScheduled(ctx context.Context, taskID string) error {
	return RedisClient.ZRem(ctx, queueTasksScheduled, taskID).Err()
}

// ========== 任务状态 ==========

// SetTaskStatus 设置任务状态
func SetTaskStatus(ctx context.Context, taskKey string, status string) error {
	return RedisClient.HSet(ctx, keyPrefixTasksStatus+taskKey, "status", status).Err()
}

// GetTaskStatus 获取任务状态
func GetTaskStatus(ctx context.Context, taskKey string) (string, error) {
	return RedisClient.HGet(ctx, keyPrefixTasksStatus+taskKey, "status").Result()
}

// SetTaskResult 设置任务结果
func SetTaskResult(ctx context.Context, taskKey string, result map[string]interface{}) error {
	return RedisClient.HSet(ctx, keyPrefixTasksResult+taskKey, result).Err()
}

// GetTaskResult 获取任务结果
func GetTaskResult(ctx context.Context, taskKey string) (map[string]string, error) {
	return RedisClient.HGetAll(ctx, keyPrefixTasksResult+taskKey).Result()
}

// GetTaskDetails 获取任务详情
func GetTaskDetails(ctx context.Context, taskKey string) (map[string]string, error) {
	return RedisClient.HGetAll(ctx, keyPrefixTasksDetails+taskKey).Result()
}

// ========== Worker 注册与发现 ==========

// RegisterWorker 注册 Worker 节点
func RegisterWorker(ctx context.Context, workerID string, data map[string]interface{}) error {
	key := keyPrefixWorkersOnline + workerID
	if err := RedisClient.HSet(ctx, key, data).Err(); err != nil {
		return err
	}
	return RedisClient.Expire(ctx, key, defaultWorkerExpire).Err()
}

// GetOnlineWorkers 获取所有在线 Worker
func GetOnlineWorkers(ctx context.Context) ([]string, error) {
	keys, err := RedisClient.Keys(ctx, keyPrefixWorkersOnline+"*").Result()
	if err != nil {
		return nil, err
	}

	workerIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		if id := extractWorkerID(key); id != "" {
			workerIDs = append(workerIDs, id)
		}
	}

	return workerIDs, nil
}

// IsWorkerOnline 检查 Worker 是否在线
func IsWorkerOnline(ctx context.Context, workerID string) (bool, error) {
	key := keyPrefixWorkersOnline + workerID
	exists, err := RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// RemoveWorkerOffline 标记 Worker 离线
func RemoveWorkerOffline(ctx context.Context, workerID string) error {
	key := keyPrefixWorkersOnline + workerID
	return RedisClient.Del(ctx, key).Err()
}

// extractWorkerID 从 key 中提取 workerID
func extractWorkerID(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}
