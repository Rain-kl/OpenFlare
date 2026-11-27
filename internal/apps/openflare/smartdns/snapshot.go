// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

const (
	defaultSnapshotIntervalSec = 10
	minSnapshotIntervalSec     = 3
	maxSnapshotIntervalSec     = 300
	snapshotDirtyDebounce      = 250 * time.Millisecond
	metricsLookbackWindow      = 5 * time.Minute
	nodeStaleOnlineWindow      = 90 * time.Second
	bitsPerByte                = 8.0
	bitsPerMegabit             = 1_000_000.0
	snapshotETagByteLen        = 16
)

// CompiledSnapshotBundle holds an immutable serialized snapshot plus its pre-compressed payload.
type CompiledSnapshotBundle struct {
	Envelope  protocol.DNSSnapshotEnvelope
	JSONBytes []byte
	GzipBytes []byte
	ETag      string
}

// SnapshotManager compiles, versions, and caches the authoritative SmartDNS runtime snapshot.
type SnapshotManager struct {
	mu      sync.Mutex
	current atomic.Pointer[CompiledSnapshotBundle]
	dirtyCh chan struct{}
}

var defaultSnapshotManager = NewSnapshotManager()

// NewSnapshotManager creates an isolated SnapshotManager instance.
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		dirtyCh: make(chan struct{}, 1),
	}
}

// DefaultSnapshotManager returns the process-wide SmartDNS snapshot manager.
func DefaultSnapshotManager() *SnapshotManager {
	return defaultSnapshotManager
}

// Current returns the latest immutable snapshot bundle (or nil if never built).
func (m *SnapshotManager) Current() *CompiledSnapshotBundle {
	if m == nil {
		return nil
	}
	return m.current.Load()
}

// NotifyDirty schedules an asynchronous snapshot rebuild with short debouncing.
func (m *SnapshotManager) NotifyDirty() {
	if m == nil || m.dirtyCh == nil {
		return
	}
	select {
	case m.dirtyCh <- struct{}{}:
	default:
	}
}

// NotifySnapshotDirty triggers a snapshot rebuild on the global manager.
func NotifySnapshotDirty(_ context.Context) {
	defaultSnapshotManager.NotifyDirty()
}

// EnsureCurrent returns the cached snapshot or builds one synchronously on cold start.
func (m *SnapshotManager) EnsureCurrent(ctx context.Context) (*CompiledSnapshotBundle, error) {
	if bundle := m.current.Load(); bundle != nil {
		return bundle, nil
	}
	return m.Build(ctx)
}

// Build compiles the authoritative SmartDNS snapshot from database state and qualityapi.
// It updates Version and broadcasts snapshot changes only when the semantic ETag changes.
func (m *SnapshotManager) Build(ctx context.Context) (*CompiledSnapshotBundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, contentETag, err := assembleSnapshotEnvelope(ctx)
	if err != nil {
		return nil, err
	}

	prev := m.current.Load()
	if prev != nil && prev.ETag == contentETag {
		return prev, nil
	}

	version := uint64(1)
	if prev != nil {
		version = prev.Version() + 1
	}
	env.Version = version
	env.ETag = contentETag

	jsonBytes, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	if _, err := gz.Write(jsonBytes); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	bundle := &CompiledSnapshotBundle{
		Envelope:  *env,
		JSONBytes: jsonBytes,
		GzipBytes: gzBuf.Bytes(),
		ETag:      contentETag,
	}
	m.current.Store(bundle)

	if prev != nil {
		websocket.NotifyDNSSnapshotChanged(ctx)
	}
	return bundle, nil
}

// Version returns the snapshot bundle version.
func (b *CompiledSnapshotBundle) Version() uint64 {
	if b == nil {
		return 0
	}
	return b.Envelope.Version
}

// StartSnapshotLoop builds the initial snapshot synchronously and starts the background compiler loop.
func StartSnapshotLoop(ctx context.Context) {
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	if _, err := defaultSnapshotManager.Build(loopCtx); err != nil {
		logger.WarnF(loopCtx, "[SmartDNS] initial snapshot build failed: %v", err)
	}
	interval := readSnapshotInterval(loopCtx)
	lifecycle.OnShutdown("smartdns snapshot loop", func(context.Context) error {
		cancel()
		<-done
		return nil
	})
	go runSnapshotLoop(loopCtx, done, defaultSnapshotManager, interval)
}

