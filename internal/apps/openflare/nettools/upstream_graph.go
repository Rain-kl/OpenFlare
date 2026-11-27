// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

const (
	defaultRIPEStatBaseURL   = "https://stat.ripe.net"
	ripeStatHTTPTimeout      = 10 * time.Second
	maxRIPEStatResponseBytes = 8 * 1024 * 1024

	upstreamCacheMaxEntries   = 512
	upstreamCacheTTL          = 6 * time.Hour
	upstreamEmptyCacheTTL     = 5 * time.Minute
	upstreamRedisKeyPrefix    = "nettools:asn_upstream:"
	neighborsRedisKeyPrefix   = "nettools:asn_neighbors:"
	upstreamInvalidationChan  = "nettools:asn_upstream_invalidation"
	upstreamPrefixScanLimit   = 200
	upstreamObservedEdgeLimit = 200
	maxCachedNeighbors        = 100
	ripeRelationLeft          = "left"
	ripeRelationRight         = "right"

	maxDirectUpstreams    = 12
	maxTransitHopsPerPath = 2
	maxTransitNodes       = 12
	maxTier1Targets       = 16
	maxNeighboursFallback = 12
	minASPathTokens       = 2
	maxTransitLayer       = 2

	asnTransition32        = 23456
	asnPrivate16Start      = 64512
	asnPrivate16End        = 65535
	asnPrivate32Start      = 4200000000
	maxValidASN            = 4294967295
	decimalBase            = 10
	bitSize32              = 32
	attributionBGPToolsAnd = "Data powered by bgp.tools"
	attributionRIPEStat    = "Data powered by bgp.tools & RIPEstat (RIPE NCC)"
	attributionCAIDA       = "Data powered by bgp.tools & CAIDA"

	upstreamRoleOrigin        = "origin"
	upstreamRoleTransit       = "transit"
	upstreamRoleTier1Upstream = "tier1_upstream"
	upstreamRoleTier1         = "tier1"

	edgeKindUpstream          = "upstream"
	edgeKindTier1Interconnect = "tier1_interconnect"
	edgeKindObserved          = "observed"

	upstreamSourceCAIDA      = "caida"
	upstreamSourceRIPEStatLG = "ripestat_lg"
	upstreamSourceRIPENeigh  = "ripestat_neighbours"
	upstreamSourceObserved   = "observed"
	upstreamSourceNone       = "none"

	prefixFamilyIPv4 = "ipv4"
	prefixFamilyIPv6 = "ipv6"
)

// ASUpstreamNode represents one AS vertex in the layered upstream routing graph.
type ASUpstreamNode struct {
	ASN       uint32   `json:"asn"`
	Name      string   `json:"name"`
	ShortName string   `json:"short_name"`
	Country   string   `json:"country"`
	Class     string   `json:"class"`
	Tags      []string `json:"tags,omitempty"`
	Role      string   `json:"role"`
	Layer     int      `json:"layer"`
	IsTier1   bool     `json:"is_tier1"`
	PathHits  int      `json:"path_hits"`
}

// ASUpstreamEdge represents one directed upstream or Tier-1 interconnect link.
type ASUpstreamEdge struct {
	From          uint32  `json:"from"`
	To            uint32  `json:"to"`
	Kind          string  `json:"kind"`
	PathHits      int     `json:"path_hits"`
	ObservedRTTMs float64 `json:"observed_rtt_ms,omitempty"`
	SampleCount   int64   `json:"sample_count,omitempty"`
}

// ASNUpstreamGraphResponse is returned by GET /api/v1/d/network-tools/lookup/asn/{asn}/upstream-graph.
type ASNUpstreamGraphResponse struct {
	OriginASN        uint32           `json:"origin_asn"`
	SampledPrefix    string           `json:"sampled_prefix,omitempty"`
	DataSource       string           `json:"data_source"`
	Attribution      string           `json:"attribution"`
	TotalPathsParsed int              `json:"total_paths_parsed"`
	Nodes            []ASUpstreamNode `json:"nodes"`
	Edges            []ASUpstreamEdge `json:"edges"`
	Cached           bool             `json:"cached"`
	GeneratedAt      time.Time        `json:"generated_at"`
}

type cachedUpstreamEntry struct {
	Response  ASNUpstreamGraphResponse `json:"response"`
	ExpiresAt time.Time                `json:"expires_at"`
}

type cachedNeighborsEntry struct {
	Items     []ASNeighborItem `json:"items"`
	ExpiresAt time.Time        `json:"expires_at"`
}

var (
	upstreamGraphRAM = ram.MustNew[string, cachedUpstreamEntry](ram.Options{
		MaximumSize: upstreamCacheMaxEntries,
	})
	ripeNeighborsRAM = ram.MustNew[string, cachedNeighborsEntry](ram.Options{
		MaximumSize: upstreamCacheMaxEntries,
	})
	upstreamListenerMu     sync.Mutex
	upstreamListenerActive bool

	ripeStatMu      sync.RWMutex
	ripeStatBaseURL = defaultRIPEStatBaseURL
)

