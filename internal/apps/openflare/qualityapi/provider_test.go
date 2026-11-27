// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package qualityapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	name string
	gen  uint64
}

func (s *stubProvider) Name() string       { return s.name }
func (s *stubProvider) Generation() uint64 { return s.gen }
func (s *stubProvider) EvaluateNode(_ context.Context, nodeID string, _ ClientProfile) NodeLinkQuality {
	return NodeLinkQuality{
		NodeID:         nodeID,
		EffectiveRTTMs: 22.5,
		RTTP95Ms:       28.0,
		Confidence:     0.91,
		QualityScore:   88.0,
		SourceTier:     SourceTierMeasuredRegion,
		ProviderDetail: "stub",
	}
}
func (s *stubProvider) EvaluateNodes(ctx context.Context, nodeIDs []string, client ClientProfile) map[string]NodeLinkQuality {
	out := make(map[string]NodeLinkQuality, len(nodeIDs))
	for _, id := range nodeIDs {
		out[id] = s.EvaluateNode(ctx, id, client)
	}
	return out
}
func (s *stubProvider) ExportSnapshot(_ context.Context, _ []string) (*CompactQualitySnapshot, error) {
	return &CompactQualitySnapshot{
		SchemaVersion: CompactSnapshotSchemaVersion,
		Generation:    s.gen,
		ProviderName:  s.name,
	}, nil
}

func TestDefaultRegistrationAndFallback(t *testing.T) {
	RegisterDefault(nil)
	t.Cleanup(func() { RegisterDefault(nil) })

	p := Default()
	assert.Equal(t, "fallback_prior", p.Name())
	assert.Equal(t, uint64(0), p.Generation())

	res := p.EvaluateNode(context.Background(), "node-1", ClientProfile{CountryISO: "CN"})
	assert.Equal(t, "node-1", res.NodeID)
	assert.Equal(t, DefaultUnknownRTTMs, res.EffectiveRTTMs)
	assert.Equal(t, SourceTierGeoPrior, res.SourceTier)

	snap, err := p.ExportSnapshot(context.Background(), []string{"node-1"})
	require.NoError(t, err)
	assert.Equal(t, CompactSnapshotSchemaVersion, snap.SchemaVersion)

	RegisterDefault(&stubProvider{name: "custom", gen: 42})
	p2 := Default()
	assert.Equal(t, "custom", p2.Name())
	assert.Equal(t, uint64(42), p2.Generation())

	batch := p2.EvaluateNodes(context.Background(), []string{"n1", "n2"}, ClientProfile{})
	require.Len(t, batch, 2)
	assert.Equal(t, 22.5, batch["n1"].EffectiveRTTMs)
	assert.Equal(t, SourceTierMeasuredRegion, batch["n2"].SourceTier)
}
