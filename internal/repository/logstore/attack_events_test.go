// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/require"
)

func TestAttackEventsMatchDrillDownAndExcludeOrdinaryAccessLogs(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	store := newTestGormStore(t)
	ctx := context.Background()
	bucket := time.Unix(1_800_000_000, 0).UTC()
	fixture := analyticsmodel.NodeAccessLog{
		NodeID: "edge-a", Host: "app.example.com", RemoteAddr: "192.0.2.2", LoggedAt: bucket.Add(time.Second),
		StatusCode: 403, WAFAction: "blocked", WAFAudit: `{"action":"blocked","flow_name":"fixture"}`,
	}
	second := fixture
	second.NodeID, second.LoggedAt = "edge-b", bucket.Add(2*time.Second)
	later := fixture
	later.LoggedAt = fixture.LoggedAt.Add(5 * time.Minute)
	otherIP := fixture
	otherIP.RemoteAddr = "192.0.2.22"
	observed := fixture
	observed.WAFAction, observed.StatusCode = "observed", 200
	ordinary := fixture
	ordinary.WAFAction, ordinary.WAFAudit = "", ""
	require.NoError(t, store.BatchInsertNodeAccessLogs(ctx, []analyticsmodel.NodeAccessLog{fixture, second, later, otherIP, observed, ordinary}))

	query := model.OpenFlareAccessLogQuery{WAFAction: "blocked", PageSize: 10}
	events, total, err := store.AttackEvents(ctx, query, 300)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, events, 3)
	require.Equal(t, model.OpenFlareAttackEvent{Host: fixture.Host, RemoteAddr: fixture.RemoteAddr, BucketUnix: bucket.Unix(), FirstUnix: fixture.LoggedAt.Unix(), LastUnix: second.LoggedAt.Unix(), Count: 2}, events[1])

	query.Since, query.Until = bucket, bucket.Add(5*time.Minute)
	query.Hosts = []string{events[1].Host}
	query.RemoteAddr, query.ExactRemoteAddr = events[1].RemoteAddr, true
	rows, err := store.List(ctx, query)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, fixture.WAFAudit, rows[0].WAFAudit)
	require.Equal(t, second.NodeID, rows[0].NodeID)
	query.NodeID = fixture.NodeID
	rows, err = store.List(ctx, query)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
