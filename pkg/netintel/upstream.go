// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"fmt"
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
	if len(s.caida.neighbors[originASN]) == 0 {
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

	directProviders := collectSortedCAIDARelatedASNs(s.caida, originASN, maxDirect, false, RelationProvider)
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
	peers := collectSortedCAIDARelatedASNs(graph, originASN, maxDirect, true, RelationPeer)
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
	raw := graph.neighbors[tier1Provider]
	seen := make(map[uint32]struct{}, len(raw))
	candidates := make([]uint32, 0)
	for _, rel := range raw {
		if rel.PeerASN == 0 || rel.PeerASN == originASN || rel.PeerASN == tier1Provider {
			continue
		}
		if rel.Relation != RelationPeer && rel.Relation != RelationProvider {
			continue
		}
		if !IsTier1ASN(rel.PeerASN) {
			continue
		}
		if _, ok := seen[rel.PeerASN]; ok {
			continue
		}
		seen[rel.PeerASN] = struct{}{}
		candidates = append(candidates, rel.PeerASN)
	}
	sort.Slice(candidates, func(i, j int) bool {
		di, dj := len(graph.neighbors[candidates[i]]), len(graph.neighbors[candidates[j]])
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
	secondHops := collectSortedCAIDARelatedASNs(graph, firstHop, maxUpstreamHopProviders, false, RelationProvider)
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
		thirdHops := collectSortedCAIDARelatedASNs(graph, h2, maxUpstreamHopProviders, false, RelationProvider)
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
	relation string,
) []uint32 {
	raw := graph.neighbors[asn]
	seen := make(map[uint32]struct{}, len(raw))
	out := make([]uint32, 0)
	for _, rel := range raw {
		if rel.PeerASN == 0 || rel.PeerASN == asn || rel.Relation != relation {
			continue
		}
		if tier1Only && !IsTier1ASN(rel.PeerASN) {
			continue
		}
		if _, ok := seen[rel.PeerASN]; ok {
			continue
		}
		seen[rel.PeerASN] = struct{}{}
		out = append(out, rel.PeerASN)
	}
	sort.Slice(out, func(i, j int) bool {
		t1i, t1j := IsTier1ASN(out[i]), IsTier1ASN(out[j])
		if t1i != t1j {
			return t1i
		}
		di, dj := len(graph.neighbors[out[i]]), len(graph.neighbors[out[j]])
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
