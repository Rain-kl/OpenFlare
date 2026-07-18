package observability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAccessLogIPSummaryWindowHours(t *testing.T) {
	t.Parallel()

	since, until, hours, err := resolveAccessLogIPSummaryWindow("", "", 0)
	require.NoError(t, err)
	assert.Equal(t, defaultAccessLogQueryDays*24, hours)
	assert.WithinDuration(t, time.Now().UTC(), until, 2*time.Second)
	assert.WithinDuration(t, until.Add(-time.Duration(hours)*time.Hour), since, time.Second)

	_, _, hours, err = resolveAccessLogIPSummaryWindow("", "", 24)
	require.NoError(t, err)
	assert.Equal(t, 24, hours)

	_, _, hours, err = resolveAccessLogIPSummaryWindow("", "", 9999)
	require.NoError(t, err)
	assert.Equal(t, maxAccessLogOverviewHours, hours)
}

func TestResolveAccessLogIPSummaryWindowCustomRange(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(72 * time.Hour)
	since, until, hours, err := resolveAccessLogIPSummaryWindow(
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		24,
	)
	require.NoError(t, err)
	assert.True(t, since.Equal(start))
	assert.True(t, until.Equal(end))
	assert.Equal(t, 72, hours)
}

func TestResolveAccessLogIPSummaryWindowErrors(t *testing.T) {
	t.Parallel()

	_, _, _, err := resolveAccessLogIPSummaryWindow("2026-07-01T00:00:00Z", "", 24)
	require.Error(t, err)

	_, _, _, err = resolveAccessLogIPSummaryWindow("bad", "2026-07-02T00:00:00Z", 24)
	require.Error(t, err)

	start := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	end := start.Add(-time.Hour)
	_, _, _, err = resolveAccessLogIPSummaryWindow(
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		24,
	)
	require.Error(t, err)

	start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end = start.Add(40 * 24 * time.Hour)
	_, _, _, err = resolveAccessLogIPSummaryWindow(
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		24,
	)
	require.Error(t, err)
}

func TestNormalizeIPSummarySortBy(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "total_requests", normalizeIPSummarySortBy(""))
	assert.Equal(t, "request_length", normalizeIPSummarySortBy("bytes_received"))
	assert.Equal(t, "request_length", normalizeIPSummarySortBy("request_length"))
	assert.Equal(t, "bytes_sent", normalizeIPSummarySortBy("bytes_sent"))
	assert.Equal(t, "success_ratio", normalizeIPSummarySortBy("success_ratio"))
	assert.Equal(t, "last_seen_at", normalizeIPSummarySortBy("last_seen_at"))
	assert.Equal(t, "remote_addr", normalizeIPSummarySortBy("remote_addr"))
}
