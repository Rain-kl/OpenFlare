// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

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
	var numStr string
	unit := "B"
	switch {
	case num/int64(sizeGB) >= 1:
		numStr = fmt.Sprintf("%.2f", float64(num)/float64(sizeGB))
		unit = "GB"
	case num/int64(sizeMB) >= 1:
		numStr = strconv.Itoa(int(float64(num) / float64(sizeMB)))
		unit = "MB"
	case num/int64(sizeKB) >= 1:
		numStr = strconv.Itoa(int(float64(num) / float64(sizeKB)))
		unit = "KB"
	default:
		numStr = strconv.FormatInt(num, 10)
	}
	return numStr + " " + unit
}

// Seconds2Time converts a number of seconds to a human-readable English duration string.
func Seconds2Time(num int) (time string) {
	if num/secondsPerYear > 0 {
		time += durationPart(num/secondsPerYear, "year") + " "
		num %= secondsPerYear
	}
	if num/secondsPerMonth > 0 {
		time += durationPart(num/secondsPerMonth, "month") + " "
		num %= secondsPerMonth
	}
	if num/secondsPerDay > 0 {
		time += durationPart(num/secondsPerDay, "day") + " "
		num %= secondsPerDay
	}
	if num/secondsPerHour > 0 {
		time += durationPart(num/secondsPerHour, "hour") + " "
		num %= secondsPerHour
	}
	if num/secondsPerMinute > 0 {
		time += durationPart(num/secondsPerMinute, "minute") + " "
		num %= secondsPerMinute
	}
	time += durationPart(num, "second")
	return
}

func durationPart(value int, unit string) string {
	if value != 1 {
		unit += "s"
	}
	return strconv.Itoa(value) + " " + unit
}
