// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

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
		{0, "0 seconds"},
		{30, "30 seconds"},
		{60, "1 minute 0 seconds"},
		{125, "2 minutes 5 seconds"},
		{3600, "1 hour 0 seconds"},
		{86400, "1 day 0 seconds"},
	}

	for _, tt := range tests {
		result := Seconds2Time(tt.input)
		if result != tt.expected {
			t.Errorf("Seconds2Time(%d) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatBinaryBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{16 * 1024, "16 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{15 * 1024 * 1024 * 1024, "15 GiB"},
		{2 * 1024 * 1024 * 1024 * 1024, "2.0 TiB"},
	}

	for _, tt := range tests {
		if got := FormatBinaryBytes(tt.input); got != tt.expected {
			t.Errorf("FormatBinaryBytes(%d) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestTrimStringFields(t *testing.T) {
	a, b := "  alpha ", "\tbeta\n"
	TrimStringFields(&a, &b, nil)
	if a != "alpha" || b != "beta" {
		t.Fatalf("TrimStringFields = (%q, %q), want (alpha, beta)", a, b)
	}
}
