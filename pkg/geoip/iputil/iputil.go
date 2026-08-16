// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package iputil provides helpers for parsing, normalizing, and scoring IP addresses.
package iputil

import (
	"net"
	"strings"
)

const (
	scorePublic  = 2
	scorePrivate = 1
)

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