// SetRIPEStatBaseURLForTest overrides the RIPEstat API base URL for unit tests and returns a cleanup callback.
func SetRIPEStatBaseURLForTest(baseURL string) func() {
	ripeStatMu.Lock()
	prev := ripeStatBaseURL
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		ripeStatBaseURL = defaultRIPEStatBaseURL
	} else {
		ripeStatBaseURL = trimmed
	}
	ripeStatMu.Unlock()
	return func() {
		ripeStatMu.Lock()
		ripeStatBaseURL = prev
		ripeStatMu.Unlock()
	}
}

func currentRIPEStatBaseURL() string {
	ripeStatMu.RLock()
	v := ripeStatBaseURL
	ripeStatMu.RUnlock()
	return v
}

// ResetUpstreamGraphCacheForTest clears process-local RAM cache entries for tests.
func ResetUpstreamGraphCacheForTest() {
	upstreamGraphRAM.InvalidateAll()
	ripeNeighborsRAM.InvalidateAll()
}

func ensureUpstreamCacheInvalidationListener(ctx context.Context) {
	if db.Redis == nil {
		return
	}
	upstreamListenerMu.Lock()
	if upstreamListenerActive {
		upstreamListenerMu.Unlock()
		return
	}
	upstreamListenerActive = true
	upstreamListenerMu.Unlock()

	subCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	pubsub := db.Redis.Subscribe(subCtx, db.PrefixedKey(upstreamInvalidationChan))
	lifecycle.OnShutdown("nettools upstream graph cache invalidation", func(context.Context) error {
		cancel()
		return pubsub.Close()
	})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorF(subCtx, "[NetTools] upstream cache invalidation listener panic recovered: %v", r)
			}
		}()
		for msg := range pubsub.Channel() {
			key := strings.TrimSpace(msg.Payload)
			if key == "" {
				continue
			}
			upstreamGraphRAM.Invalidate(key)
		}
	}()
}

// GetASNUpstreamGraph resolves a layered upstream connectivity DAG (Origins -> Upstreams -> Tier 1 ISPs)
// for target asn using local CAIDA, RIPEstat Looking Glass, RIPEstat ASN Neighbours, and observed edges.
func GetASNUpstreamGraph(
	ctx context.Context,
	asn uint32,
	preferredPrefix string,
	forceRefresh bool,
) (*ASNUpstreamGraphResponse, error) {
	if asn == 0 {
		return nil, errors.New(errInvalidASN)
	}
	ensureUpstreamCacheInvalidationListener(ctx)

	sampledPrefix := pickUpstreamSamplePrefix(asn, preferredPrefix)
	cacheKey := buildUpstreamCacheKey(asn, sampledPrefix)

	if !forceRefresh {
		if cached := loadCachedUpstreamGraph(ctx, cacheKey); cached != nil {
			return cached, nil
		}
	}

	edges, totalPaths, dataSource := resolveUpstreamEdges(ctx, asn, sampledPrefix)

	observedLookup := loadObservedEdgeLookup(ctx)
	nodes, finalEdges := buildLayeredUpstreamGraph(asn, dataSource, edges, observedLookup)
	resp := &ASNUpstreamGraphResponse{
		OriginASN:        asn,
		SampledPrefix:    sampledPrefix,
		DataSource:       dataSource,
		Attribution:      attributionForUpstreamSource(dataSource),
		TotalPathsParsed: totalPaths,
		Nodes:            nodes,
		Edges:            finalEdges,
		Cached:           false,
		GeneratedAt:      time.Now().UTC(),
	}

	saveCachedUpstreamGraph(ctx, cacheKey, resp, forceRefresh)
	return resp, nil
}

func resolveUpstreamEdges(ctx context.Context, asn uint32, sampledPrefix string) (map[edgePair]int, int, string) {
	edges := make(map[edgePair]int)
	if caidaPaths := netintel.LookupASUpstreamPaths(asn, 0, maxDirectUpstreams); len(caidaPaths) > 0 {
		for _, p := range caidaPaths {
			tallyUpstreamPathEdges(p, edges)
		}
		if len(edges) > 0 {
			return edges, len(caidaPaths), upstreamSourceCAIDA
		}
	}

	if sampledPrefix != "" {
		lgPaths, parsedCount, lgErr := fetchRIPEStatLookingGlassPaths(ctx, asn, sampledPrefix)
		if lgErr != nil {
			logger.WarnF(ctx, "[NetTools] RIPEstat looking-glass lookup for AS%d (%s) failed: %v", asn, sampledPrefix, lgErr)
		} else if len(lgPaths) > 0 {
			for _, p := range lgPaths {
				tallyUpstreamPathEdges(p, edges)
			}
			if len(edges) > 0 {
				return edges, parsedCount, upstreamSourceRIPEStatLG
			}
		}
	}

	neighPaths, neighErr := fetchRIPEStatNeighbourUpstreams(ctx, asn)
	if neighErr != nil {
		logger.WarnF(ctx, "[NetTools] RIPEstat asn-neighbours lookup for AS%d failed: %v", asn, neighErr)
	} else if len(neighPaths) > 0 {
		for _, e := range neighPaths {
			edges[edgePair{from: asn, to: e.ASN}] += max(1, e.Power)
		}
		if len(edges) > 0 {
			return edges, len(neighPaths), upstreamSourceRIPENeigh
		}
	}

	if db.DB(ctx) != nil {
		if obsEdges := collectObservedDirectUpstreams(ctx, asn); len(obsEdges) > 0 {
			for peer, hits := range obsEdges {
				edges[edgePair{from: asn, to: peer}] += hits
			}
			return edges, len(edges), upstreamSourceObserved
		}
	}
	return edges, 0, upstreamSourceNone
}

