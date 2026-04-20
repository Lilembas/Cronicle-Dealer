package models

import "time"

// LBMetric 负载均衡指标
type LBMetric struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Formula     string  `json:"formula"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Direction   string    `gorm:"type:varchar(10);default:'asc'" json:"direction"` // asc=优先最小值, desc=优先最大值
	Metrics     string    `gorm:"type:text" json:"metrics"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (LoadBalanceStrategy) TableName() string {
	return "load_balance_strategies"
}
