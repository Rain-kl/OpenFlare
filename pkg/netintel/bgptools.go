// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"bufio"
	"bytes"
	"cmp"
	"compress/gzip"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type asnMeta struct {
	ASN     uint32
	Name    string
	Class   string
	Country string
}

type tableEntry struct {
	CIDR string `json:"CIDR"`
	ASN  uint32 `json:"ASN"`
	Hits int    `json:"Hits"`
}

type prefixID uint32

const ipv6PrefixFlag prefixID = 1 << 31

type ipv4PrefixRow struct {
	address [4]byte
	asn     uint32
	hits    int
}

type ipv6PrefixRow struct {
	address [16]byte
	hits    int
	asn     uint32
	bits    uint8
}

type asnPrefixRange struct {
	start, end int
	ipv4Count  int
}

type bgpToolsIndex struct {
	asns        map[uint32]asnMeta
	tags        map[uint32][]string
	v4Buckets   [33]map[[4]byte]uint32
	v6Buckets   [129]map[[16]byte]uint32
	v6LensDesc  []int
	v4Prefixes  []ipv4PrefixRow
	v4Bits      []uint8
	v6Prefixes  []ipv6PrefixRow
	asnPrefixes map[uint32]asnPrefixRange
	asnOrder    []prefixID
}

const (
	bgpToolsDatasetCount = 3
	bgpToolsLicense      = "bgp.tools (Attribution Required)"
	csvMinColumns        = 2
	csvClassColIdx       = 2
	csvCountryColIdx     = 3
	csvClassMinCols      = 3
	csvCountryMinCols    = 4
	bgpTagsInitBufBytes  = 64 * 1024
	bgpTagsMaxBufBytes   = 8 * 1024 * 1024
	bgpTableMaxBufBytes  = 2 * 1024 * 1024
	tagLineMinFields     = 2
	ipv4BitLen           = 32
	ipv6BitLen           = 128
	ipv6ByteLen          = 16
	bitsPerByte          = 8
	byteMaskFull         = 0xff
)

func loadBGPToolsIndex(dir string, manifest *Manifest) (*bgpToolsIndex, []DatasetFileStatus) {
	idx := &bgpToolsIndex{
		asns: make(map[uint32]asnMeta),
		tags: make(map[uint32][]string),
	}
	statuses := make([]DatasetFileStatus, 0, bgpToolsDatasetCount)

	if strings.TrimSpace(dir) == "" {
		return idx, statuses
	}

	asnsStatus := initStatusFromManifest(dir, manifest, "bgptools_asns", "bgptools-asns.csv", "https://bgp.tools/asns.csv", bgpToolsLicense)
	if asnsStatus.Path != "" {
		cnt, err := idx.loadASNsCSV(asnsStatus.Path)
		if err != nil {
			asnsStatus.Error = err.Error()
		} else {
			asnsStatus.Loaded = true
			asnsStatus.RecordCount = cnt
		}
	}
	statuses = append(statuses, asnsStatus)

	tagsStatus := initStatusFromManifest(dir, manifest, "bgptools_asn_tags", "bgptools-asn-tags.txt", "https://bgp.tools/tags/*.txt", bgpToolsLicense)
	if tagsStatus.Path != "" {
		cnt, err := idx.loadASNTagMembers(tagsStatus.Path)
		if err != nil {
			tagsStatus.Error = err.Error()
		} else {
			tagsStatus.Loaded = true
			tagsStatus.RecordCount = cnt
		}
	}
	statuses = append(statuses, tagsStatus)

	tableStatus := initStatusFromManifest(dir, manifest, "bgptools_table", "bgptools-table.jsonl.gz", "https://bgp.tools/table.jsonl", bgpToolsLicense)
	if tableStatus.Path == "" {
		alt := initStatusFromManifest(dir, manifest, "bgptools_table", "bgptools-table.jsonl", "https://bgp.tools/table.jsonl", bgpToolsLicense)
		if alt.Path != "" {
			tableStatus = alt
		}
	}
	if tableStatus.Path != "" {
		cnt, err := idx.loadRoutingTable(tableStatus.Path)
		if err != nil {
			tableStatus.Error = err.Error()
		} else {
			tableStatus.Loaded = true
			tableStatus.RecordCount = cnt
		}
	}
	statuses = append(statuses, tableStatus)

	return idx, statuses
}

func applyBGPToolsManifest(status *DatasetFileStatus, manifest *Manifest) {
	if manifest == nil {
		return
	}
	entry, ok := manifest.Datasets[status.Key]
	if !ok {
		return
	}
	if entry.Filename != "" {
		status.Filename = entry.Filename
	}
	if entry.SourceURL != "" {
		status.SourceURL = entry.SourceURL
	}
	if entry.License != "" {
		status.License = entry.License
	}
	if entry.FetchedAtUnix > 0 {
		t := time.Unix(entry.FetchedAtUnix, 0).UTC()
		status.UpdatedAt = &t
	}
}

