// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	for _, tc := range []struct {
		duration time.Duration
		want     string
	}{
		{duration: 0, want: "0 seconds"},
		{duration: time.Second, want: "1 second"},
		{duration: 61 * time.Second, want: "1 minute 1 second"},
		{duration: 26 * time.Hour, want: "1 day 2 hours"},
		{duration: 48*time.Hour + 2*time.Minute + 5*time.Second, want: "2 days 2 minutes 5 seconds"},
	} {
		if got := formatDuration(tc.duration); got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.duration, got, tc.want)
		}
	}
}
