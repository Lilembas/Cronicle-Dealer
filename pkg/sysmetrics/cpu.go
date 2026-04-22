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
	
	// If called too quickly (less than 100ms), results are unreliable
	if elapsed < 100*time.Millisecond {
		return 0, cores, nil
	}

	// Correct calculation using nanoseconds for both usage delta and wall time
	// wallNanos is the total available CPU nanoseconds across all cores in the elapsed period
	wallNanos := uint64(elapsed.Nanoseconds()) * uint64(cores)
	if wallNanos == 0 {
		return 0, cores, nil
	}

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

// GetMetric 采样进程及其所有子进程当前的 CPU 和内存指标之和 (进程树)
func (pc *ProcessCollector) GetMetric() (*ProcessMetric, bool) {
	// 1. 查找所有后代进程 PID
	pids := pc.getAllDescendantPIDs(pc.pid)
	pids = append(pids, pc.pid)

	var totalUTime, totalSTime uint64
	var totalRSS int64
	var foundCount int

	// 2. 累加所有进程的资源占用
	for _, pid := range pids {
		u, s, rss, err := pc.readIndividualPIDStat(pid)
		if err == nil {
			totalUTime += u
			totalSTime += s
			totalRSS += rss
			foundCount++
		}
	}

	// 如果连父进程都没找到，说明任务结束了
	if foundCount == 0 {
		return nil, false
	}

	// 3. 读取系统总 CPU 时间
	sysTotal, _, err := ReadCPUStat()
	if err != nil {
		return &ProcessMetric{CPUUsage: 0, MemoryBytes: totalRSS}, true
	}

	// 如果是首次采样，记录初始值并返回
	if !pc.initialized {
		pc.lastUTime = totalUTime
		pc.lastSTime = totalSTime
		pc.lastSysTotal = sysTotal
		pc.initialized = true
		return &ProcessMetric{CPUUsage: 0, MemoryBytes: totalRSS}, true
	}

	totalDelta := sysTotal - pc.lastSysTotal
	procDelta := (totalUTime - pc.lastUTime) + (totalSTime - pc.lastSTime)

	pc.lastUTime = totalUTime
	pc.lastSTime = totalSTime
	pc.lastSysTotal = sysTotal

	if totalDelta == 0 {
		return &ProcessMetric{CPUUsage: 0, MemoryBytes: totalRSS}, true
	}

	// 计算相对于全系统的百分比，并乘以核心数得到进程树的总百分比 (0-100% * Cores)
	usage := float64(procDelta) / float64(totalDelta) * 100.0 * float64(pc.parent.cpuCores)

	// 对于进程监控，不需要 100% 的上限，因为可以多核并行
	if usage < 0 {
		usage = 0
	}

	return &ProcessMetric{
		CPUUsage:    usage,
		MemoryBytes: totalRSS,
	}, true
}

// readIndividualPIDStat 读取单个 PID 的统计数据
func (pc *ProcessCollector) readIndividualPIDStat(pid int) (uTime, sTime uint64, rss int64, err error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, 0, err
	}

	content := string(data)
	idx := strings.LastIndex(content, ")")
	if idx < 0 {
		return 0, 0, 0, fmt.Errorf("invalid format")
	}
	fields := strings.Fields(content[idx+2:])
	if len(fields) < 22 {
		return 0, 0, 0, fmt.Errorf("insufficient fields")
	}

	uTime, _ = strconv.ParseUint(fields[11], 10, 64)
	sTime, _ = strconv.ParseUint(fields[12], 10, 64)
	rssPages, _ := strconv.ParseInt(fields[21], 10, 64)
	rss = rssPages * int64(os.Getpagesize())
	
	return uTime, sTime, rss, nil
}

// getAllDescendantPIDs 查找所有后代进程 ID（单次全量扫描模式）
func (c *ProcessCollector) getAllDescendantPIDs(rootPID int) []int {
	// 1. 一次性读取所有进程并建立 父进程 -> 子进程列表 的映射
	parentToChildren := make(map[int][]int)
	files, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	for _, f := range files {
		pid, err := strconv.Atoi(f.Name())
		if err != nil {
			continue
		}
		if ppid, ok := getParentPID(pid); ok {
			parentToChildren[ppid] = append(parentToChildren[ppid], pid)
		}
	}

	// 2. 使用广度优先搜索 (BFS) 从 rootPID 开始查找所有后代
	descendants := make([]int, 0)
	queue := []int{rootPID}
	
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		
		if children, ok := parentToChildren[curr]; ok {
			descendants = append(descendants, children...)
			queue = append(queue, children...)
		}
	}

	return descendants
}

// getParentPID 获取指定进程的父进程 PID
func getParentPID(pid int) (int, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if ppid, err := strconv.Atoi(fields[1]); err == nil {
					return ppid, true
				}
			}
			break
		}
	}
	return 0, false
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
