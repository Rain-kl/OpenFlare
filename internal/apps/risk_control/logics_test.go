// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"testing"
	"time"
)

func TestAccessLogMaxFlushWaitInRange(t *testing.T) {
	t.Parallel()
	if accessLogMaxFlushWait < 2*time.Second || accessLogMaxFlushWait > 5*time.Second {
		t.Fatalf("accessLogMaxFlushWait = %v, want in [2s, 5s]", accessLogMaxFlushWait)
	}
}

func TestLogWriterStatsWhenNil(t *testing.T) {
	t.Parallel()
	reset := SetLogWriterForTest(nil)
	t.Cleanup(reset)

	stats := LogWriterStats()
	if stats.Name != "user_access_logs" {
		t.Fatalf("LogWriterStats().Name = %q, want user_access_logs", stats.Name)
	}
	if stats.Running {
		t.Fatal("LogWriterStats().Running = true for nil writer, want false")
	}
}
