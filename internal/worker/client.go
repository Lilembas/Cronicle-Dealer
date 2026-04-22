package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"go.uber.org/zap"

	pb "github.com/cronicle/cronicle-next/pkg/grpc/pb"
	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/sysmetrics"
	"github.com/cronicle/cronicle-next/pkg/utils"
)

const (
	workerVersion = "0.1.0"
	requestTimeout = 10 * time.Second
	heartbeatTimeout = 5 * time.Second
)

// Client Worker gRPC 客户端
type Client struct {
	cfg         *config.WorkerConfig
	conn        *grpc.ClientConn
	client      pb.CronicleServiceClient
	nodeID      string
	hostname    string
	localIP     string
	grpcAddress string    // executor gRPC 服务地址
	executor    *Executor // 引用执行器以获取运行状态
	metrics     *sysmetrics.Collector
}

// NewClient 创建 Worker 客户端
func NewClient(cfg *config.WorkerConfig) *Client {
	return &Client{
		cfg:     cfg,
		metrics: sysmetrics.NewCollector(),
	}
}

// authUnaryInterceptor 向所有 gRPC 请求注入 auth token
func (c *Client) authUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if c.cfg.AuthToken != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-auth-token", c.cfg.AuthToken)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// Connect 连接到 Manager
func (c *Client) Connect() error {
	logger.Info("连接到 Manager", zap.String("address", c.cfg.ManagerAddress))

	conn, err := grpc.Dial(
		c.cfg.ManagerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(c.authUnaryInterceptor),
	)
	if err != nil {
		return fmt.Errorf("连接 Manager 失败: %w", err)
	}

	c.conn = conn
	c.client = pb.NewCronicleServiceClient(conn)
	c.hostname = c.getHostname()
	c.localIP = utils.GetLocalIP()

	logger.Info("成功连接到 Manager")
	return nil
}

// SetGRPCAddress 设置 executor gRPC 地址
// 如果传入的host是"0.0.0.0"或空，则自动使用检测到的本地IP
func (c *Client) SetGRPCAddress(host string, port int) {
	if host == "0.0.0.0" || host == "" {
		// 使用检测到的本地IP（在Connect时已设置）
		host = c.localIP
	}
	c.grpcAddress = fmt.Sprintf("%s:%d", host, port)
	logger.Info("设置 Worker executor gRPC 地址",
		zap.String("address", c.grpcAddress),
		zap.String("local_ip", c.localIP))
}

// SetExecutor 设置执行器引用
func (c *Client) SetExecutor(e *Executor) {
	c.executor = e
}

// Register 注册节点
func (c *Client) Register() error {
	logger.Info("向 Manager 注册节点...")

	resources, err := c.getResourceInfo()
	if err != nil {
		logger.Warn("获取资源信息失败", zap.Error(err))
		resources = &pb.NodeResources{}
	}

	// 获取当前进程 PID
	pid := int32(os.Getpid())

	req := &pb.RegisterNodeRequest{
		Hostname:    c.hostname,
		Ip:          c.localIP,
		Tags:        c.cfg.Node.Tags,
		Resources:   resources,
		Version:     workerVersion,
		GrpcAddress: c.grpcAddress,
		Pid:         pid,
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

	logger.Info("节点注册成功",
		zap.String("node_id", c.nodeID),
		zap.String("hostname", c.hostname),
		zap.Int32("pid", pid))

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
	resources, err := c.getResourceInfo()
	if err != nil {
		logger.Warn("获取资源信息失败", zap.Error(err))
		resources = &pb.NodeResources{}
	}

	// 汇报运行中的任务
	var runningJobs []string
	if c.executor != nil {
		runningJobs = c.executor.GetRunningJobIDs()
	}

	req := &pb.HeartbeatRequest{
		NodeId:      c.nodeID,
		Resources:   resources,
		RunningJobs: runningJobs,
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
		"tags":           c.getTagsJSON(),
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

	// 向Manager发送下线通知
	if c.nodeID != "" && c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		logger.Info("向Manager发送下线通知", zap.String("node_id", c.nodeID))
		if _, err := c.client.UnregisterNode(ctx, &pb.UnregisterNodeRequest{
			NodeId: c.nodeID,
		}); err != nil {
			// 即使发送失败也继续关闭连接
			logger.Warn("发送下线通知失败", zap.Error(err))
		} else {
			logger.Info("已成功通知Manager下线")
		}
	}

	// 从Redis移除
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

// GetManagerClient 获取Manager gRPC客户端
func (c *Client) GetManagerClient() pb.CronicleServiceClient {
	return c.client
}

// getTagsJSON 获取 JSON 格式的标签
func (c *Client) getTagsJSON() string {
	b, _ := json.Marshal(c.cfg.Node.Tags)
	return string(b)
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

// getResourceInfo 获取资源信息
func (c *Client) getResourceInfo() (*pb.NodeResources, error) {
	info, err := c.metrics.GetResourceInfo()
	if err != nil {
		return nil, fmt.Errorf("获取资源信息失败: %w", err)
	}

	return &pb.NodeResources{
		CpuUsage:    info.CPUUsage,
		MemoryUsage: info.MemoryUsed,
		MemoryTotal: info.MemoryTotal,
		DiskUsage:   info.DiskUsed,
		DiskTotal:   info.DiskTotal,
		CpuCores:    info.CPUCores,
	}, nil
}