type edgePair struct {
	from uint32
	to   uint32
}

type observedEdgeStats struct {
	avgRTTMs    float64
	sampleCount int64
}

func buildUpstreamCacheKey(asn uint32, sampledPrefix string) string {
	norm := strings.TrimSpace(sampledPrefix)
	if norm == "" {
		norm = "auto"
	}
	return fmt.Sprintf("%s%d:%s", upstreamRedisKeyPrefix, asn, norm)
}

func loadCachedUpstreamGraph(ctx context.Context, cacheKey string) *ASNUpstreamGraphResponse {
	now := time.Now().UTC()
	if entry, ok := upstreamGraphRAM.GetIfPresent(cacheKey); ok {
		if now.Before(entry.ExpiresAt) {
			cloned := cloneUpstreamResponse(&entry.Response)
			cloned.Cached = true
			return cloned
		}
		upstreamGraphRAM.Invalidate(cacheKey)
	}
	if db.Redis != nil {
		var resp ASNUpstreamGraphResponse
		if err := db.GetJSON(ctx, cacheKey, &resp); err == nil && resp.OriginASN > 0 {
			ttl := upstreamCacheTTL
			if resp.DataSource == upstreamSourceNone {
				ttl = upstreamEmptyCacheTTL
			}
			upstreamGraphRAM.Set(cacheKey, cachedUpstreamEntry{
				Response:  *cloneUpstreamResponse(&resp),
				ExpiresAt: now.Add(ttl),
			})
			resp.Cached = true
			return &resp
		}
	}
	return nil
}

func saveCachedUpstreamGraph(ctx context.Context, cacheKey string, resp *ASNUpstreamGraphResponse, broadcast bool) {
	if resp == nil {
		return
	}
	ttl := upstreamCacheTTL
	if resp.DataSource == upstreamSourceNone || len(resp.Edges) == 0 {
		ttl = upstreamEmptyCacheTTL
	}
	upstreamGraphRAM.Set(cacheKey, cachedUpstreamEntry{
		Response:  *cloneUpstreamResponse(resp),
		ExpiresAt: time.Now().UTC().Add(ttl),
	})
	if db.Redis != nil {
		if err := db.SetJSON(ctx, cacheKey, *resp, ttl); err != nil {
			logger.WarnF(ctx, "[NetTools] persist upstream graph cache %s failed: %v", cacheKey, err)
		} else if broadcast {
			if pubErr := db.Redis.Publish(ctx, db.PrefixedKey(upstreamInvalidationChan), cacheKey).Err(); pubErr != nil {
				logger.WarnF(ctx, "[NetTools] publish upstream graph invalidation failed: %v", pubErr)
			}
		}
	}
}

func cloneUpstreamResponse(src *ASNUpstreamGraphResponse) *ASNUpstreamGraphResponse {
	if src == nil {
		return nil
	}
	copied := *src
	copied.Nodes = make([]ASUpstreamNode, len(src.Nodes))
	for i := range src.Nodes {
		copied.Nodes[i] = src.Nodes[i]
		if src.Nodes[i].Tags != nil {
			copied.Nodes[i].Tags = append([]string(nil), src.Nodes[i].Tags...)
		}
	}
	copied.Edges = make([]ASUpstreamEdge, len(src.Edges))
	copy(copied.Edges, src.Edges)
	return &copied
}

func pickUpstreamSamplePrefix(asn uint32, preferredPrefix string) string {
	trimmed := strings.TrimSpace(preferredPrefix)
	if trimmed != "" {
		if parsed, err := netip.ParsePrefix(trimmed); err == nil {
			return parsed.Masked().String()
		}
	}
	for _, fam := range []string{prefixFamilyIPv4, prefixFamilyIPv6} {
		records, _ := netintel.ListASNPrefixes(asn, fam, 1, upstreamPrefixScanLimit)
		if len(records) == 0 {
			continue
		}
		best := records[0]
		for _, rec := range records[1:] {
			if rec.Hits > best.Hits {
				best = rec
			}
		}
		if strings.TrimSpace(best.CIDR) != "" {
			return strings.TrimSpace(best.CIDR)
		}
	}
	return ""
}

