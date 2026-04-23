package manager

import (
	"fmt"

	"github.com/cronicle/cronicle-dealer/pkg/logger"
)

// WebSocketServer WebSocket服务器（WebSocket 处理已集成到 API Server，不再独立监听端口）
type WebSocketServer struct {
	hub *Hub
}

// NewWebSocketServer 创建WebSocket服务器
func NewWebSocketServer() *WebSocketServer {
	return &WebSocketServer{
		hub: NewHub(),
	}
}

// Stop 停止WebSocket服务器
func (s *WebSocketServer) Stop() error {
	logger.Info("停止WebSocket服务器...")
	return s.hub.melody.Close()
}

// GetHub 获取Hub实例
func (s *WebSocketServer) GetHub() *Hub {
	return s.hub
}

// BroadcastLog 广播任务日志
func (s *WebSocketServer) BroadcastLog(eventID, content string) error {
	roomName := fmt.Sprintf("event:%s", eventID)
	msg := ServerMessage{
		Type: "log",
		Data: map[string]interface{}{
			"event_id": eventID,
			"content":  content,
		},
	}
	return s.hub.BroadcastToRoom(roomName, msg)
}

// BroadcastTaskStatus 广播任务状态变化
func (s *WebSocketServer) BroadcastTaskStatus(eventID, jobID, status, nodeID, nodeName string, exitCode int) error {
	msg := ServerMessage{
		Type: "task_status",
		Data: map[string]interface{}{
			"event_id":  eventID,
			"job_id":    jobID,
			"status":    status,
			"node_id":   nodeID,
			"node_name": nodeName,
			"exit_code": exitCode,
		},
	}
	return s.hub.BroadcastToAll(msg)
}

// BroadcastNodeStatus 广播节点状态变化
func (s *WebSocketServer) BroadcastNodeStatus(nodeID, hostname, status string, cpuUsage, memoryPercent float64, runningJobs int) error {
	msg := ServerMessage{
		Type: "node_status",
		Data: map[string]interface{}{
			"node_id":         nodeID,
			"hostname":        hostname,
			"status":          status,
			"cpu_usage":       cpuUsage,
			"memory_percent":  memoryPercent,
			"running_jobs":    runningJobs,
		},
	}
	return s.hub.BroadcastToAll(msg)
}
