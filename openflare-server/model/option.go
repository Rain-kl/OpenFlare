package model

import (
	"strconv"
	"strings"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/utils/geoip"

	"gorm.io/gorm"
)

type Option struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

func AllOption() ([]*Option, error) {
	var options []*Option
	var err error
	err = DB.Find(&options).Error
	return options, err
}

func InitOptionMap() {
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	common.OptionMap["PasswordLoginEnabled"] = strconv.FormatBool(common.PasswordLoginEnabled)
	common.OptionMap["PasswordRegisterEnabled"] = strconv.FormatBool(common.PasswordRegisterEnabled)
	common.OptionMap["EmailVerificationEnabled"] = strconv.FormatBool(common.EmailVerificationEnabled)
	common.OptionMap["GitHubOAuthEnabled"] = strconv.FormatBool(common.GitHubOAuthEnabled)
	common.OptionMap["WeChatAuthEnabled"] = strconv.FormatBool(common.WeChatAuthEnabled)
	common.OptionMap["SMTPServer"] = ""
	common.OptionMap["SMTPPort"] = strconv.Itoa(common.SMTPPort)
	common.OptionMap["SMTPAccount"] = ""
	common.OptionMap["SMTPToken"] = ""
	common.OptionMap["Notice"] = ""
	common.OptionMap["About"] = ""
	common.OptionMap["Footer"] = common.Footer
	common.OptionMap["HomePageLink"] = common.HomePageLink
	common.OptionMap["SystemName"] = common.SystemName
	common.OptionMap["ServerAddress"] = ""
	common.OptionMap["GitHubClientId"] = ""
	common.OptionMap["GitHubClientSecret"] = ""
	common.OptionMap["WeChatServerAddress"] = ""
	common.OptionMap["WeChatServerToken"] = ""
	common.OptionMap["WeChatAccountQRCodeImageURL"] = ""
	common.OptionMap["AgentDiscoveryToken"] = ""
	common.OptionMap["AgentHeartbeatInterval"] = strconv.Itoa(common.AgentHeartbeatInterval)
	common.OptionMap["AgentWebsocketUpgradeEnabled"] = strconv.FormatBool(common.AgentWebsocketUpgradeEnabled)
	common.OptionMap["NodeOfflineThreshold"] = strconv.Itoa(int(common.NodeOfflineThreshold.Milliseconds()))
	common.OptionMap["AgentUpdateRepo"] = common.AgentUpdateRepo
	common.OptionMap["GeoIPProvider"] = common.GeoIPProvider
	common.OptionMap["DatabaseAutoCleanupEnabled"] = strconv.FormatBool(common.DatabaseAutoCleanupEnabled)
	common.OptionMap["UptimeKumaEnabled"] = strconv.FormatBool(common.UptimeKumaEnabled)
	common.OptionMap["UptimeKumaUrl"] = common.UptimeKumaUrl
	common.OptionMap["UptimeKumaUsername"] = common.UptimeKumaUsername
	common.OptionMap["UptimeKumaPassword"] = common.UptimeKumaPassword
	common.OptionMap["UptimeKumaMonitorScope"] = common.UptimeKumaMonitorScope
	common.OptionMap["UptimeKumaSelectedSites"] = common.UptimeKumaSelectedSites
	common.OptionMap["UptimeKumaSyncInterval"] = strconv.Itoa(common.UptimeKumaSyncInterval)
	common.OptionMap["UptimeKumaInterval"] = strconv.Itoa(common.UptimeKumaInterval)
	common.OptionMap["UptimeKumaRetry"] = strconv.Itoa(common.UptimeKumaRetry)
	common.OptionMap["UptimeKumaRetryInterval"] = strconv.Itoa(common.UptimeKumaRetryInterval)
	common.OptionMap["UptimeKumaTimeout"] = strconv.Itoa(common.UptimeKumaTimeout)
	common.OptionMap["DatabaseAutoCleanupRetentionDays"] = strconv.Itoa(common.DatabaseAutoCleanupRetentionDays)
	common.OptionMap["OpenRestyDefaultServerReturnStatus"] = strconv.Itoa(common.OpenRestyDefaultServerReturnStatus)
	common.OptionMap["OpenRestyWorkerProcesses"] = common.OpenRestyWorkerProcesses
	common.OptionMap["OpenRestyWorkerConnections"] = strconv.Itoa(common.OpenRestyWorkerConnections)
	common.OptionMap["OpenRestyWorkerRlimitNofile"] = strconv.Itoa(common.OpenRestyWorkerRlimitNofile)
	common.OptionMap["OpenRestyEventsUse"] = common.OpenRestyEventsUse
	common.OptionMap["OpenRestyEventsMultiAcceptEnabled"] = strconv.FormatBool(common.OpenRestyEventsMultiAcceptEnabled)
	common.OptionMap["OpenRestyKeepaliveTimeout"] = strconv.Itoa(common.OpenRestyKeepaliveTimeout)
	common.OptionMap["OpenRestyKeepaliveRequests"] = strconv.Itoa(common.OpenRestyKeepaliveRequests)
	common.OptionMap["OpenRestyClientHeaderTimeout"] = strconv.Itoa(common.OpenRestyClientHeaderTimeout)
	common.OptionMap["OpenRestyClientBodyTimeout"] = strconv.Itoa(common.OpenRestyClientBodyTimeout)
	common.OptionMap["OpenRestyClientMaxBodySize"] = common.OpenRestyClientMaxBodySize
	common.OptionMap["OpenRestyLargeClientHeaderBuffers"] = common.OpenRestyLargeClientHeaderBuffers
	common.OptionMap["OpenRestySendTimeout"] = strconv.Itoa(common.OpenRestySendTimeout)
	common.OptionMap["OpenRestyProxyConnectTimeout"] = strconv.Itoa(common.OpenRestyProxyConnectTimeout)
	common.OptionMap["OpenRestyProxySendTimeout"] = strconv.Itoa(common.OpenRestyProxySendTimeout)
	common.OptionMap["OpenRestyProxyReadTimeout"] = strconv.Itoa(common.OpenRestyProxyReadTimeout)
	common.OptionMap["OpenRestyWebsocketEnabled"] = strconv.FormatBool(common.OpenRestyWebsocketEnabled)
	common.OptionMap["OpenRestyHTTP3Enabled"] = strconv.FormatBool(common.OpenRestyHTTP3Enabled)
	common.OptionMap["OpenRestyProxyRequestBufferingEnabled"] = strconv.FormatBool(common.OpenRestyProxyRequestBufferingEnabled)
	common.OptionMap["OpenRestyProxyBufferingEnabled"] = strconv.FormatBool(common.OpenRestyProxyBufferingEnabled)
	common.OptionMap["OpenRestyProxyBuffers"] = common.OpenRestyProxyBuffers
	common.OptionMap["OpenRestyProxyBufferSize"] = common.OpenRestyProxyBufferSize
	common.OptionMap["OpenRestyProxyBusyBuffersSize"] = common.OpenRestyProxyBusyBuffersSize
	common.OptionMap["OpenRestyGzipEnabled"] = strconv.FormatBool(common.OpenRestyGzipEnabled)
	common.OptionMap["OpenRestyGzipMinLength"] = strconv.Itoa(common.OpenRestyGzipMinLength)
	common.OptionMap["OpenRestyGzipCompLevel"] = strconv.Itoa(common.OpenRestyGzipCompLevel)
	common.OptionMap["OpenRestyCacheEnabled"] = strconv.FormatBool(common.OpenRestyCacheEnabled)
	common.OptionMap["OpenRestyCachePath"] = common.OpenRestyCachePath
	common.OptionMap["OpenRestyCacheLevels"] = common.OpenRestyCacheLevels
	common.OptionMap["OpenRestyCacheInactive"] = common.OpenRestyCacheInactive
	common.OptionMap["OpenRestyCacheMaxSize"] = common.OpenRestyCacheMaxSize
	common.OptionMap["OpenRestyCacheKeyTemplate"] = common.OpenRestyCacheKeyTemplate
	common.OptionMap["OpenRestyCacheLockEnabled"] = strconv.FormatBool(common.OpenRestyCacheLockEnabled)
	common.OptionMap["OpenRestyCacheLockTimeout"] = common.OpenRestyCacheLockTimeout
	common.OptionMap["OpenRestyCacheUseStale"] = common.OpenRestyCacheUseStale
	common.OptionMap["OpenRestyMainConfigTemplate"] = common.OpenRestyMainConfigTemplate
	common.OptionMap["GlobalApiRateLimitNum"] = strconv.Itoa(common.GlobalApiRateLimitNum)
	common.OptionMap["GlobalApiRateLimitDuration"] = strconv.FormatInt(common.GlobalApiRateLimitDuration, 10)
	common.OptionMap["GlobalWebRateLimitNum"] = strconv.Itoa(common.GlobalWebRateLimitNum)
	common.OptionMap["GlobalWebRateLimitDuration"] = strconv.FormatInt(common.GlobalWebRateLimitDuration, 10)
	common.OptionMap["CriticalRateLimitNum"] = strconv.Itoa(common.CriticalRateLimitNum)
	common.OptionMap["CriticalRateLimitDuration"] = strconv.FormatInt(common.CriticalRateLimitDuration, 10)
	common.OptionMapRWMutex.Unlock()
	options, _ := AllOption()
	for _, option := range options {
		updateOptionMap(option.Key, option.Value)
	}
}

