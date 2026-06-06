package controller

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/service"
	"github.com/rain-kl/openflare/openflare-server/utils"
	"github.com/rain-kl/openflare/openflare-server/utils/geoip"

	"github.com/gin-gonic/gin"
)

var (
	openRestySizePattern          = regexp.MustCompile(`^\d+[kKmMgG]?$`)
	openRestyProxyBuffersPattern  = regexp.MustCompile(`^\d+\s+\d+[kKmMgG]?$`)
	openRestyCacheLevelsPattern   = regexp.MustCompile(`^\d{1,2}(?::\d{1,2}){0,2}$`)
	openRestyDurationTokenPattern = regexp.MustCompile(`^\d+[smhdwSMHDW]$`)
)

type optionBatchPayload struct {
	Options []model.Option `json:"options"`
}

func validateRateLimitOption(key string, value string) error {
	maxDurationSeconds := int(common.RateLimitKeyExpirationDuration.Seconds())

	switch key {
	case "GlobalApiRateLimitNum", "GlobalWebRateLimitNum", "CriticalRateLimitNum":
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue <= 0 {
			return fmt.Errorf("%s 必须为大于 0 的整数", key)
		}
		return nil
	case "GlobalApiRateLimitDuration", "GlobalWebRateLimitDuration", "CriticalRateLimitDuration":
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue <= 0 {
			return fmt.Errorf("%s 必须为大于 0 的整数秒", key)
		}
		if intValue > maxDurationSeconds {
			return fmt.Errorf("%s 不能大于 %d 秒", key, maxDurationSeconds)
		}
		return nil
	default:
		return nil
	}
}

func validatePositiveIntegerOption(key string, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil || intValue <= 0 {
		return fmt.Errorf("%s 必须为大于 0 的整数", key)
	}
	return nil
}

func validateBooleanOption(key string, value string) error {
	switch value {
	case "true", "false":
		return nil
	default:
		return fmt.Errorf("%s 必须为 true 或 false", key)
	}
}

func validateGeoIPOption(key string, value string) error {
	if key != "GeoIPProvider" {
		return nil
	}
	if !geoip.IsValidProvider(value) {
		return fmt.Errorf("%s 仅支持 disabled、mmdb、ip-api、geojs、ipinfo", key)
	}
	return nil
}

func validateDatabaseCleanupOption(key string, value string) error {
	switch key {
	case "DatabaseAutoCleanupEnabled":
		return validateBooleanOption(key, value)
	case "DatabaseAutoCleanupRetentionDays":
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue < 1 {
			return fmt.Errorf("%s 必须为大于等于 1 的整数天", key)
		}
		return nil
	default:
		return nil
	}
}

func validateAgentOption(key string, value string) error {
	switch key {
	case "AgentWebsocketUpgradeEnabled":
		return validateBooleanOption(key, strings.TrimSpace(value))
	default:
		return nil
	}
}

func validateUptimeKumaOption(key string, value string, state map[string]string) error {
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
			if password == "" && common.UptimeKumaPassword == "" {
				return fmt.Errorf("启用 Uptime Kuma 时密码不能为空")
			}
		}
	case "UptimeKumaUsername":
		if trimmed == "" && state["UptimeKumaEnabled"] == "true" {
			return fmt.Errorf("启用 Uptime Kuma 时用户名不能为空")
		}
	case "UptimeKumaPassword":
		// No specific format checks needed
	case "UptimeKumaUrl":
		if trimmed != "" {
			if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
				return fmt.Errorf("Uptime Kuma 地址必须以 http:// 或 https:// 开头")
			}
		}
	case "UptimeKumaMonitorScope":
		if trimmed != "all" && trimmed != "selected" {
			return fmt.Errorf("监控范围必须为全部站点 (all) 或选择站点 (selected)")
		}
	case "UptimeKumaSyncInterval", "UptimeKumaInterval", "UptimeKumaRetryInterval", "UptimeKumaTimeout":
		if err := validatePositiveIntegerOption(key, trimmed); err != nil {
			return err
		}
	case "UptimeKumaRetry":
		intValue, err := strconv.Atoi(trimmed)
		if err != nil || intValue < 0 {
			return fmt.Errorf("%s 必须为大于等于 0 的整数", key)
		}
	}
	return nil
}

func validateOpenRestyOption(key string, value string) error {
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
		return nil
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
		return nil
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
		return nil
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
		return nil
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
		return nil
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
		return nil
	case "OpenRestyMainConfigTemplate":
		return service.ValidateOpenRestyMainConfigTemplate(value)
	default:
		return nil
	}
}

func buildOptionValidationState(options []model.Option) map[string]string {
	common.OptionMapRWMutex.RLock()
	state := make(map[string]string, len(common.OptionMap)+len(options))
	for key, value := range common.OptionMap {
		state[key] = value
	}
	common.OptionMapRWMutex.RUnlock()

	for _, option := range options {
		state[option.Key] = option.Value
	}
	return state
}

func validateOptionWithState(option model.Option, state map[string]string) error {
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

	if err := validateRateLimitOption(option.Key, option.Value); err != nil {
		return err
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
	if err := validateUptimeKumaOption(option.Key, option.Value, state); err != nil {
		return err
	}
	return nil
}

func updateOptions(options []model.Option) error {
	if len(options) == 0 {
		return fmt.Errorf("无效的参数")
	}

	state := buildOptionValidationState(options)
	for _, option := range options {
		if strings.TrimSpace(option.Key) == "" {
			return fmt.Errorf("无效的参数")
		}
		if err := validateOptionWithState(option, state); err != nil {
			return err
		}
	}

	return model.UpdateOptions(options)
}

// GetOptions godoc
// @Summary List editable options
// @Tags Options
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/option/ [get]
func GetOptions(c *gin.Context) {
	var options []*model.Option
	common.OptionMapRWMutex.RLock()
	for k, v := range common.OptionMap {
		if strings.Contains(k, "Token") || strings.Contains(k, "Secret") || strings.Contains(k, "Password") {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: utils.Interface2String(v),
		})
	}
	common.OptionMapRWMutex.RUnlock()
	response.RespondSuccess(c, options)
}

// UpdateOption godoc
// @Summary Update option
// @Tags Options
// @Accept json
// @Produce json
// @Param payload body model.Option true "Option payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/option/update [post]
func UpdateOption(c *gin.Context) {
	var option model.Option
	if !bind.JSON(c, &option) {
		return
	}
	state := buildOptionValidationState([]model.Option{option})
	if err := validateOptionWithState(option, state); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	err := model.UpdateOption(option.Key, option.Value)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

// UpdateOptionsBatch godoc
// @Summary Batch update options
// @Tags Options
// @Accept json
// @Produce json
// @Param payload body optionBatchPayload true "Batch option payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/option/update-batch [post]
func UpdateOptionsBatch(c *gin.Context) {
	var payload optionBatchPayload
	if !bind.JSON(c, &payload) {
		return
	}
	if len(payload.Options) == 0 {
		response.RespondBadRequest(c, "无效的参数")
		return
	}

	if err := updateOptions(payload.Options); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	response.RespondSuccessMessage(c, "")
}