func attributionForUpstreamSource(source string) string {
	switch source {
	case upstreamSourceRIPEStatLG, upstreamSourceRIPENeigh:
		return attributionRIPEStat
	case upstreamSourceCAIDA:
		return attributionCAIDA
	default:
		return attributionBGPToolsAnd
	}
}

func isPublicRouteASN(asn uint32) bool {
	if asn == 0 || asn == asnTransition32 {
		return false
	}
	if asn >= asnPrivate16Start && asn <= asnPrivate16End {
		return false
	}
	if asn >= asnPrivate32Start {
		return false
	}
	return true
}

func parseAndNormalizeLGASPath(raw string, originASN uint32) ([]uint32, bool) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) < minASPathTokens {
		return nil, false
	}
	dedup := make([]uint32, 0, len(fields))
	var prev uint32
	for _, tok := range fields {
		if strings.ContainsAny(tok, "{}[],") {
			break
		}
		val, err := strconv.ParseUint(tok, decimalBase, bitSize32)
		if err != nil || val == 0 || val > maxValidASN {
			return nil, false
		}
		asn := uint32(val)
		if !isPublicRouteASN(asn) {
			continue
		}
		if asn == prev {
			continue
		}
		dedup = append(dedup, asn)
		prev = asn
	}
	if len(dedup) < minASPathTokens || dedup[len(dedup)-1] != originASN {
		return nil, false
	}
	reversed := make([]uint32, len(dedup))
	for i := range dedup {
		reversed[i] = dedup[len(dedup)-1-i]
	}
	return truncateUpstreamSkeleton(reversed), true
}

func truncateUpstreamSkeleton(originFirst []uint32) []uint32 {
	if len(originFirst) < minASPathTokens {
		return originFirst
	}
	origin := originFirst[0]
	out := []uint32{origin}
	if pkgnetintel.IsTier1ASN(origin) {
		if pkgnetintel.IsTier1ASN(originFirst[1]) && originFirst[1] != origin {
			out = append(out, originFirst[1])
		}
		return out
	}

	nonTier1Count := 0
	for i := 1; i < len(originFirst); i++ {
		hop := originFirst[i]
		if hop == origin {
			continue
		}
		if pkgnetintel.IsTier1ASN(hop) {
			out = append(out, hop)
			if i+1 < len(originFirst) {
				next := originFirst[i+1]
				if next != hop && next != origin && pkgnetintel.IsTier1ASN(next) {
					out = append(out, next)
				}
			}
			break
		}
		if nonTier1Count >= maxTransitHopsPerPath {
			break
		}
		out = append(out, hop)
		nonTier1Count++
	}
	return out
}

func tallyUpstreamPathEdges(path []uint32, edges map[edgePair]int) {
	if len(path) < minASPathTokens {
		return
	}
	for i := 0; i < len(path)-1; i++ {
		from, to := path[i], path[i+1]
		if from == 0 || to == 0 || from == to {
			continue
		}
		edges[edgePair{from: from, to: to}]++
	}
}

type ripeLGEnvelope struct {
	Data struct {
		RRCs []struct {
			Peers []struct {
				ASPath string `json:"as_path"`
			} `json:"peers"`
		} `json:"rrcs"`
	} `json:"data"`
}

func fetchRIPEStatLookingGlassPaths(ctx context.Context, originASN uint32, prefix string) ([][]uint32, int, error) {
	endpoint, err := buildRIPEStatURL("/data/looking-glass/data.json", prefix)
	if err != nil {
		return nil, 0, err
	}
	body, err := fetchRIPEStatJSON(ctx, endpoint)
	if err != nil {
		return nil, 0, err
	}
	var env ripeLGEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, 0, fmt.Errorf("decode ripestat looking-glass response: %w", err)
	}

	var paths [][]uint32
	for _, rrc := range env.Data.RRCs {
		for _, peer := range rrc.Peers {
			if normalized, ok := parseAndNormalizeLGASPath(peer.ASPath, originASN); ok && len(normalized) >= minASPathTokens {
				paths = append(paths, normalized)
			}
		}
	}
	return paths, len(paths), nil
}

type ripeNeighbourItem struct {
	ASN   uint32
	Power int
}

type ripeNeighboursEnvelope struct {
	Data struct {
		Neighbours []struct {
			ASN   uint32 `json:"asn"`
			Type  string `json:"type"`
			Power int    `json:"power"`
		} `json:"neighbours"`
	} `json:"data"`
}

