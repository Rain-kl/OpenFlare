// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const uptimeKumaTagOpenFlare = "OpenFlare"

var isSyncing atomic.Bool

// kumaConfig 封装 UptimeKuma 配置
type kumaConfig struct {
	URL           string
	Username      string
	Password      string
	MonitorScope  string
	SelectedSites string
	Interval      int
	Retry         int
	RetryInterval int
	Timeout       int
}

// loadKumaConfig 从 SystemConfig 加载 UptimeKuma 配置
func loadKumaConfig(ctx context.Context) (*kumaConfig, error) {
	url, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUptimeKumaURL)
	username, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUptimeKumaUsername)
	password, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUptimeKumaPassword)
	scope, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUptimeKumaMonitorScope)
	selected, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUptimeKumaSelectedSites)

	interval, _ := repository.GetIntByKey(ctx, model.ConfigKeyUptimeKumaInterval)
	if interval <= 0 {
		interval = 60
	}
	retry, _ := repository.GetIntByKey(ctx, model.ConfigKeyUptimeKumaRetry)
	retryInterval, _ := repository.GetIntByKey(ctx, model.ConfigKeyUptimeKumaRetryInterval)
	if retryInterval <= 0 {
		retryInterval = 60
	}
	timeout, _ := repository.GetIntByKey(ctx, model.ConfigKeyUptimeKumaTimeout)
	if timeout <= 0 {
		timeout = 48
	}

	if scope.Value == "" {
		scope.Value = "all"
	}

	return &kumaConfig{
		URL:           strings.TrimSpace(url.Value),
		Username:      strings.TrimSpace(username.Value),
		Password:      strings.TrimSpace(password.Value),
		MonitorScope:  scope.Value,
		SelectedSites: selected.Value,
		Interval:      interval,
		Retry:         retry,
		RetryInterval: retryInterval,
		Timeout:       timeout,
	}, nil
}

// SyncToUptimeKuma synchronizes enabled proxy routes to Uptime Kuma monitors.
func SyncToUptimeKuma(ctx context.Context) error {
	// 检查是否启用
	enabled, _ := repository.GetBoolByKey(ctx, model.ConfigKeyUptimeKumaEnabled)
	if !enabled {
		return fmt.Errorf("uptime Kuma integration is disabled")
	}

	if !isSyncing.CompareAndSwap(false, true) {
		return fmt.Errorf("sync task is already in progress, please try again later")
	}
	defer isSyncing.Store(false)

	// 加载配置
	config, err := loadKumaConfig(ctx)
	if err != nil {
		return err
	}

	// 验证配置
	if err := validateKumaConfig(config); err != nil {
		return err
	}

	slog.Info("Starting Uptime Kuma sync process",
		"url", config.URL,
		"username", config.Username,
		"scope", config.MonitorScope,
	)

	allRoutes, err := repository.ListProxyRoutes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list local proxy routes: %w", err)
	}

	expectedRoutes, err := filterExpectedRoutes(allRoutes, config)
	if err != nil {
		return err
	}

	client, err := connectAndLoginUptimeKuma(config.URL, config.Username, config.Password)
	if err != nil {
		return err
	}
	defer client.Close()

	openFlareTagID, err := ensureOpenFlareTag(client)
	if err != nil {
		return err
	}

	existingOpenFlareMonitors := filterOpenFlareMonitors(client.GetMonitorList(), openFlareTagID)
	expectedSitesMap := syncRouteMonitors(ctx, client, expectedRoutes, existingOpenFlareMonitors, openFlareTagID, config)
	removeStaleMonitors(client, existingOpenFlareMonitors, expectedSitesMap)

	return nil
}

func filterExpectedRoutes(allRoutes []*model.ProxyRoute, config *kumaConfig) ([]*model.ProxyRoute, error) {
	scope := config.MonitorScope
	if scope == "selected" {
		selectedList := strings.Split(config.SelectedSites, ",")
		selectedMap := make(map[string]bool)
		for _, name := range selectedList {
			trimmedName := strings.TrimSpace(name)
			if trimmedName != "" {
				selectedMap[trimmedName] = true
			}
		}
		var expectedRoutes []*model.ProxyRoute
		for _, route := range allRoutes {
			if route.Enabled && selectedMap[route.SiteName] {
				expectedRoutes = append(expectedRoutes, route)
			}
		}
		return expectedRoutes, nil
	}

	var expectedRoutes []*model.ProxyRoute
	for _, route := range allRoutes {
		if route.Enabled {
			expectedRoutes = append(expectedRoutes, route)
		}
	}
	return expectedRoutes, nil
}

