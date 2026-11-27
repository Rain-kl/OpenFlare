// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func floatPtr(v float64) *float64 {
	return &v
}

func TestNodeTagCRUDAndValidation(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	tag1, err := CreateTag(ctx, TagInput{
		Name:          "CN2 优化线路",
		Slug:          "optimized",
		Color:         "emerald",
		Description:   "优质跨境优化线路",
		DefaultWeight: floatPtr(1.5),
	})
	require.NoError(t, err)
	assert.Equal(t, "optimized", tag1.Slug)
	assert.Equal(t, "emerald", tag1.Color)
	assert.InDelta(t, 1.5, tag1.DefaultWeight, 1e-6)

	_, err = CreateTag(ctx, TagInput{Name: "重复标签", Slug: "optimized"})
	require.Error(t, err)

	_, err = CreateTag(ctx, TagInput{Name: "非法标识符", Slug: "Invalid_Slug"})
	require.Error(t, err)

	_, err = CreateTag(ctx, TagInput{Name: "非法颜色", Slug: "bad-color", Color: "neon"})
	require.Error(t, err)

	_, err = CreateTag(ctx, TagInput{Name: "权重越界", Slug: "bad-weight", DefaultWeight: floatPtr(150)})
	require.Error(t, err)

	tag2, err := CreateTag(ctx, TagInput{
		Name:          "国际 BGP",
		Slug:          "international",
		DefaultWeight: floatPtr(1.0),
	})
	require.NoError(t, err)
	assert.Equal(t, "blue", tag2.Color)

	all, err := ListTags(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)

	updated, err := UpdateTag(ctx, tag2.ID, TagInput{
		Name:          "国际多线 BGP",
		Slug:          "intl-bgp",
		Color:         "sky",
		DefaultWeight: floatPtr(1.25),
	})
	require.NoError(t, err)
	assert.Equal(t, "intl-bgp", updated.Slug)
	assert.InDelta(t, 1.25, updated.DefaultWeight, 1e-6)

	require.NoError(t, DeleteTag(ctx, tag2.ID))
	require.Error(t, DeleteTag(ctx, tag2.ID))

	remaining, err := ListTags(ctx)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "optimized", remaining[0].Slug)
}

func TestNodeCapacityAndTagBindingsLifecycle(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := CreateTag(ctx, TagInput{Name: "优化", Slug: "optimized", Color: "emerald", DefaultWeight: floatPtr(1.2)})
	require.NoError(t, err)
	_, err = CreateTag(ctx, TagInput{Name: "大带宽", Slug: "high-bw", Color: "amber", DefaultWeight: floatPtr(1.5)})
	require.NoError(t, err)

	_, err = CreateNode(ctx, Input{Name: "neg-cap", CapacityBandwidthMbps: -1})
	require.Error(t, err)

	_, err = CreateNode(ctx, Input{
		Name: "unknown-tag",
		Tags: []TagBindingInput{{Slug: "does-not-exist"}},
	})
	require.Error(t, err)

	created, err := CreateNode(ctx, Input{
		Name:                  "hk-edge-1",
		IP:                    "203.0.113.10",
		CapacityBandwidthMbps: 10000,
		CapacityConnections:   50000,
		CapacityWeight:        150,
		Tags: []TagBindingInput{
			{Slug: "optimized"},
			{Slug: "high-bw", WeightOverride: floatPtr(2.2)},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 10000, created.CapacityBandwidthMbps)
	assert.Equal(t, 50000, created.CapacityConnections)
	assert.Equal(t, 150, created.CapacityWeight)
	require.Len(t, created.Tags, 2)
	assert.Equal(t, "optimized", created.Tags[0].Slug)
	assert.InDelta(t, 1.2, created.Tags[0].EffectiveWeight, 1e-6)
	assert.Equal(t, "high-bw", created.Tags[1].Slug)
	assert.InDelta(t, 2.2, created.Tags[1].EffectiveWeight, 1e-6)

	// Partial update omitting tags and capacity_weight must preserve both.
	updated, err := UpdateNode(ctx, created.ID, Input{
		Name:         "hk-edge-1-renamed",
		NetworkGroup: "asia-east",
	})
	require.NoError(t, err)
	assert.Equal(t, "hk-edge-1-renamed", updated.Name)
	assert.Equal(t, 150, updated.CapacityWeight)
	require.Len(t, updated.Tags, 2)

	// Explicit empty slice clears tag bindings.
	cleared, err := UpdateNode(ctx, created.ID, Input{
		Name:                  "hk-edge-1-renamed",
		CapacityBandwidthMbps: 20000,
		CapacityConnections:   80000,
		CapacityWeight:        200,
		Tags:                  []TagBindingInput{},
	})
	require.NoError(t, err)
	assert.Equal(t, 20000, cleared.CapacityBandwidthMbps)
	assert.Equal(t, 80000, cleared.CapacityConnections)
	assert.Equal(t, 200, cleared.CapacityWeight)
	assert.Empty(t, cleared.Tags)

	listed, err := ListNodes(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, 20000, listed[0].CapacityBandwidthMbps)
	assert.Empty(t, listed[0].Tags)
}

func TestListNodeTagSummariesMissingTableGraceful(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()

	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&model.OpenFlareNode{}))

	db.SetDB(gdb)

	byNode, err := repository.ListNodeTagSummariesByNodeIDs(context.Background(), []string{"node-1"})
	require.NoError(t, err)
	assert.Empty(t, byNode)
}