func runSnapshotLoop(loopCtx context.Context, done chan struct{}, mgr *SnapshotManager, interval time.Duration) {
	defer close(done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-loopCtx.Done():
			return
		case <-mgr.dirtyCh:
			timer := time.NewTimer(snapshotDirtyDebounce)
			select {
			case <-loopCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			// Drain duplicate signals accumulated during debounce window.
			for len(mgr.dirtyCh) > 0 {
				<-mgr.dirtyCh
			}
			if _, err := mgr.Build(loopCtx); err != nil {
				logger.WarnF(loopCtx, "[SmartDNS] reactive snapshot rebuild failed: %v", err)
			}
		case <-ticker.C:
			nextInterval := readSnapshotInterval(loopCtx)
			if nextInterval != interval {
				interval = nextInterval
				ticker.Reset(interval)
			}
			if _, err := mgr.Build(loopCtx); err != nil {
				logger.WarnF(loopCtx, "[SmartDNS] periodic snapshot rebuild failed: %v", err)
			}
		}
	}
}

func readSnapshotInterval(ctx context.Context) time.Duration {
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySmartDNSSnapshotIntervalSeconds)
	if err != nil {
		return defaultSnapshotIntervalSec * time.Second
	}
	sec, convErr := strconv.Atoi(strings.TrimSpace(cfg.Value))
	if convErr != nil {
		return defaultSnapshotIntervalSec * time.Second
	}
	if sec < minSnapshotIntervalSec {
		sec = minSnapshotIntervalSec
	}
	if sec > maxSnapshotIntervalSec {
		sec = maxSnapshotIntervalSec
	}
	return time.Duration(sec) * time.Second
}

func assembleSnapshotEnvelope(ctx context.Context) (*protocol.DNSSnapshotEnvelope, string, error) {
	settings := readSmartDNSSettingsMap(ctx)
	snapNodes, edgeNodeIDs, err := collectSnapshotNodes(ctx)
	if err != nil {
		return nil, "", err
	}
	snapPolicies, err := collectSnapshotPolicies(ctx)
	if err != nil {
		return nil, "", err
	}
	domainRoutes := assembleDomainRoutes(ctx)
	qualitySnap := loadQualitySnapshotWithFallback(ctx, edgeNodeIDs)

	env := &protocol.DNSSnapshotEnvelope{
		GeneratedAt:      time.Now().UTC(),
		DelegationDomain: settings.DelegationDomain,
		NSHosts:          settings.NSHosts,
		DefaultTTL:       settings.DefaultTTL,
		Nodes:            snapNodes,
		Policies:         snapPolicies,
		DomainRoutes:     domainRoutes,
		Quality:          qualitySnap,
	}
	etag, err := computeSnapshotContentETag(env)
	if err != nil {
		return nil, "", err
	}
	return env, etag, nil
}

func collectSnapshotNodes(ctx context.Context) ([]protocol.DNSSnapshotNode, []string, error) {
	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, nil, err
	}
	edgeNodes := make([]model.OpenFlareNode, 0, len(nodes))
	edgeNodeIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.NodeType != "" && n.NodeType != "edge_node" {
			continue
		}
		edgeNodes = append(edgeNodes, n)
		edgeNodeIDs = append(edgeNodeIDs, n.NodeID)
	}

	tagMap, err := repository.ListNodeTagSummariesByNodeIDs(ctx, edgeNodeIDs)
	if err != nil {
		tagMap = map[string][]model.NodeTagSummary{}
	}
	metricsByNode := loadLatestSnapshotMetrics(ctx, edgeNodeIDs)

	snapNodes := make([]protocol.DNSSnapshotNode, 0, len(edgeNodes))
	now := time.Now()
	for _, n := range edgeNodes {
		snapNodes = append(snapNodes, buildSnapshotNode(n, tagMap[n.NodeID], metricsByNode[n.NodeID], now))
	}
	return snapNodes, edgeNodeIDs, nil
}

func loadLatestSnapshotMetrics(ctx context.Context, edgeNodeIDs []string) map[string]*model.OpenFlareMetricSnapshot {
	metricsByNode := map[string]*model.OpenFlareMetricSnapshot{}
	if len(edgeNodeIDs) == 0 {
		return metricsByNode
	}
	latest, err := repository.ListOpenFlareLatestMetricSnapshotsByNodeIDs(ctx, edgeNodeIDs, time.Now().Add(-metricsLookbackWindow))
	if err != nil {
		return metricsByNode
	}
	for _, m := range latest {
		if m != nil {
			metricsByNode[m.NodeID] = m
		}
	}
	return metricsByNode
}

