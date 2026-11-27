// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package qualityapi defines the stable anti-corruption contract between SmartDNS
// and underlying network quality estimation / measurement engines.
package qualityapi

import "time"

const (
	// CompactSnapshotSchemaVersion is the wire schema version for CompactQualitySnapshot.
	CompactSnapshotSchemaVersion = 1

	// SourceTierMeasuredRegion indicates direct region-level empirical measurements.
	SourceTierMeasuredRegion = "measured_region"
	// SourceTierMeasuredCountry indicates direct country-level empirical measurements.
	SourceTierMeasuredCountry = "measured_country"
	// SourceTierInferred indicates residual/similarity/sibling inference without direct measurement.
	SourceTierInferred = "inferred"
	// SourceTierGeoPrior indicates a pure geographic or unknown baseline fallback.
	SourceTierGeoPrior = "geo_prior"

	// DefaultUnknownRTTMs is the conservative baseline RTT returned when no topology or geo data exists.
	DefaultUnknownRTTMs = 120.0
	// DefaultUnknownP95Ms is the conservative P95 RTT returned when no topology or geo data exists.
	DefaultUnknownP95Ms = 150.0
	// DefaultUnknownConfidence is the floor confidence when no evidence exists.
	DefaultUnknownConfidence = 0.05
)

// ClientProfile describes normalized client network identity and location for quality evaluation.
type ClientProfile struct {
	ClientIP      string
	ASN           uint32
	CountryISO    string
	RegionCode    string
	ContinentCode string
	IPFamily      string
	Latitude      *float64
	Longitude     *float64
	GeoAccuracyKm float64
	GeoSource     string
	AnycastStatus string
}

// NodeLinkQuality represents the unified network quality metrics from a candidate edge node to a client network.
type NodeLinkQuality struct {
	NodeID          string     `json:"node_id"`
	EffectiveRTTMs  float64    `json:"effective_rtt_ms"`
	RTTP95Ms        float64    `json:"rtt_p95_ms"`
	DispersionMs    float64    `json:"dispersion_ms"`
	LossRate        float64    `json:"loss_rate"`
	Confidence      float64    `json:"confidence"`
	QualityScore    float64    `json:"quality_score"`
	GeoBaseRTTMs    float64    `json:"geo_base_rtt_ms"`
	GeoBaseKnown    bool       `json:"geo_base_known"`
	SourceTier      string     `json:"source_tier"`
	ProviderDetail  string     `json:"provider_detail,omitempty"`
	AnycastIsolated bool       `json:"anycast_isolated,omitempty"`
	LastObservedAt  *time.Time `json:"last_observed_at,omitempty"`
}

// CompactQualitySnapshot is a versioned, compact snapshot payload distributed to standalone DNS instances.
type CompactQualitySnapshot struct {
	SchemaVersion int                         `json:"schema_version"`
	Generation    uint64                      `json:"generation"`
	GeneratedAt   time.Time                   `json:"generated_at"`
	ProviderName  string                      `json:"provider_name"`
	Entries       []CompactQualityEntry       `json:"entries"`
	NodeDefaults  map[string]CompactNodePrior `json:"node_defaults"`
}

// CompactQualityEntry encodes one measured matrix slice with compact JSON field names.
type CompactQualityEntry struct {
	NodeID         string  `json:"n"`
	ClientASN      uint32  `json:"a"`
	ClientCountry  string  `json:"c"`
	ClientRegion   string  `json:"r,omitempty"`
	IPFamily       string  `json:"f"`
	Level          string  `json:"l"`
	EffectiveRTTMs float64 `json:"rtt"`
	RTTP95Ms       float64 `json:"p95"`
	DispersionMs   float64 `json:"disp,omitempty"`
	LossRate       float64 `json:"loss,omitempty"`
	Confidence     float64 `json:"conf"`
	GeoBaseRTTMs   float64 `json:"geo_rtt,omitempty"`
	GeoBaseKnown   bool    `json:"geo_ok,omitempty"`
	SourceTier     string  `json:"tier"`
}

// CompactNodePrior carries per-node baseline geometry and median residual for O(1) Rust fallback.
type CompactNodePrior struct {
	NetworkGroup string   `json:"network_group,omitempty"`
	ASN          uint32   `json:"asn,omitempty"`
	Latitude     *float64 `json:"lat,omitempty"`
	Longitude    *float64 `json:"lon,omitempty"`
	MedianExtra  float64  `json:"median_extra,omitempty"`
}
