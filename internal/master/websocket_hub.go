package master

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/olahol/melody"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// Hub WebSocket房间管理器
type Hub struct {
	melody *melody.Melody
	rooms  map[string]*Room
	mu     sync.RWMutex
}

// Room WebSocket房间
type Room struct {
	name  string
	users map[*melody.Session]bool
}

// ServerMessage 服务器消息
type ServerMessage struct {
	Type string      `json:"type"`          // log, task_status, node_status, history_log, error, pong
	Data interface{} `json:"data"`
	Room string      `json:"room,omitempty"`
}

// ClientMessage 客户端消息
type ClientMessage struct {
	Action  string `json:"action"`           // join, leave, ping
	Room    string `json:"room,omitempty"`
	EventID string `json:"event_id,omitempty"` // 兼容旧写法
}

// NewHub 创建Hub
func NewHub() *Hub {
	m := melody.New()
	m.Config.MaxMessageSize = 1024 * 1024 // 1MB

	hub := &Hub{
		melody: m,
		rooms:  make(map[string]*Room),
	}

	// 设置消息处理
	m.HandleMessage(func(session *melody.Session, msg []byte) {
		hub.handleMessage(session, msg)
	})
	m.HandleConnect(func(session *melody.Session) {
		hub.handleConnect(session)
	})
	m.HandleDisconnect(func(session *melody.Session) {
		hub.handleDisconnect(session)
	})

	return hub
}

// handleMessage 处理客户端消息
func (h *Hub) handleMessage(session *melody.Session, msg []byte) {
	var clientMsg ClientMessage
	if err := json.Unmarshal(msg, &clientMsg); err != nil {
		logger.Error("解析客户端消息失败", zap.Error(err))
		h.sendError(session, "无效的消息格式")
		return
	}

	logger.Debug("收到客户端消息",
		zap.String("action", clientMsg.Action),
		zap.String("room", clientMsg.Room))

	switch clientMsg.Action {
	case "join":
		room := clientMsg.Room
		if room == "" && clientMsg.EventID != "" {
			// 兼容旧写法
			room = "event:" + clientMsg.EventID
		}
		if err := h.Join(session, room); err != nil {
			h.sendError(session, err.Error())
		}

	case "leave":
		room := clientMsg.Room
		if room == "" && clientMsg.EventID != "" {
			room = "event:" + clientMsg.EventID
		}
		if err := h.Leave(session, room); err != nil {
			h.sendError(session, err.Error())
		}

	case "ping":
		// 响应心跳
		h.sendPong(session)

	default:
		h.sendError(session, "未知的action: "+clientMsg.Action)
	}
}

// handleConnect 新连接建立
func (h *Hub) handleConnect(session *melody.Session) {
	logger.Info("WebSocket新连接", zap.String("remote_addr", session.Request.RemoteAddr))
	h.Join(session, "global") // 默认加入全局房间
}

// handleDisconnect 连接断开
func (h *Hub) handleDisconnect(session *melody.Session) {
	logger.Info("WebSocket断开连接", zap.String("remote_addr", session.Request.RemoteAddr))
	h.mu.Lock()
	defer h.mu.Unlock()

	// 从所有房间移除
	for _, room := range h.rooms {
		delete(room.users, session)
	}
}

// Join 加入房间
func (h *Hub) Join(session *melody.Session, roomName string) error {
	if roomName == "" {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// 获取或创建房间
	room, exists := h.rooms[roomName]
	if !exists {
		room = &Room{
			name:  roomName,
			users: make(map[*melody.Session]bool),
		}
		h.rooms[roomName] = room
	}

	// 加入房间
	room.users[session] = true

	logger.Debug("客户端加入房间",
		zap.String("room", roomName),
		zap.String("remote_addr", session.Request.RemoteAddr))

	// 如果是任务房间，发送历史日志
	if len(roomName) > 6 && roomName[:6] == "event:" {
		eventID := roomName[6:]
		go h.sendHistoryLogs(session, eventID, roomName)
	}

	return nil
}

// Leave 离开房间（除了global房间）
func (h *Hub) Leave(session *melody.Session, roomName string) error {
	if roomName == "" || roomName == "global" {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	room, exists := h.rooms[roomName]
	if !exists {
		return nil
	}

	delete(room.users, session)

	// 如果房间为空，删除房间
	if len(room.users) == 0 {
		delete(h.rooms, roomName)
	}

	logger.Debug("客户端离开房间",
		zap.String("room", roomName),
		zap.String("remote_addr", session.Request.RemoteAddr))

	return nil
}

// BroadcastToRoom 向指定房间广播消息
func (h *Hub) BroadcastToRoom(roomName string, msg ServerMessage) error {
	h.mu.RLock()
	room, exists := h.rooms[roomName]
	h.mu.RUnlock()

	if !exists {
		return nil
	}

	msg.Room = roomName
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 向房间内所有用户发送消息
	for session := range room.users {
		if err := session.Write(data); err != nil {
			logger.Error("发送消息失败",
				zap.String("room", roomName),
				zap.Error(err))
		}
	}

	return nil
}

// BroadcastToAll 向所有连接广播消息
func (h *Hub) BroadcastToAll(msg ServerMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return h.melody.Broadcast(data)
}

// sendHistoryLogs 发送历史日志
func (h *Hub) sendHistoryLogs(session *melody.Session, eventID, roomName string) {
	ctx := context.Background()

	// 从存储获取历史日志（优先Redis，回退文件）
	logs, err := storage.GetLogs(ctx, eventID)
	if err != nil {
		logger.Error("获取历史日志失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		// 发送错误消息
		h.sendError(session, "获取历史日志失败")
		return
	}

	// 根据事件实际状态判断是否完成
	complete := false
	var event models.Event
	if err := storage.DB.Where("id = ?", eventID).First(&event).Error; err == nil {
		if event.Status == eventStatusSuccess || event.Status == eventStatusFailed ||
			event.Status == eventStatusAborted || event.Status == eventStatusTimeout {
			complete = true
		}
	}

	// 发送历史日志
	msg := ServerMessage{
		Type: "history_log",
		Room: roomName,
		Data: map[string]interface{}{
			"event_id": eventID,
			"logs":     logs,
			"complete": complete,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("序列化历史日志失败", zap.Error(err))
		return
	}

	if err := session.Write(data); err != nil {
		logger.Error("发送历史日志失败",
			zap.String("event_id", eventID),
			zap.Error(err))
		return
	}

	logger.Debug("发送历史日志成功",
		zap.String("event_id", eventID),
		zap.Int("length", len(logs)))
}

// sendError 发送错误消息
func (h *Hub) sendError(session *melody.Session, message string) {
	msg := ServerMessage{
		Type: "error",
		Data: map[string]string{
			"message": message,
		},
	}
	data, _ := json.Marshal(msg)
	session.Write(data)
}

// sendPong 发送心跳响应
func (h *Hub) sendPong(session *melody.Session) {
	msg := ServerMessage{
		Type: "pong",
		Data: map[string]interface{}{
			"timestamp": 0, // TODO: 使用实际时间戳
		},
	}
	data, _ := json.Marshal(msg)
	session.Write(data)
}
