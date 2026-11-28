// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"bytes"
	"compress/gzip"
	"math"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	geodata "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip/data"
)

func TestManagerLookupAndPrefixMatching(t *testing.T) {
	dir := t.TempDir()

	asnsCSV := "asn,name,class,cc\nAS13335,\"Cloudflare, Inc.\",Content,US\nAS4134,CHINANET-BACKBONE,Eyeball,CN\nAS174,Cogent Communications,Transit,US\n"
	if err := os.WriteFile(filepath.Join(dir, "bgptools-asns.csv"), []byte(asnsCSV), 0o600); err != nil {
		t.Fatalf("write asns.csv: %v", err)
	}

	tagsText := "cdn AS13335\nanycast AS13335\neyeball AS4134\n"
	if err := os.WriteFile(filepath.Join(dir, "bgptools-asn-tags.txt"), []byte(tagsText), 0o600); err != nil {
		t.Fatalf("write asn-tags: %v", err)
	}

	var tableBuf bytes.Buffer
	gz := gzip.NewWriter(&tableBuf)
	_, _ = gz.Write([]byte("{\"CIDR\":\"1.1.1.0/24\",\"ASN\":13335,\"Hits\":9000}\n"))
	_, _ = gz.Write([]byte("{\"CIDR\":\"1.0.0.0/8\",\"ASN\":4134,\"Hits\":100}\n"))
	_, _ = gz.Write([]byte("{\"CIDR\":\"2606:4700:4700::/48\",\"ASN\":13335,\"Hits\":5000}\n"))
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip table: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bgptools-table.jsonl.gz"), tableBuf.Bytes(), 0o600); err != nil {
		t.Fatalf("write table.jsonl.gz: %v", err)
	}

	fallbackCountry, err := geodata.FS.ReadFile(geodata.DefaultMMDBName)
	if err != nil {
		t.Fatalf("read fallback embedded mmdb: %v", err)
	}

	mgr := NewManager()
	if err := mgr.LoadFromDir(dir, fallbackCountry); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}

	// Longest prefix match should pick /24 (AS13335) over /8 (AS4134)
	res := mgr.LookupIP("1.1.1.1")
	if res == nil {
		t.Fatal("expected non-nil LookupIP result")
	}
	if res.ASN != 13335 || res.Prefix != "1.1.1.0/24" || res.ASNName != "Cloudflare, Inc." {
		t.Fatalf("unexpected LPM lookup result: %+v", res)
	}
	if len(res.ASNTags) != 2 || res.ASNTags[0] != "anycast" || res.ASNTags[1] != "cdn" {
		t.Fatalf("unexpected ASN tags: %+v", res.ASNTags)
	}

	// Broader /8 match
	res2 := mgr.LookupIP("1.2.3.4")
	if res2 == nil || res2.ASN != 4134 || res2.Prefix != "1.0.0.0/8" {
		t.Fatalf("expected AS4134 for 1.2.3.4, got %+v", res2)
	}

	// IPv6 match
	res6 := mgr.LookupIP("2606:4700:4700::1111")
	if res6 == nil || res6.ASN != 13335 || res6.Prefix != "2606:4700:4700::/48" {
		t.Fatalf("expected IPv6 AS13335 match, got %+v", res6)
	}

	// ASN details & prefix pagination
	info := mgr.LookupASN(13335)
	if info == nil || info.IPv4Prefixes != 1 || info.IPv6Prefixes != 1 {
		t.Fatalf("unexpected LookupASN: %+v", info)
	}
	v4, total := mgr.ListASNPrefixes(13335, "ipv4", 1, 10)
	if total != 1 || len(v4) != 1 || v4[0].CIDR != "1.1.1.0/24" {
		t.Fatalf("unexpected IPv4 prefix list: total=%d list=%+v", total, v4)
	}
	overflowRecords, overflowTotal := mgr.ListASNPrefixes(13335, "", math.MaxInt, 20)
	if overflowTotal != 2 || len(overflowRecords) != 0 {
		t.Fatalf("expected empty page on huge page number, got total=%d len=%d", overflowTotal, len(overflowRecords))
	}

	// Topology path with observed edges (no CAIDA file on disk)
	hops, path, ok := mgr.EstimateASPath(4134, 13335, map[uint32][]uint32{
		4134: {174},
		174:  {13335},
	})
	if !ok || hops != 2 || len(path) != 3 || path[0] != 4134 || path[1] != 174 || path[2] != 13335 {
		t.Fatalf("unexpected observed AS path: ok=%v hops=%d path=%v", ok, hops, path)
	}

	// Verify status report contains datasets
	statuses := mgr.Statuses()
	if len(statuses) == 0 {
		t.Fatal("expected dataset statuses")
	}
}

