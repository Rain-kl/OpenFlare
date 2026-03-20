package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"openflare/common"
	"openflare/model"
	"openflare/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var removedTemplateOptionKeys = map[string]struct{}{
	"AgentDiscoveryToken":                   {},
	"AgentHeartbeatInterval":                {},
	"NodeOfflineThreshold":                  {},
	"AgentUpdateRepo":                       {},
	"GeoIPProvider":                         {},
	"DatabaseAutoCleanupEnabled":            {},
	"DatabaseAutoCleanupRetentionDays":      {},
	"OpenRestyWorkerProcesses":              {},
	"OpenRestyWorkerConnections":            {},
	"OpenRestyWorkerRlimitNofile":           {},
	"OpenRestyEventsUse":                    {},
	"OpenRestyEventsMultiAcceptEnabled":     {},
	"OpenRestyKeepaliveTimeout":             {},
	"OpenRestyKeepaliveRequests":            {},
	"OpenRestyClientHeaderTimeout":          {},
	"OpenRestyClientBodyTimeout":            {},
	"OpenRestyClientMaxBodySize":            {},
	"OpenRestyLargeClientHeaderBuffers":     {},
	"OpenRestySendTimeout":                  {},
	"OpenRestyProxyConnectTimeout":          {},
	"OpenRestyProxySendTimeout":             {},
	"OpenRestyProxyReadTimeout":             {},
	"OpenRestyWebsocketEnabled":             {},
	"OpenRestyProxyRequestBufferingEnabled": {},
	"OpenRestyProxyBufferingEnabled":        {},
	"OpenRestyProxyBuffers":                 {},
	"OpenRestyProxyBufferSize":              {},
	"OpenRestyProxyBusyBuffersSize":         {},
	"OpenRestyGzipEnabled":                  {},
	"OpenRestyGzipMinLength":                {},
	"OpenRestyGzipCompLevel":                {},
	"OpenRestyCacheEnabled":                 {},
	"OpenRestyCachePath":                    {},
	"OpenRestyCacheLevels":                  {},
	"OpenRestyCacheInactive":                {},
	"OpenRestyCacheMaxSize":                 {},
	"OpenRestyCacheKeyTemplate":             {},
	"OpenRestyCacheLockEnabled":             {},
	"OpenRestyCacheLockTimeout":             {},
	"OpenRestyCacheUseStale":                {},
	"OpenRestyMainConfigTemplate":           {},
	"OpenRestyResolvers":                    {},
}

func isRemovedTemplateOption(key string) bool {
	_, ok := removedTemplateOptionKeys[key]
	return ok
}

func validateRateLimitOption(key string, value string) error {
	maxDurationSeconds := int(common.RateLimitKeyExpirationDuration.Seconds())

	switch key {
	case "GlobalApiRateLimitNum", "GlobalWebRateLimitNum", "UploadRateLimitNum", "DownloadRateLimitNum", "CriticalRateLimitNum":
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue <= 0 {
			return fmt.Errorf("%s must be a positive integer", key)
		}
		return nil
	case "GlobalApiRateLimitDuration", "GlobalWebRateLimitDuration", "UploadRateLimitDuration", "DownloadRateLimitDuration", "CriticalRateLimitDuration":
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue <= 0 {
			return fmt.Errorf("%s must be a positive integer", key)
		}
		if intValue > maxDurationSeconds {
			return fmt.Errorf("%s cannot exceed %d seconds", key, maxDurationSeconds)
		}
		return nil
	default:
		return nil
	}
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
			"message": "invalid request payload",
		})
		return
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "GitHub OAuth requires GitHub client configuration first",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "WeChat auth requires WeChat server configuration first",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Turnstile requires site key and secret key first",
			})
			return
		}
	}
	if isRemovedTemplateOption(option.Key) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("%s has been removed from GinNextTemplate options", option.Key),
		})
		return
	}
	if err = validateRateLimitOption(option.Key, option.Value); err != nil {
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
}
