package sysmetrics

import (
	"syscall"
)

// getDiskUsageGB 获取磁盘使用量和总量（GB）
// syscall.Statfs 是容器感知的，因为它基于文件系统命名空间
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
