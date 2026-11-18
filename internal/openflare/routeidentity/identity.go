// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package routeidentity resolves proxy route site names and normalized domains
// for OpenFlare control-plane, migration, and edge rendering.
package routeidentity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/publicsuffix"
)

const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

const (
	errZoneDomainRequired = "domain name must not be empty"
	errDomainInvalid      = "invalid domain name format"
)

// ZoneRoot extracts the effective TLD+1 registered root domain for a hostname or wildcard.
func ZoneRoot(domain string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	return publicsuffix.EffectiveTLDPlusOne(strings.TrimPrefix(domain, "*."))
}

// NormalizeDomain validates and lowercases an ASCII hostname or single left-most wildcard domain.
func NormalizeDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(raw))
	if domain == "" {
		return "", errors.New(errZoneDomainRequired)
	}
	// Only allow a single wildcard label at the left-most position; remaining labels must be valid DNS labels.
	host := strings.TrimPrefix(domain, "*.")
	if len(domain) > maxDomainLength {
		return "", errors.New(errDomainInvalid)
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > maxLabelLength || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New(errDomainInvalid)
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return "", errors.New(errDomainInvalid)
			}
		}
	}
	if _, err := ZoneRoot(domain); err != nil {
		return "", errors.New(errDomainInvalid)
	}
	return domain, nil
}

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
