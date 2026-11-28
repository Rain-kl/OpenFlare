// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	defaultUpstreamMaxDepth    = 3
	minTwoHopDepth             = 2
	defaultUpstreamMaxDirect   = 12
	maxUpstreamHopProviders    = 8
	maxTier1InterconnectTarget = 8
	maxShortASNRunes           = 18

	simWeightDirectUpstream = 0.40
	simWeightObservedPath   = 0.30
	simWeightClassAndTags   = 0.20
	simWeightTier1          = 0.07
	simWeightNeighbors      = 0.03
	simClassWeightShare     = 0.50
	simTagWeightShare       = 0.50
	simCoverageNormFloor    = 0.60
)

var tier1ShortNames = map[uint32]string{
	174:   "Cogent",
	701:   "Verizon",
	1299:  "Arelion",
	2914:  "NTT",
	3257:  "GTT",
	3320:  "DTAG",
	3356:  "Lumen",
	3491:  "PCCW",
	5511:  "Orange",
	6453:  "TATA",
	6461:  "Zayo",
	6762:  "Sparkle",
	6830:  "Liberty",
	7018:  "AT&T",
	12956: "Telxius",
}

var asnCorporateSuffixes = []string{
	" co., ltd.",
	" co., ltd",
	" co.,ltd.",
	" co.,ltd",
	", inc.",
	", inc",
	", llc",
	", ltd.",
	", ltd",
	" corporation",
	" incorporated",
	" limited",
	" corp.",
	" corp",
	" inc.",
	" inc",
	" llc",
	" ltd.",
	" ltd",
	" gmbh",
	" pte.",
	" pte",
	" b.v.",
	" s.a.",
	" n.v.",
}

// IsTier1ASN reports whether asn is in the canonical global Tier 1 ISP registry.
func IsTier1ASN(asn uint32) bool {
	_, ok := tier1ShortNames[asn]
	return ok
}

// ShortASNLabel returns a concise display label suitable for compact routing graph boxes.
func ShortASNLabel(asn uint32, fullName string) string {
	if short, ok := tier1ShortNames[asn]; ok {
		return short
	}
	clean := strings.TrimSpace(fullName)
	for clean != "" {
		lower := strings.ToLower(clean)
		trimmed := false
		for _, suffix := range asnCorporateSuffixes {
			if strings.HasSuffix(lower, suffix) {
				clean = strings.TrimSpace(clean[:len(clean)-len(suffix)])
				clean = strings.TrimRight(clean, ",.- ")
				trimmed = true
				break
			}
		}
		if !trimmed {
			break
		}
	}
	clean = strings.TrimRight(strings.TrimSpace(clean), ",.- ")
	if clean == "" {
		if asn > 0 {
			return fmt.Sprintf("AS%d", asn)
		}
		return ""
	}
	runes := []rune(clean)
	if len(runes) <= maxShortASNRunes {
		return clean
	}
	return strings.TrimSpace(string(runes[:maxShortASNRunes]))
}

// LookupASUpstreamPaths extracts bounded CAIDA upstream paths from originASN toward Tier 1 ISPs.
func (m *Manager) LookupASUpstreamPaths(originASN uint32, maxDepth, maxDirect int) [][]uint32 {
	s := m.snap.Load()
	if s == nil || s.caida == nil || originASN == 0 {
		return nil
	}
	if s.caida.degree(originASN) == 0 {
		return nil
	}
	if maxDepth <= 0 {
		maxDepth = defaultUpstreamMaxDepth
	}
	if maxDirect <= 0 {
		maxDirect = defaultUpstreamMaxDirect
	}

	if IsTier1ASN(originASN) {
		return extractTier1OriginPaths(s.caida, originASN, maxDirect)
	}

	directProviders := collectSortedCAIDARelatedASNs(s.caida, originASN, maxDirect, false, caidaProvider)
	if len(directProviders) == 0 {
		return nil
	}

	var paths [][]uint32
	for _, provider := range directProviders {
		if IsTier1ASN(provider) {
			paths = append(paths, []uint32{originASN, provider})
			for _, t2 := range collectTier1Interconnects(s.caida, originASN, provider) {
				paths = append(paths, []uint32{originASN, provider, t2})
			}
			continue
		}
		paths = append(paths, walkCAIDAUpstreamToTier1(s.caida, originASN, provider, maxDepth)...)
	}
	return paths
}

func extractTier1OriginPaths(graph *caidaGraph, originASN uint32, maxDirect int) [][]uint32 {
	peers := collectSortedCAIDARelatedASNs(graph, originASN, maxDirect, true, caidaPeer)
	if len(peers) == 0 {
		return nil
	}
	paths := make([][]uint32, 0, len(peers))
	for _, peer := range peers {
		paths = append(paths, []uint32{originASN, peer})
	}
	return paths
}

