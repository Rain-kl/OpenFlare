// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
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

type prefixPayload struct {
	ASN  uint32
	Hits int
	Len  uint8
}

type bgpToolsIndex struct {
	asns        map[uint32]asnMeta
	tags        map[uint32][]string
	v4Buckets   [33]map[[4]byte]prefixPayload
	v6Buckets   map[int]map[[16]byte]prefixPayload
	v6LensDesc  []int
	asnPrefixes map[uint32][]PrefixRecord
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
		asns:        make(map[uint32]asnMeta),
		tags:        make(map[uint32][]string),
		v6Buckets:   make(map[int]map[[16]byte]prefixPayload),
		asnPrefixes: make(map[uint32][]PrefixRecord),
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
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item tableEntry
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		prefix, err := netip.ParsePrefix(strings.TrimSpace(item.CIDR))
		if err != nil || item.ASN == 0 {
			continue
		}
		prefix = prefix.Masked()
		addr := prefix.Addr()
		bits := prefix.Bits()
		payload := prefixPayload{
			ASN:  item.ASN,
			Hits: item.Hits,
			Len:  uint8(bits), //nolint:gosec // CIDR prefix bits are 0..128
		}
		family := idx.storePrefixBucket(addr, bits, payload)
		idx.asnPrefixes[item.ASN] = append(idx.asnPrefixes[item.ASN], PrefixRecord{
			CIDR:   prefix.String(),
			ASN:    item.ASN,
			Hits:   item.Hits,
			Family: family,
		})
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("scan routing table: %w", err)
	}

	idx.v6LensDesc = make([]int, 0, len(idx.v6Buckets))
	for l := range idx.v6Buckets {
		idx.v6LensDesc = append(idx.v6LensDesc, l)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(idx.v6LensDesc)))

	for asn, list := range idx.asnPrefixes {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Hits != list[j].Hits {
				return list[i].Hits > list[j].Hits
			}
			return list[i].CIDR < list[j].CIDR
		})
		idx.asnPrefixes[asn] = list
	}
	return count, nil
}

func (idx *bgpToolsIndex) storePrefixBucket(addr netip.Addr, bits int, payload prefixPayload) string {
	if addr.Is4() {
		if bits >= 0 && bits <= ipv4BitLen {
			if idx.v4Buckets[bits] == nil {
				idx.v4Buckets[bits] = make(map[[4]byte]prefixPayload)
			}
			idx.v4Buckets[bits][addr.As4()] = payload
		}
		return "ipv4"
	}
	if bits >= 0 && bits <= ipv6BitLen {
		bucket := idx.v6Buckets[bits]
		if bucket == nil {
			bucket = make(map[[16]byte]prefixPayload)
			idx.v6Buckets[bits] = bucket
		}
		bucket[addr.As16()] = payload
	}
	return "ipv6"
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
			if p, ok := bucket[masked]; ok {
				return netip.PrefixFrom(netip.AddrFrom4(masked), bits).String(), p.ASN, p.Hits
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
		if p, ok := bucket[masked]; ok {
			return netip.PrefixFrom(netip.AddrFrom16(masked), bits).String(), p.ASN, p.Hits
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
