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
	"github.com/cronicle/cronicle-next/pkg/logger"
)

// Client Worker gRPC 客户端
type Client struct {
	cfg          *config.WorkerConfig
	conn         *grpc.ClientConn
	client       pb.CronicleServiceClient
	nodeID       string
	securityToken string
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
	
	// 创建 gRPC 连接
	conn, err := grpc.Dial(
		c.cfg.MasterAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("连接 Master 失败: %w", err)
	}
	
	c.conn = conn
	c.client = pb.NewCronicleServiceClient(conn)
	
	logger.Info("成功连接到 Master")
	return nil
}

// Register 注册节点
func (c *Client) Register() error {
	logger.Info("向 Master 注册节点...")
	
	// 获取主机名
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	
	// 如果配置了主机名，覆盖
	if c.cfg.Node.Hostname != "" {
		hostname = c.cfg.Node.Hostname
	}
	
	// 获取资源信息
	resources, err := getResourceInfo()
	if err != nil {
		logger.Warn("获取资源信息失败", zap.Error(err))
		resources = &pb.NodeResources{}
	}
	
	// 发送注册请求
	req := &pb.RegisterNodeRequest{
		Hostname:  hostname,
		Ip:        getLocalIP(),
		Tags:      c.cfg.Node.Tags,
		Resources: resources,
		Version:   "0.1.0",
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		zap.String("hostname", hostname))
	
	return nil
}

// StartHeartbeat 启动心跳
func (c *Client) StartHeartbeat() {
	ticker := time.NewTicker(time.Duration(c.cfg.Heartbeat.Interval) * time.Second)
	defer ticker.Stop()
	
	logger.Info("启动心跳", zap.Int("interval", c.cfg.Heartbeat.Interval))
	
	for range ticker.C {
		if err := c.sendHeartbeat(); err != nil {
			logger.Error("发送心跳失败", zap.Error(err))
		}
	}
}

// sendHeartbeat 发送心跳
func (c *Client) sendHeartbeat() error {
	// 获取资源信息
	resources, err := getResourceInfo()
	if err != nil {
		logger.Warn("获取资源信息失败", zap.Error(err))
		resources = &pb.NodeResources{}
	}
	
	// 构建心跳请求
	req := &pb.HeartbeatRequest{
		NodeId:      c.nodeID,
		Resources:   resources,
		RunningJobs: []string{}, // TODO: 获取正在运行的任务列表
		Timestamp:   time.Now().Unix(),
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	resp, err := c.client.Heartbeat(ctx, req)
	if err != nil {
		return err
	}
	
	if !resp.Success {
		logger.Warn("心跳失败")
	} else {
		logger.Debug("心跳成功")
	}
	
	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetNodeID 获取节点 ID
func (c *Client) GetNodeID() string {
	return c.nodeID
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
