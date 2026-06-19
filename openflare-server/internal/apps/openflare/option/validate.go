// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/model"
)

var (
	openRestySizePattern          = regexp.MustCompile(`^\d+[kKmMgG]?$`)
	openRestyProxyBuffersPattern  = regexp.MustCompile(`^\d+\s+\d+[kKmMgG]?$`)
	openRestyCacheLevelsPattern   = regexp.MustCompile(`^\d{1,2}(?::\d{1,2}){0,2}$`)
	openRestyDurationTokenPattern = regexp.MustCompile(`^\d+[smhdwSMHDW]$`)
)

func buildOptionValidationState(options []model.OpenFlareOption) map[string]string {
	model.OptionMapRWMutex.RLock()
	state := make(map[string]string, len(model.OptionMap)+len(options))
	for key, value := range model.OptionMap {
		state[key] = value
	}
	model.OptionMapRWMutex.RUnlock()

	for _, option := range options {
		state[option.Key] = option.Value
	}
	return state
}

func validateOptionWithState(option model.OpenFlareOption, state map[string]string) error {
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && strings.TrimSpace(state["GitHubClientId"]) == "" {
			return fmt.Errorf("无法启用 GitHub OAuth，请先填入 GitHub Client ID 以及 GitHub Client Secret！")
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && strings.TrimSpace(state["WeChatServerAddress"]) == "" {
			return fmt.Errorf("无法启用微信登录，请先填入微信登录相关配置信息！")
		}
	}

	if err := validateOpenRestyOption(option.Key, option.Value); err != nil {
		return err
	}
	if err := validateGeoIPOption(option.Key, option.Value); err != nil {
		return err
	}
	if err := validateDatabaseCleanupOption(option.Key, option.Value); err != nil {
		return err
	}
	if err := validateAgentOption(option.Key, option.Value); err != nil {
		return err
	}
	return validateUptimeKumaOption(option.Key, option.Value, state)
}

func validatePositiveIntegerOption(key, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil || intValue <= 0 {
		return fmt.Errorf("%s 必须为大于 0 的整数", key)
	}
	return nil
}

func validateBooleanOption(key, value string) error {
	switch value {
	case "true", "false":
		return nil
	default:
		return fmt.Errorf("%s 必须为 true 或 false", key)
	}
}

func validateGeoIPOption(key, value string) error {
	if key != "GeoIPProvider" {
		return nil
	}
	if geoip.IsValidProvider(value) {
		return nil
	}
	return fmt.Errorf("%s 仅支持 disabled、mmdb、ip-api、geojs、ipinfo", key)
}

func validateDatabaseCleanupOption(key, value string) error {
	switch key {
	case "DatabaseAutoCleanupEnabled":
		return validateBooleanOption(key, value)
	case "DatabaseAutoCleanupRetentionDays":
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue < 1 {
			return fmt.Errorf("%s 必须为大于等于 1 的整数天", key)
		}
	}
	return nil
}

func validateAgentOption(key, value string) error {
	if key == "AgentWebsocketUpgradeEnabled" {
		return validateBooleanOption(key, strings.TrimSpace(value))
	}
	return nil
}

func validateUptimeKumaOption(key, value string, state map[string]string) error {
	trimmed := strings.TrimSpace(value)
	switch key {
	case "UptimeKumaEnabled":
		if err := validateBooleanOption(key, trimmed); err != nil {
			return err
		}
		if trimmed == "true" {
			url := strings.TrimSpace(state["UptimeKumaUrl"])
			username := strings.TrimSpace(state["UptimeKumaUsername"])
			password := strings.TrimSpace(state["UptimeKumaPassword"])
			if url == "" {
				return fmt.Errorf("启用 Uptime Kuma 时地址不能为空")
			}
			if username == "" {
				return fmt.Errorf("启用 Uptime Kuma 时用户名不能为空")
			}
			if password == "" && model.UptimeKumaPassword == "" {
				return fmt.Errorf("启用 Uptime Kuma 时密码不能为空")
			}
		}
	case "UptimeKumaUsername":
		if trimmed == "" && state["UptimeKumaEnabled"] == "true" {
			return fmt.Errorf("启用 Uptime Kuma 时用户名不能为空")
		}
	case "UptimeKumaUrl":
		if trimmed != "" && !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			return fmt.Errorf("Uptime Kuma 地址必须以 http:// 或 https:// 开头")
		}
	case "UptimeKumaMonitorScope":
		if trimmed != "all" && trimmed != "selected" {
			return fmt.Errorf("监控范围必须为全部站点 (all) 或选择站点 (selected)")
		}
	case "UptimeKumaSyncInterval", "UptimeKumaInterval", "UptimeKumaRetryInterval", "UptimeKumaTimeout":
		return validatePositiveIntegerOption(key, trimmed)
	case "UptimeKumaRetry":
		intValue, err := strconv.Atoi(trimmed)
		if err != nil || intValue < 0 {
			return fmt.Errorf("%s 必须为大于等于 0 的整数", key)
		}
	}
	return nil
}

