package models

import (
	"time"
)

// Job 任务模型
type Job struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Category    string    `gorm:"type:varchar(100)" json:"category"`
	
	// 调度配置
	CronExpr    string    `gorm:"type:varchar(100);not null" json:"cron_expr"`
	Timezone    string    `gorm:"type:varchar(50);default:'UTC'" json:"timezone"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	
	// 执行配置
	TaskType    string    `gorm:"type:varchar(20);default:'shell'" json:"task_type"` // shell, http, docker
	Command     string    `gorm:"type:text;not null" json:"command"`
	WorkingDir  string    `gorm:"type:varchar(500)" json:"working_dir"`
	Env         string    `gorm:"type:text" json:"env"` // JSON 存储环境变量
	StrictMode  bool      `gorm:"default:false" json:"strict_mode"` // 严格模式：任何命令失败立即退出
	
	// 目标节点
	TargetType  string    `gorm:"type:varchar(20);default:'any'" json:"target_type"` // any, node_id, tags, group
	TargetValue string    `gorm:"type:varchar(255)" json:"target_value"`
	
	// 超时和重试
	Timeout     int       `gorm:"default:3600" json:"timeout"` // 秒
	MaxRetries  int       `gorm:"default:0" json:"max_retries"`
	RetryDelay  int       `gorm:"default:60" json:"retry_delay"` // 秒
	
	// 并发控制
	Concurrent  bool      `gorm:"default:false" json:"concurrent"` // 是否允许并发执行
	QueueMaxSize int      `gorm:"default:0" json:"queue_max_size"` // 队列最大长度，0 表示不限制
	
	// 通知配置
	NotifyOnSuccess bool   `gorm:"default:false" json:"notify_on_success"`
	NotifyOnFailure bool   `gorm:"default:true" json:"notify_on_failure"`
	NotifyWebhook   string `gorm:"type:varchar(500)" json:"notify_webhook"`
	
	// 元数据
	CreatedBy   string    `gorm:"type:varchar(100)" json:"created_by"`
	UpdatedBy   string    `gorm:"type:varchar(100)" json:"updated_by"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	
	// 统计信息
	LastRunTime *time.Time `json:"last_run_time"`
	NextRunTime *time.Time `json:"next_run_time"`
	TotalRuns   int64      `gorm:"default:0" json:"total_runs"`
	SuccessRuns int64      `gorm:"default:0" json:"success_runs"`
	FailedRuns  int64      `gorm:"default:0" json:"failed_runs"`
}

// TableName 表名
func (Job) TableName() string {
	return "jobs"
}