func fetchRIPEStatNeighbourUpstreams(ctx context.Context, originASN uint32) ([]ripeNeighbourItem, error) {
	endpoint, err := buildRIPEStatURL("/data/asn-neighbours/data.json", fmt.Sprintf("AS%d", originASN))
	if err != nil {
		return nil, err
	}
	body, err := fetchRIPEStatJSON(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var env ripeNeighboursEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode ripestat asn-neighbours response: %w", err)
	}

	items := make([]ripeNeighbourItem, 0)
	for _, nb := range env.Data.Neighbours {
		if nb.Type != ripeRelationLeft || !isPublicRouteASN(nb.ASN) || nb.ASN == originASN {
			continue
		}
		items = append(items, ripeNeighbourItem{ASN: nb.ASN, Power: nb.Power})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Power != items[j].Power {
			return items[i].Power > items[j].Power
		}
		return items[i].ASN < items[j].ASN
	})
	if len(items) > maxNeighboursFallback {
		items = items[:maxNeighboursFallback]
	}
	return items, nil
}

func buildRIPEStatURL(path, resource string) (string, error) {
	u, err := url.Parse(currentRIPEStatBaseURL() + path)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("resource", resource)
	q.Set("sourceapp", "openflare")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func fetchRIPEStatJSON(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OpenFlare-NetTools/1.0 (+https://github.com/Rain-kl/OpenFlare)")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: ripeStatHTTPTimeout}
	resp, err := client.Do(req) //nolint:gosec // endpoint builds from static path and sanitized ASN/CIDR query
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ripestat returned status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxRIPEStatResponseBytes))
}

func collectObservedDirectUpstreams(ctx context.Context, originASN uint32) map[uint32]int {
	edges, err := repository.ListOpenFlareASTopologyEdgesByASN(ctx, originASN)
	if err != nil || len(edges) == 0 {
		return nil
	}
	out := make(map[uint32]int)
	for _, edge := range edges {
		if edge == nil || edge.SrcASN == 0 || edge.DstASN == 0 || edge.SrcASN == edge.DstASN {
			continue
		}
		var peer uint32
		switch {
		case edge.DstASN == originASN:
			peer = edge.SrcASN
		case edge.SrcASN == originASN:
			peer = edge.DstASN
		default:
			continue
		}
		if pkgnetintel.IsTier1ASN(peer) || edge.SampleCount > 0 {
			out[peer] += max(1, edge.SampleCount)
		}
	}
	return out
}

func loadObservedEdgeLookup(ctx context.Context) map[edgePair]observedEdgeStats {
	lookup := make(map[edgePair]observedEdgeStats)
	if db.DB(ctx) == nil {
		return lookup
	}
	rows, err := repository.ListOpenFlareASTopologyEdges(ctx, upstreamObservedEdgeLimit)
	if err != nil {
		return lookup
	}
	for _, row := range rows {
		if row == nil || row.SrcASN == 0 || row.DstASN == 0 || row.SrcASN == row.DstASN {
			continue
		}
		stats := observedEdgeStats{
			avgRTTMs:    row.AvgHopRTTMs,
			sampleCount: int64(row.SampleCount),
		}
		lookup[edgePair{from: row.SrcASN, to: row.DstASN}] = stats
		lookup[edgePair{from: row.DstASN, to: row.SrcASN}] = stats
	}
	return lookup
}

func buildLayeredUpstreamGraph(
	originASN uint32,
	dataSource string,
	rawEdges map[edgePair]int,
	observedLookup map[edgePair]observedEdgeStats,
) ([]ASUpstreamNode, []ASUpstreamEdge) {
	kept := pruneUpstreamEdges(originASN, rawEdges)
	if len(kept) == 0 {
		node := enrichUpstreamNode(originASN, upstreamRoleOrigin, 0, 1)
		return []ASUpstreamNode{node}, []ASUpstreamEdge{}
	}

	bfsDist := computeBFSFromOrigin(originASN, kept)
	hasSecondTier1, maxTransit := classifyGraphRanges(originASN, kept, bfsDist)
	layerByASN, roleByASN := assignLayersAndRoles(originASN, kept, bfsDist, hasSecondTier1, maxTransit)

	forwardEdges := filterForwardEdges(kept, layerByASN)
	activeNodes := collectConnectedNodes(originASN, forwardEdges)
	if len(forwardEdges) == 0 {
		node := enrichUpstreamNode(originASN, upstreamRoleOrigin, 0, 1)
		return []ASUpstreamNode{node}, []ASUpstreamEdge{}
	}

	nodeHits := computeNodePathHits(originASN, forwardEdges)
	nodes := make([]ASUpstreamNode, 0, len(activeNodes))
	for asn := range activeNodes {
		nodes = append(nodes, enrichUpstreamNode(asn, roleByASN[asn], layerByASN[asn], nodeHits[asn]))
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Layer != nodes[j].Layer {
			return nodes[i].Layer < nodes[j].Layer
		}
		if nodes[i].PathHits != nodes[j].PathHits {
			return nodes[i].PathHits > nodes[j].PathHits
		}
		return nodes[i].ASN < nodes[j].ASN
	})

	finalEdges := buildResponseEdges(forwardEdges, dataSource, observedLookup)
	return nodes, finalEdges
}

type scoredEdge struct {
	pair edgePair
	hits int
}

