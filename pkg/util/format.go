// Package util provides shared formatting and string helper functions.
package util

import (
	"fmt"
	"strconv"
)

const (
	secondsPerYear   = 31104000 // 360 days
	secondsPerMonth  = 2592000  // 30 days
	secondsPerDay    = 86400
	secondsPerHour   = 3600
	secondsPerMinute = 60
)

const (
	sizeKB = 1024
	sizeMB = sizeKB * 1024
	sizeGB = sizeMB * 1024
)

// Bytes2Size converts a byte count to a human-readable string with unit (B, KB, MB, GB).
func Bytes2Size(num int64) string {
	numStr := ""
	unit := "B"
	switch {
	case num/int64(sizeGB) >= 1:
		numStr = fmt.Sprintf("%.2f", float64(num)/float64(sizeGB))
		unit = "GB"
	case num/int64(sizeMB) >= 1:
		numStr = fmt.Sprintf("%d", int(float64(num)/float64(sizeMB)))
		unit = "MB"
	case num/int64(sizeKB) >= 1:
		numStr = fmt.Sprintf("%d", int(float64(num)/float64(sizeKB)))
		unit = "KB"
	default:
		numStr = fmt.Sprintf("%d", num)
	}
	return numStr + " " + unit
}

// Seconds2Time converts a number of seconds to a human-readable Chinese duration string.
func Seconds2Time(num int) (time string) {
	if num/secondsPerYear > 0 {
		time += strconv.Itoa(num/secondsPerYear) + " 年 "
		num %= secondsPerYear
	}
	if num/secondsPerMonth > 0 {
		time += strconv.Itoa(num/secondsPerMonth) + " 个月 "
		num %= secondsPerMonth
	}
	if num/secondsPerDay > 0 {
		time += strconv.Itoa(num/secondsPerDay) + " 天 "
		num %= secondsPerDay
	}
	if num/secondsPerHour > 0 {
		time += strconv.Itoa(num/secondsPerHour) + " 小时 "
		num %= secondsPerHour
	}
	if num/secondsPerMinute > 0 {
		time += strconv.Itoa(num/secondsPerMinute) + " 分钟 "
		num %= secondsPerMinute
	}
	time += strconv.Itoa(num) + " 秒"
	return
}
