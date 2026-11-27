// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netquality"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/google/uuid"
	"github.com/miekg/dns"
)

const (
	maxDispatchNodes      = 50
	onlineNodeWindow      = 2 * time.Minute
	maxASPathObservedEdge = 10000
	defaultDNSTimeoutMs   = 4000
	maxDirectDNSAnswers   = 64

	probeProtoICMP = "icmp"
	probeProtoTCP  = "tcp"
	probeProtoUDP  = "udp"
	probeProtoHTTP = "http"

	errNodeWSUnavailable = "edge node WebSocket channel unavailable"
	attributionBGPTools  = "Data powered by bgp.tools"

	defaultPingCount      = 4
	minPingCount          = 1
	maxPingCount          = 10
	defaultPingIntervalMs = 200
	minPingIntervalMs     = 100
	maxPingIntervalMs     = 2000
	defaultPingTimeoutMs  = 3000
	minProbeTimeoutMs     = 500
	maxProbeTimeoutMs     = 10000

	defaultTraceMaxHops   = 30
	minTraceMaxHops       = 1
	maxTraceMaxHops       = 40
	defaultTraceProbes    = 3
	minTraceProbes        = 1
	maxTraceProbes        = 5
	defaultTraceTimeoutMs = 1500
	maxTraceTimeoutMs     = 5000

	maxResolvedHostnameIPs = 4
	ipNeighborsLimit       = 100
	asnNeighborsLimit      = 150
	usPerCentisecond = 10.0
	centisPerSecond  = 100.0
)

// PingDispatchInput requests a multi-node batch Ping/TCPing/HTTP probe.
type PingDispatchInput struct {
	Target     string   `json:"target"`
	Protocol   string   `json:"protocol"`
	Port       *uint16  `json:"port,omitempty"`
	Count      int      `json:"count"`
	IntervalMs int      `json:"interval_ms"`
	TimeoutMs  int      `json:"timeout_ms"`
	NodeIDs    []string `json:"node_ids,omitempty"`
}

// TracerouteDispatchInput requests a multi-node active traceroute.
type TracerouteDispatchInput struct {
	Target       string   `json:"target"`
	Protocol     string   `json:"protocol"`
	Port         *uint16  `json:"port,omitempty"`
	MaxHops      int      `json:"max_hops"`
	ProbesPerHop int      `json:"probes_per_hop"`
	TimeoutMs    int      `json:"timeout_ms"`
	NodeIDs      []string `json:"node_ids,omitempty"`
}

// TracerouteHistoryQuery filters paginated historical traceroute records.
type TracerouteHistoryQuery struct {
	Page        int
	PageSize    int
	NodeID      string
	Target      string
	TriggerType string
	TargetASN   uint32
}

// TracerouteHistoryItem exposes a decoded historical traceroute record.
type TracerouteHistoryItem struct {
	ID            uint                               `json:"id"`
	TaskID        string                             `json:"task_id"`
	TriggerType   string                             `json:"trigger_type"`
	NodeID        string                             `json:"node_id"`
	Target        string                             `json:"target"`
	ResolvedIP    string                             `json:"resolved_ip"`
	TargetASN     uint32                             `json:"target_asn"`
	TargetCountry string                             `json:"target_country"`
	Protocol      string                             `json:"protocol"`
	HopCount      int                                `json:"hop_count"`
	Reached       bool                               `json:"reached"`
	TotalRTTMs    float64                            `json:"total_rtt_ms"`
	Hops          []netquality.EnrichedTracerouteHop `json:"hops"`
	ASPath        []uint32                           `json:"as_path"`
	ErrorMessage  string                             `json:"error_message,omitempty"`
	CreatedAt     time.Time                          `json:"created_at"`
}

// TracerouteHistoryPage wraps paginated traceroute history records.
type TracerouteHistoryPage struct {
	Total   int64                   `json:"total"`
	Results []TracerouteHistoryItem `json:"results"`
}

// DNSQueryInput configures direct control-plane or multi-node edge DNS probing with optional EDNS0 ECS.
type DNSQueryInput struct {
	Domain     string   `json:"domain"`
	RecordType string   `json:"record_type"`
	Resolver   string   `json:"resolver"`
	ECSSubnet  string   `json:"ecs_subnet,omitempty"`
	TimeoutMs  int      `json:"timeout_ms,omitempty"`
	NodeIDs    []string `json:"node_ids,omitempty"`
}

// EnrichedDNSRecord augments a DNS answer record with GeoIP/ASN intelligence for A/AAAA RRs.
type EnrichedDNSRecord struct {
	protocol.DNSRecord
	CountryISO string   `json:"country_iso,omitempty"`
	CityName   string   `json:"city_name,omitempty"`
	ASN        uint32   `json:"asn,omitempty"`
	ASNName    string   `json:"asn_name,omitempty"`
	ASNTags    []string `json:"asn_tags,omitempty"`
}

// DirectDNSQueryResult carries an immediate control-plane DNS lookup response.
type DirectDNSQueryResult struct {
	Domain        string              `json:"domain"`
	RecordType    string              `json:"record_type"`
	Resolver      string              `json:"resolver"`
	ECSSubnet     string              `json:"ecs_subnet,omitempty"`
	RCode         string              `json:"rcode"`
	Authoritative bool                `json:"authoritative"`
	Truncated     bool                `json:"truncated"`
	UsedTCP       bool                `json:"used_tcp"`
	LatencyMs     float64             `json:"latency_ms"`
	Records       []EnrichedDNSRecord `json:"records"`
}

// DNSQueryResponse returns either an immediate control-plane result or a multi-node polling task.
type DNSQueryResponse struct {
	Mode         string                `json:"mode"` // "direct" or "nodes"
	DirectResult *DirectDNSQueryResult `json:"direct_result,omitempty"`
	Task         *ProbeTaskState       `json:"task,omitempty"`
}

// ASNeighborItem combines CAIDA relationships and observed traceroute hop metrics.
type ASNeighborItem struct {
	PeerASN     uint32   `json:"peer_asn"`
	PeerName    string   `json:"peer_name"`
	PeerCountry string   `json:"peer_country"`
	PeerClass   string   `json:"peer_class"`
	Relation    string   `json:"relation"`
	Tags        []string `json:"tags,omitempty"`
	ObservedRTT float64  `json:"observed_rtt_ms,omitempty"`
	SampleCount int      `json:"sample_count,omitempty"`
}

// IPLookupResponse summarizes IP/hostname geolocation, ASN, bgp.tools tags, and neighbor AS topology.
type IPLookupResponse struct {
	Query       string                        `json:"query"`
	ResolvedIPs []string                      `json:"resolved_ips"`
	Primary     *pkgnetintel.IPLookupResult   `json:"primary,omitempty"`
	AllResults  []*pkgnetintel.IPLookupResult `json:"all_results,omitempty"`
	ASNInfo     *pkgnetintel.ASNInfo          `json:"asn_info,omitempty"`
	Neighbors   []ASNeighborItem              `json:"neighbors"`
	Attribution string                        `json:"attribution"`
}

