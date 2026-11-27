// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

const (
	overloadLoadThreshold = 0.92
	hysteresisMargin      = 1.05
	hysteresisTTL         = 10 * time.Minute
	minTagMultiplier      = 0.25
	maxTagMultiplier      = 4.0
)

type hysteresisEntry struct {
	nodeID    string
	updatedAt time.Time
}

// HysteresisTracker bounds sticky state to avoid route flapping on small (<5%) quality swings.
type HysteresisTracker struct {
	mu    sync.Mutex
	cache *ram.Cache[string, hysteresisEntry]
}

// NewHysteresisTracker creates a bounded process-local sticky route cache.
func NewHysteresisTracker(maxEntries int) *HysteresisTracker {
	if maxEntries <= 0 {
		maxEntries = 16384
	}
	return &HysteresisTracker{
		cache: ram.MustNew[string, hysteresisEntry](ram.Options{MaximumSize: maxEntries}),
	}
}

func (h *HysteresisTracker) get(key string, now time.Time) string {
	if h == nil || h.cache == nil {
		return ""
	}
	entry, ok := h.cache.GetIfPresent(key)
	if !ok || now.Sub(entry.updatedAt) > hysteresisTTL {
		return ""
	}
	return entry.nodeID
}

func (h *HysteresisTracker) set(key, nodeID string, now time.Time) {
	if h == nil || h.cache == nil || nodeID == "" {
		return
	}
	h.mu.Lock()
	h.cache.Set(key, hysteresisEntry{nodeID: nodeID, updatedAt: now})
	h.mu.Unlock()
}

// SelectInput encapsulates everything needed for deterministic 4-stage SmartDNS node selection.
type SelectInput struct {
	Domain    string
	QType     string // "A" or "AAAA"
	ClientKey string // normalized /24 or /48 client prefix
	Client    qualityapi.ClientProfile
	Nodes     []protocol.DNSSnapshotNode
	Policy    protocol.DNSSnapshotPolicy
	Quality   map[string]qualityapi.NodeLinkQuality
	Hysteresis *HysteresisTracker
	Now       time.Time
}

// SelectedAnswer represents a chosen node IP answer and its telemetry attribution.
type SelectedAnswer struct {
	NodeID string  `json:"node_id"`
	Name   string  `json:"name"`
	IP     string  `json:"ip"`
	Score  float64 `json:"score"`
}

// CandidateTrace captures full per-node breakdown for debugging and the simulator UI.
type CandidateTrace struct {
	NodeID          string  `json:"node_id"`
	Name            string  `json:"name"`
	IP              string  `json:"ip"`
	Eligible        bool    `json:"eligible"`
	Overloaded      bool    `json:"overloaded"`
	Selected        bool    `json:"selected"`
	ExclusionReason string  `json:"exclusion_reason,omitempty"`
	SourceTier      string  `json:"source_tier,omitempty"`
	EffectiveRTTMs  float64 `json:"effective_rtt_ms"`
	LossRate        float64 `json:"loss_rate"`
	LoadIndex       float64 `json:"load_index"`
	LatencyScore    float64 `json:"latency_score"`
	LoadScore       float64 `json:"load_score"`
	GeoScore        float64 `json:"geo_score"`
	TagMultiplier   float64 `json:"tag_multiplier"`
	FinalScore      float64 `json:"final_score"`
}

// SelectionResult contains the selected DNS answers, matched policy branch, and scoring trace.
type SelectionResult struct {
	PolicyID          uint             `json:"policy_id"`
	PolicyName        string           `json:"policy_name"`
	Mode              string           `json:"mode"`
	TTL               int              `json:"ttl"`
	FallbackUsed      bool             `json:"fallback_used"`
	FallbackReason    string           `json:"fallback_reason,omitempty"`
	HysteresisApplied bool             `json:"hysteresis_applied"`
	MatchedNodes      []string         `json:"matched_nodes"`
	MatchedConditions []string         `json:"matched_conditions"`
	Answers           []SelectedAnswer `json:"answers"`
	Candidates        []CandidateTrace `json:"candidates"`
}

type scoredNode struct {
	node  protocol.DNSSnapshotNode
	ip    string
	trace CandidateTrace
	rank  float64 // sorting rank (may differ slightly from FinalScore for deterministic weighted LB)
}

