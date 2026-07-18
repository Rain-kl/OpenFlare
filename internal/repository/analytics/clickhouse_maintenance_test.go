// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTableTTLCutoff(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	got := tableTTLCutoff(30, now)
	assert.Equal(t, now.Add(-30*24*time.Hour), got)

	got = tableTTLCutoff(90, now)
	assert.Equal(t, now.Add(-90*24*time.Hour), got)

	// Invalid TTL floors to 1 day.
	got = tableTTLCutoff(0, now)
	assert.Equal(t, now.Add(-24*time.Hour), got)
}

func TestCleanupModeConstants(t *testing.T) {
	assert.Equal(t, "ttl_materialize", CleanupModeTTLMaterialize)
	assert.Equal(t, "truncate", CleanupModeTruncate)
}

func TestTableTTLDaysMatchDDL(t *testing.T) {
	assert.Equal(t, 90, TableTTLDaysNodeAccessLogs)
	assert.Equal(t, 30, TableTTLDaysNodeMetricSnapshots)
	assert.Equal(t, 30, TableTTLDaysNodeObs)
	assert.Equal(t, 180, TableTTLDaysUserAccessLogs)
}
