// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

const (
	// NetworkQualitySourceTCPPassive identifies passive downstream TCP_INFO samples.
	NetworkQualitySourceTCPPassive = "tcp_passive"
	// NetworkQualitySourceTCPActive identifies active TCPing batch probes.
	NetworkQualitySourceTCPActive = "tcp_active"
	// NetworkQualitySourceICMPActive identifies active ICMP ping batch probes.
	NetworkQualitySourceICMPActive = "icmp_active"
	// NetworkQualitySourceTraceroute identifies active traceroute samples.
	NetworkQualitySourceTraceroute = "traceroute"
	// MeasurementSourceNone marks rows/results without a direct measurement source.
	MeasurementSourceNone = "none"

	// QualityMatrixDataSourceMeasured marks matrix rows backed by direct measurements.
	QualityMatrixDataSourceMeasured = "measured"
	// QualityMatrixDataSourcePredicted marks matrix rows estimated via AS graph / geo distance.
	QualityMatrixDataSourcePredicted = "predicted"

	// QualityAggregationLevelCountry identifies country-level rolled-up matrix rows.
	QualityAggregationLevelCountry = "country"
	// QualityAggregationLevelRegion identifies subdivision/region-level rolled-up matrix rows.
	QualityAggregationLevelRegion = "region"

	// NetworkQualityStatsVersionLegacy identifies legacy summary-only quality samples/matrix rows.
	NetworkQualityStatsVersionLegacy = 1
	// NetworkQualityStatsVersionHist64 identifies 64-bin log histogram + prefix sketch samples/matrix rows.
	NetworkQualityStatsVersionHist64 = 2

	// TracerouteTriggerManual marks user-triggered traceroute runs.
	TracerouteTriggerManual = "manual"
	// TracerouteTriggerAuto marks background scheduled traceroute runs.
	TracerouteTriggerAuto = "auto"

	// ProbeTargetTypeAnchor classifies global public DNS/network reachability anchor targets.
	ProbeTargetTypeAnchor = "public_anchor"
	// ProbeTargetTypeCustomerISP classifies explicit ISP/region targets representative of customer networks.
	ProbeTargetTypeCustomerISP = "customer_isp"
	// ProbeTargetTypeManual classifies operator-configured arbitrary public targets.
	ProbeTargetTypeManual = "manual"

	// ProbeProtoICMP probes targets using ICMP echo.
	ProbeProtoICMP = "icmp"
	// ProbeProtoTCP probes targets using TCP SYN/connect handshake timing.
	ProbeProtoTCP = "tcp"
	// ProbeProtoTraceroute probes targets with full hop-by-hop trippy traceroute.
	ProbeProtoTraceroute = "traceroute"
)

