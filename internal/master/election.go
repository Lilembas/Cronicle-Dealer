package master

import (
	"context"
	"fmt"
	"time"
	
	"go.uber.org/zap"
	
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

const (
	masterLockKey = "cronicle:master:lock"
	masterIDKey   = "cronicle:master:id"
)

// Election Master 选举管理器
type Election struct {
	cfg       *config.ElectionConfig
	masterID  string
	isMaster  bool
	cancelCtx context.Context
	cancelFn  context.CancelFunc
}

// NewElection 创建选举管理器
func NewElection(cfg *config.ElectionConfig) *Election {
	return &Election{
		cfg: cfg,
	}
}

// Start 启动选举
func (e *Election) Start() error {
	if !e.cfg.Enabled {
		logger.Info("Master 选举已禁用，默认成为 Master")
		e.isMaster = true
		return nil
	}
	
	logger.Info("启动 Master 选举...")
	
	// 生成 Master ID
	hostname, _ := getHostname()
	e.masterID = fmt.Sprintf("master_%s_%d", hostname, time.Now().Unix())
	
	// 创建取消上下文
	e.cancelCtx, e.cancelFn = context.WithCancel(context.Background())
	
	// 尝试获取 Master 锁
	if err := e.tryAcquireLock(); err != nil {
		return err
	}
	
	// 启动锁续期 goroutine
	if e.isMaster {
		go e.renewLockLoop()
	}
	
	return nil
}

// tryAcquireLock 尝试获取 Master 锁
func (e *Election) tryAcquireLock() error {
	ctx := context.Background()
	expiration := time.Duration(e.cfg.LeaseDuration) * time.Second
	
	// 尝试获取锁
	acquired, err := storage.AcquireLock(ctx, masterLockKey, expiration)
	if err != nil {
		return fmt.Errorf("获取 Master 锁失败: %w", err)
	}
	
	if acquired {
		// 成功获取锁，成为 Master
		e.isMaster = true
		
		// 记录 Master ID
		if err := storage.RedisClient.Set(ctx, masterIDKey, e.masterID, expiration).Err(); err != nil {
			logger.Warn("记录 Master ID 失败", zap.Error(err))
		}
		
		logger.Info("成为 Master 节点", zap.String("master_id", e.masterID))
	} else {
		// 未获取到锁，成为 Backup
		e.isMaster = false
		
		// 获取当前 Master ID
		currentMaster, err := storage.RedisClient.Get(ctx, masterIDKey).Result()
		if err == nil {
			logger.Info("当前为 Backup 节点", zap.String("current_master", currentMaster))
		} else {
			logger.Info("当前为 Backup 节点")
		}
	}
	
	return nil
}

// renewLockLoop 锁续期循环
func (e *Election) renewLockLoop() {
	ticker := time.NewTicker(time.Duration(e.cfg.RenewInterval) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-e.cancelCtx.Done():
			logger.Info("停止 Master 锁续期")
			return
			
		case <-ticker.C:
			if err := e.renewLock(); err != nil {
				logger.Error("续期 Master 锁失败", zap.Error(err))
				
				// 锁续期失败，可能已失去 Master 身份
				e.isMaster = false
				logger.Warn("失去 Master 身份")
				
				// 尝试重新获取锁
				if err := e.tryAcquireLock(); err != nil {
					logger.Error("重新获取 Master 锁失败", zap.Error(err))
				}
			}
		}
	}
}

// renewLock 续期锁
func (e *Election) renewLock() error {
	ctx := context.Background()
	expiration := time.Duration(e.cfg.LeaseDuration) * time.Second
	
	// 续期锁
	if err := storage.RenewLock(ctx, masterLockKey, expiration); err != nil {
		return err
	}
	
	// 续期 Master ID
	if err := storage.RedisClient.Expire(ctx, masterIDKey, expiration).Err(); err != nil {
		logger.Warn("续期 Master ID 失败", zap.Error(err))
	}
	
	logger.Debug("Master 锁续期成功")
	return nil
}

// IsMaster 判断是否为 Master
func (e *Election) IsMaster() bool {
	return e.isMaster
}

// GetMasterID 获取 Master ID
func (e *Election) GetMasterID() string {
	return e.masterID
}

// Stop 停止选举
func (e *Election) Stop() {
	if e.cancelFn != nil {
		e.cancelFn()
	}
	
	// 如果是 Master，释放锁
	if e.isMaster {
		ctx := context.Background()
		if err := storage.ReleaseLock(ctx, masterLockKey); err != nil {
			logger.Warn("释放 Master 锁失败", zap.Error(err))
		}
		
		if err := storage.RedisClient.Del(ctx, masterIDKey).Err(); err != nil {
			logger.Warn("删除 Master ID 失败", zap.Error(err))
		}
		
		logger.Info("释放 Master 锁")
	}
}

// getHostname 获取主机名
func getHostname() (string, error) {
	// TODO: 实现获取主机名逻辑
	return "localhost", nil
}
