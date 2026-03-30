package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"openflare/common"
	"openflare/model"
	"openflare/service"
	"openflare/utils"
	"openflare/utils/geoip"
	"regexp"
	"strconv"
	"strings"
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
	case "GlobalApiRateLimitNum", "GlobalWebRateLimitNum", "UploadRateLimitNum", "DownloadRateLimitNum", "CriticalRateLimitNum":
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue <= 0 {
			return fmt.Errorf("%s 必须为大于 0 的整数", key)
		}
		return nil
	case "GlobalApiRateLimitDuration", "GlobalWebRateLimitDuration", "UploadRateLimitDuration", "DownloadRateLimitDuration", "CriticalRateLimitDuration":
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

func validateOpenRestyOption(key string, value string) error {
	trimmed := strings.TrimSpace(value)

	switch key {
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
		return fmt.Errorf("%s 已废弃，不再支持配置 resolver", key)
	case "OpenRestyEventsMultiAcceptEnabled",
		"OpenRestyWebsocketEnabled",
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
			return fmt.Errorf("鏃犳硶鍚敤 GitHub OAuth锛岃鍏堝～鍏?GitHub Client ID 浠ュ強 GitHub Client Secret锛?")
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && strings.TrimSpace(state["WeChatServerAddress"]) == "" {
			return fmt.Errorf("鏃犳硶鍚敤寰俊鐧诲綍锛岃鍏堝～鍏ュ井淇＄櫥褰曠浉鍏抽厤缃俊鎭紒")
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && strings.TrimSpace(state["TurnstileSiteKey"]) == "" {
			return fmt.Errorf("鏃犳硶鍚敤 Turnstile 鏍￠獙锛岃鍏堝～鍏?Turnstile 鏍￠獙鐩稿叧閰嶇疆淇℃伅锛?")
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
	return nil
}

func updateOptions(options []model.Option) error {
	if len(options) == 0 {
		return fmt.Errorf("鏃犳晥鐨勫弬鏁?")
	}

	state := buildOptionValidationState(options)
	for _, option := range options {
		if strings.TrimSpace(option.Key) == "" {
			return fmt.Errorf("鏃犳晥鐨勫弬鏁?")
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
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		if strings.Contains(k, "Token") || strings.Contains(k, "Secret") {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: utils.Interface2String(v),
		})
	}
	common.OptionMapRWMutex.Unlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
	return
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
	err := json.NewDecoder(c.Request.Body).Decode(&option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client ID 以及 GitHub Client Secret！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})
			return
		}
	}
	if err = validateRateLimitOption(option.Key, option.Value); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err = validateOpenRestyOption(option.Key, option.Value); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err = validateGeoIPOption(option.Key, option.Value); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err = validateDatabaseCleanupOption(option.Key, option.Value); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = model.UpdateOption(option.Key, option.Value)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
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
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil || len(payload.Options) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "鏃犳晥鐨勫弬鏁?",
		})
		return
	}

	if err := updateOptions(payload.Options); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
