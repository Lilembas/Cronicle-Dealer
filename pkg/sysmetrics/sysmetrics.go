package sysmetrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/cronicle/cronicle-next/pkg/logger"
	"go.uber.org/zap"
)

// ResourceInfo 系统资源指标
type ResourceInfo struct {
	CPUUsage    float64 // 百分比 0-100
	CPUCores    int32
	MemoryUsed  float64 // GB
	MemoryTotal float64 // GB
	DiskUsed    float64 // GB
	DiskTotal   float64 // GB
}

// Collector 系统指标采集器，每个进程创建一个实例
type Collector struct {
	mu           sync.Mutex
	lastCPUTotal uint64
	lastCPUIdle  uint64
	lastCPUTime  time.Time

	// 启动时一次性检测，之后只读
	isContainer   bool
	cgroupVersion int // 1 或 2
}

// NewCollector 创建采集器，执行一次性的容器和 cgroup 版本检测
func NewCollector() *Collector {
	c := &Collector{
		isContainer:   detectContainer(),
		cgroupVersion: detectCgroupVersion(),
	}

	if c.isContainer {
		logger.Info("检测到容器环境",
			zap.Int("cgroup_version", c.cgroupVersion),
		)
	} else {
		logger.Info("检测到宿主机环境（非容器）")
	}

	return c
}

// GetResourceInfo 采集当前 CPU、内存、磁盘指标
// CPU 使用率基于与上次调用的差值计算，首次调用返回 0
func (c *Collector) GetResourceInfo() (*ResourceInfo, error) {
	cpuUsage, cpuCores, err := c.getCPU()
	if err != nil {
		return nil, fmt.Errorf("获取 CPU 指标失败: %w", err)
	}

	memUsed, memTotal, err := c.getMemory()
	if err != nil {
		return nil, fmt.Errorf("获取内存指标失败: %w", err)
	}

	diskUsed, diskTotal, err := getDiskUsageGB("/")
	if err != nil {
		return nil, fmt.Errorf("获取磁盘指标失败: %w", err)
	}

	envType := "host"
	if c.isContainer {
		envType = fmt.Sprintf("container(cgroupv%d)", c.cgroupVersion)
	}
	logger.Debug("资源指标采集",
		zap.String("env", envType),
		zap.Float64("cpu_usage", cpuUsage),
		zap.Int32("cpu_cores", cpuCores),
		zap.Float64("mem_used_gb", memUsed),
		zap.Float64("mem_total_gb", memTotal),
		zap.Float64("disk_used_gb", diskUsed),
		zap.Float64("disk_total_gb", diskTotal),
	)

	return &ResourceInfo{
		CPUUsage:    cpuUsage,
		CPUCores:    cpuCores,
		MemoryUsed:  memUsed,
		MemoryTotal: memTotal,
		DiskUsed:    diskUsed,
		DiskTotal:   diskTotal,
	}, nil
}