// SelectNodes runs JDM policy evaluation followed by the 4-stage SmartDNS selection algorithm.
func SelectNodes(in SelectInput) SelectionResult {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	qtype := strings.ToUpper(strings.TrimSpace(in.QType))
	if qtype != "AAAA" {
		qtype = "A"
	}
	baseDecision := PolicyDecision{
		Mode:          normalizeMode(in.Policy.StrategyType),
		Weights:       defaultPolicyConfig(in.Policy.StrategyType).Weights,
		UseTagWeights: true,
		MaxAnswers:    maxInt(in.Policy.MaxAnswers, 1),
		TTL:           maxInt(in.Policy.TTL, 60),
	}
	evalCtx := EvalContext{
		Country:  strings.ToUpper(strings.TrimSpace(in.Client.CountryISO)),
		Region:   strings.ToUpper(strings.TrimSpace(in.Client.RegionCode)),
		ASN:      in.Client.ASN,
		IPFamily: familyForQType(qtype),
		Domain:   strings.TrimSuffix(strings.ToLower(strings.TrimSpace(in.Domain)), "."),
		QType:    qtype,
	}
	decision := baseDecision
	if strings.TrimSpace(in.Policy.JDMJSON) != "" {
		if evaluated, err := EvaluateJDM(in.Policy.JDMJSON, baseDecision, evalCtx); err == nil {
			decision = evaluated
		}
	}
	if decision.MaxAnswers <= 0 {
		decision.MaxAnswers = 1
	}
	if decision.TTL <= 0 {
		decision.TTL = 60
	}

	out := SelectionResult{
		PolicyID:          in.Policy.ID,
		PolicyName:        in.Policy.Name,
		Mode:              decision.Mode,
		TTL:               decision.TTL,
		MatchedNodes:      decision.MatchedNodes,
		MatchedConditions: decision.MatchedConditions,
	}

	// Stage 1 + Stage 2 + Stage 3 evaluation over all snapshot nodes.
	traces := make([]CandidateTrace, 0, len(in.Nodes))
	primary := make([]scoredNode, 0, len(in.Nodes))
	overloaded := make([]scoredNode, 0)
	unfilteredOnline := make([]scoredNode, 0, len(in.Nodes))
	anyNonQuarantined := make([]scoredNode, 0, len(in.Nodes))

	for _, node := range in.Nodes {
		ip := pickNodeIP(node, qtype)
		trace := CandidateTrace{
			NodeID: node.NodeID,
			Name:   node.Name,
			IP:     ip,
		}
		if node.Quarantined {
			trace.ExclusionReason = "quarantined"
			traces = append(traces, trace)
			continue
		}
		if ip == "" {
			trace.ExclusionReason = "missing_" + strings.ToLower(qtype)
			traces = append(traces, trace)
			continue
		}

		sn := scoreCandidate(node, ip, decision, in.Client, in.Quality[node.NodeID], in.ClientKey, evalCtx.Domain)
		anyNonQuarantined = append(anyNonQuarantined, sn)

		if !strings.EqualFold(node.Status, "online") {
			sn.trace.ExclusionReason = "offline"
			traces = append(traces, sn.trace)
			continue
		}
		unfilteredOnline = append(unfilteredOnline, sn)

		if reason := checkFilters(node, decision.Filters); reason != "" {
			sn.trace.ExclusionReason = reason
			traces = append(traces, sn.trace)
			continue
		}
		sn.trace.Eligible = true
		if sn.trace.LoadIndex > overloadLoadThreshold {
			sn.trace.Overloaded = true
			sn.trace.ExclusionReason = "load_above_0.92"
			overloaded = append(overloaded, sn)
			traces = append(traces, sn.trace)
			continue
		}
		primary = append(primary, sn)
		traces = append(traces, sn.trace)
	}

	sortScoredNodes(primary)

	var chosen []scoredNode
	if len(primary) > 0 {
		if decision.Mode == model.DNSPolicyStrategySmart && in.Hysteresis != nil {
			hystKey := hysteresisKey(in.Policy.ID, in.ClientKey, qtype)
			prevID := in.Hysteresis.get(hystKey, now)
			if prevID != "" && primary[0].node.NodeID != prevID {
				for idx := 1; idx < len(primary); idx++ {
					if primary[idx].node.NodeID == prevID {
						if primary[0].trace.FinalScore <= primary[idx].trace.FinalScore*hysteresisMargin {
							promoted := primary[idx]
							copy(primary[1:idx+1], primary[0:idx])
							primary[0] = promoted
							out.HysteresisApplied = true
						}
						break
					}
				}
			}
			in.Hysteresis.set(hystKey, primary[0].node.NodeID, now)
		}
		limit := minInt(decision.MaxAnswers, len(primary))
		chosen = append(chosen, primary[:limit]...)
	} else {
		// Stage 4: Mandatory Fallback Chain.
		out.FallbackUsed = true
		chosen, out.FallbackReason = runFallback(in, decision, overloaded, unfilteredOnline, anyNonQuarantined)
	}

	selectedSet := make(map[string]bool, len(chosen))
	out.Answers = make([]SelectedAnswer, 0, len(chosen))
	for _, item := range chosen {
		selectedSet[item.node.NodeID] = true
		out.Answers = append(out.Answers, SelectedAnswer{
			NodeID: item.node.NodeID,
			Name:   item.node.Name,
			IP:     item.ip,
			Score:  round4(item.trace.FinalScore),
		})
	}
	for i := range traces {
		if selectedSet[traces[i].NodeID] {
			traces[i].Selected = true
		}
	}
	out.Candidates = traces
	return out
}