// ASNDetailResponse summarizes ASN metadata, announced prefixes, and topology neighbors.
type ASNDetailResponse struct {
	ASNInfo     *pkgnetintel.ASNInfo `json:"asn_info"`
	Neighbors   []ASNeighborItem     `json:"neighbors"`
	Attribution string               `json:"attribution"`
}

// ASNPrefixesResponse paginates BGP prefixes announced by an ASN.
type ASNPrefixesResponse struct {
	ASN         uint32                     `json:"asn"`
	Total       int64                      `json:"total"`
	Results     []pkgnetintel.PrefixRecord `json:"results"`
	Attribution string                     `json:"attribution"`
}

// ASPathInput requests multi-node AS path and quality comparison toward a target ASN or IP.
type ASPathInput struct {
	TargetASN     uint32   `json:"target_asn"`
	TargetIP      string   `json:"target_ip"`
	TargetCountry string   `json:"target_country,omitempty"`
	TargetRegion  string   `json:"target_region,omitempty"`
	IPFamily      string   `json:"ip_family,omitempty"`
	NodeIDs       []string `json:"node_ids,omitempty"`
}

// ASPathHopNode identifies one ASN along a predicted or observed AS path.
type ASPathHopNode struct {
	ASN     uint32   `json:"asn"`
	Name    string   `json:"name"`
	Country string   `json:"country"`
	Tags    []string `json:"tags,omitempty"`
}

// NodeASPathComparison scores how each edge node reaches target_asn.
type NodeASPathComparison struct {
	NodeID                string          `json:"node_id"`
	NodeName              string          `json:"node_name"`
	NodeIP                string          `json:"node_ip"`
	NodeGeo               string          `json:"node_geo"`
	NodeASN               uint32          `json:"node_asn"`
	NodeASNName           string          `json:"node_asn_name"`
	NetworkGroup          string          `json:"network_group,omitempty"`
	GeoSource             string          `json:"geo_source,omitempty"`
	ASHops                int             `json:"as_hops"`
	Path                  []ASPathHopNode `json:"path"`
	EffectiveRTTMs        float64         `json:"effective_rtt_ms"`
	RTTP95Ms              float64         `json:"rtt_p95_ms"`
	CandidateDispersionMs float64         `json:"candidate_dispersion_ms"`
	PredictionErrorMs     *float64        `json:"prediction_error_ms,omitempty"`
	QualityScore          float64         `json:"quality_score"`
	Confidence            float64         `json:"confidence"`
	MeasurementSource     string          `json:"measurement_source"`
	PredictionMethod      string          `json:"prediction_method"`
	GeoBaseRTTMs          float64         `json:"geo_base_rtt_ms"`
	SampleCount           uint64          `json:"sample_count"`
	DistinctPrefixes      uint32          `json:"distinct_prefixes"`
	LastObservedAt        *time.Time      `json:"last_observed_at,omitempty"`
	AnycastIsolated       bool            `json:"anycast_isolated,omitempty"`
	DataSource            string          `json:"data_source"`
}

// ASPathResponse compares edge node paths toward the requested target ASN.
type ASPathResponse struct {
	TargetASN     uint32                 `json:"target_asn"`
	TargetASNInfo *pkgnetintel.ASNInfo   `json:"target_asn_info,omitempty"`
	TargetCountry string                 `json:"target_country,omitempty"`
	TargetRegion  string                 `json:"target_region,omitempty"`
	IPFamily      string                 `json:"ip_family,omitempty"`
	Nodes         []NodeASPathComparison `json:"nodes"`
	Attribution   string                 `json:"attribution"`
}

// QualityPredictInput simulates SmartDNS node ranking and latency prediction for a client network target.
type QualityPredictInput struct {
	TargetIP      string   `json:"target_ip"`
	TargetASN     uint32   `json:"target_asn"`
	TargetCountry string   `json:"target_country"`
	TargetRegion  string   `json:"target_region"`
	IPFamily      string   `json:"ip_family"`
	NodeIDs       []string `json:"node_ids,omitempty"`
}

// NodeQualityPrediction holds predicted latency, confidence, and source breakdown for one edge node.
type NodeQualityPrediction struct {
	NodeID                string     `json:"node_id"`
	NodeName              string     `json:"node_name"`
	NodeIP                string     `json:"node_ip"`
	NodeGeo               string     `json:"node_geo"`
	NodeASN               uint32     `json:"node_asn"`
	NodeASNName           string     `json:"node_asn_name"`
	NetworkGroup          string     `json:"network_group,omitempty"`
	GeoSource             string     `json:"geo_source,omitempty"`
	EffectiveRTTMs        float64    `json:"effective_rtt_ms"`
	RTTP95Ms              float64    `json:"rtt_p95_ms"`
	CandidateDispersionMs float64    `json:"candidate_dispersion_ms"`
	PredictionErrorMs     *float64   `json:"prediction_error_ms,omitempty"`
	QualityScore          float64    `json:"quality_score"`
	Confidence            float64    `json:"confidence"`
	MeasurementSource     string     `json:"measurement_source"`
	PredictionMethod      string     `json:"prediction_method"`
	GeoBaseRTTMs          float64    `json:"geo_base_rtt_ms"`
	SampleCount           uint64     `json:"sample_count"`
	DistinctPrefixes      uint32     `json:"distinct_prefixes"`
	LastObservedAt        *time.Time `json:"last_observed_at,omitempty"`
	AnycastIsolated       bool       `json:"anycast_isolated,omitempty"`
	DataSource            string     `json:"data_source"`
}

// QualityPredictResponse ranks all candidate edge nodes for a simulated client query.
type QualityPredictResponse struct {
	TargetIP      string                  `json:"target_ip,omitempty"`
	TargetASN     uint32                  `json:"target_asn"`
	TargetASNInfo *pkgnetintel.ASNInfo    `json:"target_asn_info,omitempty"`
	TargetCountry string                  `json:"target_country,omitempty"`
	TargetRegion  string                  `json:"target_region,omitempty"`
	IPFamily      string                  `json:"ip_family"`
	AnycastStatus string                  `json:"anycast_status,omitempty"`
	GeoSource     string                  `json:"geo_source,omitempty"`
	Nodes         []NodeQualityPrediction `json:"nodes"`
	Attribution   string                  `json:"attribution"`
}

// QualityMatrixItem enriches a matrix row with node metadata.
type QualityMatrixItem struct {
	*model.OpenFlareNetworkQualityMatrix
	NodeName         string `json:"node_name"`
	NodeGeo          string `json:"node_geo"`
	NodeStatus       string `json:"node_status"`
	NodeNetworkGroup string `json:"node_network_group,omitempty"`
	NodeGeoSource    string `json:"node_geo_source,omitempty"`
}

