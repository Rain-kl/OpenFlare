// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	gormLogger "gorm.io/gorm/logger"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		configuredLevel string
		want            gormLogger.LogLevel
	}{
		{
			name:            "debug enables SQL trace processing",
			configuredLevel: "debug",
			want:            gormLogger.Info,
		},
		{
			name:            "development preserves configured level",
			configuredLevel: "warn",
			want:            gormLogger.Warn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLogLevel(tt.configuredLevel); got != tt.want {
				t.Fatalf("parseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
