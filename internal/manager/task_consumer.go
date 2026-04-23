package manager

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/cronicle/cronicle-dealer/internal/config"
	"github.com/cronicle/cronicle-dealer/internal/models"
	"github.com/cronicle/cronicle-dealer/internal/storage"
	"github.com/cronicle/cronicle-dealer/pkg/logger"
	"go.uber.org/zap"
)

const (
	taskQueuePollTimeout = 5 * time.Second
)

// TaskConsumer 任务消费者
type TaskConsumer struct {
	dispatcher  *Dispatcher
	wsServer    *WebSocketServer
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

// SetWebSocketServer 设置 WebSocket 服务器（用于广播状态变化）
func (tc *TaskConsumer) SetWebSocketServer(wsServer *WebSocketServer) {
	tc.wsServer = wsServer
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
		// 设置调度时间
		if scheduledTime, err := strconv.ParseInt(taskData["scheduled_time"], 10, 64); err == nil && scheduledTime > 0 {
			event.ScheduledTime = time.Unix(scheduledTime, 0)
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

		// 重试前重置事件状态为 pending，避免 DispatchEvent 的状态检查短路
		if event.ID != "" {
			storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).
				Updates(map[string]interface{}{
					"status":        eventStatusPending,
					"error_message": "",
				})
		}

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

		logger.Warn("调度失败，将进行重试",
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
	// 从数据库查询事件的实际开始时间（CreatedAt 是事件创建时间，即首次调度开始时间）
	var dbEvent models.Event
	if err := storage.DB.Select("created_at, scheduled_time").Where("id = ?", event.ID).First(&dbEvent).Error; err == nil {
		// 使用 CreatedAt 作为开始时间，它代表事件首次进入调度的时间
		startTime := dbEvent.CreatedAt
		if !dbEvent.ScheduledTime.IsZero() && dbEvent.ScheduledTime.After(dbEvent.CreatedAt) {
			// 如果 ScheduledTime 更合理（晚于 CreatedAt），使用它
			startTime = dbEvent.ScheduledTime
		}
		duration := int64(now.Sub(startTime).Seconds())
		updateErr := storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).Updates(map[string]interface{}{
			"status":        eventStatusFailed,
			"end_time":      now,
			"start_time":    startTime,
			"duration":      duration,
			"exit_code":     1,
			"error_message": fmt.Sprintf("任务分发失败（已重试%d次后放弃）: %v", retryCount, dispatchErr),
		}).Error
	if updateErr != nil {
		logger.Error("更新分发失败事件状态失败",
			zap.String("event_id", event.ID),
			zap.String("job_id", event.JobID),
			zap.Error(updateErr))
	}
		}

		// 通过 WebSocket 广播任务状态变更
		if tc.wsServer != nil {
			if err := tc.wsServer.BroadcastTaskStatus(event.ID, event.JobID, eventStatusFailed, event.NodeID, event.NodeName, 1); err != nil {
				logger.Warn("广播任务失败状态失败",
					zap.String("event_id", event.ID),
					zap.Error(err))
			}
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
