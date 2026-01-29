package models

import (
	"time"
)

// Node Worker 节点模型
type Node struct {
	ID       string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Hostname string `gorm:"type:varchar(255);not null" json:"hostname"`
	IP       string `gorm:"type:varchar(50);not null" json:"ip"`

	// gRPC 执行器地址
	GRPCAddress string `gorm:"type:varchar(255)" json:"grpc_address"` // Worker executor gRPC 服务地址

	// 节点标签
	Tags     string `gorm:"type:varchar(500)" json:"tags"` // JSON 存储标签数组
	
	// 状态
	Status   string `gorm:"type:varchar(20);default:'online'" json:"status"` // online, offline, busy
	
	// 资源信息
	CPUCores      int     `json:"cpu_cores"`
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryTotal   float64 `json:"memory_total"`   // GB
	MemoryUsage   float64 `json:"memory_usage"`   // GB
	MemoryPercent float64 `json:"memory_percent"` // 百分比
	DiskTotal     float64 `json:"disk_total"`     // GB
	DiskUsage     float64 `json:"disk_usage"`     // GB
	DiskPercent   float64 `json:"disk_percent"`   // 百分比
	
	// 任务信息
	RunningJobs   int `json:"running_jobs"`   // 当前运行任务数
	MaxConcurrent int `gorm:"default:10" json:"max_concurrent"` // 最大并发任务数
	
	// 版本信息
	Version      string `gorm:"type:varchar(50)" json:"version"`
	
	// 心跳信息
	LastHeartbeat time.Time `json:"last_heartbeat"`
	
	// 元数据
	RegisteredAt time.Time `gorm:"autoCreateTime" json:"registered_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 表名
func (Node) TableName() string {
	return "nodes"
}

// IsOnline 判断节点是否在线
func (n *Node) IsOnline(timeout time.Duration) bool {
	if n.Status != "online" {
		return false
	}
	return time.Since(n.LastHeartbeat) < timeout
}

// CanAcceptJob 判断节点是否可以接受新任务
func (n *Node) CanAcceptJob() bool {
	return n.Status == "online" && n.RunningJobs < n.MaxConcurrent
}
