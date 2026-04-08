package utils

import (
	"fmt"
	"math"
	"time"
)

var byteUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

// FormatBytes 格式化字节大小为人类可读格式
func FormatBytes(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	val := float64(bytes)
	exp := 0
	for val >= 1024 && exp < len(byteUnits)-1 {
		val /= 1024
		exp++
	}

	return fmt.Sprintf("%.1f %s", val, byteUnits[exp])
}

// FormatSpeed 格式化速度
func FormatSpeed(bytesPerSecond int64) string {
	return FormatBytes(bytesPerSecond) + "/s"
}

// FormatETA 格式化剩余时间 (中文友好)
func FormatETA(seconds int) string {
	if seconds < 0 {
		return "未知"
	}
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}

	d := time.Duration(seconds) * time.Second
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%d小时%d分", hours, minutes)
	}
	return fmt.Sprintf("%d分%d秒", minutes, secs)
}

// FormatDuration 格式化时长为标准时间格式 (HH:MM:SS 或 MM:SS)
func FormatDuration(seconds float64) string {
	if seconds < 0 {
		return "00:00"
	}
	
	d := time.Duration(math.Round(seconds)) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// FormatDurationSeconds 兼容性别名
func FormatDurationSeconds(seconds float64) string {
	return FormatDuration(seconds)
}
