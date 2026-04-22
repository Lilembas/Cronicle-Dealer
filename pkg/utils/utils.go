package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

// GenerateID 生成唯一 ID
func GenerateID(prefix string) string {
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)
	
	if prefix != "" {
		return fmt.Sprintf("%s_%d_%s", prefix, timestamp, randomStr)
	}
	return fmt.Sprintf("%d_%s", timestamp, randomStr)
}

// Contains 检查字符串切片是否包含指定元素
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Unique 字符串切片去重
func Unique(slice []string) []string {
	keys := make(map[string]bool)
	result := []string{}
	
	for _, item := range slice {
		if _, exists := keys[item]; !exists {
			keys[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// BoolValue 获取布尔指针的值，如果为 nil 则返回 false
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// BoolPtr 创建布尔指针
func BoolPtr(b bool) *bool {
	return &b
}

// GetLocalIP 获取本机非回环 IPv4 地址
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return "127.0.0.1"
}
