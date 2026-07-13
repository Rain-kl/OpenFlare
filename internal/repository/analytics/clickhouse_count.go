// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import "math"

func safeInt64Count(count uint64) int64 {
	if count > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(count)
}