// QualityMatrixPage wraps paginated network quality matrix results.
type QualityMatrixPage struct {
	Total   int64               `json:"total"`
	Results []QualityMatrixItem `json:"results"`
}

// NetIntelStatusResponse reports loaded database statuses and attribution metadata.
type NetIntelStatusResponse struct {
	DataDir      string                          `json:"data_dir"`
	Datasets     []pkgnetintel.DatasetFileStatus `json:"datasets"`
	CAIDAEnabled bool                            `json:"caida_enabled"`
	AutoProbe    bool                            `json:"auto_probe_enabled"`
	Attribution  string                          `json:"attribution"`
}

// TriggerSyncResponse reports either queued async task ID or immediate sync result.
type TriggerSyncResponse struct {
	TaskID  string                `json:"task_id,omitempty"`
	Mode    string                `json:"mode"`
	Summary *netintel.SyncSummary `json:"summary,omitempty"`
}

// DispatchPing validates parameters and dispatches batch Ping/TCPing/HTTP probes to edge nodes.
func DispatchPing(ctx context.Context, input PingDispatchInput) (*ProbeTaskState, error) {
	proto := strings.ToLower(strings.TrimSpace(input.Protocol))
	if proto == "" {
		proto = probeProtoICMP
	}
	if proto != probeProtoICMP && proto != probeProtoTCP && proto != probeProtoHTTP {
		return nil, errors.New(errPingProtocolInvalid)
	}
	if input.Port != nil && (*input.Port == 0) {
		return nil, errors.New(errInvalidPort)
	}
	target, _, err := ValidateProbeTarget(ctx, input.Target, proto == probeProtoHTTP, false)
	if err != nil {
		return nil, err
	}

	count := clampInt(input.Count, defaultPingCount, minPingCount, maxPingCount)
	intervalMs := clampInt(input.IntervalMs, defaultPingIntervalMs, minPingIntervalMs, maxPingIntervalMs)
	timeoutMsVal := clampInt(input.TimeoutMs, defaultPingTimeoutMs, minProbeTimeoutMs, maxProbeTimeoutMs)
	timeoutMs := uint64(timeoutMsVal) //nolint:gosec // clamped positive 500..10000

	nodes, err := selectDispatchNodes(ctx, input.NodeIDs)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	taskID := "ping-" + uuid.NewString()
	state := &ProbeTaskState{
		TaskID:     taskID,
		Kind:       "ping_batch",
		Target:     target,
		CreatedAt:  now,
		ExpiresAt:  now.Add(taskTTL),
		TotalNodes: len(nodes),
		Status:     TaskStatusRunning,
		Nodes:      make([]*ProbeNodeResult, 0, len(nodes)),
	}

	for _, n := range nodes {
		res := &ProbeNodeResult{
			NodeID:   n.NodeID,
			NodeName: n.Name,
			NodeGeo:  n.GeoName,
			NodeASN:  n.ASN,
			Status:   NodeResultStatusPending,
		}
		dispatch := protocol.NetProbeTaskDispatch{
			ProbeID: taskID,
			Kind:    "ping_batch",
			Ping: &protocol.PingBatchRequest{
				Protocol:   proto,
				Target:     target,
				Port:       input.Port,
				Count:      uint8(count),       //nolint:gosec // clamped 1..10
				IntervalMs: uint64(intervalMs), //nolint:gosec // clamped positive 100..2000
				TimeoutMs:  &timeoutMs,
			},
		}
		if !websocket.DispatchNetProbeTask(ctx, n.NodeID, dispatch) {
			res.Status = NodeResultStatusOffline
			res.Error = errNodeWSUnavailable
		}
		state.Nodes = append(state.Nodes, res)
	}
	saveTaskState(ctx, state)
	return state, nil
}

// DispatchTraceroute validates parameters and dispatches trippy-core traceroutes to edge nodes.
func DispatchTraceroute(ctx context.Context, input TracerouteDispatchInput) (*ProbeTaskState, error) {
	proto := strings.ToLower(strings.TrimSpace(input.Protocol))
	if proto == "" {
		proto = probeProtoICMP
	}
	if proto != probeProtoICMP && proto != probeProtoUDP && proto != probeProtoTCP {
		return nil, errors.New(errTraceProtocolInvalid)
	}
	if input.Port != nil && (*input.Port == 0) {
		return nil, errors.New(errInvalidPort)
	}
	target, hostOnly, err := ValidateProbeTarget(ctx, input.Target, false, false)
	if err != nil {
		return nil, err
	}
	_ = target

	maxHops := clampInt(input.MaxHops, defaultTraceMaxHops, minTraceMaxHops, maxTraceMaxHops)
	probesPerHop := clampInt(input.ProbesPerHop, defaultTraceProbes, minTraceProbes, maxTraceProbes)
	timeoutMsVal := clampInt(input.TimeoutMs, defaultTraceTimeoutMs, minProbeTimeoutMs, maxTraceTimeoutMs)
	timeoutMs := uint64(timeoutMsVal) //nolint:gosec // clamped positive 500..5000

	nodes, err := selectDispatchNodes(ctx, input.NodeIDs)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	taskID := "trace-" + uuid.NewString()
	state := &ProbeTaskState{
		TaskID:     taskID,
		Kind:       "traceroute",
		Target:     hostOnly,
		CreatedAt:  now,
		ExpiresAt:  now.Add(taskTTL),
		TotalNodes: len(nodes),
		Status:     TaskStatusRunning,
		Nodes:      make([]*ProbeNodeResult, 0, len(nodes)),
	}

	for _, n := range nodes {
		res := &ProbeNodeResult{
			NodeID:   n.NodeID,
			NodeName: n.Name,
			NodeGeo:  n.GeoName,
			NodeASN:  n.ASN,
			Status:   NodeResultStatusPending,
		}
		dispatch := protocol.NetProbeTaskDispatch{
			ProbeID: taskID,
			Kind:    "traceroute",
			Traceroute: &protocol.TracerouteRequest{
				Target:       hostOnly,
				Protocol:     proto,
				Port:         input.Port,
				MaxHops:      uint8(maxHops),      //nolint:gosec // clamped 1..40
				ProbesPerHop: uint8(probesPerHop), //nolint:gosec // clamped 1..5
				TimeoutMs:    &timeoutMs,
			},
		}
		if !websocket.DispatchNetProbeTask(ctx, n.NodeID, dispatch) {
			res.Status = NodeResultStatusOffline
			res.Error = errNodeWSUnavailable
		}
		state.Nodes = append(state.Nodes, res)
	}
	saveTaskState(ctx, state)
	return state, nil
}

