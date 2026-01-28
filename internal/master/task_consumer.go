package master

import (
	"context"
	"time"

	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"go.uber.org/zap"
)

// TaskConsumer 任务消费者
type TaskConsumer struct {
	dispatcher *Dispatcher
}

// NewTaskConsumer 创建任务消费者
func NewTaskConsumer(dispatcher *Dispatcher) *TaskConsumer {
	return &TaskConsumer{
		dispatcher: dispatcher,
	}
}

// Start 启动任务消费者
func (tc *TaskConsumer) Start() {
	logger.Info("启动任务消费者...")

	for {
		// 从 Redis 队列获取任务
		taskKey, err := storage.GetTaskFromQueue(context.Background(), 30*time.Second)
		if err != nil {
			logger.Error("从 Redis 队列获取任务失败", zap.Error(err))
			continue
		}

		if taskKey == "" {
			// 超时，继续循环
			continue
		}

		logger.Info("从队列获取任务", zap.String("task_key", taskKey))

		// 解析任务详情
		taskData, err := storage.GetTaskDetails(context.Background(), taskKey)
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

		// 分发任务
		if err := tc.dispatcher.DispatchEvent(event); err != nil {
			logger.Error("分发任务失败", zap.Error(err))
			// TODO: 处理分发失败的情况（重试、记录等）
		}
	}
}