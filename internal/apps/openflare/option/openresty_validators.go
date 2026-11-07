// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
)

const (
	maxOriginErrorPageHTMLBytes = 256 << 10 // 256 KiB
	maxSWOfflineDomains         = 1000
	maxTrustedProxyCIDRs        = 256
)

var openRestyOptionValidators = map[string]func(key, value string) error{
	model.ConfigKeyOpenRestyDefaultServerReturnStatus:    validateOpenRestyDefaultServerReturnStatus,
	model.ConfigKeyOpenRestyWorkerProcesses:              validateOpenRestyWorkerProcesses,
	model.ConfigKeyOpenRestyWorkerConnections:            validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyWorkerRlimitNofile:           validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyKeepaliveTimeout:             validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyKeepaliveRequests:            validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyClientHeaderTimeout:          validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyClientBodyTimeout:            validatePositiveIntegerOption,
	model.ConfigKeyOpenRestySendTimeout:                  validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyProxyConnectTimeout:          validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyProxySendTimeout:             validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyProxyReadTimeout:             validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyGzipMinLength:                validatePositiveIntegerOption,
	model.ConfigKeyOpenRestyGzipCompLevel:                validateOpenRestyGzipCompLevel,
	model.ConfigKeyOpenRestyEventsUse:                    validateOpenRestyEventsUse,
	model.ConfigKeyOpenRestyResolvers:                    validateOpenRestyResolvers,
	model.ConfigKeyOpenRestyEventsMultiAcceptEnabled:     validateBooleanOption,
	model.ConfigKeyOpenRestyWebsocketEnabled:             validateBooleanOption,
	model.ConfigKeyOpenRestyHTTP3Enabled:                 validateBooleanOption,
	model.ConfigKeyOpenRestyProxyRequestBufferingEnabled: validateBooleanOption,
	model.ConfigKeyOpenRestyProxyBufferingEnabled:        validateBooleanOption,
	model.ConfigKeyOpenRestyGzipEnabled:                  validateBooleanOption,
	model.ConfigKeyOpenRestyCacheEnabled:                 validateBooleanOption,
	model.ConfigKeyOpenRestyCacheLockEnabled:             validateBooleanOption,
	model.ConfigKeyOpenRestyProxyBuffers:                 validateOpenRestyProxyBuffers,
	model.ConfigKeyOpenRestyLargeClientHeaderBuffers:     validateOpenRestyProxyBuffers,
	model.ConfigKeyOpenRestyProxyBufferSize:              validateOpenRestySizeValue,
	model.ConfigKeyOpenRestyProxyBusyBuffersSize:         validateOpenRestySizeValue,
	model.ConfigKeyOpenRestyCacheMaxSize:                 validateOpenRestySizeValue,
	model.ConfigKeyOpenRestyClientMaxBodySize:            validateOpenRestySizeValue,
	model.ConfigKeyOpenRestyCachePath:                    validateOpenRestyCachePath,
	model.ConfigKeyOpenRestyCacheLevels:                  validateOpenRestyCacheLevels,
	model.ConfigKeyOpenRestyCacheInactive:                validateOpenRestyDurationToken,
	model.ConfigKeyOpenRestyCacheLockTimeout:             validateOpenRestyDurationToken,
	model.ConfigKeyOpenRestyCacheKeyTemplate:             validateOpenRestyCacheKeyTemplate,
	model.ConfigKeyOpenRestyCacheUseStale:                validateOpenRestyCacheUseStale,
	model.ConfigKeyOpenRestyMainConfigTemplate:           validateOpenRestyMainConfigTemplate,
	model.ConfigKeyOpenRestyTrustedProxyCIDRs:            validateOpenRestyTrustedProxyCIDRs,
	model.ConfigKeyOpenRestyDefaultLimitConnPerServer:    validateNonNegativeIntegerOption,
	model.ConfigKeyOpenRestyDefaultLimitConnPerIP:        validateNonNegativeIntegerOption,
	model.ConfigKeyOpenRestyDefaultLimitRate:             validateOpenRestyDefaultLimitRate,
	model.ConfigKeyOpenRestyDefaultLimitReqPerIP:         validateOpenRestyDefaultLimitReqPerIP,
	model.ConfigKeyOriginErrorPageEnabled:                validateBooleanOption,
	model.ConfigKeyOriginErrorPageStatusCodes:            validateOriginErrorPageStatusCodes,
	model.ConfigKeyOriginErrorPageHTML:                   validateOriginErrorPageHTML,
	model.ConfigKeyOriginErrorPageGetOnly:                validateBooleanOption,
	model.ConfigKeySWOfflineEnabled:                      validateBooleanOption,
	model.ConfigKeySWOfflineHTML:                         validateSWOfflineHTML,
	model.ConfigKeySWOfflineDomains:                      validateSWOfflineDomains,
}

