// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"strings"
)

const (
	// MaxPrefixSketchHashes bounds the sorted bottom-k hash sketch per bucket/sample.
	MaxPrefixSketchHashes = 64

	prefixSketchPrefix      = "v1:"
	prefixSketchTag         = "of-nq-prefix-v1:"
	prefixSketchHeaderBytes = 2
	prefixSketchHashBytes   = 8
	ipv4PrefixMaskBits      = 24
	ipv4PrefixBytes         = 3
	ipv6PrefixMaskBits      = 48
	ipv6PrefixBytes         = 6
	fnv64Offset             = uint64(14695981039346656037)
	fnv64Prime              = uint64(1099511628211)
)

var errInvalidPrefixSketch = errors.New("invalid prefix sketch encoding")

// PrefixSketch is a bounded, mergeable bottom-k hash set of masked client IP prefixes
// (IPv4 /24, IPv6 /48) used to compute deduplicated prefix coverage across Rollup buckets.
type PrefixSketch struct {
	Hashes    []uint64
	Saturated bool
}

// AddIP hashes rawIP's /24 (IPv4) or /48 (IPv6) prefix and inserts it into the sketch.
func (s *PrefixSketch) AddIP(rawIP string) bool {
	if s == nil {
		return false
	}
	h, ok := HashIPPrefix(rawIP)
	if !ok {
		return false
	}
	return s.AddHash(h)
}

// AddHash inserts a precomputed 64-bit prefix hash into the bounded bottom-k set.
func (s *PrefixSketch) AddHash(h uint64) bool {
	if s == nil {
		return false
	}
	idx, exists := slices.BinarySearch(s.Hashes, h)
	if exists {
		return false
	}
	if len(s.Hashes) < MaxPrefixSketchHashes {
		s.Hashes = slices.Insert(s.Hashes, idx, h)
		return true
	}
	s.Saturated = true
	if h >= s.Hashes[len(s.Hashes)-1] {
		return false
	}
	s.Hashes = slices.Insert(s.Hashes, idx, h)
	s.Hashes = s.Hashes[:MaxPrefixSketchHashes]
	return true
}

// Union merges another PrefixSketch into s while preserving deduplication and the capacity bound.
func (s *PrefixSketch) Union(other *PrefixSketch) {
	if s == nil || other == nil {
		return
	}
	if other.Saturated {
		s.Saturated = true
	}
	if len(other.Hashes) == 0 {
		return
	}
	for _, h := range other.Hashes {
		s.AddHash(h)
	}
}

// DistinctCount returns the deduplicated prefix count observed in the sketch (capped at 64 when saturated).
func (s *PrefixSketch) DistinctCount() int {
	if s == nil {
		return 0
	}
	return len(s.Hashes)
}

// Clone returns an independent copy of the PrefixSketch.
func (s *PrefixSketch) Clone() *PrefixSketch {
	if s == nil {
		return nil
	}
	out := &PrefixSketch{
		Saturated: s.Saturated,
	}
	if len(s.Hashes) > 0 {
		out.Hashes = make([]uint64, len(s.Hashes))
		copy(out.Hashes, s.Hashes)
	}
	return out
}

// Encode serializes the sketch into a compact versioned base64 string.
func (s *PrefixSketch) Encode() string {
	if s == nil || (len(s.Hashes) == 0 && !s.Saturated) {
		return ""
	}
	count := min(len(s.Hashes), MaxPrefixSketchHashes)
	buf := make([]byte, prefixSketchHeaderBytes+count*prefixSketchHashBytes)
	if s.Saturated {
		buf[0] = 1
	}
	buf[1] = byte(count) //nolint:gosec // bounded by MaxPrefixSketchHashes (64)
	for i := 0; i < count; i++ {
		binary.BigEndian.PutUint64(buf[prefixSketchHeaderBytes+i*prefixSketchHashBytes:], s.Hashes[i])
	}
	return prefixSketchPrefix + base64.RawStdEncoding.EncodeToString(buf)
}

// DecodePrefixSketch deserializes a PrefixSketch string.
func DecodePrefixSketch(encoded string) (*PrefixSketch, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return &PrefixSketch{}, nil
	}
	if !strings.HasPrefix(trimmed, prefixSketchPrefix) {
		return nil, errInvalidPrefixSketch
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(trimmed, prefixSketchPrefix))
	if err != nil || len(raw) < prefixSketchHeaderBytes {
		return nil, errInvalidPrefixSketch
	}
	saturated := raw[0] != 0
	count := int(raw[1])
	if count > MaxPrefixSketchHashes || len(raw) != prefixSketchHeaderBytes+count*prefixSketchHashBytes {
		return nil, errInvalidPrefixSketch
	}
	sk := &PrefixSketch{
		Hashes:    make([]uint64, 0, count),
		Saturated: saturated,
	}
	for i := 0; i < count; i++ {
		sk.AddHash(binary.BigEndian.Uint64(raw[prefixSketchHeaderBytes+i*prefixSketchHashBytes:]))
	}
	return sk, nil
}

// HashIPPrefix returns a privacy-preserving 64-bit deterministic hash of the IP's /24 (IPv4) or /48 (IPv6) prefix.
func HashIPPrefix(rawIP string) (uint64, bool) {
	trimmed := strings.TrimSpace(rawIP)
	if trimmed == "" {
		return 0, false
	}
	var addr netip.Addr
	if ap, err := netip.ParseAddrPort(trimmed); err == nil {
		addr = ap.Addr()
	} else {
		trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
		parsed, err := netip.ParseAddr(trimmed)
		if err != nil {
			return 0, false
		}
		addr = parsed
	}
	addr = addr.Unmap()
	if !addr.IsValid() {
		return 0, false
	}

	h := fnv64Offset
	for i := 0; i < len(prefixSketchTag); i++ {
		h ^= uint64(prefixSketchTag[i])
		h *= fnv64Prime
	}

	if addr.Is4() {
		p := netip.PrefixFrom(addr, ipv4PrefixMaskBits).Masked()
		b := p.Addr().As4()
		h ^= 4
		h *= fnv64Prime
		for i := 0; i < ipv4PrefixBytes; i++ {
			h ^= uint64(b[i])
			h *= fnv64Prime
		}
	} else {
		p := netip.PrefixFrom(addr, ipv6PrefixMaskBits).Masked()
		b := p.Addr().As16()
		h ^= 6
		h *= fnv64Prime
		for i := 0; i < ipv6PrefixBytes; i++ {
			h ^= uint64(b[i])
			h *= fnv64Prime
		}
	}

	// SplitMix64 avalanche finalizer for uniform bottom-k hash spacing.
	h ^= h >> 30
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 27
	h *= 0x94d049bb133111eb
	h ^= h >> 31
	return h, true
}
