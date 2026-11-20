// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
)

const (
	maxOriginErrorPageHTMLBytes = 256 << 10 // 256 KiB
	maxSWOfflineDomains         = 1000
	maxTrustedProxyCIDRs        = 256
)

var edgeOptionValidators = map[string]func(key, value string) error{
	model.ConfigKeyEdgeTrustedProxyCIDRs:         validateEdgeTrustedProxyCIDRs,
	model.ConfigKeyEdgeDefaultLimitConnPerServer: validateNonNegativeIntegerOption,
	model.ConfigKeyEdgeDefaultLimitConnPerIP:     validateNonNegativeIntegerOption,
	model.ConfigKeyEdgeDefaultLimitRate:          validateEdgeDefaultLimitRate,
	model.ConfigKeyEdgeDefaultLimitReqPerIP:      validateEdgeDefaultLimitReqPerIP,
	model.ConfigKeyOriginErrorPageEnabled:        validateBooleanOption,
	model.ConfigKeyOriginErrorPageStatusCodes:    validateOriginErrorPageStatusCodes,
	model.ConfigKeyOriginErrorPageHTML:           validateOriginErrorPageHTML,
	model.ConfigKeyPoWPage:                       validatePoWPageOption,
	model.ConfigKeyOriginErrorPageGetOnly:        validateBooleanOption,
	model.ConfigKeySWOfflineEnabled:              validateBooleanOption,
	model.ConfigKeySWOfflineHTML:                 validateSWOfflineHTML,
	model.ConfigKeySWOfflineDomains:              validateSWOfflineDomains,
}

var openRestyDefaultLimitRatePattern = regexp.MustCompile(`^\d+[kKmM]?$`)

func validateEdgeOption(key, value string) error {
	// HTML 按原始字节长度校验，避免 TrimSpace 影响上限判断
	if key == model.ConfigKeyOriginErrorPageHTML || key == model.ConfigKeySWOfflineHTML || key == model.ConfigKeySiteAuthHTML {
		return validateOriginErrorPageHTML(key, value)
	}
	if model.IsRetiredEdgeOption(key) {
		return fmt.Errorf("OpenResty tuning is retired")
	}
	trimmed := strings.TrimSpace(value)
	if validator, ok := edgeOptionValidators[key]; ok {
		return validator(key, trimmed)
	}
	return nil
}

func validateEdgeTrustedProxyCIDRs(key, value string) error {
	var cidrs []string
	if err := json.Unmarshal([]byte(value), &cidrs); err != nil || cidrs == nil {
		return fmt.Errorf("%s must be a JSON array of strings", key)
	}
	if len(cidrs) > maxTrustedProxyCIDRs {
		return fmt.Errorf("%s must contain at most %d network ranges", key, maxTrustedProxyCIDRs)
	}
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil || prefix != prefix.Masked() {
			return fmt.Errorf("%s contains invalid network range %q", key, cidr)
		}
	}
	return nil
}

func validateEdgeDefaultLimitRate(key, trimmed string) error {
	if trimmed == "" || trimmed == "0" {
		return nil
	}
	if !openRestyDefaultLimitRatePattern.MatchString(strings.ToLower(trimmed)) {
		return fmt.Errorf("invalid %s format; use 512k, 1m or a number, or leave empty to disable", key)
	}
	return nil
}

var openRestyDefaultLimitReqPerIPPattern = regexp.MustCompile(`^\d+r/[sm]$`)

func validateEdgeDefaultLimitReqPerIP(key, trimmed string) error {
	if trimmed == "" || trimmed == "0" {
		return nil
	}
	if !openRestyDefaultLimitReqPerIPPattern.MatchString(strings.ToLower(trimmed)) {
		return fmt.Errorf("invalid %s format; use a rate such as 10r/s or 100r/m, or leave empty to disable", key)
	}
	return nil
}

func validateOriginErrorPageStatusCodes(key, trimmed string) error {
	if trimmed == "" {
		return fmt.Errorf("%s must not be empty", key)
	}
	var tags []string
	if err := json.Unmarshal([]byte(trimmed), &tags); err != nil {
		return fmt.Errorf("%s must be a JSON array of strings", key)
	}
	if len(tags) == 0 {
		return fmt.Errorf("%s must contain at least one status code tag", key)
	}
	codes, err := edgeconfig.ExpandStatusCodeTags(tags)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if len(codes) == 0 {
		return fmt.Errorf("%s must not be empty after expansion", key)
	}
	return nil
}

func validateOriginErrorPageHTML(key, value string) error {
	if len(value) > maxOriginErrorPageHTMLBytes {
		return fmt.Errorf("%s must not exceed %d bytes (256 KiB)", key, maxOriginErrorPageHTMLBytes)
	}
	return nil
}

func validatePoWPageOption(key, value string) error {
	var cfg edgeconfig.PoWPageConfig
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return fmt.Errorf("%s must be a valid PoW page configuration: %w", key, err)
	}
	if !json.Valid([]byte(value)) || strings.TrimSpace(value) == "null" {
		return fmt.Errorf("%s must be a JSON object", key)
	}
	return edgeconfig.ValidatePoWPageConfig(cfg)
}

func validateSWOfflineHTML(key, value string) error {
	return validateOriginErrorPageHTML(key, value)
}

func validateSWOfflineDomains(key, value string) error {
	var domains []string
	if err := json.Unmarshal([]byte(value), &domains); err != nil || domains == nil {
		return fmt.Errorf("%s must be a JSON array of strings", key)
	}
	if len(domains) > maxSWOfflineDomains {
		return fmt.Errorf("%s supports at most %d domain names", key, maxSWOfflineDomains)
	}
	seen := make(map[string]struct{}, len(domains))
	for _, raw := range domains {
		domain := strings.ToLower(strings.TrimSpace(raw))
		if domain == "" {
			return fmt.Errorf("%s contains an empty domain name", key)
		}
		if raw != domain {
			return fmt.Errorf("%s domain name must be lowercase without surrounding whitespace: %s", key, raw)
		}
		if _, ok := seen[domain]; ok {
			return fmt.Errorf("%s contains duplicate domain name %s", key, domain)
		}
		seen[domain] = struct{}{}
	}
	return nil
}