// GetTask fetches the latest aggregated multi-node diagnostic task snapshot.
func GetTask(ctx context.Context, taskID string) (*ProbeTaskState, error) {
	return loadTaskState(ctx, taskID)
}

// ListTracerouteHistory returns paginated historical traceroute records with decoded hops and AS paths.
func ListTracerouteHistory(ctx context.Context, query TracerouteHistoryQuery) (*TracerouteHistoryPage, error) {
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}
	rows, total, err := repository.ListOpenFlareTracerouteRecords(ctx, model.TracerouteRecordQuery{
		Page:        query.Page,
		PageSize:    query.PageSize,
		NodeID:      strings.TrimSpace(query.NodeID),
		Target:      strings.TrimSpace(query.Target),
		TriggerType: strings.TrimSpace(query.TriggerType),
		TargetASN:   query.TargetASN,
	})
	if err != nil {
		return nil, err
	}

	results := make([]TracerouteHistoryItem, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		var hops []netquality.EnrichedTracerouteHop
		_ = json.Unmarshal([]byte(r.HopsJSON), &hops)
		var asPath []uint32
		_ = json.Unmarshal([]byte(r.ASPathJSON), &asPath)
		results = append(results, TracerouteHistoryItem{
			ID:            r.ID,
			TaskID:        r.TaskID,
			TriggerType:   r.TriggerType,
			NodeID:        r.NodeID,
			Target:        r.Target,
			ResolvedIP:    r.ResolvedIP,
			TargetASN:     r.TargetASN,
			TargetCountry: r.TargetCountry,
			Protocol:      r.Protocol,
			HopCount:      r.HopCount,
			Reached:       r.Reached,
			TotalRTTMs:    r.TotalRTTMs,
			Hops:          hops,
			ASPath:        asPath,
			ErrorMessage:  r.ErrorMessage,
			CreatedAt:     r.CreatedAt,
		})
	}
	return &TracerouteHistoryPage{
		Total:   total,
		Results: results,
	}, nil
}

// QueryDNS executes either a direct control-plane DNS lookup (with EDNS0 ECS) or dispatches across edge nodes.
func QueryDNS(ctx context.Context, input DNSQueryInput) (*DNSQueryResponse, error) {
	domain := strings.TrimSpace(strings.TrimSuffix(input.Domain, "."))
	if domain == "" {
		return nil, errors.New(errDomainRequired)
	}
	if !isValidHostnameSyntax(domain) {
		return nil, errors.New(errTargetInvalid)
	}

	qtype, err := ValidateDNSRecordType(input.RecordType)
	if err != nil {
		return nil, err
	}
	resolver, err := ValidateResolver(ctx, input.Resolver, false)
	if err != nil {
		return nil, err
	}
	ecsSubnet, err := ValidateECSSubnet(input.ECSSubnet)
	if err != nil {
		return nil, err
	}
	timeoutMsInt := clampInt(input.TimeoutMs, defaultDNSTimeoutMs, minProbeTimeoutMs, maxProbeTimeoutMs)
	timeoutMs := uint64(timeoutMsInt) //nolint:gosec // clamped positive 500..10000

	if len(input.NodeIDs) == 0 {
		direct, err := executeDirectDNSQuery(ctx, domain, qtype, resolver, ecsSubnet, time.Duration(timeoutMsInt)*time.Millisecond)
		if err != nil {
			return nil, err
		}
		return &DNSQueryResponse{
			Mode:         "direct",
			DirectResult: direct,
		}, nil
	}

	nodes, err := selectDispatchNodes(ctx, input.NodeIDs)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	taskID := "dns-" + uuid.NewString()
	state := &ProbeTaskState{
		TaskID:     taskID,
		Kind:       "dns_query",
		Target:     domain,
		CreatedAt:  now,
		ExpiresAt:  now.Add(taskTTL),
		TotalNodes: len(nodes),
		Status:     TaskStatusRunning,
		Nodes:      make([]*ProbeNodeResult, 0, len(nodes)),
	}
	for _, n := range nodes {
		res := &ProbeNodeResult{
			NodeID:   n.NodeID,
			NodeName: n.Name,
			NodeGeo:  n.GeoName,
			NodeASN:  n.ASN,
			Status:   NodeResultStatusPending,
		}
		dispatch := protocol.NetProbeTaskDispatch{
			ProbeID: taskID,
			Kind:    "dns_query",
			DNS: &protocol.DNSQueryRequest{
				Domain:     domain,
				RecordType: qtype,
				Resolver:   resolver,
				ECSSubnet:  ecsSubnet,
				TimeoutMs:  &timeoutMs,
			},
		}
		if !websocket.DispatchNetProbeTask(ctx, n.NodeID, dispatch) {
			res.Status = NodeResultStatusOffline
			res.Error = errNodeWSUnavailable
		}
		state.Nodes = append(state.Nodes, res)
	}
	saveTaskState(ctx, state)
	return &DNSQueryResponse{
		Mode: "nodes",
		Task: state,
	}, nil
}

func executeDirectDNSQuery(
	ctx context.Context,
	domain, qtype, resolver, ecsSubnet string,
	timeout time.Duration,
) (*DirectDNSQueryResult, error) {
	rrType, ok := dns.StringToType[qtype]
	if !ok {
		return nil, errors.New(errDNSRecordTypeInvalid)
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), rrType)
	msg.RecursionDesired = true

	if ecsSubnet != "" {
		prefix, err := netip.ParsePrefix(ecsSubnet)
		if err != nil {
			return nil, errors.New(errECSSubnetInvalid)
		}
		addr := prefix.Masked().Addr()
		opt := &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
				Class:  dns.DefaultMsgSize,
			},
		}
		subnet := &dns.EDNS0_SUBNET{
			Code:        dns.EDNS0SUBNET,
			SourceScope: 0,
			Address:     net.ParseIP(addr.String()),
		}
		if addr.Is4() {
			subnet.Family = 1
			subnet.SourceNetmask = uint8(prefix.Bits()) //nolint:gosec // validated 8..32
		} else {
			subnet.Family = 2
			subnet.SourceNetmask = uint8(prefix.Bits()) //nolint:gosec // validated 24..128
		}
		opt.Option = append(opt.Option, subnet)
		msg.Extra = append(msg.Extra, opt)
	}

	usedTCP := false
	client := &dns.Client{Net: "udp", Timeout: timeout}
	resp, rtt, err := client.ExchangeContext(ctx, msg, resolver)
	if err == nil && resp != nil && resp.Truncated {
		usedTCP = true
		tcpClient := &dns.Client{Net: "tcp", Timeout: timeout}
		tcpResp, tcpRTT, tcpErr := tcpClient.ExchangeContext(ctx, msg, resolver)
		if tcpErr == nil && tcpResp != nil {
			resp = tcpResp
			rtt = tcpRTT
		}
	}
	if err != nil {
		return nil, fmt.Errorf("DNS query failed: %w", err)
	}

	records := make([]EnrichedDNSRecord, 0, min(len(resp.Answer), maxDirectDNSAnswers))
	for _, rr := range resp.Answer {
		if len(records) >= maxDirectDNSAnswers {
			break
		}
		hdr := rr.Header()
		rtype := dns.TypeToString[hdr.Rrtype]
		val := formatDNSRecordValue(rr)
		entry := EnrichedDNSRecord{
			DNSRecord: protocol.DNSRecord{
				Name:       strings.TrimSuffix(hdr.Name, "."),
				RecordType: rtype,
				TTL:        hdr.Ttl,
				Value:      val,
			},
		}
		if rtype == "A" || rtype == "AAAA" {
			if info := netintel.LookupIP(val); info != nil {
				entry.CountryISO = info.CountryISO
				entry.CityName = info.CityName
				entry.ASN = info.ASN
				entry.ASNName = info.ASNName
				entry.ASNTags = info.ASNTags
			}
		}
		records = append(records, entry)
	}

	rcodeStr := dns.RcodeToString[resp.Rcode]
	if rcodeStr == "" {
		rcodeStr = strconv.Itoa(resp.Rcode)
	}
	return &DirectDNSQueryResult{
		Domain:        domain,
		RecordType:    qtype,
		Resolver:      resolver,
		ECSSubnet:     ecsSubnet,
		RCode:         rcodeStr,
		Authoritative: resp.Authoritative,
		Truncated:     resp.Truncated,
		UsedTCP:       usedTCP,
		LatencyMs:     math.Round(float64(rtt.Microseconds())/usPerCentisecond) / centisPerSecond,
		Records:       records,
	}, nil
}

