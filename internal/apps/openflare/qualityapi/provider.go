// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package qualityapi

import (
	"context"
	"sync/atomic"
	"time"
)

const fallbackQualityScore = 40.0

// Provider is the unified quality evaluation and snapshot export contract consumed by SmartDNS.
// Underlying prediction or telemetry implementations must adapt to this interface without leaking model internals.
type Provider interface {
	Name() string
	Generation() uint64
	EvaluateNode(ctx context.Context, nodeID string, client ClientProfile) NodeLinkQuality
	EvaluateNodes(ctx context.Context, nodeIDs []string, client ClientProfile) map[string]NodeLinkQuality
	ExportSnapshot(ctx context.Context, nodeIDs []string) (*CompactQualitySnapshot, error)
}

type providerHolder struct {
	provider Provider
}

var defaultProvider atomic.Pointer[providerHolder]

// RegisterDefault sets the process-wide quality Provider used by SmartDNS.
// Wire this explicitly from internal/platform/bootstrap; passing nil resets to the built-in fallback prior.
func RegisterDefault(p Provider) {
	if p == nil {
		defaultProvider.Store(nil)
		return
	}
	defaultProvider.Store(&providerHolder{provider: p})
}

// Default returns the active quality Provider, or a zero-dependency fallback prior if none is registered.
func Default() Provider {
	if holder := defaultProvider.Load(); holder != nil && holder.provider != nil {
		return holder.provider
	}
	return FallbackProvider{}
}

// FallbackProvider returns conservative unknown-baseline estimates when no live quality provider is wired.
type FallbackProvider struct{}

// Name returns the fallback provider identifier.
func (FallbackProvider) Name() string {
	return "fallback_prior"
}

// Generation returns 0 for the stateless fallback provider.
func (FallbackProvider) Generation() uint64 {
	return 0
}

// EvaluateNode returns a conservative baseline quality result for a single node.
func (FallbackProvider) EvaluateNode(_ context.Context, nodeID string, _ ClientProfile) NodeLinkQuality {
	return DefaultFallbackQuality(nodeID)
}

// EvaluateNodes returns conservative baseline quality results for all requested nodes.
func (FallbackProvider) EvaluateNodes(_ context.Context, nodeIDs []string, _ ClientProfile) map[string]NodeLinkQuality {
	out := make(map[string]NodeLinkQuality, len(nodeIDs))
	for _, id := range nodeIDs {
		out[id] = DefaultFallbackQuality(id)
	}
	return out
}

// ExportSnapshot returns an empty valid CompactQualitySnapshot for cold-start safety.
func (FallbackProvider) ExportSnapshot(_ context.Context, _ []string) (*CompactQualitySnapshot, error) {
	return &CompactQualitySnapshot{
		SchemaVersion: CompactSnapshotSchemaVersion,
		Generation:    0,
		GeneratedAt:   time.Now().UTC(),
		ProviderName:  "fallback_prior",
		Entries:       []CompactQualityEntry{},
		NodeDefaults:  map[string]CompactNodePrior{},
	}, nil
}

// DefaultFallbackQuality builds a neutral conservative NodeLinkQuality when no prediction engine is active.
func DefaultFallbackQuality(nodeID string) NodeLinkQuality {
	return NodeLinkQuality{
		NodeID:         nodeID,
		EffectiveRTTMs: DefaultUnknownRTTMs,
		RTTP95Ms:       DefaultUnknownP95Ms,
		Confidence:     DefaultUnknownConfidence,
		QualityScore:   fallbackQualityScore,
		SourceTier:     SourceTierGeoPrior,
		ProviderDetail: "fallback",
	}
}
