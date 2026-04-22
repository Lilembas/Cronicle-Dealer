package manager

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/utils"
)

type nodeResources struct {
	CpuUsage     float64
	MemoryUsage  float64
	MemoryTotal  float64
	DiskUsage    float64
	DiskTotal    float64
	CpuCores     int32
}

var (
	cpuStatsMu     sync.Mutex
	lastCPUTotal   uint64
	lastCPUIdle    uint64
	cpuStatsInited bool
)

type Manager struct {
	cfg            *config.Config
	grpcServer     *GRPCServer
	wsServer       *WebSocketServer // WebSocket服务器
	apiServer      *APIServer
	scheduler      *Scheduler
	dispatcher     *Dispatcher
	taskConsumer   *TaskConsumer
	logSubscriber  *LogSubscriber // Redis Pub/Sub 日志订阅器
	consumerCancel context.CancelFunc
	healthChecker  *HealthChecker
	healthCancel   context.CancelFunc
	logSubCancel   context.CancelFunc // 日志订阅器取消函数
	managerNodeID   string // Manager 节点自己的 ID
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg: cfg,
	}
}

func (m *Manager) Start() error {
	logger.Info("启动 Manager 节点...")

	// 启动核心服务（必须先启动 WebSocket 服务器）
	if err := m.startServices(); err != nil {
		return err
	}

	// 注册 Manager 自己为节点（在 WebSocket 启动之后）
	if err := m.registerManagerAsNode(); err != nil {
		logger.Warn("注册 Manager 节点失败", zap.Error(err))
	}

	// 启动 Manager 心跳
	go m.startManagerHeartbeat()

	return nil
}

func (m *Manager) startServices() error {
	logger.Info("启动 Manager 核心服务...")

	m.wsServer = NewWebSocketServer(m.cfg.Manager.WebSocketPort)
	if err := m.wsServer.Start(); err != nil {
		return err
	}

	m.grpcServer = NewGRPCServer(m.cfg)
	m.grpcServer.SetWebSocketServer(m.wsServer) // 设置WebSocket服务器
	if err := m.grpcServer.Start(); err != nil {
		return err
	}

	m.dispatcher = NewDispatcher(m.wsServer)

	healthCtx, healthCancel := context.WithCancel(context.Background())
	m.healthCancel = healthCancel
	m.healthChecker = NewHealthChecker(
		&m.cfg.Manager.Heartbeat,
		&m.cfg.Storage,
		m.dispatcher,
		m.grpcServer,
		m.wsServer,
	)
	go m.healthChecker.Start(healthCtx)

	m.taskConsumer = NewTaskConsumer(m.dispatcher, m.cfg.Manager.DispatchRetry)
	m.taskConsumer.SetWebSocketServer(m.wsServer)
	consumerCtx, cancel := context.WithCancel(context.Background())
	m.consumerCancel = cancel
	go m.taskConsumer.Start(consumerCtx)

	m.scheduler = NewScheduler(&m.cfg.Manager.Scheduler)
	if err := m.scheduler.Start(); err != nil {
		return err
	}

	m.apiServer = NewAPIServer(m.cfg, m.scheduler, m.dispatcher)
	m.apiServer.SetWebSocketServer(m.wsServer)

	m.apiServer.logBuffer = NewLogBuffer(5000)
	logger.WrapCore(func(core zapcore.Core) zapcore.Core {
		return &logHookCore{Core: core, buffer: m.apiServer.logBuffer}
	})

	go m.broadcastManagerLogs()

	if err := m.apiServer.Start(); err != nil {
		return err
	}

	m.logSubscriber = NewLogSubscriber(m.wsServer)
	logSubCtx, logSubCancel := context.WithCancel(context.Background())
	m.logSubCancel = logSubCancel
	m.logSubscriber.Start(logSubCtx)

	go m.grpcServer.RecoverOrphanLogs(context.Background())

	logger.Info("Manager 核心服务启动完成")
	return nil
}

