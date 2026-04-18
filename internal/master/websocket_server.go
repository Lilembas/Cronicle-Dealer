package master

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/pkg/logger"
)

// WebSocketServer WebSocket服务器
type WebSocketServer struct {
	hub  *Hub
	port int
}

// NewWebSocketServer 创建WebSocket服务器
func NewWebSocketServer(port int) *WebSocketServer {
	return &WebSocketServer{
		hub:  NewHub(),
		port: port,
	}
}

// Start 启动WebSocket服务器
func (s *WebSocketServer) Start() error {
	// 创建Gin路由（仅用于WebSocket端点）
	router := gin.New()
	router.GET("/ws", func(c *gin.Context) {
		if err := s.hub.melody.HandleRequest(c.Writer, c.Request); err != nil {
			logger.Error("WebSocket连接失败", zap.Error(err))
		}
	})

	// 启动HTTP服务器
	addr := fmt.Sprintf(":%d", s.port)
	logger.Info("WebSocket服务器启动", zap.String("address", addr))

	go func() {
		if err := router.Run(addr); err != nil {
			logger.Error("WebSocket服务器运行失败", zap.Error(err))
		}
	}()

	return nil
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
