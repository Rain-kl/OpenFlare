// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"testing"
)

func TestNormalizeTaskTrigger(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: contracts.TaskTriggerManual, want: contracts.TaskTriggerManual},
		{in: contracts.TaskTriggerSystem, want: contracts.TaskTriggerSystem},
		{in: contracts.TaskTriggerRetry, want: contracts.TaskTriggerRetry},
		{in: contracts.TaskTriggerSchedule, want: contracts.TaskTriggerSchedule},
		{in: "http", want: contracts.TaskTriggerSystem},
		{in: "inproc_cron", want: contracts.TaskTriggerSchedule},
		{in: "cron", want: contracts.TaskTriggerSchedule},
		{in: "", want: contracts.TaskTriggerSystem},
		{in: "custom", want: "custom"},
	}
	for _, tt := range tests {
		got := normalizeTaskTrigger(tt.in)
		if got != tt.want {
			t.Errorf("normalizeTaskTrigger(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