func formatDNSRecordValue(rr dns.RR) string {
	switch v := rr.(type) {
	case *dns.A:
		return v.A.String()
	case *dns.AAAA:
		return v.AAAA.String()
	case *dns.CNAME:
		return strings.TrimSuffix(v.Target, ".")
	case *dns.NS:
		return strings.TrimSuffix(v.Ns, ".")
	case *dns.PTR:
		return strings.TrimSuffix(v.Ptr, ".")
	case *dns.MX:
		return fmt.Sprintf("%d %s", v.Preference, strings.TrimSuffix(v.Mx, "."))
	case *dns.TXT:
		return strings.Join(v.Txt, " ")
	default:
		full := rr.String()
		hdr := rr.Header().String()
		return strings.TrimSpace(strings.TrimPrefix(full, hdr))
	}
}

func resolveLookupTargetIPs(ctx context.Context, query string) ([]string, error) {
	if addr, err := netip.ParseAddr(query); err == nil {
		return []string{addr.Unmap().String()}, nil
	}
	if prefix, err := netip.ParsePrefix(query); err == nil {
		return []string{prefix.Addr().Unmap().String()}, nil
	}
	_, host, vErr := ValidateProbeTarget(ctx, query, true, false)
	if vErr != nil {
		return nil, vErr
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return []string{addr.Unmap().String()}, nil
	}
	resolveCtx, cancel := context.WithTimeout(ctx, hostResolveTimeout)
	defer cancel()
	addrs, rErr := net.DefaultResolver.LookupNetIP(resolveCtx, "ip", host)
	if rErr != nil || len(addrs) == 0 {
		return nil, errors.New(errHostnameNoPublicIP)
	}
	ips := make([]string, 0, min(len(addrs), maxResolvedHostnameIPs))
	for _, a := range addrs {
		unmapped := a.Unmap()
		if !isBlockedAddr(unmapped) {
			ips = append(ips, unmapped.String())
		}
		if len(ips) >= maxResolvedHostnameIPs {
			break
		}
	}
	if len(ips) == 0 {
		return nil, errors.New(errTargetPrivateBlocked)
	}
	return ips, nil
}

// LookupIP resolves geolocation, ASN, bgp.tools routing table metadata, and neighbor graph for an IP or hostname.
func LookupIP(ctx context.Context, rawQuery string) (*IPLookupResponse, error) {
	query := strings.TrimSpace(rawQuery)
	if query == "" {
		return nil, errors.New(errLookupInputRequired)
	}

	ips, err := resolveLookupTargetIPs(ctx, query)
	if err != nil {
		return nil, err
	}

	results := make([]*pkgnetintel.IPLookupResult, 0, len(ips))
	for _, ip := range ips {
		if res := netintel.LookupIP(ip); res != nil {
			results = append(results, res)
		}
	}
	var primary *pkgnetintel.IPLookupResult
	var asnInfo *pkgnetintel.ASNInfo
	var neighbors []ASNeighborItem
	if len(results) > 0 {
		primary = results[0]
		if primary.ASN > 0 {
			asnInfo = netintel.LookupASN(primary.ASN)
			neighbors = buildCombinedNeighbors(ctx, primary.ASN, ipNeighborsLimit)
			if asnInfo != nil && asnInfo.NeighborCnt == 0 && len(neighbors) > 0 {
				asnInfo.NeighborCnt = len(neighbors)
			}
		}
	}
	if neighbors == nil {
		neighbors = []ASNeighborItem{}
	}

	return &IPLookupResponse{
		Query:       query,
		ResolvedIPs: ips,
		Primary:     primary,
		AllResults:  results,
		ASNInfo:     asnInfo,
		Neighbors:   neighbors,
		Attribution: attributionBGPTools,
	}, nil
}

// LookupASN returns ASN metadata and merged CAIDA + observed topology neighbors.
func LookupASN(ctx context.Context, asn uint32) (*ASNDetailResponse, error) {
	if asn == 0 {
		return nil, errors.New(errInvalidASN)
	}
	info := netintel.LookupASN(asn)
	neighbors := buildCombinedNeighbors(ctx, asn, asnNeighborsLimit)
	if info == nil {
		info = &pkgnetintel.ASNInfo{ASN: asn}
	}
	if info.NeighborCnt == 0 && len(neighbors) > 0 {
		info.NeighborCnt = len(neighbors)
	}
	return &ASNDetailResponse{
		ASNInfo:     info,
		Neighbors:   neighbors,
		Attribution: attributionBGPTools,
	}, nil
}

// ListASNPrefixes returns paginated prefixes announced by asn.
func ListASNPrefixes(_ context.Context, asn uint32, family string, page, pageSize int) (*ASNPrefixesResponse, error) {
	if asn == 0 {
		return nil, errors.New(errInvalidASN)
	}
	records, total := netintel.ListASNPrefixes(asn, family, page, pageSize)
	if records == nil {
		records = []pkgnetintel.PrefixRecord{}
	}
	return &ASNPrefixesResponse{
		ASN:         asn,
		Total:       int64(total),
		Results:     records,
		Attribution: attributionBGPTools,
	}, nil
}

