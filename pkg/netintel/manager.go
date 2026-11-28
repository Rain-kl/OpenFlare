// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"encoding/json"
	"math"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Snapshot holds an immutable set of loaded NetIntel databases.
type Snapshot struct {
	dataDir  string
	manifest *Manifest
	mmdb     *mmdbBundle
	bgp      *bgpToolsIndex
	caida    *caidaGraph
	statuses []DatasetFileStatus
}

// Manager serves concurrent lock-free network intelligence queries and supports atomic reloads.
type Manager struct {
	snap atomic.Pointer[Snapshot]
}

// NewManager creates an empty NetIntel Manager.
func NewManager() *Manager {
	m := &Manager{}
	m.snap.Store(&Snapshot{
		bgp:   &bgpToolsIndex{asns: map[uint32]asnMeta{}, tags: map[uint32][]string{}, v6Buckets: map[int]map[[16]byte]prefixPayload{}, asnPrefixes: map[uint32][]PrefixRecord{}},
		caida: &caidaGraph{neighbors: map[uint32][]ASRelation{}},
	})
	return m
}

// LoadFromDir loads all available datasets from dir (with fallbackCountryBytes if no GeoIP Country/City MMDB exists on disk)
// and atomically swaps the active snapshot.
func (m *Manager) LoadFromDir(dir string, fallbackCountryBytes []byte) error {
	cleanDir := strings.TrimSpace(dir)
	var manifest *Manifest
	if cleanDir != "" {
		raw, err := os.ReadFile(filepath.Join(cleanDir, "manifest.json")) //nolint:gosec // path is inside configured netintel directory
		if err == nil {
			var parsed Manifest
			if json.Unmarshal(raw, &parsed) == nil {
				manifest = &parsed
			}
		}
	}

	mmdb, mmdbStatuses := loadMMDBBundle(cleanDir, manifest, fallbackCountryBytes)
	bgp, bgpStatuses := loadBGPToolsIndex(cleanDir, manifest)
	caida, caidaStatus := loadCAIDAGraph(cleanDir, manifest)

	statuses := make([]DatasetFileStatus, 0, len(mmdbStatuses)+len(bgpStatuses)+1)
	statuses = append(statuses, mmdbStatuses...)
	statuses = append(statuses, bgpStatuses...)
	statuses = append(statuses, caidaStatus)

	m.snap.Store(&Snapshot{
		dataDir:  cleanDir,
		manifest: manifest,
		mmdb:     mmdb,
		bgp:      bgp,
		caida:    caida,
		statuses: statuses,
	})
	return nil
}

// DataDir returns the currently loaded NetIntel directory path.
func (m *Manager) DataDir() string {
	s := m.snap.Load()
	if s == nil {
		return ""
	}
	return s.dataDir
}

// Statuses returns a copy of all dataset file load statuses.
func (m *Manager) Statuses() []DatasetFileStatus {
	s := m.snap.Load()
	if s == nil || len(s.statuses) == 0 {
		return nil
	}
	out := make([]DatasetFileStatus, len(s.statuses))
	copy(out, s.statuses)
	return out
}

// LookupIP resolves geolocation, ASN, tags, and BGP routing prefix metadata for rawIP.
func (m *Manager) LookupIP(rawIP string) *IPLookupResult {
	s := m.snap.Load()
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(rawIP)
	addr, err := netip.ParseAddr(trimmed)
	if err != nil {
		return nil
	}
	addr = addr.Unmap()
	stdIP := net.ParseIP(addr.String())
	if stdIP == nil {
		return nil
	}

	res := &IPLookupResult{
		IP:          addr.String(),
		ASNTags:     []string{},
		DataSources: []string{},
	}

	geo := s.mmdb.lookupGeo(stdIP)
	res.ContinentCode = geo.continentCode
	res.CountryISO = geo.countryISO
	res.CountryName = geo.countryName
	res.RegionCode = geo.regionCode
	res.CityName = geo.cityName
	res.Latitude = geo.latitude
	res.Longitude = geo.longitude
	res.AccuracyRadiusKm = geo.accuracyRadiusKm
	res.GeoSource = geo.geoSource
	if geo.countryISO != "" || geo.latitude != nil {
		res.DataSources = append(res.DataSources, "mmdb_geo")
	}

	prefix, tableASN, hits := s.bgp.lookupIP(addr)
	res.Prefix = prefix
	res.PrefixHits = hits
	if tableASN > 0 {
		res.ASN = tableASN
		res.DataSources = append(res.DataSources, "bgp.tools")
	}

	mmdbASN, mmdbOrg := s.mmdb.lookupASN(stdIP)
	if res.ASN == 0 && mmdbASN > 0 {
		res.ASN = mmdbASN
	}
	if mmdbASN > 0 {
		res.DataSources = append(res.DataSources, "mmdb_asn")
	}

	populateASNMetadata(s, res, mmdbOrg)
	res.AnycastStatus = EvaluateAnycastStatus(addr, geo.isAnycast, res.ASNTags)
	if res.GeoSource == "" {
		res.GeoSource = GeoSourceUnknown
	}
	return res
}

