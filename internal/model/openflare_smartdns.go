// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

const (
	// DNSInstanceModeEmbedded identifies the controller-managed local SmartDNS runner.
	DNSInstanceModeEmbedded = "embedded"
	// DNSInstanceModeStandalone identifies an externally deployed openflare-dns instance.
	DNSInstanceModeStandalone = "standalone"

	// DNSInstanceStatusPending indicates a newly created instance awaiting registration/heartbeat.
	DNSInstanceStatusPending = "pending"
	// DNSInstanceStatusOnline indicates an active instance inside the heartbeat window.
	DNSInstanceStatusOnline = "online"
	// DNSInstanceStatusOffline indicates an instance whose heartbeat timed out.
	DNSInstanceStatusOffline = "offline"
	// DNSInstanceStatusQuarantined indicates an isolated instance blocked from snapshot/WS access.
	DNSInstanceStatusQuarantined = "quarantined"

	// DNSPolicyScopeAll applies a resolution policy to every SmartDNS-managed domain.
	DNSPolicyScopeAll = "all"
	// DNSPolicyScopeDomains restricts a policy to an explicit domain / wildcard list.
	DNSPolicyScopeDomains = "domains"

	// DNSPolicyStrategySmart selects the best edge node via latency, load, geo, and tag weights.
	DNSPolicyStrategySmart = "smart"
	// DNSPolicyStrategyLoadBalance distributes traffic across nodes weighted by capacity and tags.
	DNSPolicyStrategyLoadBalance = "load_balance"
	// DNSPolicyStrategyCustom executes a compiled GoRules JDM decision graph.
	DNSPolicyStrategyCustom = "custom"

	// DNSPolicyFallbackSmart falls back to global smart selection across healthy nodes.
	DNSPolicyFallbackSmart = "smart"
	// DNSPolicyFallbackLoadBalance falls back to capacity-weighted load balancing across healthy nodes.
	DNSPolicyFallbackLoadBalance = "load_balance"
	// DNSPolicyFallbackStaticNodes falls back to an operator-specified static node list.
	DNSPolicyFallbackStaticNodes = "static_nodes"
	// DNSPolicyFallbackAllHealthy falls back to all non-quarantined online edge nodes.
	DNSPolicyFallbackAllHealthy = "all_healthy"
)