func buildSortedAdjacency(rawEdges map[edgePair]int) map[uint32][]scoredEdge {
	adj := make(map[uint32][]scoredEdge)
	for pair, hits := range rawEdges {
		if pair.from == 0 || pair.to == 0 || pair.from == pair.to || hits <= 0 {
			continue
		}
		adj[pair.from] = append(adj[pair.from], scoredEdge{pair: pair, hits: hits})
	}
	for from, list := range adj {
		sort.Slice(list, func(i, j int) bool {
			if list[i].hits != list[j].hits {
				return list[i].hits > list[j].hits
			}
			return list[i].pair.to < list[j].pair.to
		})
		adj[from] = list
	}
	return adj
}

func pruneUpstreamEdges(originASN uint32, rawEdges map[edgePair]int) map[edgePair]int {
	adj := buildSortedAdjacency(rawEdges)
	kept := make(map[edgePair]int)
	directList := adj[originASN]
	if len(directList) > maxDirectUpstreams {
		directList = directList[:maxDirectUpstreams]
	}
	for _, se := range directList {
		kept[se.pair] = se.hits
	}

	tier1Targets := make(map[uint32]bool)
	retainTier1InterconnectEdges(originASN, directList, adj, kept, tier1Targets)
	retainTransitUpstreamEdges(originASN, directList, adj, kept, tier1Targets)
	return kept
}

func retainTier1InterconnectEdges(
	originASN uint32,
	directList []scoredEdge,
	adj map[uint32][]scoredEdge,
	kept map[edgePair]int,
	tier1Targets map[uint32]bool,
) {
	for _, se := range directList {
		if !pkgnetintel.IsTier1ASN(se.pair.to) {
			continue
		}
		for _, nxt := range adj[se.pair.to] {
			if !pkgnetintel.IsTier1ASN(nxt.pair.to) || nxt.pair.to == originASN || nxt.pair.to == se.pair.to {
				continue
			}
			if !tier1Targets[nxt.pair.to] && len(tier1Targets) >= maxTier1Targets {
				continue
			}
			tier1Targets[nxt.pair.to] = true
			kept[nxt.pair] = nxt.hits
		}
	}
}

func retainTransitUpstreamEdges(
	originASN uint32,
	directList []scoredEdge,
	adj map[uint32][]scoredEdge,
	kept map[edgePair]int,
	tier1Targets map[uint32]bool,
) {
	transitSet := make(map[uint32]bool)
	for _, se := range directList {
		if pkgnetintel.IsTier1ASN(se.pair.to) {
			continue
		}
		for _, nxt := range adj[se.pair.to] {
			if nxt.pair.to == originASN || nxt.pair.to == se.pair.to {
				continue
			}
			if pkgnetintel.IsTier1ASN(nxt.pair.to) {
				if tier1Targets[nxt.pair.to] || len(tier1Targets) < maxTier1Targets {
					tier1Targets[nxt.pair.to] = true
					kept[nxt.pair] = nxt.hits
				}
				continue
			}
			if !transitSet[nxt.pair.to] && len(transitSet) >= maxTransitNodes {
				continue
			}
			transitSet[nxt.pair.to] = true
			kept[nxt.pair] = nxt.hits
			retainThirdHopTier1Edges(nxt.pair.to, adj, kept, tier1Targets)
		}
	}
}

func retainThirdHopTier1Edges(
	transitASN uint32,
	adj map[uint32][]scoredEdge,
	kept map[edgePair]int,
	tier1Targets map[uint32]bool,
) {
	for _, hop3 := range adj[transitASN] {
		if !pkgnetintel.IsTier1ASN(hop3.pair.to) {
			continue
		}
		if !tier1Targets[hop3.pair.to] && len(tier1Targets) >= maxTier1Targets {
			continue
		}
		tier1Targets[hop3.pair.to] = true
		kept[hop3.pair] = hop3.hits
	}
}

func computeBFSFromOrigin(originASN uint32, kept map[edgePair]int) map[uint32]int {
	adj := make(map[uint32][]uint32)
	for pair := range kept {
		adj[pair.from] = append(adj[pair.from], pair.to)
	}
	dist := map[uint32]int{originASN: 0}
	queue := []uint32{originASN}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		for _, nxt := range adj[cur] {
			if _, ok := dist[nxt]; ok {
				continue
			}
			dist[nxt] = dist[cur] + 1
			queue = append(queue, nxt)
		}
	}
	return dist
}

func classifyGraphRanges(originASN uint32, kept map[edgePair]int, bfsDist map[uint32]int) (bool, int) {
	maxTransit := 0
	for asn, d := range bfsDist {
		if asn == originASN || pkgnetintel.IsTier1ASN(asn) {
			continue
		}
		layer := min(max(1, d), maxTransitLayer)
		if layer > maxTransit {
			maxTransit = layer
		}
	}

	directTier1 := make(map[uint32]bool)
	for pair := range kept {
		if pair.from == originASN && pkgnetintel.IsTier1ASN(pair.to) {
			directTier1[pair.to] = true
		}
	}
	hasTier1Upstream := false
	for pair := range kept {
		if directTier1[pair.from] && pkgnetintel.IsTier1ASN(pair.to) && pair.to != pair.from {
			hasTier1Upstream = true
			break
		}
	}
	return hasTier1Upstream, maxTransit
}

