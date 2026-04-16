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

var (
	// Redis日志过期时间
	logExpireTime = 15 * time.Minute
	// 日志文件目录（默认值，会被 InitLogStorage 更新）
	logDir = "./logs"
)

// 文件写入缓存（避免频繁打开关闭文件）
var (
	fileCache      = make(map[string]*os.File)
	fileCacheMutex sync.RWMutex
	fileWriteMutex sync.Mutex // 保护并发文件写入
)

// InitLogStorage 初始化日志存储
func InitLogStorage(dir string) error {
	if dir != "" {
		logDir = dir
	}
	// 创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	logger.Info("日志存储初始化成功", zap.String("log_dir", logDir))
	return nil
}

// SaveLogChunk 保存日志片段（Redis + 文件）
func SaveLogChunk(ctx context.Context, eventID, content string) error {
	logKey := fmt.Sprintf("task_logs:%s", eventID)

	// 1. 存储到Redis（动态延长TTL）
	if err := RedisClient.Append(ctx, logKey, content).Err(); err != nil {
		logger.Error("存储日志到Redis失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		// Redis失败不阻塞文件写入
	} else {
		// 动态延长TTL：每次写入都延长到当前时间+15分钟
		// 这样任务运行期间Redis会一直保持日志
		if err := RedisClient.Expire(ctx, logKey, logExpireTime).Err(); err != nil {
			logger.Warn("延长日志TTL失败",
				zap.String("event_id", eventID),
				zap.Error(err))
		}
	}

	// 2. 同步写入文件（保证持久化）
	if err := appendToFileSync(eventID, content); err != nil {
		logger.Error("写入日志文件失败",
			zap.String("event_id", eventID),
			zap.Error(err))
	}

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

// SetLogComplete 用完整日志覆盖写入（兜底，防止 StreamLogs 传输丢失）
func SetLogComplete(ctx context.Context, eventID string, content string) error {
	logKey := fmt.Sprintf("task_logs:%s", eventID)
	return RedisClient.Set(ctx, logKey, content, 0).Err()
}

// SetLogExpiration 设置日志过期时间（任务完成时调用）
func SetLogExpiration(ctx context.Context, eventID string) error {
	logKey := fmt.Sprintf("task_logs:%s", eventID)

	// 设置15分钟后过期
	if err := RedisClient.Expire(ctx, logKey, logExpireTime).Err(); err != nil {
		logger.Error("设置日志过期时间失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		return err
	}

	logger.Info("设置日志过期时间",
		zap.String("event_id", eventID),
		zap.Duration("ttl", logExpireTime))

	return nil
}

// appendToFileAsync 异步写入文件
// 注意：此函数已废弃，保留是为了兼容性
// 实际使用 appendToFileSync 进行同步写入
func appendToFileAsync(eventID, content string) {
	appendToFileSync(eventID, content)
}

// appendToFileSync 同步写入文件并flush到磁盘
func appendToFileSync(eventID, content string) error {
	fileWriteMutex.Lock()
	defer fileWriteMutex.Unlock()

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
			return err
		}
		setCachedFile(logFilePath, file)
	}

	// 写入内容
	if _, err := file.WriteString(content); err != nil {
		logger.Error("写入日志文件失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		return err
	}

	// 立即flush到磁盘（关键：保证数据持久化）
	if err := file.Sync(); err != nil {
		logger.Error("刷新日志文件到磁盘失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		return err
	}

	return nil
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