func populateASNMetadata(s *Snapshot, res *IPLookupResult, mmdbOrg string) {
	if res.ASN == 0 {
		return
	}
	if meta, ok := s.bgp.asns[res.ASN]; ok {
		res.ASNName = meta.Name
		res.ASNClass = meta.Class
		res.ASNCountry = meta.Country
	}
	if res.ASNName == "" {
		res.ASNName = mmdbOrg
	}
	if tags := s.bgp.tags[res.ASN]; len(tags) > 0 {
		res.ASNTags = append([]string(nil), tags...)
	}
	if res.CountryISO == "" && res.ASNCountry != "" {
		res.CountryISO = res.ASNCountry
		res.CountryName = res.ASNCountry
		if res.GeoSource == "" || res.GeoSource == GeoSourceUnknown {
			res.GeoSource = GeoSourceASNCountry
		}
	}
}

// LookupASN returns metadata and summary statistics for asn.
func (m *Manager) LookupASN(asn uint32) *ASNInfo {
	s := m.snap.Load()
	if s == nil || asn == 0 {
		return nil
	}
	meta, hasMeta := s.bgp.asns[asn]
	prefixes := s.bgp.asnPrefixes[asn]
	tags := s.bgp.tags[asn]
	neighbors := s.caida.neighbors[asn]
	if !hasMeta && len(prefixes) == 0 && len(tags) == 0 && len(neighbors) == 0 {
		return nil
	}

	v4Count, v6Count := 0, 0
	for _, p := range prefixes {
		if p.Family == "ipv6" {
			v6Count++
		} else {
			v4Count++
		}
	}

	copiedTags := make([]string, len(tags))
	copy(copiedTags, tags)
	return &ASNInfo{
		ASN:          asn,
		Name:         meta.Name,
		Class:        meta.Class,
		Country:      meta.Country,
		Tags:         copiedTags,
		IPv4Prefixes: v4Count,
		IPv6Prefixes: v6Count,
		NeighborCnt:  len(neighbors),
	}
}

const maxASNPrefixPageSize = 200

// ListASNPrefixes returns paginated prefixes announced by asn, optionally filtered by family ("ipv4" or "ipv6").
func (m *Manager) ListASNPrefixes(asn uint32, family string, page, pageSize int) ([]PrefixRecord, int) {
	s := m.snap.Load()
	if s == nil || asn == 0 {
		return nil, 0
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > maxASNPrefixPageSize {
		pageSize = maxASNPrefixPageSize
	}
	fam := strings.ToLower(strings.TrimSpace(family))
	all := s.bgp.asnPrefixes[asn]
	filtered := make([]PrefixRecord, 0, len(all))
	for _, p := range all {
		if fam != "" && fam != "all" && p.Family != fam {
			continue
		}
		filtered = append(filtered, p)
	}
	total := len(filtered)
	if page-1 > math.MaxInt/pageSize {
		return []PrefixRecord{}, total
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []PrefixRecord{}, total
	}
	end := min(start+pageSize, total)
	out := make([]PrefixRecord, end-start)
	copy(out, filtered[start:end])
	return out, total
}

// LookupASNeighbors returns CAIDA-derived neighbor relationships enriched with bgp.tools ASN metadata.
func (m *Manager) LookupASNeighbors(asn uint32, limit int) []ASRelation {
	s := m.snap.Load()
	if s == nil || asn == 0 {
		return nil
	}
	raw := s.caida.neighbors[asn]
	if len(raw) == 0 {
		return []ASRelation{}
	}
	if limit <= 0 || limit > len(raw) {
		limit = len(raw)
	}
	out := make([]ASRelation, 0, limit)
	for _, item := range raw[:limit] {
		meta := s.bgp.asns[item.PeerASN]
		rel := ASRelation{
			PeerASN:     item.PeerASN,
			PeerName:    meta.Name,
			PeerCountry: meta.Country,
			PeerClass:   meta.Class,
			Relation:    item.Relation,
		}
		if tags := s.bgp.tags[item.PeerASN]; len(tags) > 0 {
			rel.Tags = append([]string(nil), tags...)
		}
		out = append(out, rel)
	}
	return out
}

// EstimateASPath computes a shortest AS hop path merging observed traceroute edges and CAIDA relationships.
func (m *Manager) EstimateASPath(srcASN, dstASN uint32, observedAdj map[uint32][]uint32) (int, []uint32, bool) {
	s := m.snap.Load()
	if s == nil {
		return 0, nil, false
	}
	return s.caida.estimatePath(srcASN, dstASN, observedAdj)
}
