// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	schemeHTTP          = "http"
	schemeHTTPS         = "https"
	schemeSeparator     = "://"
	maxTargetLength     = 253
	maxHostnameLabelLen = 63
	maxPortNumber       = 65535
	hostResolveTimeout  = 2 * time.Second
	defaultDNSResolver  = "1.1.1.1:53"
	defaultDNSPortStr   = "53"
	ipv4ECSMaskMin      = 8
	ipv4ECSMaskMax      = 32
	ipv4ECSPromoteMask  = 24
	ipv6ECSMaskMin      = 24
	ipv6ECSMaskMax      = 128
	ipv6ECSPromoteMask  = 56
)

var (
	cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")
	// 198.18.0.0/15 is reserved benchmarking space and often non-routable internal lab traffic.
	benchmarkPrefix = netip.MustParsePrefix("198.18.0.0/15")
)

var allowedDNSRecordTypes = map[string]struct{}{
	"A":     {},
	"AAAA":  {},
	"CNAME": {},
	"MX":    {},
	"TXT":   {},
	"NS":    {},
	"SOA":   {},
	"PTR":   {},
	"SRV":   {},
	"CAA":   {},
	"HTTPS": {},
	"SVCB":  {},
	"ANY":   {},
}

func isBlockedAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	addr = addr.Unmap()
	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return true
	}
	if addr.Is4() && (cgnatPrefix.Contains(addr) || benchmarkPrefix.Contains(addr)) {
		return true
	}
	return false
}

func isInternalHostname(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if h == "" || h == "localhost" {
		return true
	}
	internalSuffixes := []string{
		".localhost",
		".local",
		".internal",
		".localdomain",
		".home.arpa",
	}
	for _, suffix := range internalSuffixes {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}

func isValidHostnameChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' ||
		c == '_'
}

func isValidHostnameSyntax(host string) bool {
	trimmed := strings.TrimSuffix(strings.TrimSpace(host), ".")
	if len(trimmed) == 0 || len(trimmed) > maxTargetLength {
		return false
	}
	if strings.ContainsAny(trimmed, " \t\r\n/\\@?#[]") {
		return false
	}
	labels := strings.Split(trimmed, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > maxHostnameLabelLen {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			if !isValidHostnameChar(label[i]) {
				return false
			}
		}
	}
	return true
}

func extractProbeHost(trimmed string, allowURL bool) (string, error) {
	if strings.Contains(trimmed, schemeSeparator) {
		if !allowURL {
			return "", errors.New(errTargetInvalid)
		}
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Host == "" {
			return "", errors.New(errTargetInvalid)
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != schemeHTTP && scheme != schemeHTTPS {
			return "", errors.New(errTargetInvalid)
		}
		return stripBrackets(parsed.Hostname()), nil
	}

	host := trimmed
	if strings.Count(trimmed, ":") == 1 && !strings.HasPrefix(trimmed, "[") {
		if h, p, err := net.SplitHostPort(trimmed); err == nil {
			if portNum, pErr := strconv.Atoi(p); pErr == nil && portNum >= 1 && portNum <= maxPortNumber {
				host = h
			}
		}
	}
	return stripBrackets(host), nil
}

func stripBrackets(host string) string {
	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(host), "]"), "[")
}

func rejectResolvedPrivateHost(ctx context.Context, host string, blockedErrMsg string) error {
	resolveCtx, cancel := context.WithTimeout(ctx, hostResolveTimeout)
	defer cancel()
	ips, err := net.DefaultResolver.LookupNetIP(resolveCtx, "ip", host)
	if err != nil {
		return nil //nolint:nilerr // allow unresolved public hostname for edge-side DNS
	}
	for _, ip := range ips {
		if isBlockedAddr(ip) {
			return errors.New(blockedErrMsg)
		}
	}
	return nil
}