func initStatusFromManifest(dir string, manifest *Manifest, key, defaultFilename, defaultURL, defaultLicense string) DatasetFileStatus {
	status := DatasetFileStatus{
		Key:       key,
		Filename:  defaultFilename,
		SourceURL: defaultURL,
		License:   defaultLicense,
	}
	applyBGPToolsManifest(&status, manifest)
	p := filepath.Join(dir, status.Filename)
	info, err := os.Stat(p)
	if err == nil && !info.IsDir() && info.Size() > 0 {
		status.Path = p
		status.SizeBytes = info.Size()
		if status.UpdatedAt == nil {
			t := info.ModTime().UTC()
			status.UpdatedAt = &t
		}
	}
	return status
}

func (idx *bgpToolsIndex) loadASNsCSV(path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec // path is inside configured netintel directory
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true

	count := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("parse asns csv: %w", err)
		}
		if len(rec) < csvMinColumns {
			continue
		}
		asn, ok := parseASNNumber(rec[0])
		if !ok || asn == 0 {
			continue
		}
		var class, cc string
		if len(rec) >= csvClassMinCols {
			class = strings.TrimSpace(rec[csvClassColIdx])
		}
		if len(rec) >= csvCountryMinCols {
			cc = strings.ToUpper(strings.TrimSpace(rec[csvCountryColIdx]))
		}
		idx.asns[asn] = asnMeta{
			ASN:     asn,
			Name:    strings.TrimSpace(rec[1]),
			Class:   class,
			Country: cc,
		}
		count++
	}
	return count, nil
}

func (idx *bgpToolsIndex) loadASNTagMembers(path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec // path is inside configured netintel directory
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, bgpTagsInitBufBytes), bgpTagsMaxBufBytes)
	bindings := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < tagLineMinFields {
			continue
		}
		tag := strings.ToLower(strings.TrimSpace(parts[0]))
		for _, token := range strings.Split(parts[1], ",") {
			asn, ok := parseASNNumber(token)
			if !ok || asn == 0 {
				continue
			}
			idx.tags[asn] = append(idx.tags[asn], tag)
			bindings++
		}
	}
	if err := scanner.Err(); err != nil {
		return bindings, err
	}
	for asn := range idx.tags {
		sort.Strings(idx.tags[asn])
	}
	return bindings, nil
}

func (idx *bgpToolsIndex) loadRoutingTable(path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec // path is inside configured netintel directory
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return 0, fmt.Errorf("open gzip routing table: %w", err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, bgpTagsInitBufBytes), bgpTableMaxBufBytes)
	byASN := make(map[uint32][]prefixID, len(idx.asns))
	count := 0
	var item tableEntry
	var loadErr error
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		item = tableEntry{}
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}
		prefix, err := netip.ParsePrefix(strings.TrimSpace(item.CIDR))
		if err != nil || item.ASN == 0 {
			continue
		}
		id, err := idx.addRoutingPrefix(prefix.Masked(), item.ASN, item.Hits)
		if err != nil {
			loadErr = err
			break
		}
		byASN[item.ASN] = append(byASN[item.ASN], id)
		count++
	}
	if err := scanner.Err(); err != nil {
		loadErr = fmt.Errorf("scan routing table: %w", err)
	}
	idx.buildASNPrefixOrder(byASN, count, loadErr == nil)
	if loadErr != nil {
		return count, loadErr
	}
	for bits := ipv6BitLen; bits >= 0; bits-- {
		if len(idx.v6Buckets[bits]) > 0 {
			idx.v6LensDesc = append(idx.v6LensDesc, bits)
		}
	}
	return count, nil
}

func (idx *bgpToolsIndex) addRoutingPrefix(prefix netip.Prefix, asn uint32, hits int) (prefixID, error) {
	addr, bits := prefix.Addr(), prefix.Bits()
	if addr.Is4() {
		if uint64(len(idx.v4Prefixes)) >= uint64(ipv6PrefixFlag) {
			return 0, fmt.Errorf("too many IPv4 routing prefixes")
		}
		id := uint32(len(idx.v4Prefixes)) //nolint:gosec // checked above
		address := addr.As4()
		idx.v4Prefixes = append(idx.v4Prefixes, ipv4PrefixRow{address: address, asn: asn, hits: hits})
		idx.v4Bits = append(idx.v4Bits, uint8(bits)) //nolint:gosec // valid CIDR length is at most 32
		if idx.v4Buckets[bits] == nil {
			idx.v4Buckets[bits] = make(map[[4]byte]uint32)
		}
		idx.v4Buckets[bits][address] = id
		return prefixID(id), nil
	}
	if uint64(len(idx.v6Prefixes)) >= uint64(ipv6PrefixFlag) {
		return 0, fmt.Errorf("too many IPv6 routing prefixes")
	}
	id := uint32(len(idx.v6Prefixes)) //nolint:gosec // checked above
	address := addr.As16()
	idx.v6Prefixes = append(idx.v6Prefixes, ipv6PrefixRow{
		address: address,
		asn:     asn,
		hits:    hits,
		bits:    uint8(bits), //nolint:gosec // valid CIDR length is at most 128
	})
	if idx.v6Buckets[bits] == nil {
		idx.v6Buckets[bits] = make(map[[16]byte]uint32)
	}
	idx.v6Buckets[bits][address] = id
	return ipv6PrefixFlag | prefixID(id), nil
}

