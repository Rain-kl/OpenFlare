package service

import (
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/utils/uptimekuma"
)

var isSyncing atomic.Bool

func SyncToUptimeKuma() error {
	if !common.UptimeKumaEnabled {
		return fmt.Errorf("Uptime Kuma integration is disabled")
	}

	if !isSyncing.CompareAndSwap(false, true) {
		return fmt.Errorf("sync task is already in progress, please try again later")
	}
	defer isSyncing.Store(false)

	kumaUrl := strings.TrimSpace(common.UptimeKumaUrl)
	kumaUsername := strings.TrimSpace(common.UptimeKumaUsername)
	kumaPassword := strings.TrimSpace(common.UptimeKumaPassword)
	if kumaUrl == "" || kumaUsername == "" || kumaPassword == "" {
		return fmt.Errorf("Uptime Kuma URL, username, or password is not configured (URL: %q, Username: %q, PasswordLength: %d)", kumaUrl, kumaUsername, len(kumaPassword))
	}

	slog.Info("Starting Uptime Kuma sync process", "url", kumaUrl, "username", kumaUsername, "scope", common.UptimeKumaMonitorScope)

	// 1. Fetch expected sites
	allRoutes, err := model.ListProxyRoutes()
	if err != nil {
		return fmt.Errorf("failed to list local proxy routes: %w", err)
	}

	var expectedRoutes []*model.ProxyRoute
	scope := common.UptimeKumaMonitorScope
	if scope == "selected" {
		selectedList := strings.Split(common.UptimeKumaSelectedSites, ",")
		selectedMap := make(map[string]bool)
		for _, name := range selectedList {
			trimmedName := strings.TrimSpace(name)
			if trimmedName != "" {
				selectedMap[trimmedName] = true
			}
		}
		for _, route := range allRoutes {
			if route.Enabled && selectedMap[route.SiteName] {
				expectedRoutes = append(expectedRoutes, route)
			}
		}
	} else {
		for _, route := range allRoutes {
			if route.Enabled {
				expectedRoutes = append(expectedRoutes, route)
			}
		}
	}

	// 2. Connect to Uptime Kuma
	slog.Debug("Connecting to Uptime Kuma socket endpoint", "url", kumaUrl)
	client := uptimekuma.NewSocketIOClient(kumaUrl)
	if err := client.Connect(); err != nil {
		slog.Error("Failed to connect to Uptime Kuma endpoint", "url", kumaUrl, "error", err)
		return fmt.Errorf("failed to connect to Uptime Kuma: %w", err)
	}
	defer client.Close()

	// 3. Login
	slog.Debug("Sending login request to Uptime Kuma", "username", kumaUsername)
	var loginAck string
	loginPayload := map[string]string{
		"username": kumaUsername,
		"password": kumaPassword,
	}
	loginAck, err = client.Emit("login", loginPayload)
	if err != nil {
		slog.Error("Failed to send login request to Uptime Kuma", "username", kumaUsername, "error", err)
		return fmt.Errorf("login request failed: %w", err)
	}

	var loginResult struct {
		Ok bool `json:"ok"`
	}
	if err := uptimekuma.ParseAckResponse(loginAck, &loginResult); err != nil || !loginResult.Ok {
		slog.Error("Uptime Kuma login verification failed", "username", kumaUsername, "error", err)
		return fmt.Errorf("login failed: %w", err)
	}
	slog.Debug("Successfully logged into Uptime Kuma", "username", kumaUsername)

	// 4. Wait for monitor list event
	slog.Debug("Waiting for monitor list push from Uptime Kuma")
	select {
	case <-client.GetMonitorListChan():
		slog.Debug("Received monitor list from Uptime Kuma")
	case <-time.After(5 * time.Second):
		slog.Error("Timeout waiting for Uptime Kuma monitorList push event")
		return fmt.Errorf("timeout waiting for monitorList event from Uptime Kuma")
	}

	// 5. Get existing tags to find "OpenFlare"
	slog.Debug("Fetching tags from Uptime Kuma")
	tagsAck, err := client.Emit("getTags")
	if err != nil {
		slog.Error("Failed to request tags from Uptime Kuma", "error", err)
		return fmt.Errorf("failed to fetch tags: %w", err)
	}

	var tagsResult struct {
		Ok   bool                           `json:"ok"`
		Tags []uptimekuma.UptimeKumaTagItem `json:"tags"`
	}
	if err := uptimekuma.ParseAckResponse(tagsAck, &tagsResult); err != nil {
		slog.Error("Failed to parse tags response from Uptime Kuma", "error", err)
		return fmt.Errorf("parse tags response failed: %w", err)
	}

	var openFlareTagID int
	for _, t := range tagsResult.Tags {
		if t.Name == "OpenFlare" {
			openFlareTagID = t.ID
			break
		}
	}

	// Create "OpenFlare" tag if not exists
	if openFlareTagID == 0 {
		slog.Debug("OpenFlare tag not found, creating new tag")
		addTagAck, err := client.Emit("addTag", map[string]string{
			"name":  "OpenFlare",
			"color": "#4f46e5",
		})
		if err != nil {
			slog.Error("Failed to create OpenFlare tag in Uptime Kuma", "error", err)
			return fmt.Errorf("failed to create tag: %w", err)
		}
		var tagResult struct {
			Ok  bool `json:"ok"`
			Tag struct {
				ID int `json:"id"`
			} `json:"tag"`
		}
		if err := uptimekuma.ParseAckResponse(addTagAck, &tagResult); err != nil || tagResult.Tag.ID == 0 {
			slog.Error("Failed to parse addTag response from Uptime Kuma", "error", err)
			return fmt.Errorf("parse addTag response failed: %w", err)
		}
		openFlareTagID = tagResult.Tag.ID
		slog.Debug("Successfully created OpenFlare tag", "tag_id", openFlareTagID)
	} else {
		slog.Debug("Found existing OpenFlare tag", "tag_id", openFlareTagID)
	}

	// 6. Filter existing monitors by "OpenFlare" tag
	existingOpenFlareMonitors := make(map[string]uptimekuma.UptimeKumaMonitor)
	monitors := client.GetMonitorList()
	for _, m := range monitors {
		hasOpenFlareTag := false
		for _, tag := range m.Tags {
			if tag.Name == "OpenFlare" || tag.ID == openFlareTagID {
				hasOpenFlareTag = true
				break
			}
		}
		if hasOpenFlareTag {
			existingOpenFlareMonitors[m.Name] = m
		}
	}

	// Helper to format route URL
	getRouteURL := func(route *model.ProxyRoute) string {
		domains, err := decodeStoredDomains(route.Domains, route.Domain)
		domain := route.Domain
		if err == nil && len(domains) > 0 {
			domain = domains[0]
		}
		if route.EnableHTTPS {
			return "https://" + domain
		}
		return "http://" + domain
	}

	expectedSitesMap := make(map[string]bool)

	// 7. Sync Loop
	for _, route := range expectedRoutes {
		expectedSitesMap[route.SiteName] = true
		targetURL := getRouteURL(route)

		existing, exists := existingOpenFlareMonitors[route.SiteName]
		if !exists {
			// Create monitor
			slog.Info("Creating monitor in Uptime Kuma", "name", route.SiteName, "url", targetURL)
			monitorPayload := map[string]any{
				"type":                 "http",
				"name":                 route.SiteName,
				"url":                  targetURL,
				"interval":             common.UptimeKumaInterval,
				"maxretries":           common.UptimeKumaRetry,
				"retryInterval":        common.UptimeKumaRetryInterval,
				"timeout":              common.UptimeKumaTimeout,
				"active":               true,
				"resendInterval":       0,
				"expiryNotification":   false,
				"ignoreTls":            false,
				"accepted_statuscodes": []string{"200-299"},
				"dns_resolve_type":     "A",
				"conditions":           []any{},
			}
			addAck, err := client.Emit("add", monitorPayload)
			if err != nil {
				slog.Error("Failed to add monitor to Uptime Kuma", "name", route.SiteName, "error", err)
				continue
			}
			var addResult struct {
				Ok        bool `json:"ok"`
				MonitorID int  `json:"monitorID"`
			}
			if err := uptimekuma.ParseAckResponse(addAck, &addResult); err != nil || addResult.MonitorID == 0 {
				slog.Error("Failed to parse add monitor result", "name", route.SiteName, "error", err)
				continue
			}

			// Add tag
			slog.Debug("Adding OpenFlare tag to the new monitor", "name", route.SiteName, "monitor_id", addResult.MonitorID, "tag_id", openFlareTagID)
			tagAck, err := client.Emit("addMonitorTag", openFlareTagID, addResult.MonitorID, "")
			if err != nil {
				slog.Error("Failed to add tag to monitor in Uptime Kuma", "name", route.SiteName, "monitorID", addResult.MonitorID, "error", err)
			} else {
				if err := uptimekuma.ParseAckResponse(tagAck, nil); err != nil {
					slog.Error("Failed to parse add tag result", "name", route.SiteName, "monitorID", addResult.MonitorID, "error", err)
				} else {
					slog.Debug("OpenFlare tag successfully added to monitor", "name", route.SiteName, "monitor_id", addResult.MonitorID)
				}
			}
		} else {
			// Check if updates are needed
			needsUpdate := existing.Url != targetURL ||
				existing.Interval != common.UptimeKumaInterval ||
				existing.MaxRetries != common.UptimeKumaRetry ||
				existing.RetryInterval != common.UptimeKumaRetryInterval ||
				existing.Timeout != common.UptimeKumaTimeout

			if needsUpdate {
				slog.Info("Updating monitor in Uptime Kuma due to settings mismatch",
					"name", route.SiteName,
					"url_changed", existing.Url != targetURL,
					"interval_changed", existing.Interval != common.UptimeKumaInterval,
					"max_retries_changed", existing.MaxRetries != common.UptimeKumaRetry,
					"retry_interval_changed", existing.RetryInterval != common.UptimeKumaRetryInterval,
					"timeout_changed", existing.Timeout != common.UptimeKumaTimeout,
				)
				monitorPayload := map[string]any{
					"id":                   existing.ID,
					"type":                 "http",
					"name":                 route.SiteName,
					"url":                  targetURL,
					"interval":             common.UptimeKumaInterval,
					"maxretries":           common.UptimeKumaRetry,
					"retryInterval":        common.UptimeKumaRetryInterval,
					"timeout":              common.UptimeKumaTimeout,
					"active":               true,
					"resendInterval":       0,
					"expiryNotification":   false,
					"ignoreTls":            false,
					"accepted_statuscodes": []string{"200-299"},
					"dns_resolve_type":     "A",
					"conditions":           []any{},
				}
				editAck, err := client.Emit("editMonitor", monitorPayload)
				if err != nil {
					slog.Error("Failed to edit monitor in Uptime Kuma", "name", route.SiteName, "error", err)
				} else {
					if err := uptimekuma.ParseAckResponse(editAck, nil); err != nil {
						slog.Error("Failed to parse edit monitor result", "name", route.SiteName, "error", err)
					} else {
						slog.Info("Successfully updated monitor in Uptime Kuma", "name", route.SiteName)
					}
				}
			}
		}
	}

	// 8. Delete Loop
	for name, m := range existingOpenFlareMonitors {
		if !expectedSitesMap[name] {
			slog.Info("Deleting monitor in Uptime Kuma", "name", name, "monitorID", m.ID)
			deleteAck, err := client.Emit("deleteMonitor", m.ID)
			if err != nil {
				slog.Error("Failed to delete monitor in Uptime Kuma", "name", name, "monitorID", m.ID, "error", err)
			} else {
				if err := uptimekuma.ParseAckResponse(deleteAck, nil); err != nil {
					slog.Error("Failed to parse delete monitor result", "name", name, "monitorID", m.ID, "error", err)
				}
			}
		}
	}

	return nil
}
