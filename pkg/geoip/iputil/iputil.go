// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package iputil provides helpers for parsing, normalizing, and scoring IP addresses.
package iputil

import (
	"net"
	"net/netip"
	"strings"
)

const (
	scorePublic  = 2
	scorePrivate = 1
)

var (
	nonPublicIPv4Prefixes = mustParsePrefixes(
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.88.99.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
	)
	ipv6GlobalUnicastPrefix = netip.MustParsePrefix("2000::/3")
	nonPublicIPv6Prefixes   = mustParsePrefixes(
		"::/128",
		"::1/128",
		"::/96",
		"::ffff:0:0/96",
		"64:ff9b::/96",
		"64:ff9b:1::/48",
		"100::/64",
		"2001::/32",
		"2001:2::/48",
		"2001:3::/32",
		"2001:4:112::/48",
		"2001:10::/28",
		"2001:20::/28",
		"2001:db8::/32",
		"2002::/16",
		"3fff::/20",
		"fc00::/7",
		"fe80::/10",
		"fec0::/10",
		"ff00::/8",
	)
)

func mustParsePrefixes(cidrs ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefixes = append(prefixes, netip.MustParsePrefix(cidr))
	}
	return prefixes
}

// NormalizeIP parses and normalizes a raw IP address string, preferring the IPv4 form for mapped addresses.
func NormalizeIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	ip := net.ParseIP(trimmed)
	if ip == nil {
		return ""
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return ip.String()
}

// NormalizeRemoteAddr extracts and normalizes the IP address from a host:port remote address string.
func NormalizeRemoteAddr(remoteAddr string) string {
	trimmed := strings.TrimSpace(remoteAddr)
	if trimmed == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return NormalizeIP(host)
	}
	return NormalizeIP(trimmed)
}

// IsPublic reports whether the given IP address is a publicly routable unicast address.
func IsPublic(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

// IsPublicString parses raw and reports whether it represents a publicly routable IP address.
func IsPublicString(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	return IsPublic(ip)
}

// IsPublicRoutable reports whether ip is globally routable, excluding all private, internal, and reserved CIDRs.
func IsPublicRoutable(ip net.IP) bool {
	if ip == nil {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsValid() {
		return false
	}
	if addr.Is4() {
		for _, prefix := range nonPublicIPv4Prefixes {
			if prefix.Contains(addr) {
				return false
			}
		}
		return true
	}
	if !ipv6GlobalUnicastPrefix.Contains(addr) {
		return false
	}
	for _, prefix := range nonPublicIPv6Prefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// IsPublicRoutableString parses raw and reports whether it is globally routable outside all private and reserved CIDRs.
func IsPublicRoutableString(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	return IsPublicRoutable(ip)
}

// Score returns a preference score for the IP address: 2 for public, 1 for private, -1 for invalid or non-unicast.
func Score(ip net.IP) int {
	if ip == nil {
		return -1
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
		return -1
	}
	if IsPublic(ip) {
		return scorePublic
	}
	return scorePrivate
}