func (id prefixID) isIPv6() bool {
	return id&ipv6PrefixFlag != 0
}

func (idx *bgpToolsIndex) prefixRecord(id prefixID) PrefixRecord {
	if id.isIPv6() {
		row := idx.v6Prefixes[id&^ipv6PrefixFlag]
		return PrefixRecord{
			CIDR:   netip.PrefixFrom(netip.AddrFrom16(row.address), int(row.bits)).String(),
			ASN:    row.asn,
			Hits:   row.hits,
			Family: IPFamilyIPv6,
		}
	}
	row := idx.v4Prefixes[id]
	return PrefixRecord{
		CIDR:   netip.PrefixFrom(netip.AddrFrom4(row.address), int(idx.v4Bits[id])).String(),
		ASN:    row.asn,
		Hits:   row.hits,
		Family: IPFamilyIPv4,
	}
}

func (idx *bgpToolsIndex) buildASNPrefixOrder(byASN map[uint32][]prefixID, count int, sorted bool) {
	type sortKey struct {
		cidr string
		hits int
		id   prefixID
	}
	var keys []sortKey
	idx.asnPrefixes = make(map[uint32]asnPrefixRange, len(byASN))
	idx.asnOrder = make([]prefixID, 0, count)
	for asn, ids := range byASN {
		start := len(idx.asnOrder)
		ipv4Count := 0
		if !sorted || len(ids) == 1 {
			idx.asnOrder = append(idx.asnOrder, ids...)
			for _, id := range ids {
				if !id.isIPv6() {
					ipv4Count++
				}
			}
		} else {
			keys = keys[:0]
			for _, id := range ids {
				if !id.isIPv6() {
					ipv4Count++
				}
				record := idx.prefixRecord(id)
				keys = append(keys, sortKey{cidr: record.CIDR, hits: record.Hits, id: id})
			}
			slices.SortFunc(keys, func(a, b sortKey) int {
				if order := cmp.Compare(b.hits, a.hits); order != 0 {
					return order
				}
				return strings.Compare(a.cidr, b.cidr)
			})
			for _, key := range keys {
				idx.asnOrder = append(idx.asnOrder, key.id)
			}
		}
		idx.asnPrefixes[asn] = asnPrefixRange{start: start, end: len(idx.asnOrder), ipv4Count: ipv4Count}
	}
}

func (idx *bgpToolsIndex) lookupIP(addr netip.Addr) (prefix string, asn uint32, hits int) {
	if idx == nil || !addr.IsValid() {
		return "", 0, 0
	}
	addr = addr.Unmap()
	if addr.Is4() {
		raw := addr.As4()
		for bits := ipv4BitLen; bits >= 0; bits-- {
			bucket := idx.v4Buckets[bits]
			if len(bucket) == 0 {
				continue
			}
			masked := mask4(raw, bits)
			if id, ok := bucket[masked]; ok {
				row := idx.v4Prefixes[id]
				return netip.PrefixFrom(netip.AddrFrom4(masked), bits).String(), row.asn, row.hits
			}
		}
		return "", 0, 0
	}

	raw := addr.As16()
	for _, bits := range idx.v6LensDesc {
		bucket := idx.v6Buckets[bits]
		if len(bucket) == 0 {
			continue
		}
		masked := mask16(raw, bits)
		if id, ok := bucket[masked]; ok {
			row := idx.v6Prefixes[id]
			return netip.PrefixFrom(netip.AddrFrom16(masked), bits).String(), row.asn, row.hits
		}
	}
	return "", 0, 0
}

func mask4(raw [4]byte, bits int) [4]byte {
	if bits >= ipv4BitLen {
		return raw
	}
	if bits <= 0 {
		return [4]byte{}
	}
	v := binary.BigEndian.Uint32(raw[:])
	mask := ^uint32(0) << uint(ipv4BitLen-bits)
	v &= mask
	var out [4]byte
	binary.BigEndian.PutUint32(out[:], v)
	return out
}

func mask16(raw [16]byte, bits int) [16]byte {
	if bits >= ipv6BitLen {
		return raw
	}
	if bits <= 0 {
		return [16]byte{}
	}
	out := raw
	fullBytes := bits / bitsPerByte
	remBits := bits % bitsPerByte
	if remBits > 0 && fullBytes < ipv6ByteLen {
		mask := byte(byteMaskFull << (bitsPerByte - remBits))
		out[fullBytes] &= mask
		fullBytes++
	}
	for i := fullBytes; i < ipv6ByteLen; i++ {
		out[i] = 0
	}
	return out
}

func parseASNNumber(raw string) (uint32, bool) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(strings.ToUpper(trimmed), "AS")
	if trimmed == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}