func buildSnapshotNode(
	n model.OpenFlareNode,
	tags []model.NodeTagSummary,
	metric *model.OpenFlareMetricSnapshot,
	now time.Time,
) protocol.DNSSnapshotNode {
	ipv4, ipv6 := splitNodeIPs(n.IP)
	status := resolveSnapshotNodeStatus(n, now)
	country := lookupNodeCountry(n.IP)
	snapTags := make([]protocol.DNSSnapshotTag, 0, len(tags))
	for _, t := range tags {
		snapTags = append(snapTags, protocol.DNSSnapshotTag{
			Slug:            t.Slug,
			Name:            t.Name,
			EffectiveWeight: t.EffectiveWeight,
		})
	}

	cpuPct, memPct, bwMbps, activeConn := extractSnapshotNodeLoad(n, metric)
	capWeight := n.CapacityWeight
	if capWeight <= 0 {
		capWeight = defaultCapacityWeight
	}

	return protocol.DNSSnapshotNode{
		NodeID:                n.NodeID,
		Name:                  n.Name,
		IPv4:                  ipv4,
		IPv6:                  ipv6,
		Status:                status,
		Quarantined:           n.IsQuarantined(),
		NetworkGroup:          strings.TrimSpace(n.NetworkGroup),
		ASN:                   n.ASN,
		Country:               country,
		GeoLatitude:           n.GeoLatitude,
		GeoLongitude:          n.GeoLongitude,
		CapacityBandwidthMbps: n.CapacityBandwidthMbps,
		CapacityConnections:   n.CapacityConnections,
		CapacityWeight:        capWeight,
		CPUUsagePercent:       cpuPct,
		MemoryUsagePercent:    memPct,
		BandwidthMbps:         bwMbps,
		ActiveConnections:     activeConn,
		Tags:                  snapTags,
	}
}

func resolveSnapshotNodeStatus(n model.OpenFlareNode, now time.Time) string {
	if n.IsQuarantined() {
		return model.DNSInstanceStatusQuarantined
	}
	if n.LastSeenAt != nil && now.Sub(*n.LastSeenAt) > nodeStaleOnlineWindow && n.Status == model.DNSInstanceStatusOnline {
		return model.DNSInstanceStatusOffline
	}
	return n.Status
}

func lookupNodeCountry(ip string) string {
	if strings.TrimSpace(ip) == "" {
		return ""
	}
	if lookup := netintel.LookupIP(ip); lookup != nil {
		return lookup.CountryISO
	}
	return ""
}

func extractSnapshotNodeLoad(n model.OpenFlareNode, m *model.OpenFlareMetricSnapshot) (cpuPct, memPct, bwMbps float64, activeConn int64) {
	activeConn = n.OpenrestyConnections
	if m == nil {
		return 0, 0, 0, activeConn
	}
	cpuPct = m.CPUUsagePercent
	if m.MemoryTotalBytes > 0 {
		memPct = float64(m.MemoryUsedBytes) / float64(m.MemoryTotalBytes) * percentScale
	}
	if m.NetOut > 0 {
		bwMbps = float64(m.NetOut) * bitsPerByte / bitsPerMegabit
	}
	if int64(m.TCPConnections) > activeConn {
		activeConn = int64(m.TCPConnections)
	}
	return cpuPct, memPct, bwMbps, activeConn
}

func collectSnapshotPolicies(ctx context.Context) ([]protocol.DNSSnapshotPolicy, error) {
	dbPolicies, err := repository.ListEnabledDNSPolicies(ctx)
	if err != nil {
		return nil, err
	}
	snapPolicies := make([]protocol.DNSSnapshotPolicy, 0, len(dbPolicies))
	for _, p := range dbPolicies {
		var scopeDomains, fallbackNodeIDs []string
		_ = json.Unmarshal([]byte(p.ScopeDomainsJSON), &scopeDomains)
		_ = json.Unmarshal([]byte(p.FallbackNodeIDsJSON), &fallbackNodeIDs)
		if scopeDomains == nil {
			scopeDomains = []string{}
		}
		if fallbackNodeIDs == nil {
			fallbackNodeIDs = []string{}
		}
		jdmJSON := strings.TrimSpace(p.JDMJSON)
		if jdmJSON == "" || jdmJSON == "{}" {
			var cfg PolicyConfig
			_ = json.Unmarshal([]byte(p.ConfigJSON), &cfg)
			if compiled, cErr := CompilePresetJDM(p.StrategyType, cfg); cErr == nil {
				jdmJSON = compiled
			}
		}
		snapPolicies = append(snapPolicies, protocol.DNSSnapshotPolicy{
			ID:              p.ID,
			Name:            p.Name,
			ScopeType:       p.ScopeType,
			ScopeDomains:    scopeDomains,
			StrategyType:    p.StrategyType,
			Priority:        p.Priority,
			TTL:             p.TTL,
			MaxAnswers:      p.MaxAnswers,
			IsDefault:       p.IsDefault,
			FallbackStrat:   p.FallbackStrategy,
			FallbackNodeIDs: fallbackNodeIDs,
			JDMJSON:         jdmJSON,
			Revision:        p.Revision,
		})
	}
	return snapPolicies, nil
}