func (m *Manager) Stop() {
	logger.Info("停止 Manager 节点...")

	if m.managerNodeID != "" {
		storage.DB.Model(&models.Node{}).Where("id = ?", m.managerNodeID).Update("status", "offline")
	}

	if m.consumerCancel != nil {
		m.consumerCancel()
	}
	if m.taskConsumer != nil {
		if ok := m.taskConsumer.Wait(10 * time.Second); !ok {
			logger.Warn("任务消费者停止超时")
		}
	}

	if m.healthCancel != nil {
		m.healthCancel()
	}
	if m.healthChecker != nil {
		if ok := m.healthChecker.Wait(10 * time.Second); !ok {
			logger.Warn("健康检查器停止超时")
		}
	}

	if m.logSubCancel != nil {
		m.logSubCancel()
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

	logger.Info("Manager 节点已停止")
}

func (m *Manager) registerManagerAsNode() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("获取主机名失败: %w", err)
	}

	ip := getLocalIP()
	pid := int32(os.Getpid())
	resources, _ := m.getResourceInfo()

	var existingNode models.Node
	err = storage.DB.Where("hostname = ? AND ip = ?", hostname, ip).First(&existingNode).Error

	var nodeID string
	if err == nil {
		nodeID = existingNode.ID

		updates := map[string]interface{}{
			"tags":            "manager",
			"pid":             pid,
			"status":          "online",
			"last_heartbeat":   time.Now(),
			"cpu_usage":       resources.CpuUsage,
			"cpu_cores":       resources.CpuCores,
			"memory_usage":    resources.MemoryUsage,
			"memory_total":    resources.MemoryTotal,
			"memory_percent":  calculatePercent(resources.MemoryUsage, resources.MemoryTotal),
			"disk_usage":      resources.DiskUsage,
			"disk_total":      resources.DiskTotal,
			"disk_percent":    calculatePercent(resources.DiskUsage, resources.DiskTotal),
		}
		if err := storage.DB.Model(&models.Node{}).Where("id = ?", nodeID).Updates(updates).Error; err != nil {
			return fmt.Errorf("更新 Manager 节点失败: %w", err)
		}
		logger.Info("更新现有节点为 Manager",
			zap.String("node_id", nodeID),
			zap.String("hostname", hostname),
			zap.String("ip", ip),
			zap.String("old_tags", existingNode.Tags),
			zap.Int32("pid", pid))
	} else {
		nodeID = utils.GenerateID("node")
		node := &models.Node{
			ID:             nodeID,
			Hostname:       hostname,
			IP:             ip,
			GRPCAddress:    fmt.Sprintf("%s:%d", m.cfg.Manager.Host, m.cfg.Manager.GRPCPort),
			Tags:           "manager",
			PID:            pid,
			Status:         "online",
			Version:        "0.1.0",
			LastHeartbeat:  time.Now(),
			CPUUsage:       resources.CpuUsage,
			CPUCores:       int(resources.CpuCores),
			MemoryUsage:    resources.MemoryUsage,
			MemoryTotal:    resources.MemoryTotal,
			MemoryPercent:  calculatePercent(resources.MemoryUsage, resources.MemoryTotal),
			DiskUsage:      resources.DiskUsage,
			DiskTotal:      resources.DiskTotal,
			DiskPercent:    calculatePercent(resources.DiskUsage, resources.DiskTotal),
		}
		if err := storage.DB.Create(node).Error; err != nil {
			return fmt.Errorf("创建 Manager 节点失败: %w", err)
		}
		logger.Info("创建新的 Manager 节点记录",
			zap.String("node_id", nodeID),
			zap.String("hostname", hostname),
			zap.String("ip", ip),
			zap.Int32("pid", pid))
	}

	m.managerNodeID = nodeID

	if m.wsServer != nil {
		if err := m.wsServer.BroadcastNodeStatus(nodeID, hostname, "online", 0, 0, 0); err != nil {
			logger.Warn("推送 Manager 节点状态失败", zap.Error(err))
		} else {
			logger.Info("已推送 Manager 节点上线状态", zap.String("node_id", nodeID))
		}
	}

	return nil
}

func (m *Manager) broadcastManagerLogs() {
	if m.apiServer.logBuffer == nil || m.wsServer == nil {
		return
	}
	ch := m.apiServer.logBuffer.Subscribe()
	defer m.apiServer.logBuffer.Unsubscribe(ch)

	existing := m.apiServer.logBuffer.GetEntries()
	for _, entry := range existing {
		msg := ServerMessage{
			Type: "manager_log",
			Data: entry,
		}
		_ = m.wsServer.GetHub().BroadcastToRoom("manager:logs", msg)
	}

	for entry := range ch {
		msg := ServerMessage{
			Type: "manager_log",
			Data: entry,
		}
		_ = m.wsServer.GetHub().BroadcastToRoom("manager:logs", msg)
	}
}

func (m *Manager) startManagerHeartbeat() {
	if err := m.updateManagerHeartbeat(); err != nil {
		logger.Error("初始 Manager 心跳更新失败", zap.Error(err))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := m.updateManagerHeartbeat(); err != nil {
			logger.Error("更新 Manager 心跳失败", zap.Error(err))
		}
	}
}

func (m *Manager) updateManagerHeartbeat() error {
	if m.managerNodeID == "" {
		return nil
	}

	resources, err := m.getResourceInfo()
	if err != nil {
		logger.Warn("获取 Manager 资源信息失败", zap.Error(err))
		resources = &nodeResources{}
	}

	updates := map[string]interface{}{
		"last_heartbeat":   time.Now(),
		"status":           "online",
		"cpu_usage":        resources.CpuUsage,
		"cpu_cores":        resources.CpuCores,
		"memory_usage":     resources.MemoryUsage,
		"memory_total":     resources.MemoryTotal,
		"memory_percent":   calculatePercent(resources.MemoryUsage, resources.MemoryTotal),
		"disk_usage":       resources.DiskUsage,
		"disk_total":       resources.DiskTotal,
		"disk_percent":     calculatePercent(resources.DiskUsage, resources.DiskTotal),
	}

	if err := storage.DB.Model(&models.Node{}).Where("id = ?", m.managerNodeID).Updates(updates).Error; err != nil {
		return err
	}

	return nil
}

func (m *Manager) getResourceInfo() (*nodeResources, error) {
	cpuUsage, err := getCPUUsagePercent()
	if err != nil {
		return nil, err
	}

	memUsedGB, memTotalGB, err := getMemoryUsageGB()
	if err != nil {
		return nil, err
	}

	diskUsedGB, diskTotalGB, err := getDiskUsageGB("/")
	if err != nil {
		return nil, err
	}

	return &nodeResources{
		CpuUsage:    cpuUsage,
		MemoryUsage: memUsedGB,
		MemoryTotal: memTotalGB,
		DiskUsage:   diskUsedGB,
		DiskTotal:   diskTotalGB,
		CpuCores:    int32(runtime.NumCPU()),
	}, nil
}

func getLocalIP() string {
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

	return "127.0.0.1"
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