// ValidateProbeTarget checks target syntax and protects against SSRF toward internal or private networks.
// Returns the normalized target passed to probes and the extracted bare host/IP.
func ValidateProbeTarget(ctx context.Context, raw string, allowURL bool, allowPrivate bool) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", errors.New(errTargetRequired)
	}
	if len(trimmed) > maxTargetLength || strings.ContainsAny(trimmed, " \t\r\n") {
		return "", "", errors.New(errTargetInvalid)
	}

	host, err := extractProbeHost(trimmed, allowURL)
	if err != nil || host == "" {
		return "", "", errors.New(errTargetInvalid)
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		if !allowPrivate && isBlockedAddr(addr) {
			return "", "", errors.New(errTargetPrivateBlocked)
		}
		return trimmed, addr.Unmap().String(), nil
	}

	if !isValidHostnameSyntax(host) {
		return "", "", errors.New(errTargetInvalid)
	}
	if !allowPrivate && isInternalHostname(host) {
		return "", "", errors.New(errTargetPrivateBlocked)
	}
	if !allowPrivate {
		if err := rejectResolvedPrivateHost(ctx, host, errTargetPrivateBlocked); err != nil {
			return "", "", err
		}
	}
	return trimmed, host, nil
}

// ValidateResolver validates and normalizes a DNS resolver address ("host:port").
func ValidateResolver(ctx context.Context, raw string, allowPrivate bool) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultDNSResolver, nil
	}
	if strings.ContainsAny(trimmed, " \t\r\n/\\") {
		return "", errors.New(errResolverInvalid)
	}

	host := trimmed
	port := defaultDNSPortStr
	if addr, err := netip.ParseAddr(trimmed); err == nil {
		host = addr.Unmap().String()
	} else if h, p, err := net.SplitHostPort(trimmed); err == nil {
		host = h
		port = p
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > maxPortNumber {
		return "", errors.New(errResolverInvalid)
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		if !allowPrivate && isBlockedAddr(addr) {
			return "", errors.New(errResolverPrivate)
		}
		return net.JoinHostPort(addr.Unmap().String(), strconv.Itoa(portNum)), nil
	}
	if !isValidHostnameSyntax(host) {
		return "", errors.New(errResolverInvalid)
	}
	if !allowPrivate && isInternalHostname(host) {
		return "", errors.New(errResolverPrivate)
	}
	if !allowPrivate {
		if err := rejectResolvedPrivateHost(ctx, host, errResolverPrivate); err != nil {
			return "", err
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(portNum)), nil
}

// ValidateECSSubnet validates an optional EDNS0 Client Subnet CIDR prefix.
func ValidateECSSubnet(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	// Bare IP automatically promotes to /24 (IPv4) or /56 (IPv6).
	if addr, err := netip.ParseAddr(trimmed); err == nil {
		addr = addr.Unmap()
		if isBlockedAddr(addr) {
			return "", errors.New(errECSSubnetInvalid)
		}
		if addr.Is4() {
			return netip.PrefixFrom(addr, ipv4ECSPromoteMask).Masked().String(), nil
		}
		return netip.PrefixFrom(addr, ipv6ECSPromoteMask).Masked().String(), nil
	}

	prefix, err := netip.ParsePrefix(trimmed)
	if err != nil {
		return "", errors.New(errECSSubnetInvalid)
	}
	masked := prefix.Masked()
	if isBlockedAddr(masked.Addr()) {
		return "", errors.New(errECSSubnetInvalid)
	}
	if masked.Addr().Is4() && (masked.Bits() < ipv4ECSMaskMin || masked.Bits() > ipv4ECSMaskMax) {
		return "", errors.New(errECSSubnetInvalid)
	}
	if masked.Addr().Is6() && (masked.Bits() < ipv6ECSMaskMin || masked.Bits() > ipv6ECSMaskMax) {
		return "", errors.New(errECSSubnetInvalid)
	}
	return masked.String(), nil
}

// ValidateDNSRecordType validates and normalizes a DNS query RR type.
func ValidateDNSRecordType(raw string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	if upper == "" {
		return "A", nil
	}
	if _, ok := allowedDNSRecordTypes[upper]; !ok {
		return "", errors.New(errDNSRecordTypeInvalid)
	}
	return upper, nil
}
