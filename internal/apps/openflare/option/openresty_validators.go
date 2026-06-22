// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
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
}

func validateOpenRestyOption(key, value string) error {
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
		return fmt.Errorf("%s 必须在 100 到 999 之间", key)
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
		return fmt.Errorf("%s 不能大于 %d", key, maxOpenRestyGzipCompLevel)
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
		return fmt.Errorf("%s 仅支持 epoll、kqueue、poll、select、rtsig、/dev/poll、eventport 或留空", key)
	}
}

func validateOpenRestyResolvers(key, trimmed string) error {
	if trimmed == "" {
		return nil
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9.:\-\s]+$`).MatchString(trimmed) {
		return fmt.Errorf("%s 包含非法字符，请填入有效的 IP 地址或域名，以空格分隔", key)
	}
	return nil
}

func validateOpenRestyProxyBuffers(key, trimmed string) error {
	if openRestyProxyBuffersPattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s 格式必须类似 \"16 16k\"", key)
}

func validateOpenRestySizeValue(key, trimmed string) error {
	if openRestySizePattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s 格式必须为整数或带 k/m/g 单位的大小值", key)
}

func validateOpenRestyCachePath(key, trimmed string) error {
	if strings.ContainsAny(trimmed, "\r\n\t") {
		return fmt.Errorf("%s 不能包含换行或制表符", key)
	}
	return nil
}

func validateOpenRestyCacheLevels(key, trimmed string) error {
	if openRestyCacheLevelsPattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s 格式必须类似 \"1:2\" 或 \"1:2:2\"", key)
}

func validateOpenRestyDurationToken(key, trimmed string) error {
	if openRestyDurationTokenPattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("%s 格式必须为带单位的时长，例如 30m 或 5s", key)
}

func validateOpenRestyCacheKeyTemplate(key, trimmed string) error {
	if trimmed == "" {
		return fmt.Errorf("%s 不能为空", key)
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return fmt.Errorf("%s 不能包含换行", key)
	}
	return nil
}

func validateOpenRestyCacheUseStale(key, trimmed string) error {
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
	return nil
}

func validateOpenRestyMainConfigTemplate(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s 不能为空", key)
	}
	return nil
}
