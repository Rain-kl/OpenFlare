// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/protocol"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
)

type activeConfigSnapshot struct {
	WAF openrestyrender.WAFDocument `json:"waf"`
}

type runtimeIPMatchConfig struct {
	IPs        []string `json:"ips,omitempty"`
	CIDRs      []string `json:"cidrs,omitempty"`
	IPGroupIDs []uint   `json:"ip_group_ids,omitempty"`
}

// WAFIPGroupsForAgent builds agent-facing WAF IP group payloads for the given ids.
func WAFIPGroupsForAgent(ctx context.Context, ids []uint) ([]WAFIPGroup, error) {
	return validatedAgentWAFIPGroups(ctx, ids, false)
}

// ChangedWAFIPGroupsForAgent returns WAF IP groups whose checksums differ from the agent state.
func ChangedWAFIPGroupsForAgent(ctx context.Context, ids []uint, checksums map[string]string) ([]WAFIPGroup, error) {
	groups, err := validatedAgentWAFIPGroups(ctx, ids, true)
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

func validatedAgentWAFIPGroups(ctx context.Context, ids []uint, fallbackToActive bool) ([]WAFIPGroup, error) {
	targetIDs := uniqueUintIDs(ids)
	activeIDs, err := activeConfigWAFIPGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(targetIDs) == 0 && fallbackToActive {
		targetIDs = activeIDs
	}
	if len(targetIDs) == 0 {
		return []WAFIPGroup{}, nil
	}

	validationIDs := uniqueUintIDs(append(append([]uint{}, activeIDs...), targetIDs...))
	allGroups, err := buildAgentWAFIPGroups(ctx, validationIDs)
	if err != nil {
		return nil, err
	}
	runtimeGroups := make(map[string]protocol.WAFIPGroup, len(allGroups))
	for _, group := range allGroups {
		runtimeGroups[strconv.FormatUint(uint64(group.ID), 10)] = group
	}
	if err = protocol.ValidateWAFIPGroupSnapshotSize(runtimeGroups); err != nil {
		return nil, err
	}

	targetSet := make(map[uint]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		targetSet[id] = struct{}{}
	}
	result := make([]WAFIPGroup, 0, len(targetIDs))
	for _, group := range allGroups {
		if _, ok := targetSet[group.ID]; ok {
			result = append(result, group)
		}
	}
	return result, nil
}

func buildAgentWAFIPGroups(ctx context.Context, ids []uint) ([]WAFIPGroup, error) {
	ids = uniqueUintIDs(ids)
	if len(ids) == 0 {
		return []WAFIPGroup{}, nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	groups, err := repository.ListOpenFlareWAFIPGroupsByIDs(ctx, ids)
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
	version, err := repository.GetActiveConfigVersion(ctx)
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
		// Retain legacy flattened references while older active snapshots may
		// still exist during a rolling Server upgrade.
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
		for nodeID, node := range group.Graph.Nodes {
			if node.Type != "ip_match" {
				continue
			}
			ids, err := runtimeIPMatchGroupIDs(node.Config)
			if err != nil {
				return nil, fmt.Errorf("活动配置 WAF 规则 %d 节点 %s 的 IP 匹配配置无效: %w", group.ID, nodeID, err)
			}
			for _, id := range ids {
				if id > 0 {
					idSet[id] = struct{}{}
				}
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

func runtimeIPMatchGroupIDs(raw json.RawMessage) ([]uint, error) {
	var config runtimeIPMatchConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	return config.IPGroupIDs, nil
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
		snapshot.WAF.RuleGroups = []openrestyrender.WAFRuleGroup{}
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
