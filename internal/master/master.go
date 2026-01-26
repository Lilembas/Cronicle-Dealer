package master

import (
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// Master Master 节点管理器
type Master struct {
	cfg        *config.Config
	election   *Election
	grpcServer *GRPCServer
	apiServer  *APIServer
	scheduler  *Scheduler
	dispatcher *Dispatcher
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
	
	// 启动选举
	m.election = NewElection(&m.cfg.Master.Election)
	if err := m.election.Start(); err != nil {
		return err
	}
	
	// 如果是 Master，启动核心服务
	if m.election.IsMaster() {
		return m.startServices()
	}
	
	logger.Info("当前为 Backup 节点")
	return nil
}

// startServices 启动核心服务
func (m *Master) startServices() error {
	logger.Info("启动 Master 核心服务...")
	
	// 启动 gRPC 服务器
	m.grpcServer = NewGRPCServer(m.cfg)
	if err := m.grpcServer.Start(); err != nil {
		return err
	}
	
	// 创建分发器
	m.dispatcher = NewDispatcher()
	
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
	
	if m.scheduler != nil {
		m.scheduler.Stop()
	}
	
	if m.grpcServer != nil {
		m.grpcServer.Stop()
	}
	
	if m.dispatcher != nil {
		m.dispatcher.Close()
	}
	
	if m.election != nil {
		m.election.Stop()
	}
	
	logger.Info("Master 节点已停止")
}