// OpenFlareNetworkQualitySample represents a single aggregated quality observation stored in logstore.
type OpenFlareNetworkQualitySample struct {
	ID                  uint64    `json:"id,string" gorm:"column:id;primaryKey"`
	NodeID              string    `json:"node_id" gorm:"column:node_id;size:64;not null"`
	NodeASN             uint32    `json:"node_asn" gorm:"column:node_asn;not null;default:0"`
	ClientASN           uint32    `json:"client_asn" gorm:"column:client_asn;not null;default:0"`
	ClientCountry       string    `json:"client_country" gorm:"column:client_country;size:16;not null;default:''"`
	ClientRegion        string    `json:"client_region" gorm:"column:client_region;size:32;not null;default:''"`
	IPFamily            string    `json:"ip_family" gorm:"column:ip_family;size:8;not null;default:'unknown'"`
	StatsVersion        int       `json:"stats_version" gorm:"column:stats_version;not null;default:1"`
	Source              string    `json:"source" gorm:"column:source;size:16;not null;default:'tcp_passive'"`
	SampleCount         int       `json:"sample_count" gorm:"column:sample_count;not null;default:0"`
	RTTMinMs            float64   `json:"rtt_min_ms" gorm:"column:rtt_min_ms;not null;default:0"`
	RTTP50Ms            float64   `json:"rtt_p50_ms" gorm:"column:rtt_p50_ms;not null;default:0"`
	RTTP95Ms            float64   `json:"rtt_p95_ms" gorm:"column:rtt_p95_ms;not null;default:0"`
	RTTHist             string    `json:"rtt_hist" gorm:"column:rtt_hist;type:text;not null;default:''"`
	PrefixSketch        string    `json:"prefix_sketch" gorm:"column:prefix_sketch;type:text;not null;default:''"`
	DistinctPrefixCount int       `json:"distinct_prefix_count" gorm:"column:distinct_prefix_count;not null;default:0"`
	ClientLat           *float64  `json:"client_lat,omitempty" gorm:"column:client_lat"`
	ClientLon           *float64  `json:"client_lon,omitempty" gorm:"column:client_lon"`
	GeoAccuracyKm       float64   `json:"geo_accuracy_km" gorm:"column:geo_accuracy_km;not null;default:0"`
	GeoSource           string    `json:"geo_source" gorm:"column:geo_source;size:16;not null;default:''"`
	RetransRate         float64   `json:"retrans_rate" gorm:"column:retrans_rate;not null;default:0"`
	LossRate            float64   `json:"loss_rate" gorm:"column:loss_rate;not null;default:0"`
	HopCount            int       `json:"hop_count" gorm:"column:hop_count;not null;default:0"`
	CapturedAt          time.Time `json:"captured_at" gorm:"column:captured_at"`
	CreatedAt           time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName returns the GORM table name.
func (OpenFlareNetworkQualitySample) TableName() string {
	return "of_network_quality_samples"
}

// OpenFlareNetworkQualityMatrix stores measured network quality matrix per (node_id, client_asn, client_country, client_region, ip_family, aggregation_level).
type OpenFlareNetworkQualityMatrix struct {
	ID                  uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID              string     `json:"node_id" gorm:"column:node_id;size:64;not null;uniqueIndex:uidx_of_nq_matrix_node_asn_geo_v2"`
	NodeASN             uint32     `json:"node_asn" gorm:"column:node_asn;not null;default:0"`
	ClientASN           uint32     `json:"client_asn" gorm:"column:client_asn;not null;uniqueIndex:uidx_of_nq_matrix_node_asn_geo_v2;index:idx_of_nq_matrix_client_asn"`
	ClientCountry       string     `json:"client_country" gorm:"column:client_country;size:16;not null;default:'';uniqueIndex:uidx_of_nq_matrix_node_asn_geo_v2;index:idx_of_nq_matrix_country;index:idx_of_nq_matrix_region"`
	ClientRegion        string     `json:"client_region" gorm:"column:client_region;size:32;not null;default:'';uniqueIndex:uidx_of_nq_matrix_node_asn_geo_v2;index:idx_of_nq_matrix_region"`
	IPFamily            string     `json:"ip_family" gorm:"column:ip_family;size:8;not null;default:'unknown';uniqueIndex:uidx_of_nq_matrix_node_asn_geo_v2;index:idx_of_nq_matrix_level_family"`
	AggregationLevel    string     `json:"aggregation_level" gorm:"column:aggregation_level;size:16;not null;default:'country';uniqueIndex:uidx_of_nq_matrix_node_asn_geo_v2;index:idx_of_nq_matrix_level_family"`
	StatsVersion        int        `json:"stats_version" gorm:"column:stats_version;not null;default:1"`
	ClientASNName       string     `json:"client_asn_name" gorm:"column:client_asn_name;size:255;not null;default:''"`
	TCPRTTP50Ms         float64    `json:"tcp_rtt_p50_ms" gorm:"column:tcp_rtt_p50_ms;not null;default:0"`
	TCPRTTP95Ms         float64    `json:"tcp_rtt_p95_ms" gorm:"column:tcp_rtt_p95_ms;not null;default:0"`
	TCPMinRTTMs         float64    `json:"tcp_min_rtt_ms" gorm:"column:tcp_min_rtt_ms;not null;default:0"`
	TCPRetransRate      float64    `json:"tcp_retrans_rate" gorm:"column:tcp_retrans_rate;not null;default:0"`
	TCPSampleCount      int        `json:"tcp_sample_count" gorm:"column:tcp_sample_count;not null;default:0"`
	TCPLastSeenAt       *time.Time `json:"tcp_last_seen_at" gorm:"column:tcp_last_seen_at"`
	ProbeRTTP50Ms       float64    `json:"probe_rtt_p50_ms" gorm:"column:probe_rtt_p50_ms;not null;default:0"`
	ProbeLossRate       float64    `json:"probe_loss_rate" gorm:"column:probe_loss_rate;not null;default:0"`
	ProbeSampleCount    int        `json:"probe_sample_count" gorm:"column:probe_sample_count;not null;default:0"`
	ProbeProto          string     `json:"probe_proto" gorm:"column:probe_proto;size:16;not null;default:''"`
	ProbeLastSeenAt     *time.Time `json:"probe_last_seen_at" gorm:"column:probe_last_seen_at"`
	TraceRTTP50Ms       float64    `json:"trace_rtt_p50_ms" gorm:"column:trace_rtt_p50_ms;not null;default:0"`
	TraceLossRate       float64    `json:"trace_loss_rate" gorm:"column:trace_loss_rate;not null;default:0"`
	TraceHopCount       int        `json:"trace_hop_count" gorm:"column:trace_hop_count;not null;default:0"`
	TraceASPath         string     `json:"trace_as_path" gorm:"column:trace_as_path;type:text;not null;default:'[]'"`
	TraceSampleCount    int        `json:"trace_sample_count" gorm:"column:trace_sample_count;not null;default:0"`
	TraceLastSeenAt     *time.Time `json:"trace_last_seen_at" gorm:"column:trace_last_seen_at"`
	EffectiveRTTMs      float64    `json:"effective_rtt_ms" gorm:"column:effective_rtt_ms;not null;default:0"`
	PredictedRTTMs      float64    `json:"predicted_rtt_ms" gorm:"column:predicted_rtt_ms;not null;default:0"`
	RTTEWMAms           float64    `json:"rtt_ewma_ms" gorm:"column:rtt_ewma_ms;not null;default:0"`
	DistinctPrefixCount int        `json:"distinct_prefix_count" gorm:"column:distinct_prefix_count;not null;default:0"`
	GeoBaseRTTMs        float64    `json:"geo_base_rtt_ms" gorm:"column:geo_base_rtt_ms;not null;default:0"`
	GeoAccuracyKm       float64    `json:"geo_accuracy_km" gorm:"column:geo_accuracy_km;not null;default:0"`
	GeoSource           string     `json:"geo_source" gorm:"column:geo_source;size:16;not null;default:''"`
	GeoConfidence       float64    `json:"geo_confidence" gorm:"column:geo_confidence;not null;default:0"`
	MeasurementSource   string     `json:"measurement_source" gorm:"column:measurement_source;size:16;not null;default:''"`
	QualityScore        float64    `json:"quality_score" gorm:"column:quality_score;not null;default:0;index:idx_of_nq_matrix_score"`
	Confidence          float64    `json:"confidence" gorm:"column:confidence;not null;default:0"`
	DataSource          string     `json:"data_source" gorm:"column:data_source;size:16;not null;default:'measured'"`
	LastObservedAt      *time.Time `json:"last_observed_at" gorm:"column:last_observed_at"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareNetworkQualityMatrix) TableName() string {
	return "of_network_quality_matrix"
}

// OpenFlareASTopologyEdge stores an observed AS-to-AS hop link discovered via traceroute.
type OpenFlareASTopologyEdge struct {
	ID          uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	SrcASN      uint32     `json:"src_asn" gorm:"column:src_asn;not null;uniqueIndex:uidx_of_as_edges_pair"`
	DstASN      uint32     `json:"dst_asn" gorm:"column:dst_asn;not null;uniqueIndex:uidx_of_as_edges_pair;index:idx_of_as_edges_dst"`
	EdgeType    string     `json:"edge_type" gorm:"column:edge_type;size:16;not null;default:'observed'"`
	AvgHopRTTMs float64    `json:"avg_hop_rtt_ms" gorm:"column:avg_hop_rtt_ms;not null;default:0"`
	MinHopRTTMs float64    `json:"min_hop_rtt_ms" gorm:"column:min_hop_rtt_ms;not null;default:0"`
	SampleCount int        `json:"sample_count" gorm:"column:sample_count;not null;default:0"`
	LastSeenAt  *time.Time `json:"last_seen_at" gorm:"column:last_seen_at"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareASTopologyEdge) TableName() string {
	return "of_as_topology_edges"
}

// OpenFlareTracerouteRecord stores manual or scheduled traceroute outcomes.
type OpenFlareTracerouteRecord struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID        string    `json:"task_id" gorm:"column:task_id;size:64;not null;default:'';index:idx_of_traceroute_task"`
	TriggerType   string    `json:"trigger_type" gorm:"column:trigger_type;size:16;not null;default:'manual'"`
	NodeID        string    `json:"node_id" gorm:"column:node_id;size:64;not null;index:idx_of_traceroute_node_time"`
	Target        string    `json:"target" gorm:"column:target;size:255;not null"`
	ResolvedIP    string    `json:"resolved_ip" gorm:"column:resolved_ip;size:64;not null;default:''"`
	TargetASN     uint32    `json:"target_asn" gorm:"column:target_asn;not null;default:0;index:idx_of_traceroute_target_asn"`
	TargetCountry string    `json:"target_country" gorm:"column:target_country;size:16;not null;default:''"`
	TargetRegion  string    `json:"target_region" gorm:"column:target_region;size:32;not null;default:''"`
	IPFamily      string    `json:"ip_family" gorm:"column:ip_family;size:8;not null;default:'unknown'"`
	Protocol      string    `json:"protocol" gorm:"column:protocol;size:16;not null;default:'icmp'"`
	HopCount      int       `json:"hop_count" gorm:"column:hop_count;not null;default:0"`
	Reached       bool      `json:"reached" gorm:"column:reached;not null;default:false"`
	TotalRTTMs    float64   `json:"total_rtt_ms" gorm:"column:total_rtt_ms;not null;default:0"`
	HopsJSON      string    `json:"hops_json" gorm:"column:hops_json;type:text;not null;default:'[]'"`
	ASPathJSON    string    `json:"as_path_json" gorm:"column:as_path_json;type:text;not null;default:'[]'"`
	ErrorMessage  string    `json:"error_message" gorm:"column:error_message;type:text;not null;default:''"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime;index:idx_of_traceroute_created_at"`
}