func ensureOpenFlareTag(client *SocketIOClient) (int, error) {
	slog.Debug("Fetching tags from Uptime Kuma")
	tagsAck, err := client.Emit("getTags")
	if err != nil {
		slog.Error("Failed to request tags from Uptime Kuma", "error", err)
		return 0, fmt.Errorf("failed to fetch tags: %w", err)
	}

	var tagsResult struct {
		Ok   bool      `json:"ok"`
		Tags []TagItem `json:"tags"`
	}
	if err := ParseAckResponse(tagsAck, &tagsResult); err != nil {
		slog.Error("Failed to parse tags response from Uptime Kuma", "error", err)
		return 0, fmt.Errorf("parse tags response failed: %w", err)
	}

	for _, tag := range tagsResult.Tags {
		if tag.Name == uptimeKumaTagOpenFlare {
			slog.Debug("Found existing OpenFlare tag", "tag_id", tag.ID)
			return tag.ID, nil
		}
	}

	slog.Debug("OpenFlare tag not found, creating new tag")
	addTagAck, err := client.Emit("addTag", map[string]string{
		"name":  uptimeKumaTagOpenFlare,
		"color": "#4f46e5",
	})
	if err != nil {
		slog.Error("Failed to create OpenFlare tag in Uptime Kuma", "error", err)
		return 0, fmt.Errorf("failed to create tag: %w", err)
	}

	var tagResult struct {
		Ok  bool `json:"ok"`
		Tag struct {
			ID int `json:"id"`
		} `json:"tag"`
	}
	if err := ParseAckResponse(addTagAck, &tagResult); err != nil || tagResult.Tag.ID == 0 {
		slog.Error("Failed to parse addTag response from Uptime Kuma", "error", err)
		return 0, fmt.Errorf("parse addTag response failed: %w", err)
	}

	slog.Debug("Successfully created OpenFlare tag", "tag_id", tagResult.Tag.ID)
	return tagResult.Tag.ID, nil
}

func filterOpenFlareMonitors(monitors map[string]Monitor, openFlareTagID int) map[string]Monitor {
	existingOpenFlareMonitors := make(map[string]Monitor)
	for _, monitor := range monitors {
		hasOpenFlareTag := false
		for _, tag := range monitor.Tags {
			if tag.Name == uptimeKumaTagOpenFlare || tag.ID == openFlareTagID {
				hasOpenFlareTag = true
				break
			}
		}
		if hasOpenFlareTag {
			existingOpenFlareMonitors[monitor.Name] = monitor
		}
	}
	return existingOpenFlareMonitors
}

func routeMonitorURL(ctx context.Context, route *model.ProxyRoute) (string, error) {
	if route == nil {
		return "", fmt.Errorf("proxy route is nil")
	}
	domains, err := repository.ListZoneDomainsByRouteID(ctx, route.ID)
	if err != nil {
		return "", err
	}
	if len(domains) == 0 {
		return "", fmt.Errorf("route %s has no zone domains", route.SiteName)
	}
	domain := domains[0].Domain
	if route.EnableHTTPS {
		return "https://" + domain, nil
	}
	return "http://" + domain, nil
}

func monitorPayload(id int, name, targetURL string, config *kumaConfig) map[string]any {
	payload := map[string]any{
		"type":                 "http",
		"name":                 name,
		"url":                  targetURL,
		"interval":             config.Interval,
		"maxretries":           config.Retry,
		"retryInterval":        config.RetryInterval,
		"timeout":              config.Timeout,
		"active":               true,
		"resendInterval":       0,
		"expiryNotification":   false,
		"ignoreTls":            false,
		"accepted_statuscodes": []string{"200-299"},
		"dns_resolve_type":     "A",
		"conditions":           []any{},
	}
	if id > 0 {
		payload["id"] = id
	}
	return payload
}

func monitorNeedsUpdate(existing Monitor, targetURL string, config *kumaConfig) bool {
	return existing.URL != targetURL ||
		existing.Interval != config.Interval ||
		existing.MaxRetries != config.Retry ||
		existing.RetryInterval != config.RetryInterval ||
		existing.Timeout != config.Timeout
}

func createMonitor(client *SocketIOClient, siteName, targetURL string, openFlareTagID int, config *kumaConfig) error {
	slog.Info("Creating monitor in Uptime Kuma", "name", siteName, "url", targetURL)
	addAck, err := client.Emit("add", monitorPayload(0, siteName, targetURL, config))
	if err != nil {
		return err
	}

	var addResult struct {
		Ok        bool `json:"ok"`
		MonitorID int  `json:"monitorID"`
	}
	if err := ParseAckResponse(addAck, &addResult); err != nil || addResult.MonitorID == 0 {
		return fmt.Errorf("parse add monitor result failed: %w", err)
	}

	slog.Debug("Adding OpenFlare tag to the new monitor",
		"name", siteName,
		"monitor_id", addResult.MonitorID,
		"tag_id", openFlareTagID,
	)
	tagAck, err := client.Emit("addMonitorTag", openFlareTagID, addResult.MonitorID, "")
	if err != nil {
		return err
	}
	if err := ParseAckResponse(tagAck, nil); err != nil {
		return fmt.Errorf("parse add tag result failed: %w", err)
	}

	slog.Debug("OpenFlare tag successfully added to monitor", "name", siteName, "monitor_id", addResult.MonitorID)
	return nil
}

func updateMonitor(client *SocketIOClient, monitorID int, siteName, targetURL string, config *kumaConfig) error {
	slog.Info("Updating monitor in Uptime Kuma due to settings mismatch", "name", siteName)
	editAck, err := client.Emit("editMonitor", monitorPayload(monitorID, siteName, targetURL, config))
	if err != nil {
		return err
	}
	if err := ParseAckResponse(editAck, nil); err != nil {
		return fmt.Errorf("parse edit monitor result failed: %w", err)
	}
	slog.Info("Successfully updated monitor in Uptime Kuma", "name", siteName)
	return nil
}
