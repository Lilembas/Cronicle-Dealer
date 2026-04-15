package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

var (
	cpuStatsMu      sync.Mutex
	lastCPUTotal    uint64
	lastCPUIdle     uint64
	cpuStatsInited  bool
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
	grpcAddress   string // executor gRPC 服务地址
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

// Register 注册节点
func (c *Client) Register() error {
	logger.Info("向 Master 注册节点...")

	resources, err := getResourceInfo()
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
	c.securityToken = resp.SecurityToken

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

	// 向Master发送下线通知
	if c.nodeID != "" && c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		logger.Info("向Master发送下线通知", zap.String("node_id", c.nodeID))
		if _, err := c.client.UnregisterNode(ctx, &pb.UnregisterNodeRequest{
			NodeId: c.nodeID,
		}); err != nil {
			// 即使发送失败也继续关闭连接
			logger.Warn("发送下线通知失败", zap.Error(err))
		} else {
			logger.Info("已成功通知Master下线")
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

// GetMasterClient 获取Master gRPC客户端
func (c *Client) GetMasterClient() pb.CronicleServiceClient {
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

// getLocalIP 获取本地 IP
func getLocalIP() string {
	// 尝试获取真实的本地IP地址
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	// 如果找不到非loopback地址，返回127.0.0.1
	return "127.0.0.1"
}

// getResourceInfo 获取资源信息
func getResourceInfo() (*pb.NodeResources, error) {
	cpuUsage, err := getCPUUsagePercent()
	if err != nil {
		return nil, fmt.Errorf("获取 CPU 使用率失败: %w", err)
	}

	memUsedGB, memTotalGB, err := getMemoryUsageGB()
	if err != nil {
		return nil, fmt.Errorf("获取内存使用失败: %w", err)
	}

	diskUsedGB, diskTotalGB, err := getDiskUsageGB("/")
	if err != nil {
		return nil, fmt.Errorf("获取磁盘使用失败: %w", err)
	}

	return &pb.NodeResources{
		CpuUsage:     cpuUsage,
		MemoryUsage:  memUsedGB,
		MemoryTotal:  memTotalGB,
		DiskUsage:    diskUsedGB,
		DiskTotal:    diskTotalGB,
		CpuCores:     int32(runtime.NumCPU()),
	}, nil
}

func getCPUUsagePercent() (float64, error) {
	total, idle, err := readCPUStat()
	if err != nil {
		return 0, err
	}

	cpuStatsMu.Lock()
	defer cpuStatsMu.Unlock()

	if !cpuStatsInited {
		lastCPUTotal = total
		lastCPUIdle = idle
		cpuStatsInited = true
		return 0, nil
	}

	totalDelta := total - lastCPUTotal
	idleDelta := idle - lastCPUIdle
	lastCPUTotal = total
	lastCPUIdle = idle

	if totalDelta == 0 {
		return 0, nil
	}

	usage := (1 - float64(idleDelta)/float64(totalDelta)) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, nil
}

func readCPUStat() (uint64, uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		if scanErr := scanner.Err(); scanErr != nil {
			return 0, 0, scanErr
		}
		return 0, 0, fmt.Errorf("读取 /proc/stat 失败")
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("无效的 /proc/stat 格式")
	}

	var total uint64
	for i := 1; i < len(fields); i++ {
		v, parseErr := strconv.ParseUint(fields[i], 10, 64)
		if parseErr != nil {
			return 0, 0, parseErr
		}
		total += v
	}

	idle, err := strconv.ParseUint(fields[4], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if len(fields) > 5 {
		// iowait 也计入空闲
		iowait, parseErr := strconv.ParseUint(fields[5], 10, 64)
		if parseErr == nil {
			idle += iowait
		}
	}

	return total, idle, nil
}

func getMemoryUsageGB() (usedGB, totalGB float64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var totalKB, availableKB uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB, _ = parseMemInfoKB(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			availableKB, _ = parseMemInfoKB(line)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return 0, 0, scanErr
	}

	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal 无效")
	}
	if availableKB > totalKB {
		availableKB = 0
	}

	usedKB := totalKB - availableKB
	totalGB = float64(totalKB) / 1024.0 / 1024.0
	usedGB = float64(usedKB) / 1024.0 / 1024.0
	return usedGB, totalGB, nil
}

func parseMemInfoKB(line string) (uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("无效 meminfo 行: %s", line)
	}
	return strconv.ParseUint(fields[1], 10, 64)
}

func getDiskUsageGB(path string) (usedGB, totalGB float64, err error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return 0, 0, err
	}

	total := fs.Blocks * uint64(fs.Bsize)
	available := fs.Bfree * uint64(fs.Bsize)
	used := total - available

	totalGB = float64(total) / 1024.0 / 1024.0 / 1024.0
	usedGB = float64(used) / 1024.0 / 1024.0 / 1024.0
	return usedGB, totalGB, nil
}
