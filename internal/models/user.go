package models

import (
	"time"
	"golang.org/x/crypto/bcrypt"
)

// User 用户模型
type User struct {
	ID       string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Username string `gorm:"type:varchar(100);uniqueIndex;not null" json:"username"`
	Password string `gorm:"type:varchar(255);not null" json:"-"` // 不在 JSON 中返回密码
	Email    string `gorm:"type:varchar(255)" json:"email"`
	
	// 角色权限
	Role     string `gorm:"type:varchar(20);default:'user'" json:"role"` // admin, user, viewer
	Active   *bool  `gorm:"default:true" json:"active"`
	
	// 个人信息
	FullName string `gorm:"type:varchar(255)" json:"full_name"`
	Avatar   string `gorm:"type:varchar(500)" json:"avatar"`
	
	// 元数据
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

// TableName 表名
func (User) TableName() string {
	return "users"
}

// SetPassword 设置密码（bcrypt 加密）
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword 验证密码
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// IsAdmin 判断是否为管理员
func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}