type resolvedTargetQuery struct {
	targetIP      string
	targetASN     uint32
	targetCountry string
	targetRegion  string
	ipFamily      string
	clientLat     *float64
	clientLon     *float64
	geoAccuracyKm float64
	geoSource     string
	anycastStatus string
}

func enrichResolvedTargetFromIP(out *resolvedTargetQuery) {
	if out.targetIP == "" {
		return
	}
	if out.ipFamily == "" {
		out.ipFamily = pkgnetintel.DetectIPFamily(out.targetIP)
	}
	res := netintel.LookupIP(out.targetIP)
	if res == nil {
		return
	}
	if out.targetASN == 0 {
		out.targetASN = res.ASN
	}
	if out.targetCountry == "" {
		out.targetCountry = res.CountryISO
	}
	if out.targetRegion == "" {
		out.targetRegion = res.RegionCode
	}
	out.clientLat = res.Latitude
	out.clientLon = res.Longitude
	out.geoAccuracyKm = res.AccuracyRadiusKm
	out.geoSource = res.GeoSource
	out.anycastStatus = res.AnycastStatus
}

func resolveTargetQueryContext(
	targetIP string,
	targetASN uint32,
	targetCountry string,
	targetRegion string,
	ipFamily string,
) resolvedTargetQuery {
	out := resolvedTargetQuery{
		targetIP:      strings.TrimSpace(targetIP),
		targetASN:     targetASN,
		targetCountry: strings.ToUpper(strings.TrimSpace(targetCountry)),
		targetRegion:  strings.ToUpper(strings.TrimSpace(targetRegion)),
		ipFamily:      strings.ToLower(strings.TrimSpace(ipFamily)),
	}
	enrichResolvedTargetFromIP(&out)
	if out.targetCountry == "" && out.targetASN > 0 {
		if info := netintel.LookupASN(out.targetASN); info != nil && info.Country != "" {
			out.targetCountry = strings.ToUpper(strings.TrimSpace(info.Country))
		}
	}
	if out.targetCountry != "" && out.targetRegion != "" {
		out.targetRegion = pkgnetintel.NormalizeMatrixRegion(out.targetCountry, out.targetRegion)
	}
	if out.ipFamily != pkgnetintel.IPFamilyIPv4 && out.ipFamily != pkgnetintel.IPFamilyIPv6 {
		out.ipFamily = pkgnetintel.IPFamilyUnknown
	}
	return out
}

func predictionDataSource(method string) string {
	if method == netquality.PredictionMethodExactRegion || method == netquality.PredictionMethodCountryRollup {
		return model.QualityMatrixDataSourceMeasured
	}
	return model.QualityMatrixDataSourcePredicted
}

func buildPredictQueryForNode(nodeID string, target resolvedTargetQuery) netquality.PredictQuery {
	return netquality.PredictQuery{
		NodeID:        nodeID,
		ClientASN:     target.targetASN,
		ClientCountry: target.targetCountry,
		ClientRegion:  target.targetRegion,
		IPFamily:      target.ipFamily,
		ClientLat:     target.clientLat,
		ClientLon:     target.clientLon,
		GeoAccuracyKm: target.geoAccuracyKm,
		GeoSource:     target.geoSource,
		AnycastStatus: target.anycastStatus,
	}
}

func buildASPathComparisonItem(
	n model.OpenFlareNode,
	target resolvedTargetQuery,
	predictor *netquality.Predictor,
	matrixByNode map[string]*model.OpenFlareNetworkQualityMatrix,
	observedAdj map[uint32][]uint32,
) NodeASPathComparison {
	if n.ASN == 0 && strings.TrimSpace(n.IP) != "" {
		netintel.EnrichNodeASN(&n)
	}
	predRes := predictor.Predict(buildPredictQueryForNode(n.NodeID, target))
	item := NodeASPathComparison{
		NodeID:                n.NodeID,
		NodeName:              n.Name,
		NodeIP:                n.IP,
		NodeGeo:               n.GeoName,
		NodeASN:               n.ASN,
		NodeASNName:           n.ASNOrg,
		NetworkGroup:          n.NetworkGroup,
		GeoSource:             n.GeoSource,
		EffectiveRTTMs:        predRes.RTTMs,
		RTTP95Ms:              predRes.RTTP95Ms,
		CandidateDispersionMs: predRes.CandidateDispersionMs,
		PredictionErrorMs:     predRes.ErrorMs,
		QualityScore:          netquality.ComputeQualityScore(predRes),
		Confidence:            predRes.Confidence,
		MeasurementSource:     predRes.MeasurementSource,
		PredictionMethod:      predRes.PredictionMethod,
		GeoBaseRTTMs:          predRes.GeoBaseRTTMs,
		SampleCount:           predRes.SampleCount,
		DistinctPrefixes:      predRes.DistinctPrefixes,
		LastObservedAt:        predRes.LastObservedAt,
		AnycastIsolated:       predRes.AnycastIsolated,
		DataSource:            predictionDataSource(predRes.PredictionMethod),
	}
	if m := matrixByNode[n.NodeID]; m != nil {
		var recordedPath []uint32
		if json.Unmarshal([]byte(m.TraceASPath), &recordedPath) == nil && len(recordedPath) > 0 {
			item.ASHops = max(0, len(recordedPath)-1)
			item.Path = enrichASPathHops(recordedPath)
		}
	}
	if len(item.Path) == 0 && n.ASN > 0 && target.targetASN > 0 {
		if hops, path, ok := netintel.EstimateASPath(n.ASN, target.targetASN, observedAdj); ok {
			item.ASHops = hops
			item.Path = enrichASPathHops(path)
		}
	}
	if item.Path == nil {
		item.Path = []ASPathHopNode{}
	}
	return item
}

func loadObservedAdjacencyMap(ctx context.Context) map[uint32][]uint32 {
	edges, _ := repository.ListOpenFlareASTopologyEdges(ctx, maxASPathObservedEdge)
	observedAdj := make(map[uint32][]uint32)
	for _, e := range edges {
		if e == nil || e.SrcASN == 0 || e.DstASN == 0 || e.SrcASN == e.DstASN {
			continue
		}
		observedAdj[e.SrcASN] = append(observedAdj[e.SrcASN], e.DstASN)
		observedAdj[e.DstASN] = append(observedAdj[e.DstASN], e.SrcASN)
	}
	return observedAdj
}

