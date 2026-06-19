// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

var isSyncing atomic.Bool

// SyncToUptimeKuma synchronizes enabled proxy routes to Uptime Kuma monitors.
func SyncToUptimeKuma(ctx context.Context) error {
	if !model.UptimeKumaEnabled {
		return fmt.Errorf("Uptime Kuma integration is disabled")
	}

	if !isSyncing.CompareAndSwap(false, true) {
		return fmt.Errorf("sync task is already in progress, please try again later")
	}
	defer isSyncing.Store(false)

	kumaURL := strings.TrimSpace(model.UptimeKumaUrl)
	kumaUsername := strings.TrimSpace(model.UptimeKumaUsername)
	kumaPassword := strings.TrimSpace(model.UptimeKumaPassword)
	if kumaURL == "" || kumaUsername == "" || kumaPassword == "" {
		return fmt.Errorf(
			"Uptime Kuma URL, username, or password is not configured (URL: %q, Username: %q, PasswordLength: %d)",
			kumaURL, kumaUsername, len(kumaPassword),
		)
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

	slog.Debug("Connecting to Uptime Kuma socket endpoint", "url", kumaURL)
	client := NewSocketIOClient(kumaURL)
	if err := client.Connect(); err != nil {
		slog.Error("Failed to connect to Uptime Kuma endpoint", "url", kumaURL, "error", err)
		return fmt.Errorf("failed to connect to Uptime Kuma: %w", err)
	}
	defer client.Close()

	slog.Debug("Sending login request to Uptime Kuma", "username", kumaUsername)
	loginPayload := map[string]string{
		"username": kumaUsername,
		"password": kumaPassword,
	}
	loginAck, err := client.Emit("login", loginPayload)
	if err != nil {
		slog.Error("Failed to send login request to Uptime Kuma", "username", kumaUsername, "error", err)
		return fmt.Errorf("login request failed: %w", err)
	}

	var loginResult struct {
		Ok bool `json:"ok"`
	}
	if err := ParseAckResponse(loginAck, &loginResult); err != nil || !loginResult.Ok {
		slog.Error("Uptime Kuma login verification failed", "username", kumaUsername, "error", err)
		return fmt.Errorf("login failed: %w", err)
	}
	slog.Debug("Successfully logged into Uptime Kuma", "username", kumaUsername)

	slog.Debug("Waiting for monitor list push from Uptime Kuma")
	select {
	case <-client.GetMonitorListChan():
		slog.Debug("Received monitor list from Uptime Kuma")
	case <-time.After(5 * time.Second):
		slog.Error("Timeout waiting for Uptime Kuma monitorList push event")
		return fmt.Errorf("timeout waiting for monitorList event from Uptime Kuma")
	}

	openFlareTagID, err := ensureOpenFlareTag(client)
	if err != nil {
		return err
	}

	existingOpenFlareMonitors := filterOpenFlareMonitors(client.GetMonitorList(), openFlareTagID)
	expectedSitesMap := make(map[string]bool)

	for _, route := range expectedRoutes {
		expectedSitesMap[route.SiteName] = true
		targetURL, urlErr := routeMonitorURL(route)
		if urlErr != nil {
			slog.Error("Failed to resolve monitor URL", "name", route.SiteName, "error", urlErr)
			continue
		}

		existing, exists := existingOpenFlareMonitors[route.SiteName]
		if !exists {
			if err := createMonitor(client, route.SiteName, targetURL, openFlareTagID); err != nil {
				slog.Error("Failed to add monitor to Uptime Kuma", "name", route.SiteName, "error", err)
			}
			continue
		}

		if monitorNeedsUpdate(existing, targetURL) {
			if err := updateMonitor(client, existing.ID, route.SiteName, targetURL); err != nil {
				slog.Error("Failed to edit monitor in Uptime Kuma", "name", route.SiteName, "error", err)
			}
		}
	}

	for name, monitor := range existingOpenFlareMonitors {
		if expectedSitesMap[name] {
			continue
		}
		slog.Info("Deleting monitor in Uptime Kuma", "name", name, "monitorID", monitor.ID)
		deleteAck, err := client.Emit("deleteMonitor", monitor.ID)
		if err != nil {
			slog.Error("Failed to delete monitor in Uptime Kuma", "name", name, "monitorID", monitor.ID, "error", err)
			continue
		}
		if err := ParseAckResponse(deleteAck, nil); err != nil {
			slog.Error("Failed to parse delete monitor result", "name", name, "monitorID", monitor.ID, "error", err)
		}
	}

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
		Ok   bool                `json:"ok"`
		Tags []UptimeKumaTagItem `json:"tags"`
	}
	if err := ParseAckResponse(tagsAck, &tagsResult); err != nil {
		slog.Error("Failed to parse tags response from Uptime Kuma", "error", err)
		return 0, fmt.Errorf("parse tags response failed: %w", err)
	}

	for _, tag := range tagsResult.Tags {
		if tag.Name == "OpenFlare" {
			slog.Debug("Found existing OpenFlare tag", "tag_id", tag.ID)
			return tag.ID, nil
		}
	}

	slog.Debug("OpenFlare tag not found, creating new tag")
	addTagAck, err := client.Emit("addTag", map[string]string{
		"name":  "OpenFlare",
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

func filterOpenFlareMonitors(monitors map[string]UptimeKumaMonitor, openFlareTagID int) map[string]UptimeKumaMonitor {
	existingOpenFlareMonitors := make(map[string]UptimeKumaMonitor)
	for _, monitor := range monitors {
		hasOpenFlareTag := false
		for _, tag := range monitor.Tags {
			if tag.Name == "OpenFlare" || tag.ID == openFlareTagID {
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
	domains, err := decodeStoredDomains(route.Domains, route.Domain)
	if err != nil {
		return "", err
	}
	domain := route.Domain
	if len(domains) > 0 {
		domain = domains[0]
	}
	if route.EnableHTTPS {
		return "https://" + domain, nil
	}
	return "http://" + domain, nil
}

func decodeStoredDomains(raw string, fallbackDomain string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		if strings.TrimSpace(fallbackDomain) == "" {
			return nil, fmt.Errorf("domain is empty")
		}
		return []string{fallbackDomain}, nil
	}

	var domains []string
	if err := json.Unmarshal([]byte(text), &domains); err != nil {
		return nil, fmt.Errorf("domains payload is invalid: %w", err)
	}
	if len(domains) == 0 {
		if strings.TrimSpace(fallbackDomain) == "" {
			return nil, fmt.Errorf("domain is empty")
		}
		return []string{fallbackDomain}, nil
	}
	return domains, nil
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

func monitorNeedsUpdate(existing UptimeKumaMonitor, targetURL string) bool {
	return existing.Url != targetURL ||
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
