// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package netintel provides lock-free in-memory IP geolocation, ASN, bgp.tools routing table,
// and optional CAIDA AS-relationship lookups backed by hot-reloadable snapshot datasets.
package netintel

import "time"

// Relation type constants for AS topology edges.
const (
	RelationProvider = "provider"
	RelationCustomer = "customer"
	RelationPeer     = "peer"
	RelationObserved = "observed"
)

// IPLookupResult aggregates geolocation, ASN, tag, and routing prefix metadata for an IP address.
type IPLookupResult struct {
	IP          string   `json:"ip"`
	CountryISO  string   `json:"country_iso"`
	CountryName string   `json:"country_name"`
	CityName    string   `json:"city_name"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	ASN         uint32   `json:"asn"`
	ASNName     string   `json:"asn_name"`
	ASNClass    string   `json:"asn_class"`
	ASNCountry  string   `json:"asn_country"`
	ASNTags     []string `json:"asn_tags"`
	Prefix      string   `json:"prefix"`
	PrefixHits  int      `json:"prefix_hits"`
	DataSources []string `json:"data_sources"`
}

// ASNInfo describes metadata and prefix counts for an Autonomous System Number.
type ASNInfo struct {
	ASN          uint32   `json:"asn"`
	Name         string   `json:"name"`
	Class        string   `json:"class"`
	Country      string   `json:"country"`
	Tags         []string `json:"tags"`
	IPv4Prefixes int      `json:"ipv4_prefixes"`
	IPv6Prefixes int      `json:"ipv6_prefixes"`
	NeighborCnt  int      `json:"neighbor_count"`
}

// PrefixRecord describes one BGP routing table prefix owned by an ASN.
type PrefixRecord struct {
	CIDR   string `json:"cidr"`
	ASN    uint32 `json:"asn"`
	Hits   int    `json:"hits"`
	Family string `json:"family"`
}

// ASRelation represents a neighbor relationship of a queried ASN.
type ASRelation struct {
	PeerASN     uint32   `json:"peer_asn"`
	PeerName    string   `json:"peer_name"`
	PeerCountry string   `json:"peer_country"`
	PeerClass   string   `json:"peer_class"`
	Relation    string   `json:"relation"`
	Tags        []string `json:"tags,omitempty"`
}

// DatasetFileStatus reports loading metadata for a single NetIntel dataset file.
type DatasetFileStatus struct {
	Key         string     `json:"key"`
	Filename    string     `json:"filename"`
	Loaded      bool       `json:"loaded"`
	Path        string     `json:"path"`
	SizeBytes   int64      `json:"size_bytes"`
	RecordCount int        `json:"record_count"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	SourceURL   string     `json:"source_url"`
	License     string     `json:"license"`
	Error       string     `json:"error,omitempty"`
}

// ManifestEntry mirrors a single dataset record in data/netintel/manifest.json.
type ManifestEntry struct {
	Filename      string `json:"filename"`
	SourceURL     string `json:"source_url"`
	License       string `json:"license"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	FetchedAtUnix int64  `json:"fetched_at_unix"`
}

// Manifest describes the downloaded NetIntel dataset inventory on disk.
type Manifest struct {
	GeneratedAtUnix int64                    `json:"generated_at_unix"`
	GeoIPSource     string                   `json:"geoip_source"`
	Datasets        map[string]ManifestEntry `json:"datasets"`
}