func indexPreferredMatrixTraceByNode(
	rows []*model.OpenFlareNetworkQualityMatrix,
	country, ipFamily string,
) map[string]*model.OpenFlareNetworkQualityMatrix {
	best := make(map[string]*model.OpenFlareNetworkQualityMatrix, len(rows))
	bestScore := make(map[string]int, len(rows))
	for _, m := range rows {
		if m == nil || strings.TrimSpace(m.TraceASPath) == "" || m.TraceASPath == "[]" {
			continue
		}
		score := 1
		if country != "" && strings.EqualFold(m.ClientCountry, country) {
			score += 2
		}
		if ipFamily != "" && strings.EqualFold(m.IPFamily, ipFamily) {
			score++
		}
		if prevScore, ok := bestScore[m.NodeID]; !ok || score > prevScore {
			best[m.NodeID] = m
			bestScore[m.NodeID] = score
		}
	}
	return best
}

// TraceASPath compares estimated AS paths and measured/predicted quality across edge nodes toward a target ASN or IP.
func TraceASPath(ctx context.Context, input ASPathInput) (*ASPathResponse, error) {
	resolved := resolveTargetQueryContext(input.TargetIP, input.TargetASN, input.TargetCountry, input.TargetRegion, input.IPFamily)
	if resolved.targetASN == 0 {
		return nil, errors.New(errASPathTargetRequired)
	}
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}

	nodes, err := selectComparisonNodes(ctx, input.NodeIDs)
	if err != nil {
		return nil, err
	}
	predictor, err := netquality.LoadLivePredictor(ctx)
	if err != nil {
		return nil, err
	}
	observedAdj := loadObservedAdjacencyMap(ctx)
	matrixRows, _ := repository.ListOpenFlareNetworkQualityMatrixByClientASN(ctx, resolved.targetASN)
	matrixByNode := indexPreferredMatrixTraceByNode(matrixRows, resolved.targetCountry, resolved.ipFamily)

	comparisons := make([]NodeASPathComparison, 0, len(nodes))
	for _, n := range nodes {
		comparisons = append(comparisons, buildASPathComparisonItem(n, resolved, predictor, matrixByNode, observedAdj))
	}

	sort.Slice(comparisons, func(i, j int) bool {
		if comparisons[i].QualityScore != comparisons[j].QualityScore {
			return comparisons[i].QualityScore > comparisons[j].QualityScore
		}
		return comparisons[i].ASHops < comparisons[j].ASHops
	})

	return &ASPathResponse{
		TargetASN:     resolved.targetASN,
		TargetASNInfo: netintel.LookupASN(resolved.targetASN),
		TargetCountry: resolved.targetCountry,
		TargetRegion:  resolved.targetRegion,
		IPFamily:      resolved.ipFamily,
		Nodes:         comparisons,
		Attribution:   attributionBGPTools,
	}, nil
}

// PredictQualityMatrix predicts client-to-node latency and SmartDNS ranking scores across edge nodes.
func PredictQualityMatrix(ctx context.Context, input QualityPredictInput) (*QualityPredictResponse, error) {
	resolved := resolveTargetQueryContext(input.TargetIP, input.TargetASN, input.TargetCountry, input.TargetRegion, input.IPFamily)
	if resolved.targetIP == "" && resolved.targetASN == 0 && resolved.targetCountry == "" {
		return nil, errors.New(errPredictTargetRequired)
	}
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}
	nodes, err := selectComparisonNodes(ctx, input.NodeIDs)
	if err != nil {
		return nil, err
	}
	predictor, err := netquality.LoadLivePredictor(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]NodeQualityPrediction, 0, len(nodes))
	for _, n := range nodes {
		if n.ASN == 0 && strings.TrimSpace(n.IP) != "" {
			netintel.EnrichNodeASN(&n)
		}
		res := predictor.Predict(buildPredictQueryForNode(n.NodeID, resolved))
		out = append(out, NodeQualityPrediction{
			NodeID:                n.NodeID,
			NodeName:              n.Name,
			NodeIP:                n.IP,
			NodeGeo:               n.GeoName,
			NodeASN:               n.ASN,
			NodeASNName:           n.ASNOrg,
			NetworkGroup:          n.NetworkGroup,
			GeoSource:             n.GeoSource,
			EffectiveRTTMs:        res.RTTMs,
			RTTP95Ms:              res.RTTP95Ms,
			CandidateDispersionMs: res.CandidateDispersionMs,
			PredictionErrorMs:     res.ErrorMs,
			QualityScore:          netquality.ComputeQualityScore(res),
			Confidence:            res.Confidence,
			MeasurementSource:     res.MeasurementSource,
			PredictionMethod:      res.PredictionMethod,
			GeoBaseRTTMs:          res.GeoBaseRTTMs,
			SampleCount:           res.SampleCount,
			DistinctPrefixes:      res.DistinctPrefixes,
			LastObservedAt:        res.LastObservedAt,
			AnycastIsolated:       res.AnycastIsolated,
			DataSource:            predictionDataSource(res.PredictionMethod),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].QualityScore != out[j].QualityScore {
			return out[i].QualityScore > out[j].QualityScore
		}
		if out[i].EffectiveRTTMs != out[j].EffectiveRTTMs {
			return out[i].EffectiveRTTMs < out[j].EffectiveRTTMs
		}
		return out[i].Confidence > out[j].Confidence
	})

	var targetInfo *pkgnetintel.ASNInfo
	if resolved.targetASN > 0 {
		targetInfo = netintel.LookupASN(resolved.targetASN)
	}
	return &QualityPredictResponse{
		TargetIP:      resolved.targetIP,
		TargetASN:     resolved.targetASN,
		TargetASNInfo: targetInfo,
		TargetCountry: resolved.targetCountry,
		TargetRegion:  resolved.targetRegion,
		IPFamily:      resolved.ipFamily,
		AnycastStatus: resolved.anycastStatus,
		GeoSource:     resolved.geoSource,
		Nodes:         out,
		Attribution:   attributionBGPTools,
	}, nil
}

// ListQualityMatrix returns paginated network quality matrix entries enriched with node identity and geo info.
func ListQualityMatrix(ctx context.Context, query model.NetworkQualityMatrixQuery) (*QualityMatrixPage, error) {
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}
	rows, total, err := repository.ListOpenFlareNetworkQualityMatrix(ctx, query)
	if err != nil {
		return nil, err
	}
	nodes, _ := repository.ListOpenFlareNodes(ctx)
	nodeMap := make(map[string]model.OpenFlareNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].NodeID] = nodes[i]
	}

	items := make([]QualityMatrixItem, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		n := nodeMap[r.NodeID]
		items = append(items, QualityMatrixItem{
			OpenFlareNetworkQualityMatrix: r,
			NodeName:                      n.Name,
			NodeGeo:                       n.GeoName,
			NodeStatus:                    n.Status,
			NodeNetworkGroup:              n.NetworkGroup,
			NodeGeoSource:                 n.GeoSource,
		})
	}
	return &QualityMatrixPage{
		Total:   total,
		Results: items,
	}, nil
}

