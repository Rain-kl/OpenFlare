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

// DecodeDomains parses legacy route domain fields for the goose upgrade importer.
// Runtime consumers must read ZoneDomain bindings instead.
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