// TableName returns the GORM table name.
func (OpenFlareTracerouteRecord) TableName() string {
	return "of_traceroute_records"
}

// NetworkQualityMatrixQuery filters paginated network quality matrix queries.
type NetworkQualityMatrixQuery struct {
	Page              int
	PageSize          int
	NodeID            string
	ClientASN         uint32
	ClientCountry     string
	ClientRegion      string
	IPFamily          string
	AggregationLevel  string
	MeasurementSource string
	DataSource        string
	Search            string
	SortBy            string
	SortOrder         string
}

// TracerouteRecordQuery filters paginated traceroute history queries.
type TracerouteRecordQuery struct {
	Page        int
	PageSize    int
	NodeID      string
	Target      string
	TriggerType string
	TargetASN   uint32
}

// OpenFlareProbeTarget stores a managed, SSRF-validated active probe target.
type OpenFlareProbeTarget struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string    `json:"name" gorm:"column:name;size:128;not null;default:''"`
	IP            string    `json:"ip" gorm:"column:ip;size:64;not null;uniqueIndex:uidx_of_probe_targets_ip_proto_port"`
	IPFamily      string    `json:"ip_family" gorm:"column:ip_family;size:8;not null;default:'ipv4'"`
	ProbeProto    string    `json:"probe_proto" gorm:"column:probe_proto;size:16;not null;default:'icmp';uniqueIndex:uidx_of_probe_targets_ip_proto_port"`
	Port          int       `json:"port" gorm:"column:port;not null;default:0;uniqueIndex:uidx_of_probe_targets_ip_proto_port"`
	ASN           uint32    `json:"asn" gorm:"column:asn;not null;default:0;index:idx_of_probe_targets_asn_enabled"`
	Country       string    `json:"country" gorm:"column:country;size:16;not null;default:'';index:idx_of_probe_targets_geo"`
	Region        string    `json:"region" gorm:"column:region;size:32;not null;default:'';index:idx_of_probe_targets_geo"`
	TargetType    string    `json:"target_type" gorm:"column:target_type;size:16;not null;default:'public_anchor'"`
	AnycastStatus string    `json:"anycast_status" gorm:"column:anycast_status;size:16;not null;default:'unknown'"`
	Enabled       bool      `json:"enabled" gorm:"column:enabled;not null;default:true;index:idx_of_probe_targets_asn_enabled"`
	Notes         string    `json:"notes" gorm:"column:notes;size:255;not null;default:''"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareProbeTarget) TableName() string {
	return "of_probe_targets"
}

// OpenFlareProbeTargetState tracks per-node probe dispatch cooldown and failure backoff for one target.
type OpenFlareProbeTargetState struct {
	ID                  uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID              string     `json:"node_id" gorm:"column:node_id;size:64;not null;uniqueIndex:uidx_of_probe_target_states_node_target"`
	TargetID            uint       `json:"target_id" gorm:"column:target_id;not null;uniqueIndex:uidx_of_probe_target_states_node_target;index:idx_of_probe_target_states_next_probe"`
	ConsecutiveFailures int        `json:"consecutive_failures" gorm:"column:consecutive_failures;not null;default:0"`
	LastProbedAt        *time.Time `json:"last_probed_at" gorm:"column:last_probed_at"`
	LastSuccessAt       *time.Time `json:"last_success_at" gorm:"column:last_success_at"`
	NextProbeAt         *time.Time `json:"next_probe_at" gorm:"column:next_probe_at;index:idx_of_probe_target_states_next_probe"`
	LastError           string     `json:"last_error" gorm:"column:last_error;type:text;not null;default:''"`
	LastRTTMs           float64    `json:"last_rtt_ms" gorm:"column:last_rtt_ms;not null;default:0"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareProbeTargetState) TableName() string {
	return "of_probe_target_states"
}

// ProbeTargetQuery filters paginated active probe targets.
type ProbeTargetQuery struct {
	Page          int
	PageSize      int
	ASN           uint32
	Country       string
	IPFamily      string
	TargetType    string
	AnycastStatus string
	Enabled       *bool
	Search        string
}