func collectTier1Interconnects(graph *caidaGraph, originASN, tier1Provider uint32) []uint32 {
	start, end := graph.neighborBounds(tier1Provider)
	raw := graph.peers[start:end]
	seen := make(map[uint32]struct{}, len(raw))
	candidates := make([]uint32, 0)
	for i, peer := range raw {
		if peer == 0 || peer == originASN || peer == tier1Provider {
			continue
		}
		relation := graph.relations[start+i]
		if relation != caidaPeer && relation != caidaProvider {
			continue
		}
		if !IsTier1ASN(peer) {
			continue
		}
		if _, ok := seen[peer]; ok {
			continue
		}
		seen[peer] = struct{}{}
		candidates = append(candidates, peer)
	}
	sort.Slice(candidates, func(i, j int) bool {
		di, dj := graph.degree(candidates[i]), graph.degree(candidates[j])
		if di != dj {
			return di > dj
		}
		return candidates[i] < candidates[j]
	})
	if len(candidates) > maxTier1InterconnectTarget {
		candidates = candidates[:maxTier1InterconnectTarget]
	}
	return candidates
}

func walkCAIDAUpstreamToTier1(graph *caidaGraph, originASN, firstHop uint32, maxDepth int) [][]uint32 {
	if maxDepth < minTwoHopDepth {
		return [][]uint32{{originASN, firstHop}}
	}
	secondHops := collectSortedCAIDARelatedASNs(graph, firstHop, maxUpstreamHopProviders, false, caidaProvider)
	if len(secondHops) == 0 {
		return [][]uint32{{originASN, firstHop}}
	}

	var paths [][]uint32
	for _, h2 := range secondHops {
		if h2 == originASN || h2 == firstHop {
			continue
		}
		if IsTier1ASN(h2) || maxDepth < defaultUpstreamMaxDepth {
			paths = append(paths, []uint32{originASN, firstHop, h2})
			continue
		}
		thirdHops := collectSortedCAIDARelatedASNs(graph, h2, maxUpstreamHopProviders, false, caidaProvider)
		added := false
		for _, h3 := range thirdHops {
			if h3 == originASN || h3 == firstHop || h3 == h2 {
				continue
			}
			paths = append(paths, []uint32{originASN, firstHop, h2, h3})
			added = true
		}
		if !added {
			paths = append(paths, []uint32{originASN, firstHop, h2})
		}
	}
	if len(paths) == 0 {
		return [][]uint32{{originASN, firstHop}}
	}
	return paths
}

