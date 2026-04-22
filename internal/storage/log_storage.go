package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cronicle/cronicle-next/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Redis Pub/Sub 频道名
	logPubSubChannel = "cronicle:logs"
	// 日志消息分隔符
	logMessageSep = "\t"
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

	// 1. 存储到Redis（不设置TTL，由Manager在任务完成后统一管理）
	if err := RedisClient.Append(ctx, logKey, content).Err(); err != nil {
		logger.Error("存储日志到Redis失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		// Redis失败不阻塞文件写入
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

// SetLogComplete 用完整日志覆盖写入 Redis（任务完成时保证日志完整）
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

// ScanOrphanLogs 扫描 Redis 中所有无过期时间的 task_logs key（孤儿日志）
// 返回 eventID 列表
func ScanOrphanLogs(ctx context.Context) ([]string, error) {
	var orphanIDs []string
	var cursor uint64

	for {
		keys, nextCursor, err := RedisClient.Scan(ctx, cursor, "task_logs:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("扫描 task_logs 失败: %w", err)
		}
		cursor = nextCursor

		for _, key := range keys {
			// 提取 eventID（key 格式: task_logs:{eventID}）
			eventID := strings.TrimPrefix(key, "task_logs:")
			if eventID == "" {
				continue
			}
			// TTL=-1 表示永不过期（孤儿日志）
			ttl, err := RedisClient.TTL(ctx, key).Result()
			if err != nil {
				logger.Warn("获取日志TTL失败", zap.String("key", key), zap.Error(err))
				continue
			}
			if ttl < 0 { // -1 = no TTL, -2 = key not exist
				orphanIDs = append(orphanIDs, eventID)
			}
		}

		if cursor == 0 {
			break
		}
	}

	return orphanIDs, nil
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

// SaveLogToFile 用完整内容覆盖写入日志文件（Manager下载全量日志时使用）
func SaveLogToFile(eventID, content string) error {
	logFilePath := getLogFilePath(eventID)

	// 先关闭已缓存的文件句柄（如果有），避免写入冲突
	CloseLogHandle(eventID)

	fileWriteMutex.Lock()
	defer fileWriteMutex.Unlock()

	if err := os.WriteFile(logFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入日志文件失败: %w", err)
	}

	return nil
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

// PublishLog 通过 Redis Pub/Sub 发布日志（供 Manager 实时推送前端）
func PublishLog(ctx context.Context, eventID, content string) {
	msg := eventID + logMessageSep + content
	if err := RedisClient.Publish(ctx, logPubSubChannel, msg).Err(); err != nil {
		logger.Warn("发布日志到Pub/Sub失败",
			zap.String("event_id", eventID),
			zap.Error(err))
	}
}

// SubscribeLog 订阅 Redis Pub/Sub 日志频道
// 返回消息 channel（格式 "eventID\tcontent"）和取消函数
func SubscribeLog(ctx context.Context) (<-chan string, func()) {
	msgChan := make(chan string, 100)
	sub := RedisClient.Subscribe(ctx, logPubSubChannel)

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			sub.Unsubscribe(ctx, logPubSubChannel)
			sub.Close()
			close(msgChan)
		})
	}

	go func() {
		defer cancel()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				select {
				case msgChan <- msg.Payload:
				default:
					// channel 满了丢弃，避免阻塞
					logger.Warn("日志订阅channel已满，丢弃消息")
				}
			}
		}
	}()

	return msgChan, cancel
}

// CloseLogHandle 关闭指定 eventID 的日志文件句柄
func CloseLogHandle(eventID string) {
	closeCachedFile(getLogFilePath(eventID))
}

// ParseLogMessage 解析 Pub/Sub 日志消息，返回 eventID 和 content
func ParseLogMessage(msg string) (eventID, content string) {
	idx := strings.Index(msg, logMessageSep)
	if idx < 0 {
		return msg, ""
	}
	return msg[:idx], msg[idx+len(logMessageSep):]
}

// CleanupOldLogs 清理过期日志文件
func CleanupOldLogs(days int) error {
	if days <= 0 {
		return nil
	}

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

		if info.ModTime().Before(cutoffTime) {
			path := filepath.Join(logDir, entry.Name())
			// 先关闭可能缓存的文件句柄
			closeCachedFile(path)
			if err := os.Remove(path); err != nil {
				logger.Warn("删除旧日志文件失败",
					zap.String("path", path),
					zap.Error(err))
			} else {
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		logger.Info("清理过期日志完成",
			zap.Int("retention_days", days),
			zap.Int("cleaned_count", cleanedCount))
	}

	return nil
}

// TruncateOverSizeLogs 截断超过大小限制的日志文件（保留尾部内容）
func TruncateOverSizeLogs(maxSizeMB int) error {
	if maxSizeMB <= 0 {
		return nil
	}

	maxBytes := int64(maxSizeMB) * 1024 * 1024

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("读取日志目录失败: %w", err)
	}

	truncatedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Size() <= maxBytes {
			continue
		}

		path := filepath.Join(logDir, entry.Name())
		// 先关闭可能缓存的文件句柄
		closeCachedFile(path)

		// 保留文件尾部 maxSizeMB 的内容
		if err := truncateFileTail(path, maxBytes); err != nil {
			logger.Warn("截断日志文件失败",
				zap.String("path", path),
				zap.Int64("size", info.Size()),
				zap.Error(err))
		} else {
			truncatedCount++
		}
	}

	if truncatedCount > 0 {
		logger.Info("截断超大日志完成",
			zap.Int("max_size_mb", maxSizeMB),
			zap.Int("truncated_count", truncatedCount))
	}

	return nil
}

// closeCachedFile 关闭并移除指定路径的缓存文件句柄
func closeCachedFile(path string) {
	fileCacheMutex.Lock()
	defer fileCacheMutex.Unlock()

	if file, ok := fileCache[path]; ok {
		file.Close()
		delete(fileCache, path)
	}
}

// truncateFileTail 截断文件，只保留尾部 maxBytes 字节
func truncateFileTail(path string, maxBytes int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() <= maxBytes {
		return nil
	}

	if _, err := f.Seek(fi.Size()-maxBytes, io.SeekStart); err != nil {
		return err
	}

	// 跳到第一个换行符，避免截断在行中间
	buf := make([]byte, 4096)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			if idx := bytes.IndexByte(buf[:n], '\n'); idx >= 0 {
				f.Seek(fi.Size()-maxBytes+int64(idx)+1, io.SeekStart)
				break
			}
		}
		if readErr != nil {
			break
		}
	}

	tmpPath := path + ".tmp"
	tf, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(tf, f); err != nil {
		tf.Close()
		os.Remove(tmpPath)
		return err
	}
	tf.Close()

	return os.Rename(tmpPath, path)
}
