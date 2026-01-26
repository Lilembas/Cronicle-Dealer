package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
