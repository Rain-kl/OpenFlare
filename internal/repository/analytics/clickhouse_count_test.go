// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"math"
	"testing"
)

func TestSafeInt64Count(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count uint64
		want  int64
	}{
		{name: "zero", count: 0, want: 0},
		{name: "small", count: 42, want: 42},
		{name: "max int64", count: math.MaxInt64, want: math.MaxInt64},
		{name: "overflow clamps", count: math.MaxUint64, want: math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := safeInt64Count(tt.count); got != tt.want {
				t.Fatalf("safeInt64Count(%d) = %d, want %d", tt.count, got, tt.want)
			}
		})
	}
}

