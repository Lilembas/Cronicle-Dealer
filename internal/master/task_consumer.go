package master

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"go.uber.org/zap"
)

const (
	taskQueuePollTimeout = 5 * time.Second
)

// TaskConsumer 任务消费者
type TaskConsumer struct {
	dispatcher  *Dispatcher
	retryCfg    config.DispatchRetryConfig
	done        chan struct{}
}

// NewTaskConsumer 创建任务消费者
func NewTaskConsumer(dispatcher *Dispatcher, retryCfg config.DispatchRetryConfig) *TaskConsumer {
	return &TaskConsumer{
		dispatcher: dispatcher,
		retryCfg:   retryCfg,
		done:       make(chan struct{}),
	}
}

// Start 启动任务消费者
func (tc *TaskConsumer) Start(ctx context.Context) {
	logger.Info("启动任务消费者...")
	defer close(tc.done)

	for {
		if ctx.Err() != nil {
			logger.Info("任务消费者停止")
			return
		}

		// 从 Redis 队列获取任务
		taskKey, err := storage.GetTaskFromQueue(ctx, taskQueuePollTimeout)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				// 队列超时无任务，继续轮询
				continue
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if ctx.Err() != nil {
					logger.Info("任务消费者收到退出信号")
					return
				}
				continue
			}
			logger.Error("从 Redis 队列获取任务失败", zap.Error(err))
			continue
		}

		if taskKey == "" {
			// 超时，继续循环
			continue
		}

		logger.Info("从队列获取任务", zap.String("task_key", taskKey))

		// 解析任务详情
		taskData, err := storage.GetTaskDetails(ctx, taskKey)
		if err != nil {
			logger.Error("获取任务详情失败", zap.Error(err))
			continue
		}

		if len(taskData) == 0 {
			logger.Warn("任务详情不存在", zap.String("task_key", taskKey))
			continue
		}

		// 创建事件对象
		event := &models.Event{
			ID:      taskData["event_id"],
			JobID:   taskData["job_id"],
			JobName: taskData["job_name"],
			Status:  "pending",
		}

		// 分发任务（传递 taskDetails 以支持 ad-hoc 任务）
		if err := tc.dispatcher.DispatchEvent(event, taskData); err != nil {
			tc.handleDispatchFailure(ctx, taskKey, taskData, event, err)
		}
	}
}

// Wait 等待消费者退出
func (tc *TaskConsumer) Wait(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-tc.done:
		return true
	case <-timer.C:
		return false
	}
}

func (tc *TaskConsumer) handleDispatchFailure(ctx context.Context, taskKey string, taskData map[string]string, event *models.Event, dispatchErr error) {
	retryCount := parseRetryCount(taskData["dispatch_retry_count"])
	maxRetries := tc.retryCfg.MaxRetries
	baseDelay := time.Duration(tc.retryCfg.BaseDelaySec) * time.Second
	maxDelay := time.Duration(tc.retryCfg.MaxDelaySec) * time.Second

	if retryCount < maxRetries {
		nextRetry := retryCount + 1
		delay := baseDelay * time.Duration(1<<(nextRetry-1))
		if delay > maxDelay {
			delay = maxDelay
		}

		detailsKey := fmt.Sprintf("tasks:details:%s", taskKey)
		_ = storage.RedisClient.HSet(ctx, detailsKey, "dispatch_retry_count", strconv.Itoa(nextRetry)).Err()

		go func() {
			timer := time.NewTimer(delay)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}

			if err := storage.AddTaskToQueue(context.Background(), taskKey); err != nil {
				logger.Error("重试任务重新入队失败",
					zap.String("task_key", taskKey),
					zap.String("job_id", event.JobID),
					zap.String("event_id", event.ID),
					zap.Int("retry", nextRetry),
					zap.Error(err))
			}
		}()

		logger.Warn("任务分发失败，稍后重试",
			zap.String("task_key", taskKey),
			zap.String("job_id", event.JobID),
			zap.String("event_id", event.ID),
			zap.Int("retry", nextRetry),
			zap.Int("max_retries", maxRetries),
			zap.Duration("delay", delay),
			zap.Error(dispatchErr))
		return
	}

	now := time.Now()
	updateErr := storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).Updates(map[string]interface{}{
		"status":        eventStatusFailed,
		"end_time":      &now,
		"error_message": fmt.Sprintf("任务分发失败（重试%d次后放弃）: %v", retryCount, dispatchErr),
	}).Error
	if updateErr != nil {
		logger.Error("更新分发失败事件状态失败",
			zap.String("event_id", event.ID),
			zap.String("job_id", event.JobID),
			zap.Error(updateErr))
	}

	logger.Error("任务分发失败，达到最大重试次数",
		zap.String("task_key", taskKey),
		zap.String("job_id", event.JobID),
		zap.String("event_id", event.ID),
		zap.Int("retry", retryCount),
		zap.Int("max_retries", maxRetries),
		zap.Error(dispatchErr))
}

func parseRetryCount(value string) int {
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
