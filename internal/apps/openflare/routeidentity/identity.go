// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package routeidentity resolves proxy route site names and normalized domains
// for OpenFlare control-plane and edge rendering.
package routeidentity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

// NormalizeDomains lowercases, deduplicates, and validates proxy route domains.
func NormalizeDomains(rawDomains []string) ([]string, error) {
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
		return nil, errors.New("domain is required")
	}
	return normalized, nil
}

// DecodeDomains parses stored domains JSON or falls back to a single domain value.
func DecodeDomains(raw string, fallbackDomain string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return NormalizeDomains([]string{fallbackDomain})
	}
	var domains []string
	if err := json.Unmarshal([]byte(text), &domains); err != nil {
		return nil, errors.New("domains payload is invalid")
	}
	return NormalizeDomains(domains)
}

// ResolveSiteName returns the runtime site identifier for a proxy route.
func ResolveSiteName(route *model.ProxyRoute, raw, primaryDomain string) string {
	siteName := strings.TrimSpace(raw)
	if siteName != "" {
		return siteName
	}
	if route != nil && strings.TrimSpace(route.SiteName) != "" {
		return strings.TrimSpace(route.SiteName)
	}
	return strings.TrimSpace(primaryDomain)
}

// ResolveFromRoute decodes domains and resolves the site name for a stored route.
func ResolveFromRoute(route *model.ProxyRoute) (siteName string, domains []string, err error) {
	if route == nil {
		return "", nil, errors.New("proxy route is nil")
	}
	domains, err = DecodeDomains(route.Domains, route.Domain)
	if err != nil {
		return "", nil, err
	}
	return ResolveSiteName(route, route.SiteName, domains[0]), domains, nil
}
