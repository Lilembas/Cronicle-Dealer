package sysmetrics

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readUintFromFile(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	return val, nil
}

func readIntFromFile(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	return val, nil
}
