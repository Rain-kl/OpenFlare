// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
)

// IPGroupSummary reports group metadata without subscription credentials.
type IPGroupSummary struct {
	GroupID        string `json:"group_id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Enabled        bool   `json:"enabled"`
	EntryCount     int    `json:"entry_count"`
	SyncedCount    int    `json:"synced_entry_count"`
	FlowReferences int    `json:"flow_reference_count"`
	LastSyncedAt   string `json:"last_synced_at,omitempty"`
	NextSyncAt     string `json:"next_sync_at,omitempty"`
	LastSyncStatus string `json:"last_sync_status,omitempty"`
}

// IPGroupList contains summaries without individual entries.
type IPGroupList struct {
	Results []IPGroupSummary `json:"results"`
}

// IPGroupDetails includes both configured and synchronized entries.
type IPGroupDetails struct {
	Group   IPGroupSummary         `json:"group"`
	Entries []string               `json:"entries"`
	Synced  []waf.IPGroupExtIPView `json:"synced_entries"`
}

func summarizeIPGroup(group waf.IPGroupView) IPGroupSummary {
	return IPGroupSummary{
		GroupID: decimalID(group.ID), Name: group.Name, Type: group.Type, Enabled: group.Enabled,
		EntryCount: len(group.IPList), SyncedCount: len(group.ExtIPs), FlowReferences: group.ReferencedByRuleCount,
		LastSyncedAt: group.LastSyncedAt, NextSyncAt: group.NextSyncAt, LastSyncStatus: group.LastSyncStatus,
	}
}

// @ID openflare_ip_group_list
// @Summary List WAF IP groups
// @Description Read IP group IDs, entry counts, flow reference counts, and synchronization status. Use openflare_ip_group_get to read the entries of a selected group.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Success 200 {object} OperationResponse{data=IPGroupList}
// @Failure 401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"ip-group list","scope":"security:read","effect":"read"}
// @Router /api/v1/automation/waf/ip-groups [get]
func listIPGroups(ctx context.Context, _ identity, _ map[string]string) (any, *operationError) {
	groups, err := waf.ListIPGroups(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list IP groups", err)
	}
	result := IPGroupList{Results: make([]IPGroupSummary, 0, len(groups))}
	for _, group := range groups {
		result.Results = append(result.Results, summarizeIPGroup(group))
	}
	return result, nil
}

// @ID openflare_ip_group_get
// @Summary Read entries of a WAF IP group
// @Description Return manual IP addresses and CIDRs, synchronized entries, and refresh status. Subscription URLs and source configurations are omitted.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param group_id path string true "IP group ID from openflare_ip_group_list"
// @Success 200 {object} OperationResponse{data=IPGroupDetails}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"ip-group get","scope":"security:read","effect":"read","parameters":{"group_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/waf/ip-groups/{group_id} [get]
func getIPGroup(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "group_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	group, err := waf.GetIPGroup(ctx, id)
	if err != nil {
		return nil, resourceFailure(ctx, "get IP group", err)
	}
	return IPGroupDetails{Group: summarizeIPGroup(*group), Entries: group.IPList, Synced: group.ExtIPs}, nil
}
