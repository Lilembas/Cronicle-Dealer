package models

import (
	"time"
)

// NodeMetric 节点负载历史数据模型，用于图表展示
type NodeMetric struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	NodeID        string    `gorm:"index;type:varchar(64)" json:"node_id"`
	CPUUsage      float64   `json:"cpu_usage"`
	CPUCores      int       `json:"cpu_cores"`
	MemoryPercent float64   `json:"memory_percent"`
	MemoryTotal   float64   `json:"memory_total"` // GB
	MemoryUsage   float64   `json:"memory_usage"` // GB
	DiskPercent   float64   `json:"disk_percent"`
	DiskTotal     float64   `json:"disk_total"`   // GB
	DiskUsage     float64   `json:"disk_usage"`   // GB
	RunningJobs   int       `json:"running_jobs"`
	Timestamp     time.Time `gorm:"index" json:"timestamp"`
}

// TableName 表名
func (NodeMetric) TableName() string {
	return "node_metrics"
}
