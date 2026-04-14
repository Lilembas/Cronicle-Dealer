package master

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/utils"
)

// Scheduler 任务调度器
type Scheduler struct {
	cfg    *config.SchedulerConfig
	cron   *cron.Cron
	jobs   sync.Map // map[string]*models.Job
	entries sync.Map // map[string]cron.EntryID
}

// NewScheduler 创建调度器
func NewScheduler(cfg *config.SchedulerConfig) *Scheduler {
	return &Scheduler{
		cfg:  cfg,
		cron: cron.New(cron.WithSeconds()),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	if !s.cfg.Enabled {
		logger.Info("调度器已禁用")
		return nil
	}

	logger.Info("启动调度器...")

	if err := s.LoadJobs(); err != nil {
		return fmt.Errorf("加载任务失败: %w", err)
	}

	s.cron.Start()

	logger.Info("调度器启动成功")
	return nil
}

// LoadJobs 从数据库加载任务
func (s *Scheduler) LoadJobs() error {
	var jobs []models.Job
	if err := storage.DB.Where("enabled = ?", true).Find(&jobs).Error; err != nil {
		return err
	}

	logger.Info("加载任务", zap.Int("count", len(jobs)))

	for _, job := range jobs {
		if err := s.AddJob(&job); err != nil {
			logger.Error("添加任务到调度器失败",
				zap.String("job_id", job.ID),
				zap.String("job_name", job.Name),
				zap.Error(err))
		}
	}

	return nil
}

// AddJob 添加任务到调度器
func (s *Scheduler) AddJob(job *models.Job) error {
	expr := job.CronExpr
	// 如果是 5 位表达式，自动在前面加 0 变为 6 位（因为使用了 WithSeconds）
	if fields := strings.Fields(expr); len(fields) == 5 {
		expr = "0 " + expr
	}

	entryID, err := s.cron.AddFunc(expr, func() {
		s.triggerJob(job.ID)
	})
	if err != nil {
		return fmt.Errorf("解析 Cron 表达式失败: %w", err)
	}

	s.jobs.Store(job.ID, job)
	s.entries.Store(job.ID, entryID)

	s.updateNextRunTime(job.ID, entryID)

	logger.Info("添加任务到调度器",
		zap.String("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.String("cron_expr", job.CronExpr))

	return nil
}

// RemoveJob 从调度器移除任务
func (s *Scheduler) RemoveJob(jobID string) {
	if entryVal, ok := s.entries.Load(jobID); ok {
		entryID := entryVal.(cron.EntryID)
		s.cron.Remove(entryID)
		s.jobs.Delete(jobID)
		s.entries.Delete(jobID)
		logger.Info("从调度器移除任务", zap.String("job_id", jobID))
	}
}

// UpdateJob 更新任务
func (s *Scheduler) UpdateJob(job *models.Job) error {
	s.RemoveJob(job.ID)
	if job.Enabled {
		return s.AddJob(job)
	}
	return nil
}

// triggerJob 触发任务执行
func (s *Scheduler) triggerJob(jobID string) {
	logger.Info("触发任务", zap.String("job_id", jobID))

	jobVal, ok := s.jobs.Load(jobID)
	if !ok {
		logger.Error("任务不存在", zap.String("job_id", jobID))
		return
	}

	job := jobVal.(*models.Job)

	event := &models.Event{
		ID:            utils.GenerateID("event"),
		JobID:         job.ID,
		JobName:       job.Name,
		Status:        "pending",
		ScheduledTime: time.Now(),
	}

	if err := storage.DB.Create(event).Error; err != nil {
		logger.Error("创建任务执行记录失败",
			zap.String("job_id", jobID),
			zap.Error(err))
		return
	}

	s.updateJobStats(jobID)
	s.enqueueTask(job, event)

	logger.Info("任务已添加到 Redis 队列，等待 Worker 消费",
		zap.String("event_id", event.ID),
		zap.String("job_name", job.Name))
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if s.cron != nil {
		logger.Info("停止调度器...")
		ctx := s.cron.Stop()
		<-ctx.Done()
		logger.Info("调度器已停止")
	}
}

// GetNextRunTime 获取任务下次执行时间
func (s *Scheduler) GetNextRunTime(jobID string) *time.Time {
	if entryVal, ok := s.entries.Load(jobID); ok {
		entryID := entryVal.(cron.EntryID)
		entry := s.cron.Entry(entryID)
		next := entry.Next
		return &next
	}
	return nil
}

// updateNextRunTime 更新任务的下次执行时间
func (s *Scheduler) updateNextRunTime(jobID string, entryID cron.EntryID) {
	entry := s.cron.Entry(entryID)
	storage.DB.Model(&models.Job{}).Where("id = ?", jobID).
		Update("next_run_time", entry.Next)
}

// updateJobStats 更新任务统计信息
func (s *Scheduler) updateJobStats(jobID string) {
	now := time.Now()
	storage.DB.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"last_run_time": now,
		"total_runs":    storage.DB.Raw("total_runs + 1"),
	})
}

// enqueueTask 将任务添加到队列
func (s *Scheduler) enqueueTask(job *models.Job, event *models.Event) {
	ctx := context.Background()
	taskKey := fmt.Sprintf("%s:%s", job.ID, event.ID)

	taskData := map[string]interface{}{
		"job_id":          job.ID,
		"event_id":        event.ID,
		"job_name":        job.Name,
		"command":         job.Command,
		"task_type":       job.TaskType,
		"timeout":         job.Timeout,
		"working_dir":     job.WorkingDir,
		"env":             job.Env,
		"target_type":     job.TargetType,
		"target_value":    job.TargetValue,
		"strict_mode":     fmt.Sprintf("%v", job.StrictMode),
		"scheduled_time":  event.ScheduledTime.Unix(),
	}

	if err := storage.RedisClient.HSet(ctx, "tasks:details:"+taskKey, taskData).Err(); err != nil {
		logger.Error("存储任务详情到 Redis 失败",
			zap.String("task_key", taskKey),
			zap.Error(err))
		return
	}

	if err := storage.AddTaskToQueue(ctx, taskKey); err != nil {
		logger.Error("添加任务到就绪队列失败",
			zap.String("task_key", taskKey),
			zap.Error(err))
	}
}
