package models

import "time"

// SystemConfig 系统配置模型（数据库覆盖配置文件）
type SystemConfig struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	UpdatedBy string    `gorm:"type:varchar(100)" json:"updated_by"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SystemConfig) TableName() string {
	return "system_configs"
}
