// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const attackLevelOff = "off"

var attackCategories = map[string]struct{}{
	"sql_injection": {}, "xss": {}, "malicious_upload": {}, "path_traversal": {},
	"file_inclusion": {}, "command_injection": {}, "ssrf": {}, "xxe": {}, "crlf_injection": {},
	"java_injection": {}, "java_deserialization": {}, "php_injection": {},
	"php_deserialization": {}, "asp_injection": {}, "template_injection": {},
	"csrf": {}, "malformed_http": {},
}

type attackGroupInput struct {
	Name          string            `json:"name"`
	Levels        map[string]string `json:"levels"`
	RuleOverrides map[string]string `json:"rule_overrides"`
}

func saveAttackGroup(ctx context.Context, id uint, input attackGroupInput) (*model.OpenFlareAttackGroup, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 || strings.ContainsRune(name, '\x00') {
		return nil, &RuleValidationError{Err: errors.New(errAttackGroupName)}
	}
	if input.Levels == nil {
		return nil, &RuleValidationError{Err: errors.New(errAttackGroupLevels)}
	}
	for category, level := range input.Levels {
		_, validCategory := attackCategories[category]
		if !validCategory || (level != attackLevelOff && level != "observe" && level != "balanced" && level != "strict") {
			return nil, &RuleValidationError{Err: errors.New(errAttackGroupLevels)}
		}
	}
	for key, action := range input.RuleOverrides {
		ruleID, err := strconv.ParseUint(key, 10, 32)
		_, known := slices.BinarySearch(attackRuleIDs, uint32(ruleID))
		if err != nil || !known || key != strconv.FormatUint(ruleID, 10) ||
			(action != attackLevelOff && action != "enabled" && action != "observe") {
			return nil, &RuleValidationError{Err: fmt.Errorf(errAttackRuleOverride, key)}
		}
	}
	group := &model.OpenFlareAttackGroup{ID: id, Name: name, Levels: input.Levels, RuleOverrides: input.RuleOverrides}
	if err := repository.SaveOpenFlareAttackGroup(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
}

func loadAttackGroupReferences(ctx context.Context, graphs ...RuntimeRuleGraph) (map[uint]model.OpenFlareAttackGroup, error) {
	var ids []uint
	for _, graph := range graphs {
		for _, node := range graph.Nodes {
			if config, ok := node.Config.(SecurityCheckConfig); ok && config.AttackGroupID != nil {
				ids = append(ids, *config.AttackGroupID)
			}
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	groups, err := repository.ListOpenFlareAttackGroups(ctx, sortedUniqueUints(ids))
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]model.OpenFlareAttackGroup, len(groups))
	for _, group := range groups {
		byID[group.ID] = group
	}
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return nil, &RuleValidationError{Err: fmt.Errorf(errAttackGroupReference, id)}
		}
	}
	return byID, nil
}

// ResolveAttackGroups copies referenced policies into the snapshot. Only upgraded Edge
// binaries understand the explicit runtime node, so older nodes reject it atomically.
func ResolveAttackGroups(ctx context.Context, graphs ...RuntimeRuleGraph) error {
	groups, err := loadAttackGroupReferences(ctx, graphs...)
	if err != nil {
		return err
	}
	for _, graph := range graphs {
		for id, node := range graph.Nodes {
			if cfg, ok := node.Config.(SecurityCheckConfig); ok && cfg.AttackGroupID != nil {
				group := groups[*cfg.AttackGroupID]
				levels := maps.Clone(group.Levels)
				for _, category := range []string{"csrf", "malformed_http"} {
					if levels[category] == attackLevelOff {
						delete(levels, category)
					}
				}
				node.Type = "attack_protection"
				node.Config = runtimeAttackGroup{
					ID: group.ID, Name: group.Name, Levels: levels, RuleOverrides: group.RuleOverrides,
				}
				graph.Nodes[id] = node
			}
		}
	}
	return nil
}

type runtimeAttackGroup struct {
	ID            uint              `json:"id"`
	Name          string            `json:"name"`
	Levels        map[string]string `json:"levels"`
	RuleOverrides map[string]string `json:"rule_overrides,omitempty"`
}
