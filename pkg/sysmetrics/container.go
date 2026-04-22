package sysmetrics

import (
	"bufio"
	"os"
	"strings"
)

// detectContainer 检测当前进程是否运行在容器中
// 按优先级检查：/.dockerenv → /run/.containerenv → /proc/1/cgroup
func detectContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}

	f, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return false
	}
	defer f.Close()

	keywords := []string{"docker", "containerd", "lxc", "kubepods"}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				return true
			}
		}
	}
	return false
}

// detectCgroupVersion 检测 cgroup 版本
// /sys/fs/cgroup/cgroup.controllers 存在则为 v2，否则为 v1
func detectCgroupVersion() int {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return 2
	}
	return 1
}
