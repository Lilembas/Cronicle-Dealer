package manager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
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

	// 定期扫描孤儿日志（每 20 个检查周期触发一次）
	orphanLogCheckCounter := 0
	const orphanLogCheckInterval = 20

	for {
		select {
		case <-ctx.Done():
			logger.Info("节点健康检查器停止")
			return
		case <-ticker.C:
			h.checkAllNodes()

			orphanLogCheckCounter++
			if orphanLogCheckCounter >= orphanLogCheckInterval {
				orphanLogCheckCounter = 0
				h.grpcServer.RecoverOrphanLogs(ctx)
			}
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

// isManagerNode 判断节点是否为 Manager
func isManagerNode(tags string) bool {
	return strings.Contains(tags, "manager")
}

// checkAllNodes 检查所有在线节点的健康状态
func (h *HealthChecker) checkAllNodes() {
	var nodes []models.Node
	if err := storage.DB.Where("status = ?", nodeStatusOnline).Find(&nodes).Error; err != nil {
		logger.Error("健康检查：查询节点失败", zap.Error(err))
		return
	}

	// 构建在线 Worker 节点 ID 集合（用于孤儿事件扫描）
	onlineWorkerIDs := make(map[string]bool)
	for _, node := range nodes {
		if !isManagerNode(node.Tags) {
			onlineWorkerIDs[node.ID] = true
		}
	}

	// 扫描孤儿事件：running 状态但所属节点不在线（或不存在）
	h.cleanupOrphanEventsOnNonexistentNodes(onlineWorkerIDs)

	if len(nodes) == 0 {
		return
	}

	timeout := time.Duration(h.cfg.Timeout) * time.Second
	threshold := time.Now().Add(-timeout)

	for _, node := range nodes {
		if isManagerNode(node.Tags) {
			continue
		}

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

	if err := storage.DB.Model(&models.Node{}).Where("id = ?", node.ID).
		Updates(map[string]interface{}{
			"status":       nodeStatusOffline,
			"running_jobs": 0,
		}).Error; err != nil {
		logger.Error("更新节点状态失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
	}

	h.grpcServer.nodes.Delete(node.ID)

	if err := storage.RemoveWorkerOffline(context.Background(), node.ID); err != nil {
		logger.Warn("清理 Redis Worker 键失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
	}

	if h.wsServer != nil {
		if err := h.wsServer.BroadcastNodeStatus(node.ID, node.Hostname, nodeStatusOffline, 0, 0, 0); err != nil {
			logger.Warn("推送节点下线状态失败",
				zap.String("node_id", node.ID),
				zap.Error(err))
		}
	}

	h.dispatcher.RemoveNodeClient(node.ID)
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

	for _, event := range orphanedEvents {
		errMsg := fmt.Sprintf("Worker 节点 %s (%s) 离线，任务被迫终止", node.Hostname, node.ID)
		h.failOrphanedEvent(event, errMsg)
	}
}

// cleanupOrphanEventsOnNonexistentNodes 清理所属节点不在线（或已删除）的孤儿事件
func (h *HealthChecker) cleanupOrphanEventsOnNonexistentNodes(onlineWorkerIDs map[string]bool) {
	var orphanedEvents []models.Event
	if err := storage.DB.Where("status = ?", eventStatusRunning).
		Find(&orphanedEvents).Error; err != nil {
		logger.Error("查询运行中事件失败", zap.Error(err))
		return
	}

	for _, event := range orphanedEvents {
		if onlineWorkerIDs[event.NodeID] {
			continue
		}

		nodeName := event.NodeName
		if nodeName == "" {
			nodeName = event.NodeID
		}

		errMsg := fmt.Sprintf("Worker 节点 %s (%s) 不在线或已删除，任务被迫终止", nodeName, event.NodeID)
		h.failOrphanedEvent(event, errMsg)
	}
}

// failOrphanedEvent 将单个孤儿事件标记为失败，并完成日志归档、广播、统计更新
func (h *HealthChecker) failOrphanedEvent(event models.Event, errMsg string) {
	now := time.Now()
	var startTime time.Time
	if event.StartTime != nil {
		startTime = *event.StartTime
	} else {
		startTime = now.Add(-1 * time.Second)
	}
	duration := int64(now.Sub(startTime).Seconds())

	if err := storage.DB.Model(&models.Event{}).Where("id = ?", event.ID).
		Updates(map[string]interface{}{
			"status":        eventStatusFailed,
			"start_time":    startTime,
			"end_time":      now,
			"duration":      duration,
			"exit_code":     1,
			"error_message": errMsg,
		}).Error; err != nil {
		logger.Error("更新孤儿事件状态失败",
			zap.String("event_id", event.ID), zap.Error(err))
		return
	}

	logMsg := fmt.Sprintf("[%s] [Manager] ❌ %s\n",
		now.Format("2006-01-02 15:04:05"), errMsg)
	if err := storage.SaveLogChunk(context.Background(), event.ID, logMsg); err != nil {
		logger.Warn("写入孤儿事件日志失败", zap.Error(err))
	}

	h.grpcServer.DownloadAndExpireLog(context.Background(), event.ID)

	nodeName := event.NodeName
	if nodeName == "" {
		nodeName = event.NodeID
	}

	if h.wsServer != nil {
		if err := h.wsServer.BroadcastTaskStatus(event.ID, event.JobID, eventStatusFailed, event.NodeID, nodeName, 1); err != nil {
			logger.Warn("推送孤儿事件状态失败",
				zap.String("event_id", event.ID),
				zap.Error(err))
		}
	}

	if err := storage.DB.Model(&models.Job{}).Where("id = ?", event.JobID).Updates(map[string]interface{}{
		"last_run_time": now,
		"failed_runs":   gorm.Expr("failed_runs + 1"),
	}).Error; err != nil {
		logger.Warn("更新任务统计信息失败", zap.String("job_id", event.JobID), zap.Error(err))
	}

	logger.Info("孤儿事件已标记为失败",
		zap.String("event_id", event.ID),
		zap.String("job_id", event.JobID),
		zap.String("node_id", event.NodeID))
}
