package master

import (
	"context"
	"time"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// Master Master 节点管理器
type Master struct {
	cfg         *config.Config
	grpcServer  *GRPCServer
	wsServer    *WebSocketServer // WebSocket服务器
	apiServer   *APIServer
	scheduler   *Scheduler
	dispatcher  *Dispatcher
	taskConsumer *TaskConsumer
	consumerCancel context.CancelFunc
}

// NewMaster 创建 Master 节点
func NewMaster(cfg *config.Config) *Master {
	return &Master{
		cfg: cfg,
	}
}

// Start 启动 Master 节点
func (m *Master) Start() error {
	logger.Info("启动 Master 节点...")

	// 启动核心服务
	return m.startServices()
}

// startServices 启动核心服务
func (m *Master) startServices() error {
	logger.Info("启动 Master 核心服务...")

	// 启动 WebSocket 服务器
	m.wsServer = NewWebSocketServer(m.cfg.Server.WebSocketPort)
	if err := m.wsServer.Start(); err != nil {
		return err
	}

	// 启动 gRPC 服务器
	m.grpcServer = NewGRPCServer(m.cfg)
	m.grpcServer.SetWebSocketServer(m.wsServer) // 设置WebSocket服务器
	if err := m.grpcServer.Start(); err != nil {
		return err
	}

	// 创建分发器
	m.dispatcher = NewDispatcher()

	// 启动任务消费者
	m.taskConsumer = NewTaskConsumer(m.dispatcher, m.cfg.Master.DispatchRetry)
	consumerCtx, cancel := context.WithCancel(context.Background())
	m.consumerCancel = cancel
	go m.taskConsumer.Start(consumerCtx)

	// 启动调度器
	m.scheduler = NewScheduler(&m.cfg.Master.Scheduler)
	if err := m.scheduler.Start(); err != nil {
		return err
	}

	// 启动 API 服务器
	m.apiServer = NewAPIServer(m.cfg, m.scheduler, m.dispatcher)
	if err := m.apiServer.Start(); err != nil {
		return err
	}

	logger.Info("Master 核心服务启动完成")
	return nil
}

// Stop 停止 Master 节点
func (m *Master) Stop() {
	logger.Info("停止 Master 节点...")

	if m.consumerCancel != nil {
		m.consumerCancel()
	}
	if m.taskConsumer != nil {
		if ok := m.taskConsumer.Wait(10 * time.Second); !ok {
			logger.Warn("任务消费者停止超时")
		}
	}

	if m.scheduler != nil {
		m.scheduler.Stop()
	}

	if m.wsServer != nil {
		m.wsServer.Stop()
	}

	if m.grpcServer != nil {
		m.grpcServer.Stop()
	}

	if m.dispatcher != nil {
		m.dispatcher.Close()
	}

	logger.Info("Master 节点已停止")
}
