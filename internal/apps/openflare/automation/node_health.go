// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/observability"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const nodeHealthOperation = "openflare_node_health"

// NodeHealthEvent identifies a currently active alert without diagnostic payloads.
type NodeHealthEvent struct {
	Type          string    `json:"type"`
	Severity      string    `json:"severity"`
	LastTriggered time.Time `json:"last_triggered_at"`
}

// NodeHealthSummary separates connectivity, reported runtime health, and snapshot sync.
type NodeHealthSummary struct {
	Node          NodeSummary       `json:"node"`
	RuntimeHealth string            `json:"runtime_health"`
	ConfigSync    string            `json:"config_sync"`
	ActiveEvents  []NodeHealthEvent `json:"active_events"`
}

// NodeHealthPage contains health reports for a single page of nodes.
type NodeHealthPage struct {
	Total    int64               `json:"total"`
	Results  []NodeHealthSummary `json:"results"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// @ID openflare_node_health
// @Summary Check node connectivity, runtime health and site configuration sync
// @Description Return a page of nodes with heartbeat status, reported runtime health, active health events, and comparison against the active site snapshot for edge nodes. Offline nodes show the last reported runtime health. Use node metrics for resource time series.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param page query integer false "Page number, starting at 1" minimum(1) default(1)
// @Param page_size query integer false "Number of records per page" minimum(1) maximum(100) default(20)
// @Param search query string false "Literal substring of the node name, registration identifier or IP address" maxlength(256)
// @Param status query string false "Filter by heartbeat state; omit for all states" Enums(pending,online,offline,quarantined)
// @Param node_type query string false "Filter by node type; omit for all types" Enums(edge_node)
// @Success 200 {object} OperationResponse{data=NodeHealthPage}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"node health","scope":"nodes:read","effect":"read"}
// @Router /api/v1/automation/nodes/health [get]
func nodeHealth(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	page, failure := nodePage(ctx, nodeHealthOperation, input)
	if failure != nil {
		return nil, failure
	}
	result := NodeHealthPage{Total: page.Total, Page: page.Page, PageSize: page.PageSize, Results: make([]NodeHealthSummary, 0, len(page.Results))}
	nodeIDs := make([]string, 0, len(page.Results))
	hasEdge := false
	for _, view := range page.Results {
		if view.NodeID != "" {
			nodeIDs = append(nodeIDs, view.NodeID)
		}
		hasEdge = hasEdge || view.NodeType == edgeNodeType
	}
	var desiredVersion string
	if hasEdge {
		active, err := repository.GetActivePingoraConfigMetadata(ctx)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, operationFailed(ctx, nodeHealthOperation, err)
		}
		if err == nil {
			desiredVersion = active.Version
		}
	}
	events, err := repository.ListOpenFlareActiveHealthEventsByNodeIDs(ctx, nodeIDs)
	if err != nil {
		return nil, operationFailed(ctx, nodeHealthOperation, err)
	}
	byNode := observability.ActiveHealthEventsByNode(events)
	for _, view := range page.Results {
		entry := NodeHealthSummary{
			Node: summarizeNode(view), RuntimeHealth: view.OpenrestyStatus,
			ConfigSync: "not_applicable", ActiveEvents: make([]NodeHealthEvent, 0, len(byNode[view.NodeID])),
		}
		if view.NodeType == edgeNodeType {
			entry.Node.DesiredVersion = desiredVersion
			entry.ConfigSync = siteConfigSync(view.CurrentVersion, desiredVersion)
		}
		for _, event := range byNode[view.NodeID] {
			entry.ActiveEvents = append(entry.ActiveEvents, NodeHealthEvent{Type: event.EventType, Severity: event.Severity, LastTriggered: event.LastTriggeredAt})
		}
		result.Results = append(result.Results, entry)
	}
	return result, nil
}

func siteConfigSync(current, desired string) string {
	switch {
	case desired == "":
		return "no_target"
	case current == "":
		return "not_reported"
	case current == desired:
		return "synced"
	default:
		return "out_of_sync"
	}
}