// OpenFlareDNSInstance stores a registered embedded or standalone SmartDNS authoritative server instance.
type OpenFlareDNSInstance struct {
	ID                 uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	InstanceID         string     `json:"instance_id" gorm:"column:instance_id;size:64;not null;uniqueIndex:uidx_of_dns_instances_instance_id"`
	Name               string     `json:"name" gorm:"size:128;not null"`
	Mode               string     `json:"mode" gorm:"size:16;not null;default:'standalone'"`
	PublicAddr         string     `json:"public_addr" gorm:"size:128;not null;default:''"`
	ListenAddr         string     `json:"listen_addr" gorm:"size:64;not null;default:':53'"`
	GeoName            string     `json:"geo_name" gorm:"size:128;not null;default:''"`
	GeoLatitude        *float64   `json:"geo_latitude"`
	GeoLongitude       *float64   `json:"geo_longitude"`
	AccessToken        string     `json:"-" gorm:"column:access_token;size:128;not null;default:'';index:idx_of_dns_instances_token"`
	Version            string     `json:"version" gorm:"size:64;not null;default:''"`
	Status             string     `json:"status" gorm:"size:16;not null;default:'pending'"`
	SnapshotVersion    uint64     `json:"snapshot_version" gorm:"not null;default:0"`
	SnapshotETag       string     `json:"snapshot_etag" gorm:"size:64;not null;default:''"`
	QPS                float64    `json:"qps" gorm:"not null;default:0"`
	P50Ms              float64    `json:"p50_ms" gorm:"not null;default:0"`
	P95Ms              float64    `json:"p95_ms" gorm:"not null;default:0"`
	P99Ms              float64    `json:"p99_ms" gorm:"not null;default:0"`
	SuccessTotal       uint64     `json:"success_total" gorm:"not null;default:0"`
	ErrorTotal         uint64     `json:"error_total" gorm:"not null;default:0"`
	CacheHitRatio      float64    `json:"cache_hit_ratio" gorm:"not null;default:0"`
	CPUUsagePercent    float64    `json:"cpu_usage_percent" gorm:"not null;default:0"`
	MemoryUsagePercent float64    `json:"memory_usage_percent" gorm:"not null;default:0"`
	LastSeenAt         *time.Time `json:"last_seen_at"`
	LastError          string     `json:"last_error" gorm:"type:text;not null;default:''"`
	QuarantinedAt      *time.Time `json:"quarantined_at"`
	QuarantineReason   string     `json:"quarantine_reason" gorm:"size:255;not null;default:''"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareDNSInstance) TableName() string {
	return "of_dns_instances"
}

// IsQuarantined reports whether the DNS instance has been quarantined.
func (d *OpenFlareDNSInstance) IsQuarantined() bool {
	return d != nil && d.QuarantinedAt != nil && !d.QuarantinedAt.IsZero()
}

// OpenFlareDNSPolicy stores a SmartDNS traffic routing policy and its compiled JDM decision graph.
type OpenFlareDNSPolicy struct {
	ID                  uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name                string    `json:"name" gorm:"size:128;not null"`
	Description         string    `json:"description" gorm:"size:255;not null;default:''"`
	ScopeType           string    `json:"scope_type" gorm:"size:16;not null;default:'all'"`
	ScopeDomainsJSON    string    `json:"scope_domains_json" gorm:"column:scope_domains_json;type:text;not null;default:'[]'"`
	StrategyType        string    `json:"strategy_type" gorm:"size:24;not null;default:'smart'"`
	Priority            int       `json:"priority" gorm:"not null;default:100;index:idx_of_dns_policies_priority"`
	TTL                 int       `json:"ttl" gorm:"not null;default:60"`
	MaxAnswers          int       `json:"max_answers" gorm:"not null;default:1"`
	Enabled             bool      `json:"enabled" gorm:"not null;default:true;index:idx_of_dns_policies_priority"`
	IsDefault           bool      `json:"is_default" gorm:"not null;default:false"`
	FallbackStrategy    string    `json:"fallback_strategy" gorm:"size:24;not null;default:'smart'"`
	FallbackNodeIDsJSON string    `json:"fallback_node_ids_json" gorm:"column:fallback_node_ids_json;type:text;not null;default:'[]'"`
	ConfigJSON          string    `json:"config_json" gorm:"column:config_json;type:text;not null;default:'{}'"`
	JDMJSON             string    `json:"jdm_json" gorm:"column:jdm_json;type:text;not null;default:'{}'"`
	Revision            int       `json:"revision" gorm:"not null;default:1"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareDNSPolicy) TableName() string {
	return "of_dns_policies"
}

// OpenFlareDNSMetricSnapshot mirrors of_dns_metric_snapshots for controller APIs and logstore insertion.
type OpenFlareDNSMetricSnapshot struct {
	ID                 uint64    `json:"id,string" gorm:"column:id;primaryKey"`
	InstanceID         string    `json:"instance_id" gorm:"column:instance_id;size:64;not null"`
	CapturedAt         time.Time `json:"captured_at" gorm:"column:captured_at"`
	QueriesTotal       uint64    `json:"queries_total" gorm:"column:queries_total"`
	QueriesSuccess     uint64    `json:"queries_success" gorm:"column:queries_success"`
	QueriesNXDomain    uint64    `json:"queries_nxdomain" gorm:"column:queries_nxdomain"`
	QueriesServFail    uint64    `json:"queries_servfail" gorm:"column:queries_servfail"`
	QueriesRateLimited uint64    `json:"queries_ratelimited" gorm:"column:queries_ratelimited"`
	QPS                float64   `json:"qps" gorm:"column:qps"`
	LatencyP50Ms       float64   `json:"latency_p50_ms" gorm:"column:latency_p50_ms"`
	LatencyP95Ms       float64   `json:"latency_p95_ms" gorm:"column:latency_p95_ms"`
	LatencyP99Ms       float64   `json:"latency_p99_ms" gorm:"column:latency_p99_ms"`
	CacheHitRatio      float64   `json:"cache_hit_ratio" gorm:"column:cache_hit_ratio"`
	SnapshotVersion    uint64    `json:"snapshot_version" gorm:"column:snapshot_version"`
	CPUUsagePercent    float64   `json:"cpu" gorm:"column:cpu"`
	MemoryUsagePercent float64   `json:"ram_percent" gorm:"column:ram_percent"`
	CreatedAt          time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName returns the GORM table name.
func (OpenFlareDNSMetricSnapshot) TableName() string {
	return "of_dns_metric_snapshots"
}
