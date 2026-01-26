package utils

import (
	"time"
)

// UnixToTime 将 Unix 时间戳转换为 time.Time
func UnixToTime(timestamp int64) *time.Time {
	if timestamp == 0 {
		return nil
	}
	t := time.Unix(timestamp, 0)
	return &t
}

// TimeToUnix 将 time.Time 转换为 Unix 时间戳
func TimeToUnix(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}
