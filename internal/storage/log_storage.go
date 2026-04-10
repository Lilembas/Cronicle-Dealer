package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cronicle/cronicle-next/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Redis日志过期时间
	logExpireTime = 15 * time.Minute
	// 日志文件目录
	logDir = "/var/log/cronicle/events"
)

// 文件写入缓存（避免频繁打开关闭文件）
var (
	fileCache      = make(map[string]*os.File)
	fileCacheMutex sync.RWMutex
)

// InitLogStorage 初始化日志存储
func InitLogStorage() error {
	// 创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	logger.Info("日志存储初始化成功", zap.String("log_dir", logDir))
	return nil
}

// SaveLogChunk 保存日志片段（Redis + 文件）
func SaveLogChunk(ctx context.Context, eventID, content string) error {
	// 1. 存储到Redis（快速缓存，15分钟过期）
	logKey := fmt.Sprintf("task_logs:%s", eventID)
	if err := RedisClient.Append(ctx, logKey, content).Err(); err != nil {
		logger.Error("存储日志到Redis失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		return err
	}

	// 设置过期时间
	if err := RedisClient.Expire(ctx, logKey, logExpireTime).Err(); err != nil {
		logger.Warn("设置日志过期时间失败", zap.Error(err))
	}

	// 2. 异步写入文件（持久化）
	go appendToFileAsync(eventID, content)

	return nil
}

// GetLogs 获取日志（优先Redis，回退文件）
func GetLogs(ctx context.Context, eventID string) (string, error) {
	logKey := fmt.Sprintf("task_logs:%s", eventID)

	// 1. 先尝试从Redis获取（15分钟内的日志）
	logs, err := RedisClient.Get(ctx, logKey).Result()
	if err == nil && logs != "" {
		logger.Debug("从Redis获取日志",
			zap.String("event_id", eventID),
			zap.Int("length", len(logs)))
		return logs, nil
	}

	// 2. Redis没有，从文件读取
	logFilePath := getLogFilePath(eventID)
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("日志不存在: event_id=%s", eventID)
	}

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	logger.Debug("从文件获取日志",
		zap.String("event_id", eventID),
		zap.Int("length", len(content)))

	return string(content), nil
}

// appendToFileAsync 异步写入文件
func appendToFileAsync(eventID, content string) {
	logFilePath := getLogFilePath(eventID)

	// 从缓存获取或打开文件
	file := getCachedFile(logFilePath)
	if file == nil {
		var err error
		file, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.Error("打开日志文件失败",
				zap.String("event_id", eventID),
				zap.String("path", logFilePath),
				zap.Error(err))
			return
		}
		setCachedFile(logFilePath, file)
	}

	// 写入内容
	if _, err := file.WriteString(content); err != nil {
		logger.Error("写入日志文件失败",
			zap.String("event_id", eventID),
			zap.Error(err))
	}
}

// getLogFilePath 获取日志文件路径
func getLogFilePath(eventID string) string {
	return filepath.Join(logDir, fmt.Sprintf("%s.log", eventID))
}

// getCachedFile 从缓存获取文件句柄
func getCachedFile(path string) *os.File {
	fileCacheMutex.RLock()
	defer fileCacheMutex.RUnlock()
	return fileCache[path]
}

// setCachedFile 缓存文件句柄
func setCachedFile(path string, file *os.File) {
	fileCacheMutex.Lock()
	defer fileCacheMutex.Unlock()
	fileCache[path] = file
}

// CloseAllLogFiles 关闭所有缓存的文件句柄
func CloseAllLogFiles() error {
	fileCacheMutex.Lock()
	defer fileCacheMutex.Unlock()

	var lastErr error
	for path, file := range fileCache {
		if err := file.Close(); err != nil {
			logger.Error("关闭日志文件失败",
				zap.String("path", path),
				zap.Error(err))
			lastErr = err
		}
	}

	// 清空缓存
	fileCache = make(map[string]*os.File)

	return lastErr
}

// CleanupOldLogs 清理旧日志文件（定期调用）
func CleanupOldLogs(days int) error {
	cutoffTime := time.Now().AddDate(0, 0, -days)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("读取日志目录失败: %w", err)
	}

	cleanedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 删除过期文件
		if info.ModTime().Before(cutoffTime) {
			path := filepath.Join(logDir, entry.Name())
			if err := os.Remove(path); err != nil {
				logger.Warn("删除旧日志文件失败",
					zap.String("path", path),
					zap.Error(err))
			} else {
				cleanedCount++
			}
		}
	}

	logger.Info("清理旧日志完成",
		zap.Int("days", days),
		zap.Int("cleaned_count", cleanedCount))

	return nil
}