func loadQualitySnapshotWithFallback(ctx context.Context, edgeNodeIDs []string) *qualityapi.CompactQualitySnapshot {
	qualitySnap, qErr := qualityapi.Default().ExportSnapshot(ctx, edgeNodeIDs)
	if qErr == nil && qualitySnap != nil {
		return qualitySnap
	}
	if qErr != nil && !errors.Is(qErr, gorm.ErrRecordNotFound) {
		logger.WarnF(ctx, "[SmartDNS] qualityapi ExportSnapshot warning: %v", qErr)
	}
	fallback, _ := qualityapi.FallbackProvider{}.ExportSnapshot(ctx, edgeNodeIDs)
	return fallback
}

func assembleDomainRoutes(ctx context.Context) []protocol.DNSSnapshotDomainRoute {
	groups, err := repository.ListCFPointingGroups(ctx)
	if err != nil {
		return []protocol.DNSSnapshotDomainRoute{}
	}
	routes := make([]protocol.DNSSnapshotDomainRoute, 0)
	for _, g := range groups {
		if !g.Enabled || g.TargetMode != model.CFPointingTargetModeSmartDNS || g.SmartDNSPolicyID == nil {
			continue
		}
		targetHost := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(g.SmartDNSTargetHost)), ".")
		if targetHost == "" {
			continue
		}
		members, mErr := repository.ListCFPointingMembersByGroupID(ctx, g.ID)
		memberDomains := make([]string, 0, len(members))
		if mErr == nil {
			for _, mem := range members {
				zd, zErr := repository.GetZoneDomainByID(ctx, mem.ZoneDomainID)
				if zErr == nil && zd != nil && strings.TrimSpace(zd.Domain) != "" {
					memberDomains = append(memberDomains, strings.TrimSuffix(strings.ToLower(strings.TrimSpace(zd.Domain)), "."))
				}
			}
		}
		routes = append(routes, protocol.DNSSnapshotDomainRoute{
			TargetHost:    targetHost,
			PolicyID:      *g.SmartDNSPolicyID,
			GroupID:       g.ID,
			MemberDomains: memberDomains,
		})
	}
	return routes
}

func splitNodeIPs(raw string) (string, string) {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.IsUnspecified() {
		return "", ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String(), ""
	}
	return "", ip.String()
}

func computeSnapshotContentETag(env *protocol.DNSSnapshotEnvelope) (string, error) {
	type hashInput struct {
		DelegationDomain string                             `json:"delegation_domain"`
		NSHosts          []string                           `json:"ns_hosts"`
		DefaultTTL       int                                `json:"default_ttl"`
		Nodes            []protocol.DNSSnapshotNode         `json:"nodes"`
		Policies         []protocol.DNSSnapshotPolicy       `json:"policies"`
		DomainRoutes     []protocol.DNSSnapshotDomainRoute  `json:"domain_routes"`
		Quality          *qualityapi.CompactQualitySnapshot `json:"quality"`
	}
	var qualityCopy *qualityapi.CompactQualitySnapshot
	if env.Quality != nil {
		qc := *env.Quality
		qc.GeneratedAt = time.Time{} // ignore build timestamp for ETag stability
		qualityCopy = &qc
	}
	b, err := json.Marshal(hashInput{
		DelegationDomain: env.DelegationDomain,
		NSHosts:          env.NSHosts,
		DefaultTTL:       env.DefaultTTL,
		Nodes:            env.Nodes,
		Policies:         env.Policies,
		DomainRoutes:     env.DomainRoutes,
		Quality:          qualityCopy,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:snapshotETagByteLen]) + `"`, nil
}
