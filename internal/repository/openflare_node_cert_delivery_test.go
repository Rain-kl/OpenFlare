// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNodeCertDeliveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&model.OpenFlareNodeCertDelivery{}))
	db.SetDB(conn)
	t.Cleanup(func() { db.SetDB(nil) })
	return conn
}

func TestUpsertAndListNodeCertDeliveries(t *testing.T) {
	setupNodeCertDeliveryTestDB(t)
	ctx := context.Background()

	require.NoError(t, UpsertNodeCertDeliveries(ctx, nil))

	t1 := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(5 * time.Minute)
	digest1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digest2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	require.NoError(t, UpsertNodeCertDeliveries(ctx, []model.OpenFlareNodeCertDelivery{
		{
			NodeID:           "node-1",
			CertificateID:    42,
			CertSHA256:       digest1,
			ReleaseSequence:  10,
			DeliveryCount:    1,
			FirstDeliveredAt: t1,
			LastDeliveredAt:  t1,
		},
	}))

	// Repeated delivery of same (node, cert, sha) increments delivery_count and updates sequence/timestamp.
	require.NoError(t, UpsertNodeCertDeliveries(ctx, []model.OpenFlareNodeCertDelivery{
		{
			NodeID:           "node-1",
			CertificateID:    42,
			CertSHA256:       digest1,
			ReleaseSequence:  11,
			DeliveryCount:    1,
			FirstDeliveredAt: t2,
			LastDeliveredAt:  t2,
		},
		{
			NodeID:           "node-1",
			CertificateID:    42,
			CertSHA256:       digest2,
			ReleaseSequence:  12,
			DeliveryCount:    1,
			FirstDeliveredAt: t2,
			LastDeliveredAt:  t2,
		},
		{
			NodeID:           "node-2",
			CertificateID:    42,
			CertSHA256:       digest1,
			ReleaseSequence:  12,
			DeliveryCount:    1,
			FirstDeliveredAt: t1,
			LastDeliveredAt:  t1,
		},
	}))

	byNode, err := ListNodeCertDeliveriesByNodeID(ctx, "node-1")
	require.NoError(t, err)
	require.Len(t, byNode, 2)

	var gotDigest1 *model.OpenFlareNodeCertDelivery
	for i := range byNode {
		if byNode[i].CertSHA256 == digest1 {
			gotDigest1 = &byNode[i]
		}
	}
	require.NotNil(t, gotDigest1)
	assert.Equal(t, int64(2), gotDigest1.DeliveryCount)
	assert.Equal(t, int64(11), gotDigest1.ReleaseSequence)
	assert.Equal(t, t1.Unix(), gotDigest1.FirstDeliveredAt.Unix())
	assert.Equal(t, t2.Unix(), gotDigest1.LastDeliveredAt.Unix())

	byCert, err := ListNodeCertDeliveriesByCertificateID(ctx, 42)
	require.NoError(t, err)
	require.Len(t, byCert, 3)
}
