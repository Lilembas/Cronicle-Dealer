package sysmetrics

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// getMemory 采集内存使用量和总量（GB）
func (c *Collector) getMemory() (usedGB, totalGB float64, err error) {
	if !c.isContainer {
		return getMemoryHost()
	}

	if c.cgroupVersion == 2 {
		return c.getMemoryCgroupV2()
	}
	return c.getMemoryCgroupV1()
}

// --- 宿主机：/proc/meminfo ---

func getMemoryHost() (usedGB, totalGB float64, err error) {
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

// --- cgroup v1 Memory ---

func (c *Collector) getMemoryCgroupV1() (usedGB, totalGB float64, err error) {
	limitBytes, err := readUintFromFile("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil || isUnlimitedMemory(limitBytes) {
		return getMemoryHost()
	}

	usageBytes, err := readUintFromFile("/sys/fs/cgroup/memory/memory.usage_in_bytes")
	if err != nil {
		return getMemoryHost()
	}

	usedGB, totalGB = bytesToGB(limitBytes, usageBytes)
	return usedGB, totalGB, nil
}

// --- cgroup v2 Memory ---

func (c *Collector) getMemoryCgroupV2() (usedGB, totalGB float64, err error) {
	data, err := os.ReadFile("/sys/fs/cgroup/memory.max")
	if err != nil {
		return getMemoryHost()
	}
	val := strings.TrimSpace(string(data))
	if val == "max" {
		return getMemoryHost()
	}

	limitBytes, err := strconv.ParseUint(val, 10, 64)
	if err != nil || isUnlimitedMemory(limitBytes) {
		return getMemoryHost()
	}

	usageBytes, err := readUintFromFile("/sys/fs/cgroup/memory.current")
	if err != nil {
		return getMemoryHost()
	}

	usedGB, totalGB = bytesToGB(limitBytes, usageBytes)
	return usedGB, totalGB, nil
}

func bytesToGB(limitBytes, usageBytes uint64) (usedGB, totalGB float64) {
	const gbDivisor = 1024.0 * 1024.0 * 1024.0
	totalGB = float64(limitBytes) / gbDivisor
	usedGB = float64(usageBytes) / gbDivisor
	if usedGB > totalGB {
		usedGB = totalGB
	}
	return usedGB, totalGB
}

func isUnlimitedMemory(val uint64) bool {
	const maxInt64 uint64 = 9223372036854771712
	return val >= maxInt64
}

// getProcessMemory 读取特定进程的内存占用 (RSS) 字节数
func (c *Collector) getProcessMemory(pid int) (int64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}

	content := string(data)
	idx := strings.LastIndex(content, ")")
	if idx < 0 {
		return 0, fmt.Errorf("invalid stat format")
	}
	fields := strings.Fields(content[idx+2:])
	if len(fields) < 22 {
		return 0, fmt.Errorf("insufficient fields in stat")
	}

	// rss 是第 21 个字段 (从 ')' 之后算起)
	rssPages, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	return rssPages * int64(os.Getpagesize()), nil
}
