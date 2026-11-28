// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
)

// SmartDNS wire protocol headers and WebSocket message types.
const (
	SmartDNSContractVersion = 1

	HeaderDNSToken          = "X-DNS-Token"           //nolint:gosec // HTTP header name, not a credential
	HeaderDNSDiscoveryToken = "X-DNS-Discovery-Token" //nolint:gosec // HTTP header name, not a credential

	DNSWSMessageTypeSnapshotChanged = "dns_snapshot_changed"
	DNSWSMessageTypeDisconnect      = "dns_disconnect"
)

// DNSInstanceRegisterRequest self-registers a standalone openflare-dns instance via discovery token.
type DNSInstanceRegisterRequest struct {
	InstanceID   string   `json:"instance_id,omitempty"`
	Name         string   `json:"name"`
	PublicAddr   string   `json:"public_addr,omitempty"`
	ListenAddr   string   `json:"listen_addr,omitempty"`
	GeoName      string   `json:"geo_name,omitempty"`
	GeoLatitude  *float64 `json:"geo_latitude,omitempty"`
	GeoLongitude *float64 `json:"geo_longitude,omitempty"`
	Version      string   `json:"version,omitempty"`
}

// DNSInstanceRegisterResponse returns assigned instance identity and bearer token after registration.
type DNSInstanceRegisterResponse struct {
	ID          uint   `json:"id"`
	InstanceID  string `json:"instance_id"`
	Name        string `json:"name"`
	AccessToken string `json:"access_token"`
}

// DNSHeartbeatRequest reports periodic throughput, latency percentiles, and system health from a SmartDNS worker.
type DNSHeartbeatRequest struct {
	Version               string            `json:"version,omitempty"`
	PublicAddr            string            `json:"public_addr,omitempty"`
	ListenAddr            string            `json:"listen_addr,omitempty"`
	SnapshotVersion       uint64            `json:"snapshot_version"`
	SnapshotETag          string            `json:"snapshot_etag,omitempty"`
	QPS                   float64           `json:"qps"`
	P50Ms                 float64           `json:"p50_ms"`
	P95Ms                 float64           `json:"p95_ms"`
	P99Ms                 float64           `json:"p99_ms"`
	QueriesTotal          uint64            `json:"queries_total"`
	QueriesSuccess        uint64            `json:"queries_success"`
	QueriesNXDomain       uint64            `json:"queries_nxdomain"`
	QueriesServFail       uint64            `json:"queries_servfail"`
	QueriesRateLimited    uint64            `json:"queries_ratelimited"`
	CacheHitRatio         float64           `json:"cache_hit_ratio"`
	CPUUsagePercent       float64           `json:"cpu_usage_percent"`
	MemoryUsagePercent    float64           `json:"memory_usage_percent"`
	LastError             string            `json:"last_error,omitempty"`
	CapturedAt            *time.Time        `json:"captured_at,omitempty"`
	RemoteUpdateSupported bool              `json:"remote_update_supported,omitempty"`
	UpdateReceipt         *DNSUpdateReceipt `json:"update_receipt,omitempty"`
}

// DNSHeartbeatResponse acknowledges heartbeat ingestion and reports latest control-plane snapshot metadata.
type DNSHeartbeatResponse struct {
	OK              bool              `json:"ok"`
	SnapshotVersion uint64            `json:"snapshot_version"`
	SnapshotETag    string            `json:"snapshot_etag"`
	Update          *DNSUpdateCommand `json:"update,omitempty"`
}

// DNSUpdateCommand identifies a release update a worker should stage.
type DNSUpdateCommand struct {
	ID      string `json:"id"`
	Channel string `json:"channel"`
}

// DNSUpdateReceipt reports the progress of a staged update.
type DNSUpdateReceipt struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// DNSSnapshotEnvelope is the immutable compiled runtime snapshot consumed by Go and Rust SmartDNS data planes.
type DNSSnapshotEnvelope struct {
	Version          uint64                             `json:"version"`
	ETag             string                             `json:"etag"`
	GeneratedAt      time.Time                          `json:"generated_at"`
	DelegationDomain string                             `json:"delegation_domain"`
	NSHosts          []string                           `json:"ns_hosts"`
	DefaultTTL       int                                `json:"default_ttl"`
	Nodes            []DNSSnapshotNode                  `json:"nodes"`
	Policies         []DNSSnapshotPolicy                `json:"policies"`
	DomainRoutes     []DNSSnapshotDomainRoute           `json:"domain_routes"`
	Quality          *qualityapi.CompactQualitySnapshot `json:"quality"`
}

// DNSSnapshotTag describes a node's resolved tag binding inside the immutable runtime snapshot.
type DNSSnapshotTag struct {
	Slug            string  `json:"slug"`
	Name            string  `json:"name"`
	EffectiveWeight float64 `json:"effective_weight"`
}

// DNSSnapshotNode represents one edge node candidate in the compiled DNS runtime snapshot.
type DNSSnapshotNode struct {
	NodeID                string           `json:"node_id"`
	Name                  string           `json:"name"`
	IPv4                  string           `json:"ipv4"`
	IPv6                  string           `json:"ipv6,omitempty"`
	Status                string           `json:"status"`
	Quarantined           bool             `json:"quarantined"`
	NetworkGroup          string           `json:"network_group,omitempty"`
	ASN                   uint32           `json:"asn"`
	Country               string           `json:"country,omitempty"`
	GeoLatitude           *float64         `json:"geo_latitude,omitempty"`
	GeoLongitude          *float64         `json:"geo_longitude,omitempty"`
	CapacityBandwidthMbps int              `json:"capacity_bandwidth_mbps"`
	CapacityConnections   int              `json:"capacity_connections"`
	CapacityWeight        int              `json:"capacity_weight"`
	CPUUsagePercent       float64          `json:"cpu_usage_percent"`
	MemoryUsagePercent    float64          `json:"memory_usage_percent"`
	BandwidthMbps         float64          `json:"bandwidth_mbps"`
	ActiveConnections     int64            `json:"active_connections"`
	Tags                  []DNSSnapshotTag `json:"tags"`
}

// DNSSnapshotPolicy contains a compiled SmartDNS policy and its GoRules JDM definition.
type DNSSnapshotPolicy struct {
	ID              uint     `json:"id"`
	Name            string   `json:"name"`
	ScopeType       string   `json:"scope_type"`
	ScopeDomains    []string `json:"scope_domains"`
	StrategyType    string   `json:"strategy_type"`
	Priority        int      `json:"priority"`
	TTL             int      `json:"ttl"`
	MaxAnswers      int      `json:"max_answers"`
	IsDefault       bool     `json:"is_default"`
	FallbackStrat   string   `json:"fallback_strategy"`
	FallbackNodeIDs []string `json:"fallback_node_ids"`
	JDMJSON         string   `json:"jdm_json"`
	Revision        int      `json:"revision"`
}

// DNSSnapshotDomainRoute binds a delegated SmartDNS CNAME target host and its Cloudflare member domains to a policy.
type DNSSnapshotDomainRoute struct {
	TargetHost    string   `json:"target_host"`
	PolicyID      uint     `json:"policy_id"`
	GroupID       uint     `json:"group_id"`
	MemberDomains []string `json:"member_domains"`
}
