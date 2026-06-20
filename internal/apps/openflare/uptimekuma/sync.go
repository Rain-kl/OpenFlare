// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/routeidentity"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const uptimeKumaTagOpenFlare = "OpenFlare"

var isSyncing atomic.Bool

// SyncToUptimeKuma synchronizes enabled proxy routes to Uptime Kuma monitors.
func SyncToUptimeKuma(ctx context.Context) error {
	if !model.UptimeKumaEnabled {
		return fmt.Errorf("uptime Kuma integration is disabled")
	}

	if !isSyncing.CompareAndSwap(false, true) {
		return fmt.Errorf("sync task is already in progress, please try again later")
	}
	defer isSyncing.Store(false)

	kumaURL, kumaUsername, kumaPassword, err := validateUptimeKumaConfig()
	if err != nil {
		return err
	}

	slog.Info("Starting Uptime Kuma sync process",
		"url", kumaURL,
		"username", kumaUsername,
		"scope", model.UptimeKumaMonitorScope,
	)

	allRoutes, err := model.ListProxyRoutes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list local proxy routes: %w", err)
	}

	expectedRoutes, err := filterExpectedRoutes(allRoutes)
	if err != nil {
		return err
	}

	client, err := connectAndLoginUptimeKuma(kumaURL, kumaUsername, kumaPassword)
	if err != nil {
		return err
	}
	defer client.Close()

	openFlareTagID, err := ensureOpenFlareTag(client)
	if err != nil {
		return err
	}

	existingOpenFlareMonitors := filterOpenFlareMonitors(client.GetMonitorList(), openFlareTagID)
	expectedSitesMap := syncRouteMonitors(client, expectedRoutes, existingOpenFlareMonitors, openFlareTagID)
	removeStaleMonitors(client, existingOpenFlareMonitors, expectedSitesMap)

	return nil
}

func filterExpectedRoutes(allRoutes []*model.ProxyRoute) ([]*model.ProxyRoute, error) {
	scope := model.UptimeKumaMonitorScope
	if scope == "selected" {
		selectedList := strings.Split(model.UptimeKumaSelectedSites, ",")
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

func routeMonitorURL(route *model.ProxyRoute) (string, error) {
	domains, err := routeidentity.DecodeDomains(route.Domains, route.Domain)
	if err != nil {
		return "", err
	}
	domain := domains[0]
	if route.EnableHTTPS {
		return "https://" + domain, nil
	}
	return "http://" + domain, nil
}

func monitorPayload(id int, name, targetURL string) map[string]any {
	payload := map[string]any{
		"type":                 "http",
		"name":                 name,
		"url":                  targetURL,
		"interval":             model.UptimeKumaInterval,
		"maxretries":           model.UptimeKumaRetry,
		"retryInterval":        model.UptimeKumaRetryInterval,
		"timeout":              model.UptimeKumaTimeout,
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

func monitorNeedsUpdate(existing Monitor, targetURL string) bool {
	return existing.URL != targetURL ||
		existing.Interval != model.UptimeKumaInterval ||
		existing.MaxRetries != model.UptimeKumaRetry ||
		existing.RetryInterval != model.UptimeKumaRetryInterval ||
		existing.Timeout != model.UptimeKumaTimeout
}

func createMonitor(client *SocketIOClient, siteName, targetURL string, openFlareTagID int) error {
	slog.Info("Creating monitor in Uptime Kuma", "name", siteName, "url", targetURL)
	addAck, err := client.Emit("add", monitorPayload(0, siteName, targetURL))
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

func updateMonitor(client *SocketIOClient, monitorID int, siteName, targetURL string) error {
	slog.Info("Updating monitor in Uptime Kuma due to settings mismatch", "name", siteName)
	editAck, err := client.Emit("editMonitor", monitorPayload(monitorID, siteName, targetURL))
	if err != nil {
		return err
	}
	if err := ParseAckResponse(editAck, nil); err != nil {
		return fmt.Errorf("parse edit monitor result failed: %w", err)
	}
	slog.Info("Successfully updated monitor in Uptime Kuma", "name", siteName)
	return nil
}