func assignLayersAndRoles(
	originASN uint32,
	kept map[edgePair]int,
	bfsDist map[uint32]int,
	hasTier1Upstream bool,
	maxTransit int,
) (map[uint32]int, map[uint32]string) {
	directTier1 := make(map[uint32]bool)
	hasOutgoingToTier1 := make(map[uint32]bool)
	for pair := range kept {
		if pair.from == originASN && pkgnetintel.IsTier1ASN(pair.to) {
			directTier1[pair.to] = true
		}
		if pkgnetintel.IsTier1ASN(pair.from) && pkgnetintel.IsTier1ASN(pair.to) && pair.to != pair.from {
			hasOutgoingToTier1[pair.from] = true
		}
	}

	layers := make(map[uint32]int, len(bfsDist))
	roles := make(map[uint32]string, len(bfsDist))
	layers[originASN] = 0
	roles[originASN] = upstreamRoleOrigin

	for asn, d := range bfsDist {
		if asn == originASN {
			continue
		}
		if !pkgnetintel.IsTier1ASN(asn) {
			layers[asn] = min(max(1, d), maxTransitLayer)
			roles[asn] = upstreamRoleTransit
			continue
		}
		if directTier1[asn] && hasOutgoingToTier1[asn] {
			layers[asn] = maxTransit + 1
			roles[asn] = upstreamRoleTier1Upstream
			continue
		}
		if hasTier1Upstream {
			layers[asn] = maxTransit + maxTransitLayer
		} else {
			layers[asn] = maxTransit + 1
		}
		roles[asn] = upstreamRoleTier1
	}
	return layers, roles
}

func filterForwardEdges(kept map[edgePair]int, layers map[uint32]int) map[edgePair]int {
	forward := make(map[edgePair]int, len(kept))
	for pair, hits := range kept {
		lFrom, okFrom := layers[pair.from]
		lTo, okTo := layers[pair.to]
		if !okFrom || !okTo || lFrom >= lTo {
			continue
		}
		forward[pair] = hits
	}
	return forward
}

func collectConnectedNodes(originASN uint32, forward map[edgePair]int) map[uint32]bool {
	nodes := map[uint32]bool{originASN: true}
	for pair := range forward {
		nodes[pair.from] = true
		nodes[pair.to] = true
	}
	return nodes
}

func computeNodePathHits(originASN uint32, forward map[edgePair]int) map[uint32]int {
	hits := make(map[uint32]int)
	for pair, h := range forward {
		if pair.from == originASN {
			hits[originASN] += h
		}
		hits[pair.to] += h
	}
	return hits
}

func enrichUpstreamNode(asn uint32, role string, layer, pathHits int) ASUpstreamNode {
	info := netintel.LookupASN(asn)
	fullName := ""
	country := ""
	class := ""
	var tags []string
	if info != nil {
		fullName = info.Name
		country = info.Country
		class = info.Class
		if len(info.Tags) > 0 {
			tags = append([]string(nil), info.Tags...)
		}
	}
	shortName := pkgnetintel.ShortASNLabel(asn, fullName)
	if fullName == "" {
		fullName = shortName
	}
	return ASUpstreamNode{
		ASN:       asn,
		Name:      fullName,
		ShortName: shortName,
		Country:   country,
		Class:     class,
		Tags:      tags,
		Role:      role,
		Layer:     layer,
		IsTier1:   pkgnetintel.IsTier1ASN(asn),
		PathHits:  pathHits,
	}
}

