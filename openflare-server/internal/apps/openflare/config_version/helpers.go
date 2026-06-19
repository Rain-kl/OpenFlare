// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

type customHeaderInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func normalizeProxyRouteSiteName(route *model.ProxyRoute, raw, primaryDomain string) string {
	siteName := strings.TrimSpace(raw)
	if siteName != "" {
		return siteName
	}
	if route != nil && strings.TrimSpace(route.SiteName) != "" {
		return strings.TrimSpace(route.SiteName)
	}
	return primaryDomain
}

func normalizeProxyRouteDomains(rawDomains []string) ([]string, error) {
	normalized := make([]string, 0, len(rawDomains))
	seen := make(map[string]struct{}, len(rawDomains))
	for _, rawDomain := range rawDomains {
		domain := strings.ToLower(strings.TrimSpace(rawDomain))
		if domain == "" {
			continue
		}
		if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
			return nil, fmt.Errorf("domain %q is invalid", rawDomain)
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		normalized = append(normalized, domain)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("domain is required")
	}
	return normalized, nil
}

func decodeStoredDomains(raw string, fallbackDomain string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return normalizeProxyRouteDomains([]string{fallbackDomain})
	}
	var domains []string
	if err := json.Unmarshal([]byte(text), &domains); err != nil {
		return nil, fmt.Errorf("domains payload is invalid")
	}
	return normalizeProxyRouteDomains(domains)
}

func decodeStoredUpstreams(raw string, fallbackOriginURL string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return normalizeUpstreams(fallbackOriginURL, nil)
	}
	var upstreams []string
	if err := json.Unmarshal([]byte(text), &upstreams); err != nil {
		return nil, fmt.Errorf("upstreams payload is invalid")
	}
	return normalizeUpstreams(fallbackOriginURL, upstreams)
}

func normalizeUpstreams(originURL string, upstreams []string) ([]string, error) {
	candidates := upstreams
	if len(candidates) == 0 {
		candidates = []string{originURL}
	}
	normalized := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, item := range candidates {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("upstream is required")
	}
	return normalized, nil
}

func decodeStoredCustomHeaders(raw string) ([]customHeaderInput, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []customHeaderInput{}, nil
	}
	var headers []customHeaderInput
	if err := json.Unmarshal([]byte(text), &headers); err != nil {
		return nil, fmt.Errorf("custom_headers payload is invalid")
	}
	return headers, nil
}

func decodeStoredCacheRules(raw string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []string{}, nil
	}
	var rules []string
	if err := json.Unmarshal([]byte(text), &rules); err != nil {
		return nil, fmt.Errorf("cache_rules payload is invalid")
	}
	normalized := make([]string, 0, len(rules))
	for _, rule := range rules {
		item := strings.TrimSpace(rule)
		if item == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func decodeStoredCertIDs(raw string, fallbackCertID *uint) ([]uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		if fallbackCertID == nil || *fallbackCertID == 0 {
			return []uint{}, nil
		}
		return []uint{*fallbackCertID}, nil
	}
	var certIDs []uint
	if err := json.Unmarshal([]byte(text), &certIDs); err != nil {
		return nil, fmt.Errorf("cert_ids payload is invalid")
	}
	normalized := make([]uint, 0, len(certIDs))
	seen := make(map[uint]struct{}, len(certIDs))
	for _, certID := range certIDs {
		if certID == 0 {
			continue
		}
		if _, ok := seen[certID]; ok {
			continue
		}
		seen[certID] = struct{}{}
		normalized = append(normalized, certID)
	}
	if len(normalized) == 0 && fallbackCertID != nil && *fallbackCertID != 0 {
		return []uint{*fallbackCertID}, nil
	}
	return normalized, nil
}