var openRestyDefaultLimitRatePattern = regexp.MustCompile(`^\d+[kKmM]?$`)

func validateOpenRestyOption(key, value string) error {
	// HTML 按原始字节长度校验，避免 TrimSpace 影响上限判断
	if key == model.ConfigKeyOriginErrorPageHTML || key == model.ConfigKeySWOfflineHTML {
		return validateOriginErrorPageHTML(key, value)
	}
	trimmed := strings.TrimSpace(value)
	if validator, ok := openRestyOptionValidators[key]; ok {
		return validator(key, trimmed)
	}
	return nil
}

func validateOpenRestyDefaultServerReturnStatus(key, trimmed string) error {
	if err := validatePositiveIntegerOption(key, trimmed); err != nil {
		return err
	}
	statusCode, _ := strconv.Atoi(trimmed)
	if statusCode < 100 || statusCode > 999 {
		return fmt.Errorf("%s must be between 100 and 999", key)
	}
	return nil
}

func validateOpenRestyWorkerProcesses(key, trimmed string) error {
	if trimmed == "auto" {
		return nil
	}
	return validatePositiveIntegerOption(key, trimmed)
}

func validateOpenRestyGzipCompLevel(key, trimmed string) error {
	if err := validatePositiveIntegerOption(key, trimmed); err != nil {
		return err
	}
	level, _ := strconv.Atoi(trimmed)
	if level > maxOpenRestyGzipCompLevel {
		return fmt.Errorf("%s must not exceed %d", key, maxOpenRestyGzipCompLevel)
	}
	return nil
}

func validateOpenRestyEventsUse(key, trimmed string) error {
	if trimmed == "" {
		return nil
	}
	switch trimmed {
	case "epoll", "kqueue", "poll", "select", "rtsig", "/dev/poll", "eventport":
		return nil
	default:
		return fmt.Errorf("%s must be epoll, kqueue, poll, select, rtsig, /dev/poll, eventport or empty", key)
	}
}

func validateOpenRestyResolvers(key, trimmed string) error {
	if trimmed == "" {
		return nil
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9.:\-\s]+$`).MatchString(trimmed) {
		return fmt.Errorf("%s contains invalid characters; enter valid IP addresses or domain names separated by spaces", key)
	}
	return nil
}

func validateOpenRestyProxyBuffers(key, trimmed string) error {
	if openRestyProxyBuffersPattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s must use a format such as \\\"16 16k\\\"", key)
}

func validateOpenRestySizeValue(key, trimmed string) error {
	if openRestySizePattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s must be an integer or a size with a k/m/g suffix", key)
}

func validateOpenRestyCachePath(key, trimmed string) error {
	if strings.ContainsAny(trimmed, "\r\n\t") {
		return fmt.Errorf("%s must not contain newlines or tabs", key)
	}
	return nil
}

func validateOpenRestyCacheLevels(key, trimmed string) error {
	if openRestyCacheLevelsPattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s must use a format such as \\\"1:2\\\" or \\\"1:2:2\\\"", key)
}

func validateOpenRestyDurationToken(key, trimmed string) error {
	if openRestyDurationTokenPattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s must be a duration with a unit, such as 30m or 5s", key)
}

func validateOpenRestyCacheKeyTemplate(key, trimmed string) error {
	if trimmed == "" {
		return fmt.Errorf("%s must not be empty", key)
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return fmt.Errorf("%s must not contain newlines", key)
	}
	return nil
}

func validateOpenRestyCacheUseStale(key, trimmed string) error {
	if trimmed == "" {
		return fmt.Errorf("%s must not be empty", key)
	}
	allowedTokens := map[string]struct{}{
		"error": {}, "timeout": {}, "invalid_header": {}, "updating": {},
		"http_500": {}, "http_502": {}, "http_503": {}, "http_504": {},
		"http_403": {}, "http_404": {}, "http_429": {}, "off": {},
	}
	for token := range strings.FieldsSeq(trimmed) {
		if _, ok := allowedTokens[token]; !ok {
			return fmt.Errorf("%s contains unsupported value %q", key, token)
		}
	}
	return nil
}

func validateOpenRestyMainConfigTemplate(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", key)
	}
	return nil
}

func validateOpenRestyTrustedProxyCIDRs(key, value string) error {
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

func validateOpenRestyDefaultLimitRate(key, trimmed string) error {
	if trimmed == "" || trimmed == "0" {
		return nil
	}
	if !openRestyDefaultLimitRatePattern.MatchString(strings.ToLower(trimmed)) {
		return fmt.Errorf("invalid %s format; use 512k, 1m or a number, or leave empty to disable", key)
	}
	return nil
}

var openRestyDefaultLimitReqPerIPPattern = regexp.MustCompile(`^\d+r/[sm]$`)

func validateOpenRestyDefaultLimitReqPerIP(key, trimmed string) error {
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
	codes, err := openrestyrender.ExpandStatusCodeTags(tags)
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