func buildResponseEdges(
	forward map[edgePair]int,
	dataSource string,
	observedLookup map[edgePair]observedEdgeStats,
) []ASUpstreamEdge {
	out := make([]ASUpstreamEdge, 0, len(forward))
	for pair, hits := range forward {
		kind := edgeKindUpstream
		if dataSource == upstreamSourceObserved {
			kind = edgeKindObserved
		} else if pkgnetintel.IsTier1ASN(pair.from) && pkgnetintel.IsTier1ASN(pair.to) {
			kind = edgeKindTier1Interconnect
		}
		obs := observedLookup[pair]
		out = append(out, ASUpstreamEdge{
			From:          pair.from,
			To:            pair.to,
			Kind:          kind,
			PathHits:      hits,
			ObservedRTTMs: obs.avgRTTMs,
			SampleCount:   obs.sampleCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PathHits != out[j].PathHits {
			return out[i].PathHits > out[j].PathHits
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

func cloneNeighborItems(items []ASNeighborItem) []ASNeighborItem {
	out := make([]ASNeighborItem, len(items))
	for i := range items {
		out[i] = items[i]
		if len(items[i].Tags) > 0 {
			out[i].Tags = append([]string(nil), items[i].Tags...)
		}
	}
	return out
}

func sliceNeighborItems(items []ASNeighborItem, limit int) []ASNeighborItem {
	if len(items) == 0 {
		return nil
	}
	n := len(items)
	if limit > 0 && limit < n {
		n = limit
	}
	return cloneNeighborItems(items[:n])
}

func getRIPEStatNeighborsCached(ctx context.Context, originASN uint32, limit int) []ASNeighborItem {
	if originASN == 0 {
		return nil
	}
	cacheKey := fmt.Sprintf("%s%d", neighborsRedisKeyPrefix, originASN)
	now := time.Now().UTC()

	if entry, ok := ripeNeighborsRAM.GetIfPresent(cacheKey); ok {
		if now.Before(entry.ExpiresAt) {
			return sliceNeighborItems(entry.Items, limit)
		}
		ripeNeighborsRAM.Invalidate(cacheKey)
	}
	if db.Redis != nil {
		var cached []ASNeighborItem
		if err := db.GetJSON(ctx, cacheKey, &cached); err == nil {
			ttl := upstreamCacheTTL
			if len(cached) == 0 {
				ttl = upstreamEmptyCacheTTL
			}
			ripeNeighborsRAM.Set(cacheKey, cachedNeighborsEntry{
				Items:     cloneNeighborItems(cached),
				ExpiresAt: now.Add(ttl),
			})
			return sliceNeighborItems(cached, limit)
		}
	}

	fetched, err := fetchRIPEStatAllNeighbors(ctx, originASN)
	if err != nil {
		logger.WarnF(ctx, "[NetTools] RIPEstat full neighbors fallback for AS%d failed: %v", originASN, err)
		return nil
	}
	ttl := upstreamCacheTTL
	if len(fetched) == 0 {
		ttl = upstreamEmptyCacheTTL
	}
	ripeNeighborsRAM.Set(cacheKey, cachedNeighborsEntry{
		Items:     cloneNeighborItems(fetched),
		ExpiresAt: now.Add(ttl),
	})
	if db.Redis != nil {
		if saveErr := db.SetJSON(ctx, cacheKey, fetched, ttl); saveErr != nil {
			logger.WarnF(ctx, "[NetTools] persist RIPEstat neighbors cache %s failed: %v", cacheKey, saveErr)
		}
	}
	return sliceNeighborItems(fetched, limit)
}

type aggregatedNeighbor struct {
	asn      uint32
	hasLeft  bool
	hasRight bool
	power    int
}

func fetchRIPEStatAllNeighbors(ctx context.Context, originASN uint32) ([]ASNeighborItem, error) {
	endpoint, err := buildRIPEStatURL("/data/asn-neighbours/data.json", fmt.Sprintf("AS%d", originASN))
	if err != nil {
		return nil, err
	}
	body, err := fetchRIPEStatJSON(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var env ripeNeighboursEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode ripestat asn-neighbours response: %w", err)
	}

	byASN := make(map[uint32]*aggregatedNeighbor)
	for _, nb := range env.Data.Neighbours {
		if !isPublicRouteASN(nb.ASN) || nb.ASN == originASN {
			continue
		}
		agg := byASN[nb.ASN]
		if agg == nil {
			agg = &aggregatedNeighbor{asn: nb.ASN}
			byASN[nb.ASN] = agg
		}
		agg.power += max(0, nb.Power)
		switch nb.Type {
		case ripeRelationLeft:
			agg.hasLeft = true
		case ripeRelationRight:
			agg.hasRight = true
		}
	}

	sorted := make([]*aggregatedNeighbor, 0, len(byASN))
	for _, agg := range byASN {
		sorted = append(sorted, agg)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].power != sorted[j].power {
			return sorted[i].power > sorted[j].power
		}
		return sorted[i].asn < sorted[j].asn
	})
	if len(sorted) > maxCachedNeighbors {
		sorted = sorted[:maxCachedNeighbors]
	}

	out := make([]ASNeighborItem, 0, len(sorted))
	for _, agg := range sorted {
		out = append(out, buildRIPEStatNeighborItem(agg))
	}
	return out, nil
}

func buildRIPEStatNeighborItem(agg *aggregatedNeighbor) ASNeighborItem {
	relation := pkgnetintel.RelationPeer
	switch {
	case agg.hasLeft && agg.hasRight:
		relation = pkgnetintel.RelationPeer
	case agg.hasLeft:
		relation = pkgnetintel.RelationProvider
	case agg.hasRight:
		relation = pkgnetintel.RelationCustomer
	}
	item := ASNeighborItem{
		PeerASN:  agg.asn,
		Relation: relation,
	}
	if info := netintel.LookupASN(agg.asn); info != nil {
		item.PeerName = info.Name
		item.PeerCountry = info.Country
		item.PeerClass = info.Class
		if len(info.Tags) > 0 {
			item.Tags = append([]string(nil), info.Tags...)
		}
	}
	if strings.TrimSpace(item.PeerName) == "" {
		item.PeerName = pkgnetintel.ShortASNLabel(agg.asn, "")
	}
	return item
}
