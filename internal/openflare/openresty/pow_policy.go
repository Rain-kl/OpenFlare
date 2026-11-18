// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

const maxPoWListEntries = 100

// RenderPoWConfig publishes standalone policies by site identity, independently of WAF.
func RenderPoWConfig(routes []Route) (string, error) {
	type sitePolicy struct {
		Enabled bool       `json:"enabled"`
		Config  *PoWConfig `json:"config"`
	}
	sites := make(map[string]sitePolicy)
	for _, route := range routes {
		if route.PoWConfig == nil {
			continue
		}
		if err := ValidatePoWConfig(*route.PoWConfig); err != nil {
			return "", fmt.Errorf("route %d: %w", route.ID, err)
		}
		sites[resolveRouteSiteName(route)] = sitePolicy{Enabled: route.PoWEnabled, Config: route.PoWConfig}
	}
	raw, err := json.Marshal(sites)
	return string(raw), err
}

// ValidatePoWConfig validates a standalone site challenge policy.
func ValidatePoWConfig(cfg PoWConfig) error {
	if cfg.Algorithm != "fast" && cfg.Algorithm != "slow" {
		return fmt.Errorf("PoW algorithm must be fast or slow")
	}
	if cfg.Difficulty < 1 || cfg.Difficulty > 16 {
		return fmt.Errorf("PoW difficulty must be between 1 and 16")
	}
	if cfg.SessionTTL < 60 || cfg.SessionTTL > 604800 {
		return fmt.Errorf("PoW session TTL must be between 60 and 604800 seconds")
	}
	if cfg.ChallengeTTL < 30 || cfg.ChallengeTTL > 3600 {
		return fmt.Errorf("PoW challenge TTL must be between 30 and 3600 seconds")
	}
	for _, list := range []PoWListConfig{cfg.Whitelist, cfg.Blacklist} {
		if len(list.IPs)+len(list.IPCidrs)+len(list.Paths)+len(list.PathRegexes)+len(list.UserAgents) > maxPoWListEntries {
			return fmt.Errorf("PoW list must not exceed 100 entries")
		}
		for _, ip := range list.IPs {
			if _, err := netip.ParseAddr(ip); err != nil {
				return fmt.Errorf("invalid PoW IP: %w", err)
			}
		}
		for _, cidr := range list.IPCidrs {
			if _, err := netip.ParsePrefix(cidr); err != nil {
				return fmt.Errorf("invalid PoW CIDR: %w", err)
			}
		}
		for _, pattern := range list.PathRegexes {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid PoW regex: %w", err)
			}
		}
	}
	return validatePoWPathRules(cfg.PathRules)
}

func validatePoWPathRules(rules *PoWPathRules) error {
	if rules == nil {
		return nil
	}
	if rules.Mode != "all" && rules.Mode != "blacklist" {
		return fmt.Errorf("PoW path mode for policy must be all or blacklist")
	}
	if rules.Mode == "blacklist" && len(rules.Blacklist) == 0 {
		return fmt.Errorf("PoW policy requires at least one blacklist path")
	}
	for _, list := range [][]PoWPathMatch{rules.Whitelist, rules.Blacklist} {
		if len(list) > maxPoWListEntries {
			return fmt.Errorf("PoW policy must not exceed 100 paths per list")
		}
		for _, rule := range list {
			if rule.Value == "" || len(rule.Value) > 1024 || strings.ContainsAny(rule.Value, "\r\n\x00") {
				return fmt.Errorf("PoW policy contains an invalid path value")
			}
			switch rule.Type {
			case "exact", "prefix", "wildcard":
				if !strings.HasPrefix(rule.Value, "/") {
					return fmt.Errorf("PoW path in policy must start with /")
				}
			case "regex":
				if _, err := regexp.Compile(rule.Value); err != nil {
					return fmt.Errorf("invalid PoW path regex in policy: %w", err)
				}
			default:
				return fmt.Errorf("PoW policy contains an invalid path match type")
			}
		}
	}
	return nil
}
