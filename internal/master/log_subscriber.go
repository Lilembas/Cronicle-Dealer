package master

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// LogSubscriber 订阅 Redis Pub/Sub 日志频道，推送到 WebSocket 前端
type LogSubscriber struct {
	wsServer    *WebSocketServer
	cancel      context.CancelFunc
	retryDelay  time.Duration
}

// NewLogSubscriber 创建日志订阅器
func NewLogSubscriber(wsServer *WebSocketServer) *LogSubscriber {
	return &LogSubscriber{
		wsServer:   wsServer,
		retryDelay: 3 * time.Second,
	}
}

// Start 启动日志订阅（带自动重连）
func (s *LogSubscriber) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.subscribeLoop(ctx)
	logger.Info("日志订阅器已启动")
}

// Stop 停止日志订阅
func (s *LogSubscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	logger.Info("日志订阅器已停止")
}

// subscribeLoop 订阅循环，断线自动重连
func (s *LogSubscriber) subscribeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgChan, cancelSub := storage.SubscribeLog(ctx)
		if msgChan == nil {
			// Redis 连接失败，等待重试
			logger.Warn("日志订阅连接失败，等待重试")
			select {
			case <-ctx.Done():
				return
			case <-time.After(s.retryDelay):
			}
			continue
		}

		s.processMessages(ctx, msgChan)
		cancelSub()

		// channel 关闭说明连接断开，等待重试
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.retryDelay):
			logger.Info("日志订阅重连中...")
		}
	}
}

// processMessages 处理订阅消息
func (s *LogSubscriber) processMessages(ctx context.Context, msgChan <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			eventID, content := storage.ParseLogMessage(msg)
			if eventID == "" || content == "" {
				continue
			}

			if s.wsServer != nil {
				if err := s.wsServer.BroadcastLog(eventID, content); err != nil {
					logger.Warn("WebSocket推送日志失败",
						zap.String("event_id", eventID),
						zap.Error(err))
				}
			}
		}
	}
}
