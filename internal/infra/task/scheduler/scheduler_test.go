// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package scheduler

import "testing"

func TestWaitForStop(t *testing.T) {
	tests := []struct {
		name        string
		closeDone   bool
		closeSignal bool
		wantSignal  bool
	}{
		{
			name:      "explicit stop",
			closeDone: true,
		},
		{
			name:        "process signal",
			closeSignal: true,
			wantSignal:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			signals := make(chan struct{})
			if tt.closeDone {
				close(done)
			}
			if tt.closeSignal {
				close(signals)
			}

			if got := waitForStop(done, signals); got != tt.wantSignal {
				t.Errorf("waitForStop() = %t, want %t", got, tt.wantSignal)
			}
		})
	}
}