func collectSortedCAIDARelatedASNs(
	graph *caidaGraph,
	asn uint32,
	limit int,
	tier1Only bool,
	relation caidaRelation,
) []uint32 {
	start, end := graph.neighborBounds(asn)
	raw := graph.peers[start:end]
	seen := make(map[uint32]struct{}, len(raw))
	out := make([]uint32, 0)
	for i, peer := range raw {
		if peer == 0 || peer == asn || graph.relations[start+i] != relation {
			continue
		}
		if tier1Only && !IsTier1ASN(peer) {
			continue
		}
		if _, ok := seen[peer]; ok {
			continue
		}
		seen[peer] = struct{}{}
		out = append(out, peer)
	}
	sort.Slice(out, func(i, j int) bool {
		t1i, t1j := IsTier1ASN(out[i]), IsTier1ASN(out[j])
		if t1i != t1j {
			return t1i
		}
		di, dj := graph.degree(out[i]), graph.degree(out[j])
		if di != dj {
			return di > dj
		}
		return out[i] < out[j]
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ASUpstreamProfile summarizes an ASN's upstream providers, Tier-1 reachability, neighbors, and class/tags.
type ASUpstreamProfile struct {
	ASN             uint32
	Class           string
	Country         string
	Tags            []string
	DirectProviders []uint32
	Tier1Upstreams  []uint32
	Neighbors       []uint32
}

// NetworkSimilarityResult reports network similarity in [0, 1] and whether sufficient evidence existed.
type NetworkSimilarityResult struct {
	Score float64
	Known bool
}

// ExtractASUpstreamProfile extracts an immutable upstream and metadata profile for asn.
func (m *Manager) ExtractASUpstreamProfile(asn uint32) ASUpstreamProfile {
	prof := ASUpstreamProfile{ASN: asn}
	if m == nil || asn == 0 {
		return prof
	}
	s := m.snap.Load()
	if s == nil {
		return prof
	}
	if s.bgp != nil {
		if meta, ok := s.bgp.asns[asn]; ok {
			prof.Class = strings.TrimSpace(meta.Class)
			prof.Country = strings.ToUpper(strings.TrimSpace(meta.Country))
		}
		if tags := s.bgp.tags[asn]; len(tags) > 0 {
			prof.Tags = append([]string(nil), tags...)
		}
	}
	if start, end := s.caida.neighborBounds(asn); start != end {
		prof.DirectProviders = collectSortedCAIDARelatedASNs(s.caida, asn, defaultUpstreamMaxDirect, false, caidaProvider)
		peers := s.caida.peers[start:end]
		neighSeen := make(map[uint32]struct{}, len(peers))
		for _, peer := range peers {
			if peer == 0 || peer == asn {
				continue
			}
			if _, ok := neighSeen[peer]; ok {
				continue
			}
			neighSeen[peer] = struct{}{}
			prof.Neighbors = append(prof.Neighbors, peer)
		}
		sort.Slice(prof.Neighbors, func(i, j int) bool { return prof.Neighbors[i] < prof.Neighbors[j] })
	}

	tier1Seen := make(map[uint32]struct{})
	if IsTier1ASN(asn) {
		tier1Seen[asn] = struct{}{}
	}
	for _, p := range m.LookupASUpstreamPaths(asn, defaultUpstreamMaxDepth, defaultUpstreamMaxDirect) {
		for _, hop := range p {
			if IsTier1ASN(hop) && hop != asn {
				tier1Seen[hop] = struct{}{}
			}
		}
	}
	for t1 := range tier1Seen {
		prof.Tier1Upstreams = append(prof.Tier1Upstreams, t1)
	}
	sort.Slice(prof.Tier1Upstreams, func(i, j int) bool { return prof.Tier1Upstreams[i] < prof.Tier1Upstreams[j] })
	return prof
}

// ComputeNetworkSimilarity evaluates tiered similarity between two AS profiles without treating missing data as identical.
func ComputeNetworkSimilarity(a, b ASUpstreamProfile, observedASPathSimilarity float64) NetworkSimilarityResult {
	var weightedSum, activeWeight float64

	if len(a.DirectProviders) > 0 && len(b.DirectProviders) > 0 {
		weightedSum += simWeightDirectUpstream * uint32Jaccard(a.DirectProviders, b.DirectProviders)
		activeWeight += simWeightDirectUpstream
	} else if len(a.DirectProviders) > 0 || len(b.DirectProviders) > 0 {
		// One side has known direct providers while the other lacks them: count active weight as mismatch (0).
		activeWeight += simWeightDirectUpstream
	}

	if observedASPathSimilarity > 0 {
		clamped := math.Max(0, math.Min(1.0, observedASPathSimilarity))
		weightedSum += simWeightObservedPath * clamped
		activeWeight += simWeightObservedPath
	}

	classTagSim, hasClassTag := computeClassAndTagSimilarity(a, b)
	if hasClassTag {
		weightedSum += simWeightClassAndTags * classTagSim
		activeWeight += simWeightClassAndTags
	}

	if len(a.Tier1Upstreams) > 0 && len(b.Tier1Upstreams) > 0 {
		weightedSum += simWeightTier1 * uint32Jaccard(a.Tier1Upstreams, b.Tier1Upstreams)
		activeWeight += simWeightTier1
	}

	if len(a.Neighbors) > 0 && len(b.Neighbors) > 0 {
		weightedSum += simWeightNeighbors * uint32Jaccard(a.Neighbors, b.Neighbors)
		activeWeight += simWeightNeighbors
	}

	if activeWeight <= 0 {
		return NetworkSimilarityResult{Score: 0, Known: false}
	}

	raw := weightedSum / activeWeight
	coverageDamping := math.Min(1.0, activeWeight/simCoverageNormFloor)
	return NetworkSimilarityResult{
		Score: math.Max(0, math.Min(1.0, raw*coverageDamping)),
		Known: true,
	}
}

func computeClassAndTagSimilarity(a, b ASUpstreamProfile) (float64, bool) {
	aTags := filterInformativeTags(a.Tags)
	bTags := filterInformativeTags(b.Tags)
	hasClass := a.Class != "" && b.Class != ""
	hasTags := len(aTags) > 0 && len(bTags) > 0
	if !hasClass && !hasTags {
		return 0, false
	}
	var sum, weight float64
	if hasClass {
		weight += simClassWeightShare
		if strings.EqualFold(a.Class, b.Class) {
			sum += simClassWeightShare
		}
	}
	if hasTags {
		weight += simTagWeightShare
		sum += simTagWeightShare * stringJaccard(aTags, bTags)
	}
	return sum / weight, true
}

func filterInformativeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm == "" || norm == "anycast" {
			continue
		}
		out = append(out, norm)
	}
	return out
}

func uint32Jaccard(a, b []uint32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[uint32]struct{}, len(a))
	for _, v := range a {
		setA[v] = struct{}{}
	}
	var inter, union int
	seenB := make(map[uint32]struct{}, len(b))
	for _, v := range b {
		if _, dup := seenB[v]; dup {
			continue
		}
		seenB[v] = struct{}{}
		if _, ok := setA[v]; ok {
			inter++
		}
	}
	union = len(setA) + len(seenB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func stringJaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]struct{}, len(a))
	for _, v := range a {
		setA[v] = struct{}{}
	}
	var inter int
	seenB := make(map[string]struct{}, len(b))
	for _, v := range b {
		if _, dup := seenB[v]; dup {
			continue
		}
		seenB[v] = struct{}{}
		if _, ok := setA[v]; ok {
			inter++
		}
	}
	union := len(setA) + len(seenB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