func runFallback(
	in SelectInput,
	decision PolicyDecision,
	overloaded []scoredNode,
	unfilteredOnline []scoredNode,
	anyNonQuarantined []scoredNode,
) ([]scoredNode, string) {
	maxAns := maxInt(decision.MaxAnswers, 1)
	strat := strings.ToLower(strings.TrimSpace(in.Policy.FallbackStrat))
	if strat == "" {
		strat = model.DNSPolicyFallbackSmart
	}

	if strat == model.DNSPolicyFallbackStaticNodes && len(in.Policy.FallbackNodeIDs) > 0 {
		allowed := make(map[string]bool, len(in.Policy.FallbackNodeIDs))
		for _, id := range in.Policy.FallbackNodeIDs {
			allowed[id] = true
		}
		var static []scoredNode
		for _, cand := range unfilteredOnline {
			if allowed[cand.node.NodeID] {
				static = append(static, cand)
			}
		}
		sortScoredNodes(static)
		if len(static) > 0 {
			return static[:minInt(maxAns, len(static))], "static_nodes"
		}
	}

	if len(overloaded) > 0 {
		sortScoredNodes(overloaded)
		return overloaded[:minInt(maxAns, len(overloaded))], "overloaded_filtered_pool"
	}
	if len(unfilteredOnline) > 0 {
		sortScoredNodes(unfilteredOnline)
		return unfilteredOnline[:minInt(maxAns, len(unfilteredOnline))], strat
	}
	if len(anyNonQuarantined) > 0 {
		sortScoredNodes(anyNonQuarantined)
		return anyNonQuarantined[:minInt(maxAns, len(anyNonQuarantined))], "non_quarantined_safety_net"
	}
	return nil, "no_eligible_nodes"
}

func scoreCandidate(
	node protocol.DNSSnapshotNode,
	ip string,
	decision PolicyDecision,
	client qualityapi.ClientProfile,
	link qualityapi.NodeLinkQuality,
	clientKey string,
	domain string,
) scoredNode {
	var bwUtil, connUtil float64
	if node.CapacityBandwidthMbps > 0 && node.BandwidthMbps > 0 {
		bwUtil = clampFloat(node.BandwidthMbps/float64(node.CapacityBandwidthMbps), 0, 1.5)
	}
	if node.CapacityConnections > 0 && node.ActiveConnections > 0 {
		connUtil = clampFloat(float64(node.ActiveConnections)/float64(node.CapacityConnections), 0, 1.5)
	}
	capUtil := math.Max(bwUtil, connUtil)
	cpuRatio := clampFloat(node.CPUUsagePercent, 0, 100) / 100.0
	memRatio := clampFloat(node.MemoryUsagePercent, 0, 100) / 100.0
	loadIndex := 0.45*cpuRatio + 0.25*memRatio + 0.30*capUtil
	sLoad := math.Max(0, 1.0-math.Pow(clampFloat(loadIndex, 0, 1.5), 1.8))

	sGeo := 0.5
	if client.Latitude != nil && client.Longitude != nil && node.GeoLatitude != nil && node.GeoLongitude != nil {
		distKm := pkgnetintel.DistanceKm(*client.Latitude, *client.Longitude, *node.GeoLatitude, *node.GeoLongitude)
		sGeo = 1.0 / (1.0 + distKm/2000.0)
	}

	effRTT := link.EffectiveRTTMs
	loss := clampFloat(link.LossRate, 0, 1)
	var sLat float64
	if effRTT > 0 {
		sLat = clampFloat((1.0/(1.0+effRTT/80.0))*(1.0-loss), 0, 1)
	} else if link.QualityScore > 0 {
		sLat = clampFloat(link.QualityScore/100.0, 0, 1)
	} else {
		sLat = 0.4
	}

	tagMult := 1.0
	if decision.UseTagWeights && len(node.Tags) > 0 {
		prod := 1.0
		for _, tag := range node.Tags {
			if tag.EffectiveWeight > 0 {
				prod *= tag.EffectiveWeight
			}
		}
		tagMult = clampFloat(prod, minTagMultiplier, maxTagMultiplier)
	}

	capWt := float64(maxInt(node.CapacityWeight, 100))
	if node.CapacityWeight > 0 {
		capWt = float64(node.CapacityWeight)
	}

	var finalScore, sortRank float64
	if decision.Mode == model.DNSPolicyStrategyLoadBalance {
		g := (capWt / 100.0) * math.Max(0.05, sLoad) * tagMult
		finalScore = g
		u := deterministicUnitFloat(clientKey, domain, node.NodeID)
		sortRank = g / math.Max(1e-6, -math.Log(u))
	} else {
		wLat, wLoad, wGeo := decision.Weights.Latency, decision.Weights.Load, decision.Weights.Geo
		wSum := wLat + wLoad + wGeo
		if wSum <= 0 {
			wLat, wLoad, wGeo = 0.55, 0.30, 0.15
		} else {
			wLat, wLoad, wGeo = wLat/wSum, wLoad/wSum, wGeo/wSum
		}
		finalScore = (wLat*sLat + wLoad*sLoad + wGeo*sGeo) * tagMult * math.Pow(capWt/100.0, 0.15)
		sortRank = finalScore
	}

	return scoredNode{
		node: node,
		ip:   ip,
		rank: sortRank,
		trace: CandidateTrace{
			NodeID:         node.NodeID,
			Name:           node.Name,
			IP:             ip,
			SourceTier:     link.SourceTier,
			EffectiveRTTMs: round4(effRTT),
			LossRate:       round4(loss),
			LoadIndex:      round4(loadIndex),
			LatencyScore:   round4(sLat),
			LoadScore:      round4(sLoad),
			GeoScore:       round4(sGeo),
			TagMultiplier:  round4(tagMult),
			FinalScore:     round4(finalScore),
		},
	}
}

