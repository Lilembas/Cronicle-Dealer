package sysmetrics

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func (c *Collector) getCPU() (float64, int32, error) {
	if !c.isContainer {
		return c.getCPUHost()
	}

	cores := c.getCPUCoresCgroup()
	usageNanos, err := c.readCgroupCPUUsage()
	if err != nil {
		return c.getCPUHost()
	}
	return c.computeCPUFromCgroup(usageNanos, cores)
}

// --- 宿主机：/proc/stat ---

func (c *Collector) getCPUHost() (float64, int32, error) {
	total, idle, err := ReadCPUStat()
	if err != nil {
		return 0, 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lastCPUTime.IsZero() {
		c.lastCPUTotal = total
		c.lastCPUIdle = idle
		c.lastCPUTime = time.Now()
		return 0, int32(runtime.NumCPU()), nil
	}

	totalDelta := total - c.lastCPUTotal
	idleDelta := idle - c.lastCPUIdle
	c.lastCPUTotal = total
	c.lastCPUIdle = idle

	if totalDelta == 0 {
		return 0, int32(runtime.NumCPU()), nil
	}

	usage := (1 - float64(idleDelta)/float64(totalDelta)) * 100
	return clampPercent(usage), int32(runtime.NumCPU()), nil
}

// ReadCPUStat 读取 /proc/stat 第一行的全局 CPU 时间
func ReadCPUStat() (total uint64, idle uint64, err error) {
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

	for i := 1; i < len(fields); i++ {
		v, parseErr := strconv.ParseUint(fields[i], 10, 64)
		if parseErr != nil {
			return 0, 0, parseErr
		}
		total += v
	}

	idle, err = strconv.ParseUint(fields[4], 10, 64)
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

// --- cgroup CPU 采集 ---

// readCgroupCPUUsage 按 cgroup 版本读取累计 CPU 使用时间（纳秒）
func (c *Collector) readCgroupCPUUsage() (uint64, error) {
	if c.cgroupVersion == 2 {
		micros, err := readCgroupV2CPUUsage()
		if err != nil {
			return 0, err
		}
		return micros * 1000, nil
	}
	return readUintFromFile("/sys/fs/cgroup/cpuacct/cpuacct.usage")
}

// computeCPUFromCgroup 基于累计使用量差值计算 CPU 使用率百分比
func (c *Collector) computeCPUFromCgroup(usageNanos uint64, cores int32) (float64, int32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if c.lastCPUTime.IsZero() {
		c.lastCPUTotal = usageNanos
		c.lastCPUTime = now
		return 0, cores, nil
	}

	delta := usageNanos - c.lastCPUTotal
	c.lastCPUTotal = usageNanos

	elapsed := now.Sub(c.lastCPUTime)
	c.lastCPUTime = now
	if elapsed <= 0 {
		return 0, cores, nil
	}

	wallNanos := uint64(elapsed.Seconds()) * uint64(time.Second) * uint64(cores)
	usage := float64(delta) / float64(wallNanos) * 100
	return clampPercent(usage), cores, nil
}

func (c *Collector) getCPUCoresCgroup() int32 {
	if c.cgroupVersion == 2 {
		return getCPUCoresCgroupV2()
	}
	return getCPUCoresCgroupV1()
}

func getCPUCoresCgroupV1() int32 {
	quota, err := readIntFromFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	if err != nil || quota <= 0 {
		return int32(runtime.NumCPU())
	}
	period, err := readIntFromFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if err != nil || period <= 0 {
		return int32(runtime.NumCPU())
	}
	return calcCPUCoreFromQuotaPeriod(quota, period)
}

func getCPUCoresCgroupV2() int32 {
	data, err := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if err != nil {
		return int32(runtime.NumCPU())
	}
	parts := strings.Fields(strings.TrimSpace(string(data)))
	if len(parts) < 2 || parts[0] == "max" {
		return int32(runtime.NumCPU())
	}
	quota, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || quota <= 0 {
		return int32(runtime.NumCPU())
	}
	period, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || period <= 0 {
		return int32(runtime.NumCPU())
	}
	return calcCPUCoreFromQuotaPeriod(quota, period)
}

func calcCPUCoreFromQuotaPeriod(quota, period int64) int32 {
	cores := int32((quota + period - 1) / period)
	if cores <= 0 {
		return int32(runtime.NumCPU())
	}
	return cores
}

func readCgroupV2CPUUsage() (uint64, error) {
	f, err := os.Open("/sys/fs/cgroup/cpu.stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "usage_usec ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.ParseUint(fields[1], 10, 64)
			}
		}
	}
	return 0, fmt.Errorf("cpu.stat 中未找到 usage_usec")
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
