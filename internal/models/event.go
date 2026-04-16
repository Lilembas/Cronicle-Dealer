package models

import (
	"time"
)

// Event 任务执行记录模型
type Event struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	JobID     string    `gorm:"type:varchar(64);not null;index" json:"job_id"`
	JobName   string    `gorm:"type:varchar(255)" json:"job_name"`
	
	// 执行信息
	NodeID    string    `gorm:"type:varchar(64)" json:"node_id"`
	NodeName  string    `gorm:"type:varchar(255)" json:"node_name"`
	
	// 状态
	Status    string    `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending, running, success, failed, aborted, timeout
	
	// 时间
	ScheduledTime time.Time  `json:"scheduled_time"` // 计划执行时间
	StartTime     *time.Time `json:"start_time"`     // 实际开始时间
	EndTime       *time.Time `json:"end_time"`       // 结束时间
	Duration      int64      `json:"duration"`       // 执行时长（秒）
	
	// 执行结果
	ExitCode     int    `json:"exit_code"`
	ErrorMessage string `gorm:"type:text" json:"error_message"`
	
	// 日志
	LogPath      string `gorm:"type:varchar(500)" json:"log_path"`
	LogSize      int64  `json:"log_size"` // 字节
	
	// 资源使用
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryBytes  int64   `json:"memory_bytes"`
	
	// 重试信息
	RetryCount   int    `gorm:"default:0" json:"retry_count"`
	IsRetry      bool   `gorm:"default:false" json:"is_retry"`
	ParentEventID string `gorm:"type:varchar(64)" json:"parent_event_id"`
	
	// 元数据
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// JOIN 字段（非数据库列）
	JobCategory  string    `gorm:"-" json:"job_category,omitempty"`
}

// TableName 表名
func (Event) TableName() string {
	return "events"
}

// IsCompleted 判断任务是否已完成
func (e *Event) IsCompleted() bool {
	return e.Status == "success" || e.Status == "failed" || e.Status == "aborted" || e.Status == "timeout"
}

// IsRunning 判断任务是否正在运行
func (e *Event) IsRunning() bool {
	return e.Status == "running"
}
