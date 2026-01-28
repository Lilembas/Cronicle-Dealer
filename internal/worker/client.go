package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"go.uber.org/zap"

	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
)

const (
	workerVersion = "0.1.0"
	requestTimeout = 10 * time.Second
	heartbeatTimeout = 5 * time.Second
)

// Client Worker gRPC 客户端
type Client struct {
	cfg           *config.WorkerConfig
	conn          *grpc.ClientConn
	client        pb.CronicleServiceClient
	nodeID        string
	securityToken string
	hostname      string
	localIP       string
}

// NewClient 创建 Worker 客户端
func NewClient(cfg *config.WorkerConfig) *Client {
	return &Client{
		cfg: cfg,
	}
}

// Connect 连接到 Master
func (c *Client) Connect() error {
	logger.Info("连接到 Master", zap.String("address", c.cfg.MasterAddress))

	conn, err := grpc.Dial(
		c.cfg.MasterAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("连接 Master 失败: %w", err)
	}

	c.conn = conn
	c.client = pb.NewCronicleServiceClient(conn)
	c.hostname = c.getHostname()
	c.localIP = getLocalIP()

	logger.Info("成功连接到 Master")
	return nil
}

// Register 注册节点
func (c *Client) Register() error {
	logger.Info("向 Master 注册节点...")

	resources, err := getResourceInfo()
	if err != nil {
		logger.Warn("获取资源信息失败", zap.Error(err))
		resources = &pb.NodeResources{}
	}

	req := &pb.RegisterNodeRequest{
		Hostname:  c.hostname,
		Ip:        c.localIP,
		Tags:      c.cfg.Node.Tags,
		Resources: resources,
		Version:   workerVersion,
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	resp, err := c.client.RegisterNode(ctx, req)
	if err != nil {
		return fmt.Errorf("注册节点失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("注册被拒绝: %s", resp.Message)
	}

	c.nodeID = resp.NodeId
	c.securityToken = resp.SecurityToken

	logger.Info("节点注册成功",
		zap.String("node_id", c.nodeID),
		zap.String("hostname", c.hostname))

	if err := c.updateRedisWorker(ctx, resources); err != nil {
		logger.Warn("注册到 Redis 失败", zap.Error(err))
	}

	return nil
}

// StartHeartbeat 启动心跳
func (c *Client) StartHeartbeat() {
	interval := time.Duration(c.cfg.Heartbeat.Interval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("启动心跳", zap.Duration("interval", interval))

	for range ticker.C {
		if err := c.sendHeartbeat(); err != nil {
			logger.Error("发送心跳失败", zap.Error(err))
		}
	}
}

// sendHeartbeat 发送心跳
func (c *Client) sendHeartbeat() error {
	resources, err := getResourceInfo()
	if err != nil {
		logger.Warn("获取资源信息失败", zap.Error(err))
		resources = &pb.NodeResources{}
	}

	req := &pb.HeartbeatRequest{
		NodeId:      c.nodeID,
		Resources:   resources,
		RunningJobs: []string{}, // TODO: 需要与 Executor 共享运行任务信息
		Timestamp:   time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), heartbeatTimeout)
	defer cancel()

	resp, err := c.client.Heartbeat(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		logger.Warn("心跳失败")
		return nil
	}

	logger.Debug("心跳成功")

	if err := c.updateRedisWorker(ctx, resources); err != nil {
		logger.Warn("更新 Redis Worker 状态失败", zap.Error(err))
	}

	return nil
}

// updateRedisWorker 更新 Redis 中的 Worker 信息
func (c *Client) updateRedisWorker(ctx context.Context, resources *pb.NodeResources) error {
	data := map[string]interface{}{
		"hostname":       c.hostname,
		"ip":             c.localIP,
		"tags":           fmt.Sprintf("%v", c.cfg.Node.Tags),
		"cpu_cores":      resources.CpuCores,
		"cpu_usage":      resources.CpuUsage,
		"memory_total":   resources.MemoryTotal,
		"memory_usage":   resources.MemoryUsage,
		"disk_total":     resources.DiskTotal,
		"disk_usage":     resources.DiskUsage,
		"version":        workerVersion,
		"last_heartbeat": time.Now().Unix(),
	}
	return storage.RegisterWorker(ctx, c.nodeID, data)
}

// Close 关闭连接
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	ctx := context.Background()
	if err := storage.RemoveWorkerOffline(ctx, c.nodeID); err != nil {
		logger.Warn("从 Redis 移除 Worker 失败", zap.Error(err))
	}

	return c.conn.Close()
}

// GetNodeID 获取节点 ID
func (c *Client) GetNodeID() string {
	return c.nodeID
}

// getHostname 获取主机名
func (c *Client) getHostname() string {
	if c.cfg.Node.Hostname != "" {
		return c.cfg.Node.Hostname
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// getLocalIP 获取本地 IP
func getLocalIP() string {
	// TODO: 实现获取本地 IP 逻辑
	return "127.0.0.1"
}

// getResourceInfo 获取资源信息
func getResourceInfo() (*pb.NodeResources, error) {
	// TODO: 实现真实的资源监控
	return &pb.NodeResources{
		CpuUsage:     20.5,
		MemoryUsage:  4.0,
		MemoryTotal:  16.0,
		DiskUsage:    50.0,
		DiskTotal:    500.0,
		CpuCores:     8,
	}, nil
}
