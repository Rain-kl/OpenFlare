package controller

import (
	"atsflare/common"
	"atsflare/model"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

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
			Value: common.Interface2String(v),
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
// @Router /api/option/ [put]
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