func resolveDomainCertIDs(domains []string, certIDs []uint, rawDomainCertIDs string) ([]uint, error) {
	text := strings.TrimSpace(rawDomainCertIDs)
	if text != "" {
		var domainCertIDs []uint
		if err := json.Unmarshal([]byte(text), &domainCertIDs); err != nil {
			return nil, fmt.Errorf("domain_cert_ids payload is invalid")
		}
		if len(domains) > 0 && len(domainCertIDs) != len(domains) {
			return nil, fmt.Errorf("domain_cert_ids length is invalid")
		}
		return domainCertIDs, nil
	}
	if len(certIDs) == 0 {
		return []uint{}, nil
	}
	if len(certIDs) == 1 {
		result := make([]uint, len(domains))
		for index := range result {
			result[index] = certIDs[0]
		}
		return result, nil
	}
	if len(certIDs) == len(domains) {
		result := make([]uint, len(certIDs))
		copy(result, certIDs)
		return result, nil
	}
	return []uint{}, nil
}

func mustDecodeCertIDs(route *model.ProxyRoute) []uint {
	if route == nil {
		return []uint{}
	}
	certIDs, err := decodeStoredCertIDs(route.CertIDs, route.CertID)
	if err != nil {
		return []uint{}
	}
	return certIDs
}

func mustDecodeDomainCertIDs(route *model.ProxyRoute, domains []string) []uint {
	if route == nil {
		return []uint{}
	}
	certIDs, err := decodeStoredCertIDs(route.CertIDs, route.CertID)
	if err != nil {
		return []uint{}
	}
	domainCertIDs, err := resolveDomainCertIDs(domains, certIDs, route.DomainCertIDs)
	if err != nil {
		return []uint{}
	}
	return domainCertIDs
}

func normalizeUpstreamType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "tunnel", "pages":
		return value
	default:
		return "direct"
	}
}

func normalizeTunnelTargetProtocol(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "http", "https", "tcp":
		return value
	default:
		return "http"
	}
}

func normalizePEM(content string) string {
	return strings.TrimSpace(content) + "\n"
}

func certificateCertFileName(id uint) string {
	return fmt.Sprintf("%d.crt", id)
}

func certificateKeyFileName(id uint) string {
	return fmt.Sprintf("%d.key", id)
}

func dedupeSupportFiles(files []SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	unique := make(map[string]SupportFile, len(files))
	for _, file := range files {
		unique[file.Path] = file
	}
	result := make([]SupportFile, 0, len(unique))
	for _, file := range unique {
		result = append(result, file)
	}
	return result
}

func uintPtrEqual(left *uint, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func uintSliceEqual(left []uint, right []uint) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func relayAgentAddress(node *model.OpenFlareNode) string {
	if node == nil {
		return ""
	}
	port := node.RelayVhostHTTPPort
	if port <= 0 {
		port = 8080
	}
	addr := strings.TrimSpace(node.RelayAgentAccessAddr)
	if addr == "" {
		addr = strings.TrimSpace(node.RelayClientAccessAddr)
	}
	if addr == "" {
		addr = strings.TrimSpace(node.IP)
	}
	if addr == "" {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	if strings.Contains(addr, ":") && strings.Count(addr, ":") > 1 {
		return net.JoinHostPort(addr, strconv.Itoa(port))
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func resolveTunnelOpenRestyUpstreamURL(ctx context.Context) string {
	nodes, err := model.ListOpenFlareNodes(ctx)
	if err == nil {
		for index := range nodes {
			node := &nodes[index]
			if node.NodeType != "tunnel_relay" {
				continue
			}
			addr := relayAgentAddress(node)
			if addr != "" {
				return "http://" + addr
			}
		}
	}
	return "http://127.0.0.1:8080"
}

func listWAFIPGroupsByIDs(ctx context.Context, ids []uint) ([]*model.OpenFlareWAFIPGroup, error) {
	if len(ids) == 0 {
		return []*model.OpenFlareWAFIPGroup{}, nil
	}
	groups := make([]*model.OpenFlareWAFIPGroup, 0, len(ids))
	for _, id := range ids {
		group, err := model.GetOpenFlareWAFIPGroupByID(ctx, id)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, nil
}