func sortScoredNodes(items []scoredNode) {
	sort.Slice(items, func(i, j int) bool {
		if math.Abs(items[i].rank-items[j].rank) > 1e-9 {
			return items[i].rank > items[j].rank
		}
		return items[i].node.NodeID < items[j].node.NodeID
	})
}

func checkFilters(node protocol.DNSSnapshotNode, filters PolicyFilters) string {
	if len(filters.NodeIDs) > 0 && !containsIgnoreCase(filters.NodeIDs, node.NodeID) {
		return "filtered_node_id"
	}
	if len(filters.Countries) > 0 && !containsIgnoreCase(filters.Countries, node.Country) {
		return "filtered_country"
	}
	if len(filters.NetworkGroups) > 0 && !containsIgnoreCase(filters.NetworkGroups, node.NetworkGroup) {
		return "filtered_network_group"
	}
	if len(filters.Tags) > 0 {
		matched := false
		for _, nt := range node.Tags {
			if containsIgnoreCase(filters.Tags, nt.Slug) {
				matched = true
				break
			}
		}
		if !matched {
			return "filtered_tag"
		}
	}
	return ""
}

func pickNodeIP(node protocol.DNSSnapshotNode, qtype string) string {
	if qtype == "AAAA" {
		if ip := net.ParseIP(strings.TrimSpace(node.IPv6)); ip != nil && ip.To4() == nil {
			return ip.String()
		}
		return ""
	}
	if ip := net.ParseIP(strings.TrimSpace(node.IPv4)); ip != nil && ip.To4() != nil {
		return ip.String()
	}
	return ""
}

func normalizeMode(strategy string) string {
	if strings.EqualFold(strings.TrimSpace(strategy), model.DNSPolicyStrategyLoadBalance) {
		return model.DNSPolicyStrategyLoadBalance
	}
	return model.DNSPolicyStrategySmart
}

func familyForQType(qtype string) string {
	if qtype == "AAAA" {
		return "ipv6"
	}
	return "ipv4"
}

func hysteresisKey(policyID uint, clientKey, qtype string) string {
	var b strings.Builder
	b.Grow(len(clientKey) + len(qtype) + 16)
	b.WriteString(strconvUint(uint64(policyID)))
	b.WriteByte(':')
	b.WriteString(clientKey)
	b.WriteByte(':')
	b.WriteString(qtype)
	return b.String()
}

func strconvUint(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func deterministicUnitFloat(clientKey, domain, nodeID string) float64 {
	sum := sha256.Sum256([]byte(clientKey + "|" + domain + "|" + nodeID))
	raw := binary.BigEndian.Uint64(sum[:8])
	// Map onto (0, 1) strictly away from 0 and 1.
	return (float64(raw%1000000) + 1.0) / 1000002.0
}

func containsIgnoreCase(slice []string, target string) bool {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return false
	}
	for _, s := range slice {
		if strings.EqualFold(strings.TrimSpace(s), trimmed) {
			return true
		}
	}
	return false
}

func clampFloat(v, minV, maxV float64) float64 {
	if math.IsNaN(v) {
		return minV
	}
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func round4(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*10000) / 10000
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
