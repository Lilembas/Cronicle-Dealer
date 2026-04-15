package master

import (
	"context"
	"fmt"
	"time"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"go.uber.org/zap"
)

// HealthChecker 节点健康检查器
// 定期扫描 Worker 节点心跳，检测离线节点并清理孤儿事件
type HealthChecker struct {
	cfg        *config.HeartbeatConfig
	dispatcher *Dispatcher
	grpcServer *GRPCServer
	wsServer   *WebSocketServer
	done       chan struct{}
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(
	cfg *config.HeartbeatConfig,
	dispatcher *Dispatcher,
	grpcServer *GRPCServer,
	wsServer *WebSocketServer,
) *HealthChecker {
	return &HealthChecker{
		cfg:        cfg,
		dispatcher: dispatcher,
		grpcServer: grpcServer,
		wsServer:   wsServer,
		done:       make(chan struct{}),
	}
}

// Start 启动健康检查主循环
func (h *HealthChecker) Start(ctx context.Context) {
	logger.Info("启动节点健康检查器",
		zap.Int("timeout_sec", h.cfg.Timeout),
		zap.Int("check_interval_sec", h.cfg.CheckInterval))

	defer close(h.done)

	// 启动时立即执行一次检查
	h.checkAllNodes()

	interval := time.Duration(h.cfg.CheckInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("节点健康检查器停止")
			return
		case <-ticker.C:
			h.checkAllNodes()
		}
	}
}

// Wait 等待健康检查器停止
func (h *HealthChecker) Wait(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-h.done:
		return true
	case <-timer.C:
		return false
	}
}

// checkAllNodes 检查所有在线节点的健康状态
func (h *HealthChecker) checkAllNodes() {
	var nodes []models.Node
	if err := storage.DB.Where("status = ?", nodeStatusOnline).Find(&nodes).Error; err != nil {
		logger.Error("健康检查：查询节点失败", zap.Error(err))
		return
	}

	if len(nodes) == 0 {
		return
	}

	timeout := time.Duration(h.cfg.Timeout) * time.Second
	threshold := time.Now().Add(-timeout)

	for _, node := range nodes {
		// 跳过 Master 节点（Master 心跳由独立机制维护）
		if node.Tags == "master" {
			continue
		}

		// 检查心跳是否超时
		if node.LastHeartbeat.Before(threshold) {
			h.handleOfflineNode(node)
		}
	}
}

// handleOfflineNode 处理离线节点
func (h *HealthChecker) handleOfflineNode(node models.Node) {
	logger.Warn("检测到节点离线",
		zap.String("node_id", node.ID),
		zap.String("hostname", node.Hostname),
		zap.Time("last_heartbeat", node.LastHeartbeat))

	// 1. 标记节点为离线
	if err := storage.DB.Model(&models.Node{}).Where("id = ?", node.ID).
		Updates(map[string]interface{}{
			"status":        nodeStatusOffline,
			"running_jobs":  0,
		}).Error; err != nil {
		logger.Error("更新节点状态失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
	}

	// 2. 从内存缓存中移除
	h.grpcServer.nodes.Delete(node.ID)

	// 3. 清理 Redis Worker 键
	if err := storage.RemoveWorkerOffline(context.Background(), node.ID); err != nil {
		logger.Warn("清理 Redis Worker 键失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
	}

	// 4. WebSocket 广播节点下线
	if h.wsServer != nil {
		if err := h.wsServer.BroadcastNodeStatus(node.ID, node.Hostname, nodeStatusOffline, 0, 0, 0); err != nil {
			logger.Warn("推送节点下线状态失败",
				zap.String("node_id", node.ID),
				zap.Error(err))
		}
	}

	// 5. 清理该节点的 gRPC 连接
	h.dispatcher.RemoveNodeClient(node.ID)

	// 6. 清理孤儿事件：将该节点上所有 running 状态的事件标记为 failed
	h.cleanupOrphanedEvents(node)
}

// cleanupOrphanedEvents 清理离线节点上的孤儿事件
func (h *HealthChecker) cleanupOrphanedEvents(node models.Node) {
	var orphanedEvents []models.Event
	if err := storage.DB.Where("node_id = ? AND status = ?", node.ID, eventStatusRunning).
		Find(&orphanedEvents).Error; err != nil {
		logger.Error("查询孤儿事件失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
		return
	}

	if len(orphanedEvents) == 0 {
		return
	}

	logger.Warn("清理孤儿事件",
		zap.String("node_id", node.ID),
		zap.String("hostname", node.Hostname),
		zap.Int("count", len(orphanedEvents)))

	now := time.Now()
	for _, event := range orphanedEvents {
		errMsg := fmt.Sprintf("Worker 节点 %s (%s) 离线，任务被迫终止", node.Hostname, node.ID)
		if err := storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).
			Updates(map[string]interface{}{
				"status":        eventStatusFailed,
				"end_time":      &now,
				"error_message": errMsg,
			}).Error; err != nil {
			logger.Error("更新孤儿事件状态失败",
				zap.String("event_id", event.ID),
				zap.Error(err))
			continue
		}

		// 写入日志
		logMsg := fmt.Sprintf("[%s] [Master] ❌ Worker 节点离线，任务被迫终止: %s\n",
			now.Format("2006-01-02 15:04:05"), errMsg)
		if err := storage.SaveLogChunk(context.Background(), event.ID, logMsg); err != nil {
			logger.Warn("写入孤儿事件日志失败", zap.Error(err))
		}

		// WebSocket 广播事件状态变更
		if h.wsServer != nil {
			if err := h.wsServer.BroadcastTaskStatus(event.ID, event.JobID, eventStatusFailed, 1); err != nil {
				logger.Warn("推送孤儿事件状态失败",
					zap.String("event_id", event.ID),
					zap.Error(err))
			}
		}

		logger.Info("孤儿事件已标记为失败",
			zap.String("event_id", event.ID),
			zap.String("job_id", event.JobID),
			zap.String("node_id", node.ID))
	}
}