// GetNetIntelStatus returns status of all NetIntel dataset files and runtime switches.
func GetNetIntelStatus(ctx context.Context) (*NetIntelStatusResponse, error) {
	caidaEnabled := false
	autoProbe := true
	if db.DB(ctx) != nil {
		if cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelCAIDAEnabled); err == nil {
			caidaEnabled = strings.EqualFold(strings.TrimSpace(cfg.Value), "true")
		}
		if cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelAutoProbeEnabled); err == nil && strings.TrimSpace(cfg.Value) != "" {
			autoProbe = !strings.EqualFold(strings.TrimSpace(cfg.Value), "false")
		}
	}
	datasets := netintel.Statuses()
	if datasets == nil {
		datasets = []pkgnetintel.DatasetFileStatus{}
	}
	return &NetIntelStatusResponse{
		DataDir:      netintel.DataDir(),
		Datasets:     datasets,
		CAIDAEnabled: caidaEnabled,
		AutoProbe:    autoProbe,
		Attribution:  attributionBGPTools,
	}, nil
}

// TriggerNetIntelSync enqueues a background NetIntel sync task (or runs synchronously when Asynq is disabled).
func TriggerNetIntelSync(ctx context.Context, force bool) (*TriggerSyncResponse, error) {
	if task.AsynqClient != nil && task.GetTaskMeta(netintel.TaskTypeNetIntelSync) != nil {
		payloadBytes, _ := json.Marshal(netintel.SyncPayload{Force: force})
		taskID, err := task.DispatchTask(ctx, netintel.TaskTypeNetIntelSync, payloadBytes, "manual")
		if err == nil {
			return &TriggerSyncResponse{
				TaskID: taskID,
				Mode:   "queued",
			}, nil
		}
		logger.WarnF(ctx, "[NetTools] enqueue of_netintel_sync failed, falling back to inline sync: %v", err)
	}
	summary, err := netintel.SyncDatasets(ctx, force)
	if err != nil {
		return nil, err
	}
	return &TriggerSyncResponse{
		Mode:    "inline",
		Summary: summary,
	}, nil
}

func selectDispatchNodes(ctx context.Context, requestedIDs []string) ([]model.OpenFlareNode, error) {
	if db.DB(ctx) == nil {
		return nil, errors.New(errDatabaseUnavailable)
	}
	var candidates []model.OpenFlareNode
	var err error
	cleanIDs := cleanNodeIDSlice(requestedIDs)
	if len(cleanIDs) > 0 {
		candidates, err = repository.ListOpenFlareNodesByNodeIDs(ctx, cleanIDs)
	} else {
		candidates, err = repository.ListOpenFlareNodes(ctx)
	}
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	selected := make([]model.OpenFlareNode, 0, min(len(candidates), maxDispatchNodes))
	for i := range candidates {
		n := candidates[i]
		if n.IsQuarantined() {
			continue
		}
		if n.NodeType != "" && n.NodeType != "edge_node" {
			continue
		}
		if len(cleanIDs) == 0 && n.Status != "online" && (n.LastSeenAt == nil || now.Sub(*n.LastSeenAt) > onlineNodeWindow) {
			continue
		}
		if n.ASN == 0 && strings.TrimSpace(n.IP) != "" {
			netintel.EnrichNodeASN(&n)
		}
		selected = append(selected, n)
		if len(selected) >= maxDispatchNodes {
			break
		}
	}
	if len(selected) == 0 {
		return nil, errors.New(errNoEligibleNodes)
	}
	return selected, nil
}

func selectComparisonNodes(ctx context.Context, requestedIDs []string) ([]model.OpenFlareNode, error) {
	cleanIDs := cleanNodeIDSlice(requestedIDs)
	if len(cleanIDs) > 0 {
		return repository.ListOpenFlareNodesByNodeIDs(ctx, cleanIDs)
	}
	all, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.OpenFlareNode, 0, len(all))
	for i := range all {
		if all[i].NodeType == "" || all[i].NodeType == "edge_node" {
			out = append(out, all[i])
		}
	}
	return out, nil
}

func buildCombinedNeighbors(ctx context.Context, asn uint32, limit int) []ASNeighborItem {
	seen := make(map[uint32]int)
	out := make([]ASNeighborItem, 0)

	for _, n := range netintel.LookupASNeighbors(asn, limit) {
		seen[n.PeerASN] = len(out)
		out = append(out, ASNeighborItem{
			PeerASN:     n.PeerASN,
			PeerName:    n.PeerName,
			PeerCountry: n.PeerCountry,
			PeerClass:   n.PeerClass,
			Relation:    n.Relation,
			Tags:        n.Tags,
		})
	}
	if len(out) == 0 {
		for _, n := range getRIPEStatNeighborsCached(ctx, asn, limit) {
			seen[n.PeerASN] = len(out)
			out = append(out, n)
		}
	}

	if db.DB(ctx) == nil {
		return out
	}
	observed, err := repository.ListOpenFlareASTopologyEdgesByASN(ctx, asn)
	if err != nil {
		return out
	}
	for _, edge := range observed {
		out = mergeObservedNeighbor(out, seen, asn, edge, limit)
	}
	return out
}

func mergeObservedNeighbor(
	out []ASNeighborItem,
	seen map[uint32]int,
	asn uint32,
	edge *model.OpenFlareASTopologyEdge,
	limit int,
) []ASNeighborItem {
	if edge == nil {
		return out
	}
	peerASN := edge.DstASN
	if peerASN == asn {
		peerASN = edge.SrcASN
	}
	if peerASN == 0 || peerASN == asn {
		return out
	}
	if idx, ok := seen[peerASN]; ok {
		out[idx].ObservedRTT = edge.AvgHopRTTMs
		out[idx].SampleCount = edge.SampleCount
		return out
	}
	if len(out) >= limit {
		return out
	}
	item := ASNeighborItem{
		PeerASN:     peerASN,
		Relation:    pkgnetintel.RelationObserved,
		ObservedRTT: edge.AvgHopRTTMs,
		SampleCount: edge.SampleCount,
	}
	if peerInfo := netintel.LookupASN(peerASN); peerInfo != nil {
		item.PeerName = peerInfo.Name
		item.PeerCountry = peerInfo.Country
		item.PeerClass = peerInfo.Class
		item.Tags = peerInfo.Tags
	}
	seen[peerASN] = len(out)
	return append(out, item)
}

func enrichASPathHops(path []uint32) []ASPathHopNode {
	out := make([]ASPathHopNode, 0, len(path))
	for _, asn := range path {
		node := ASPathHopNode{ASN: asn}
		if info := netintel.LookupASN(asn); info != nil {
			node.Name = info.Name
			node.Country = info.Country
			node.Tags = info.Tags
		}
		out = append(out, node)
	}
	return out
}

func cleanNodeIDSlice(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func clampInt(val, def, minVal, maxVal int) int {
	if val <= 0 {
		val = def
	}
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