func TestCAIDABzip2ParsingAndPath(t *testing.T) {
	if _, err := exec.LookPath("bzip2"); err != nil {
		t.Skip("bzip2 binary not installed in environment; skipping bz2 generation check")
	}
	dir := t.TempDir()
	txt := "# comment\n174|13335|-1|source\n4134|174|0\n4134|3356|0\n3356|13335|-1\n"
	txtPath := filepath.Join(dir, "caida-as-rel2.txt")
	if err := os.WriteFile(txtPath, []byte(txt), 0o600); err != nil {
		t.Fatalf("write caida txt: %v", err)
	}
	if out, err := exec.Command("bzip2", "-f", txtPath).CombinedOutput(); err != nil { //nolint:gosec // test helper
		t.Fatalf("bzip2 failed: %v (%s)", err, string(out))
	}

	mgr := NewManager()
	if err := mgr.LoadFromDir(dir, nil); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	neighbors := mgr.LookupASNeighbors(174, 10)
	if len(neighbors) != 2 || neighbors[0].PeerASN != 13335 || neighbors[0].Relation != RelationCustomer ||
		neighbors[1].PeerASN != 4134 || neighbors[1].Relation != RelationPeer {
		t.Fatalf("unexpected ordered neighbors for AS174: %+v", neighbors)
	}
	if peers := mgr.LookupASNeighbors(13335, 1); len(peers) != 1 || peers[0].PeerASN != 174 || peers[0].Relation != RelationProvider {
		t.Fatalf("unexpected reverse provider relationship: %+v", peers)
	}
	hops, path, ok := mgr.EstimateASPath(4134, 13335, nil)
	if !ok || hops != 2 || len(path) != 3 || path[1] != 174 {
		t.Fatalf("unexpected CAIDA path: ok=%v hops=%d path=%v", ok, hops, path)
	}
	hops, path, ok = mgr.EstimateASPath(4134, 13335, map[uint32][]uint32{4134: {3356}})
	if !ok || hops != 2 || len(path) != 3 || path[1] != 3356 {
		t.Fatalf("expected observed edge to take priority: ok=%v hops=%d path=%v", ok, hops, path)
	}
}

func TestGeoMathSphericalCentroidAndAnycastClassification(t *testing.T) {
	// Antimeridian average of +179 and -179 must land near +/-180, NOT 0!
	var c SphericalCentroid
	c.Add(10, 179, 1)
	c.Add(10, -179, 1)
	lat, lon, ok := c.Result()
	if !ok {
		t.Fatal("expected valid spherical centroid")
	}
	if math.Abs(lat-10) > 0.1 || math.Abs(math.Abs(lon)-180) > 0.1 {
		t.Fatalf("unexpected antimeridian centroid: lat=%f lon=%f", lat, lon)
	}

	// Distance & fiber base RTT sanity check (~1400km Beijing->Shanghai -> ~1050-1100km -> ~10.5ms)
	dist := DistanceKm(39.9042, 116.4074, 31.2304, 121.4737)
	if dist < 1000 || dist > 1150 {
		t.Fatalf("unexpected BJ-SH distance: %f", dist)
	}
	if rtt := GeoBaseRTTMs(dist); rtt < 10 || rtt > 12 {
		t.Fatalf("unexpected BJ-SH base RTT: %f", rtt)
	}

	if got := FormatSubdivisionCode("cn", "gd"); got != "CN-GD" {
		t.Fatalf("FormatSubdivisionCode = %q, want CN-GD", got)
	}
	if got := NormalizeMatrixRegion("CN", "44"); got != "CN-44" {
		t.Fatalf("NormalizeMatrixRegion(CN) = %q, want CN-44", got)
	}
	if got := NormalizeMatrixRegion("SG", "01"); got != "" {
		t.Fatalf("NormalizeMatrixRegion(SG) = %q, want empty", got)
	}
	if DetectIPFamily("203.0.113.1:443") != IPFamilyIPv4 || DetectIPFamily("[2606:4700::1]:443") != IPFamilyIPv6 {
		t.Fatalf("unexpected IP family detection")
	}
	if EvaluateAnycastStatus(netip.MustParseAddr("1.1.1.1"), false, nil) != AnycastStatusConfirmed {
		t.Fatalf("expected 1.1.1.1 confirmed anycast")
	}
	if EvaluateAnycastStatus(netip.MustParseAddr("203.0.113.5"), false, []string{"anycast"}) != AnycastStatusUnknown {
		t.Fatalf("expected ASN-tag-only anycast to be unknown")
	}
	if EvaluateAnycastStatus(netip.MustParseAddr("203.0.113.5"), false, []string{"eyeball"}) != AnycastStatusUnicast {
		t.Fatalf("expected eyeball IP to be unicast")
	}

	var rec mmdbGeoRecord
	rec.Continent.Code = "as"
	rec.Country.ISOCode = "cn"
	rec.Country.Names = map[string]string{"en": "China"}
	rec.Subdivisions = []struct {
		ISOCode string `maxminddb:"iso_code"`
	}{{ISOCode: "gd"}}
	rec.City.Names = map[string]string{"en": "Shenzhen"}
	rec.Location.Latitude = 22.5431
	rec.Location.Longitude = 114.0579
	rec.Location.AccuracyRadius = 20
	fields := extractGeoFields(&rec, true)
	if fields.continentCode != "AS" || fields.regionCode != "CN-GD" || fields.geoSource != GeoSourceCity || fields.accuracyRadiusKm != 20 {
		t.Fatalf("unexpected extracted MMDB fields: %+v", fields)
	}
}