func UpdateOption(key string, value string) error {
	return UpdateOptions([]Option{{
		Key:   key,
		Value: value,
	}})
}

func UpdateOptions(options []Option) error {
	if len(options) == 0 {
		return nil
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range options {
			if item.Key == "UptimeKumaPassword" && strings.TrimSpace(item.Value) == "" {
				continue
			}
			option := Option{
				Key: item.Key,
			}
			if err := tx.FirstOrCreate(&option, Option{Key: item.Key}).Error; err != nil {
				return err
			}
			option.Value = item.Value
			if err := tx.Save(&option).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	for _, item := range options {
		if item.Key == "UptimeKumaPassword" && strings.TrimSpace(item.Value) == "" {
			continue
		}
		updateOptionMap(item.Key, item.Value)
	}
	return nil
}

func updateOptionMap(key string, value string) {
	shouldRefreshGeoIP := false
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[key] = value
	if strings.HasSuffix(key, "Enabled") {
		boolValue := value == "true"
		switch key {
		case "PasswordRegisterEnabled":
			common.PasswordRegisterEnabled = boolValue
		case "PasswordLoginEnabled":
			common.PasswordLoginEnabled = boolValue
		case "EmailVerificationEnabled":
			common.EmailVerificationEnabled = boolValue
		case "GitHubOAuthEnabled":
			common.GitHubOAuthEnabled = boolValue
		case "WeChatAuthEnabled":
			common.WeChatAuthEnabled = boolValue
		}
	}
	switch key {
	case "SMTPServer":
		common.SMTPServer = value
	case "SMTPPort":
		intValue, _ := strconv.Atoi(value)
		common.SMTPPort = intValue
	case "SMTPAccount":
		common.SMTPAccount = value
	case "SMTPToken":
		common.SMTPToken = value
	case "ServerAddress":
		common.ServerAddress = value
	case "GitHubClientId":
		common.GitHubClientId = value
	case "GitHubClientSecret":
		common.GitHubClientSecret = value
	case "Footer":
		common.Footer = value
	case "HomePageLink":
		common.HomePageLink = value
	case "SystemName":
		common.SystemName = value
	case "WeChatServerAddress":
		common.WeChatServerAddress = value
	case "WeChatServerToken":
		common.WeChatServerToken = value
	case "WeChatAccountQRCodeImageURL":
		common.WeChatAccountQRCodeImageURL = value
	case "AgentDiscoveryToken":
		common.AgentDiscoveryToken = value
	case "AgentHeartbeatInterval":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.AgentHeartbeatInterval = v
		}
	case "AgentWebsocketUpgradeEnabled":
		common.AgentWebsocketUpgradeEnabled = value == "true"
	case "NodeOfflineThreshold":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.NodeOfflineThreshold = time.Duration(v) * time.Millisecond
		}
	case "AgentUpdateRepo":
		if value != "" {
			common.AgentUpdateRepo = value
		}
	case "GeoIPProvider":
		if geoip.IsValidProvider(value) {
			common.GeoIPProvider = value
			shouldRefreshGeoIP = true
		}
	case "UptimeKumaEnabled":
		common.UptimeKumaEnabled = value == "true"
	case "UptimeKumaUrl":
		common.UptimeKumaUrl = value
	case "UptimeKumaUsername":
		common.UptimeKumaUsername = value
	case "UptimeKumaPassword":
		common.UptimeKumaPassword = value
	case "UptimeKumaMonitorScope":
		common.UptimeKumaMonitorScope = value
	case "UptimeKumaSelectedSites":
		common.UptimeKumaSelectedSites = value
	case "UptimeKumaSyncInterval":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.UptimeKumaSyncInterval = v
		}
	case "UptimeKumaInterval":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.UptimeKumaInterval = v
		}
	case "UptimeKumaRetry":
		if v, err := strconv.Atoi(value); err == nil && v >= 0 {
			common.UptimeKumaRetry = v
		}
	case "UptimeKumaRetryInterval":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.UptimeKumaRetryInterval = v
		}
	case "UptimeKumaTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.UptimeKumaTimeout = v
		}
	case "DatabaseAutoCleanupEnabled":
		common.DatabaseAutoCleanupEnabled = value == "true"
	case "DatabaseAutoCleanupRetentionDays":
		if v, err := strconv.Atoi(value); err == nil && v >= 1 {
			common.DatabaseAutoCleanupRetentionDays = v
		}
	case "OpenRestyDefaultServerReturnStatus":
		if v, err := strconv.Atoi(value); err == nil && v >= 100 && v <= 999 {
			common.OpenRestyDefaultServerReturnStatus = v
		}
	case "OpenRestyWorkerProcesses":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyWorkerProcesses = value
		}
	case "OpenRestyWorkerConnections":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyWorkerConnections = v
		}
	case "OpenRestyWorkerRlimitNofile":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyWorkerRlimitNofile = v
		}
	case "OpenRestyEventsUse":
		common.OpenRestyEventsUse = value
	case "OpenRestyResolvers":
		common.OpenRestyResolvers = value
	case "OpenRestyEventsMultiAcceptEnabled":
		common.OpenRestyEventsMultiAcceptEnabled = value == "true"
	case "OpenRestyKeepaliveTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyKeepaliveTimeout = v
		}
	case "OpenRestyKeepaliveRequests":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyKeepaliveRequests = v
		}
	case "OpenRestyClientHeaderTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyClientHeaderTimeout = v
		}
	case "OpenRestyClientBodyTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyClientBodyTimeout = v
		}
	case "OpenRestyClientMaxBodySize":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyClientMaxBodySize = value
		}
	case "OpenRestyLargeClientHeaderBuffers":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyLargeClientHeaderBuffers = value
		}
	case "OpenRestySendTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestySendTimeout = v
		}
	case "OpenRestyProxyConnectTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyProxyConnectTimeout = v
		}
	case "OpenRestyProxySendTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyProxySendTimeout = v
		}
	case "OpenRestyProxyReadTimeout":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyProxyReadTimeout = v
		}
	case "OpenRestyWebsocketEnabled":
		common.OpenRestyWebsocketEnabled = value == "true"
	case "OpenRestyHTTP3Enabled":
		common.OpenRestyHTTP3Enabled = value == "true"
	case "OpenRestyProxyRequestBufferingEnabled":
		common.OpenRestyProxyRequestBufferingEnabled = value == "true"
	case "OpenRestyProxyBufferingEnabled":
		common.OpenRestyProxyBufferingEnabled = value == "true"
	case "OpenRestyProxyBuffers":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyProxyBuffers = value
		}
	case "OpenRestyProxyBufferSize":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyProxyBufferSize = value
		}
	case "OpenRestyProxyBusyBuffersSize":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyProxyBusyBuffersSize = value
		}
	case "OpenRestyGzipEnabled":
		common.OpenRestyGzipEnabled = value == "true"
	case "OpenRestyGzipMinLength":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyGzipMinLength = v
		}
	case "OpenRestyGzipCompLevel":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.OpenRestyGzipCompLevel = v
		}
	case "OpenRestyCacheEnabled":
		common.OpenRestyCacheEnabled = value == "true"
	case "OpenRestyCachePath":
		common.OpenRestyCachePath = value
	case "OpenRestyCacheLevels":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyCacheLevels = value
		}
	case "OpenRestyCacheInactive":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyCacheInactive = value
		}
	case "OpenRestyCacheMaxSize":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyCacheMaxSize = value
		}
	case "OpenRestyCacheKeyTemplate":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyCacheKeyTemplate = value
		}
	case "OpenRestyCacheLockEnabled":
		common.OpenRestyCacheLockEnabled = value == "true"
	case "OpenRestyCacheLockTimeout":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyCacheLockTimeout = value
		}
	case "OpenRestyCacheUseStale":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyCacheUseStale = value
		}
	case "OpenRestyMainConfigTemplate":
		if strings.TrimSpace(value) != "" {
			common.OpenRestyMainConfigTemplate = value
		}
	case "GlobalApiRateLimitNum":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.GlobalApiRateLimitNum = v
		}
	case "GlobalApiRateLimitDuration":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
			common.GlobalApiRateLimitDuration = v
		}
	case "GlobalWebRateLimitNum":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.GlobalWebRateLimitNum = v
		}
	case "GlobalWebRateLimitDuration":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
			common.GlobalWebRateLimitDuration = v
		}
	case "CriticalRateLimitNum":
		if v, err := strconv.Atoi(value); err == nil && v > 0 {
			common.CriticalRateLimitNum = v
		}
	case "CriticalRateLimitDuration":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
			common.CriticalRateLimitDuration = v
		}
	}
	common.OptionMapRWMutex.Unlock()
	if shouldRefreshGeoIP {
		geoip.InitGeoIP(common.GeoIPProvider)
	}
}
