// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

type snapshotWAFRuleGroupRef struct {
	IPWhitelistGroups []uint `json:"ip_whitelist_group_ids,omitempty"`
	IPBlacklistGroups []uint `json:"ip_blacklist_group_ids,omitempty"`
}

type snapshotWAFSection struct {
	RuleGroups []snapshotWAFRuleGroupRef `json:"rule_groups"`
}

type activeConfigSnapshot struct {
	WAF snapshotWAFSection `json:"waf"`
}

// WAFIPGroupsForAgent builds agent-facing WAF IP group payloads for the given ids.
func WAFIPGroupsForAgent(ctx context.Context, ids []uint) ([]WAFIPGroup, error) {
	return buildAgentWAFIPGroups(ctx, ids)
}

// ChangedWAFIPGroupsForAgent returns WAF IP groups whose checksums differ from the agent state.
func ChangedWAFIPGroupsForAgent(ctx context.Context, ids []uint, checksums map[string]string) ([]WAFIPGroup, error) {
	targetIDs := uniqueUintIDs(ids)
	if len(targetIDs) == 0 {
		activeIDs, err := activeConfigWAFIPGroupIDs(ctx)
		if err != nil {
			return nil, err
		}
		targetIDs = activeIDs
	}
	if len(targetIDs) == 0 {
		return []WAFIPGroup{}, nil
	}
	groups, err := buildAgentWAFIPGroups(ctx, targetIDs)
	if err != nil {
		return nil, err
	}
	changed := make([]WAFIPGroup, 0, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(checksums[fmt.Sprintf("%d", group.ID)]) == group.Checksum {
			continue
		}
		changed = append(changed, group)
	}
	return changed, nil
}

func buildAgentWAFIPGroups(ctx context.Context, ids []uint) ([]WAFIPGroup, error) {
	ids = uniqueUintIDs(ids)
	if len(ids) == 0 {
		return []WAFIPGroup{}, nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	groups, err := model.ListOpenFlareWAFIPGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	groupByID := make(map[uint]*model.OpenFlareWAFIPGroup, len(groups))
	for _, group := range groups {
		groupByID[group.ID] = group
	}
	result := make([]WAFIPGroup, 0, len(ids))
	for _, id := range ids {
		group := groupByID[id]
		if group == nil {
			continue
		}
		agentGroup, err := buildAgentWAFIPGroup(group)
		if err != nil {
			return nil, err
		}
		result = append(result, agentGroup)
	}
	return result, nil
}

func buildAgentWAFIPGroup(group *model.OpenFlareWAFIPGroup) (WAFIPGroup, error) {
	if group == nil {
		return WAFIPGroup{}, errors.New("IP 组不存在")
	}
	ips, err := decodeWAFIPGroupStringList(group.IPList)
	if err != nil {
		return WAFIPGroup{}, err
	}
	if !group.Enabled {
		ips = []string{}
	}
	agentGroup := WAFIPGroup{
		ID:      group.ID,
		Name:    group.Name,
		Type:    group.Type,
		Enabled: group.Enabled,
		IPList:  ips,
	}
	agentGroup.Checksum = checksumAgentWAFIPGroup(agentGroup)
	return agentGroup, nil
}

func checksumAgentWAFIPGroup(group WAFIPGroup) string {
	payload := struct {
		ID      uint     `json:"id"`
		Enabled bool     `json:"enabled"`
		IPList  []string `json:"ip_list"`
	}{
		ID:      group.ID,
		Enabled: group.Enabled,
		IPList:  append([]string{}, group.IPList...),
	}
	sort.Strings(payload.IPList)
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func activeConfigWAFIPGroupIDs(ctx context.Context) ([]uint, error) {
	version, err := loadActiveConfigVersion(ctx)
	if err != nil {
		if isActiveConfigNotFound(err) {
			return []uint{}, nil
		}
		return nil, err
	}
	snapshot, err := parseActiveConfigSnapshot(version.SnapshotJSON)
	if err != nil {
		return nil, err
	}
	idSet := make(map[uint]struct{})
	for _, group := range snapshot.WAF.RuleGroups {
		for _, id := range group.IPWhitelistGroups {
			if id > 0 {
				idSet[id] = struct{}{}
			}
		}
		for _, id := range group.IPBlacklistGroups {
			if id > 0 {
				idSet[id] = struct{}{}
			}
		}
	}
	ids := make([]uint, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func parseActiveConfigSnapshot(snapshotJSON string) (*activeConfigSnapshot, error) {
	text := strings.TrimSpace(snapshotJSON)
	if text == "" {
		return &activeConfigSnapshot{}, nil
	}
	var snapshot activeConfigSnapshot
	if err := json.Unmarshal([]byte(text), &snapshot); err != nil {
		return nil, err
	}
	if snapshot.WAF.RuleGroups == nil {
		snapshot.WAF.RuleGroups = []snapshotWAFRuleGroupRef{}
	}
	return &snapshot, nil
}

func decodeWAFIPGroupStringList(raw string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []string{}, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func uniqueUintIDs(ids []uint) []uint {
	normalized := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}
