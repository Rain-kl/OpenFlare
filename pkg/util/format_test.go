package util

import (
	"testing"
)

func TestBytes2Size(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{2048, "2 KB"},
		{1024 * 1024, "1 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 2, "2.00 GB"},
	}

	for _, tt := range tests {
		result := Bytes2Size(tt.input)
		if result != tt.expected {
			t.Errorf("Bytes2Size(%d) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSeconds2Time(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0 秒"},
		{30, "30 秒"},
		{60, "1 分钟 0 秒"},
		{125, "2 分钟 5 秒"},
		{3600, "1 小时 0 秒"},
		{86400, "1 天 0 秒"},
	}

	for _, tt := range tests {
		result := Seconds2Time(tt.input)
		if result != tt.expected {
			t.Errorf("Seconds2Time(%d) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
