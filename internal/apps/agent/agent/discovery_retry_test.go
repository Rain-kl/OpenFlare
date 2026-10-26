// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"testing"
	"time"
)

func TestDiscoveryRetryBackoffIsIndependentOfHeartbeat(t *testing.T) {
	now := time.Unix(100, 0)
	var retry discoveryRetry
	for _, delay := range []time.Duration{3, 6, 12, 24, 48, 60, 60} {
		if !retry.ready(now) {
			t.Fatal("Discovery retry not ready at scheduled time")
		}
		retry.failed(now)
		wantNext := now.Add(delay * time.Second)
		if retry.next != wantNext {
			t.Errorf("Discovery retry next = %v, want %v", retry.next, wantNext)
		}
		for simulatedHeartbeat := now; simulatedHeartbeat.Before(wantNext); simulatedHeartbeat = simulatedHeartbeat.Add(time.Millisecond) {
			if retry.ready(simulatedHeartbeat) {
				t.Fatal("Discovery retry ready before independent enrollment backoff expires")
			}
		}
		now = wantNext
	}
}
