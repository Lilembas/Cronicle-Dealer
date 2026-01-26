package master

import (
	"fmt"
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
	jobs   sync.Map // map[string]*models.Job 存储任务配置
	entries sync.Map // map[string]cron.EntryID 存储 cron 条目 ID
}

// NewScheduler 创建调度器
func NewScheduler(cfg *config.SchedulerConfig) *Scheduler {
	return &Scheduler{
		cfg: cfg,
		cron: cron.New(cron.WithSeconds()), // 支持秒级调度
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	if !s.cfg.Enabled {
		logger.Info("调度器已禁用")
		return nil
	}
	
	logger.Info("启动调度器...")
	
	// 从数据库加载所有启用的任务
	if err := s.LoadJobs(); err != nil {
		return fmt.Errorf("加载任务失败: %w", err)
	}
	
	// 启动 cron
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
	// 解析 Cron 表达式
	entryID, err := s.cron.AddFunc(job.CronExpr, func() {
		s.triggerJob(job.ID)
	})
	
	if err != nil {
		return fmt.Errorf("解析 Cron 表达式失败: %w", err)
	}
	
	// 存储任务和条目 ID
	s.jobs.Store(job.ID, job)
	s.entries.Store(job.ID, entryID)
	
	// 更新下次执行时间
	entry := s.cron.Entry(entryID)
	nextRunTime := entry.Next
	
	storage.DB.Model(&models.Job{}).Where("id = ?", job.ID).Update("next_run_time", nextRunTime)
	
	logger.Info("添加任务到调度器",
		zap.String("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.String("cron_expr", job.CronExpr),
		zap.Time("next_run", nextRunTime))
	
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
	// 先移除旧的
	s.RemoveJob(job.ID)
	
	// 如果任务已启用，重新添加
	if job.Enabled {
		return s.AddJob(job)
	}
	
	return nil
}

// triggerJob 触发任务执行
func (s *Scheduler) triggerJob(jobID string) {
	logger.Info("触发任务", zap.String("job_id", jobID))
	
	// 获取任务配置
	jobVal, ok := s.jobs.Load(jobID)
	if !ok {
		logger.Error("任务不存在", zap.String("job_id", jobID))
		return
	}
	
	job := jobVal.(*models.Job)
	
	// 创建任务执行记录
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
	
	// 更新任务统计
	now := time.Now()
	storage.DB.Model(&models.Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"last_run_time": now,
		"total_runs":    storage.DB.Raw("total_runs + 1"),
	})
	
	// TODO: 分发任务到 Worker
	// 这里需要调用任务分发器
	logger.Info("任务已创建，等待分发",
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