func validateOpenRestyOption(key, value string) error {
	trimmed := strings.TrimSpace(value)

	switch key {
	case "OpenRestyDefaultServerReturnStatus":
		if err := validatePositiveIntegerOption(key, trimmed); err != nil {
			return err
		}
		statusCode, _ := strconv.Atoi(trimmed)
		if statusCode < 100 || statusCode > 999 {
			return fmt.Errorf("%s 必须在 100 到 999 之间", key)
		}
	case "OpenRestyWorkerProcesses":
		if trimmed == "auto" {
			return nil
		}
		return validatePositiveIntegerOption(key, trimmed)
	case "OpenRestyWorkerConnections",
		"OpenRestyWorkerRlimitNofile",
		"OpenRestyKeepaliveTimeout",
		"OpenRestyKeepaliveRequests",
		"OpenRestyClientHeaderTimeout",
		"OpenRestyClientBodyTimeout",
		"OpenRestySendTimeout",
		"OpenRestyProxyConnectTimeout",
		"OpenRestyProxySendTimeout",
		"OpenRestyProxyReadTimeout",
		"OpenRestyGzipMinLength":
		return validatePositiveIntegerOption(key, trimmed)
	case "OpenRestyGzipCompLevel":
		if err := validatePositiveIntegerOption(key, trimmed); err != nil {
			return err
		}
		level, _ := strconv.Atoi(trimmed)
		if level > 9 {
			return fmt.Errorf("%s 不能大于 9", key)
		}
	case "OpenRestyEventsUse":
		if trimmed == "" {
			return nil
		}
		switch trimmed {
		case "epoll", "kqueue", "poll", "select", "rtsig", "/dev/poll", "eventport":
			return nil
		default:
			return fmt.Errorf("%s 仅支持 epoll、kqueue、poll、select、rtsig、/dev/poll、eventport 或留空", key)
		}
	case "OpenRestyResolvers":
		if trimmed == "" {
			return nil
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9.:\-\s]+$`).MatchString(trimmed) {
			return fmt.Errorf("%s 包含非法字符，请填入有效的 IP 地址或域名，以空格分隔", key)
		}
	case "OpenRestyEventsMultiAcceptEnabled",
		"OpenRestyWebsocketEnabled",
		"OpenRestyHTTP3Enabled",
		"OpenRestyProxyRequestBufferingEnabled",
		"OpenRestyProxyBufferingEnabled",
		"OpenRestyGzipEnabled",
		"OpenRestyCacheEnabled",
		"OpenRestyCacheLockEnabled":
		return validateBooleanOption(key, trimmed)
	case "OpenRestyProxyBuffers", "OpenRestyLargeClientHeaderBuffers":
		if openRestyProxyBuffersPattern.MatchString(trimmed) {
			return nil
		}
		return fmt.Errorf("%s 格式必须类似 \"16 16k\"", key)
	case "OpenRestyProxyBufferSize", "OpenRestyProxyBusyBuffersSize", "OpenRestyCacheMaxSize", "OpenRestyClientMaxBodySize":
		if openRestySizePattern.MatchString(trimmed) {
			return nil
		}
		return fmt.Errorf("%s 格式必须为整数或带 k/m/g 单位的大小值", key)
	case "OpenRestyCachePath":
		if strings.ContainsAny(trimmed, "\r\n\t") {
			return fmt.Errorf("%s 不能包含换行或制表符", key)
		}
	case "OpenRestyCacheLevels":
		if openRestyCacheLevelsPattern.MatchString(trimmed) {
			return nil
		}
		return fmt.Errorf("%s 格式必须类似 \"1:2\" 或 \"1:2:2\"", key)
	case "OpenRestyCacheInactive", "OpenRestyCacheLockTimeout":
		if openRestyDurationTokenPattern.MatchString(trimmed) {
			return nil
		}
		return fmt.Errorf("%s 格式必须为带单位的时长，例如 30m 或 5s", key)
	case "OpenRestyCacheKeyTemplate":
		if trimmed == "" {
			return fmt.Errorf("%s 不能为空", key)
		}
		if strings.ContainsAny(trimmed, "\r\n") {
			return fmt.Errorf("%s 不能包含换行", key)
		}
	case "OpenRestyCacheUseStale":
		if trimmed == "" {
			return fmt.Errorf("%s 不能为空", key)
		}
		allowedTokens := map[string]struct{}{
			"error": {}, "timeout": {}, "invalid_header": {}, "updating": {},
			"http_500": {}, "http_502": {}, "http_503": {}, "http_504": {},
			"http_403": {}, "http_404": {}, "http_429": {}, "off": {},
		}
		for _, token := range strings.Fields(trimmed) {
			if _, ok := allowedTokens[token]; !ok {
				return fmt.Errorf("%s 包含不支持的值 %q", key, token)
			}
		}
	case "OpenRestyMainConfigTemplate":
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s 不能为空", key)
		}
	}
	return nil
}

func validateOptions(options []model.OpenFlareOption) error {
	if len(options) == 0 {
		return fmt.Errorf(errInvalidParams)
	}

	state := buildOptionValidationState(options)
	for _, option := range options {
		if strings.TrimSpace(option.Key) == "" {
			return fmt.Errorf(errInvalidParams)
		}
		if err := validateOptionWithState(option, state); err != nil {
			return err
		}
	}
	return nil
}
