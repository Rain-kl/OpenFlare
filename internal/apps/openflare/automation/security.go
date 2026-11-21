// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// WAFFlow identifies a flow that can be applied to sites.
type WAFFlow struct {
	FlowID  string `json:"flow_id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Global  bool   `json:"global"`
}

// WAFFlowList contains available flows and optional ordered site bindings.
type WAFFlowList struct {
	Results      []WAFFlow `json:"results"`
	BoundFlowIDs []string  `json:"bound_flow_ids"`
}

// SiteWAFBody selects the desired state of one binding.
type SiteWAFBody struct {
	// True appends the flow if absent; false removes only this flow.
	Bound bool `json:"bound" binding:"required"`
}

// @ID openflare_waf_list
// @Summary List WAF flows and optionally website bindings
// @Description Discover existing WAF flows by name and ID. Supply site_id to read the site's explicit flow IDs in execution order. Enabled global flows apply independently of site bindings.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param site_id query string false "Site record ID from openflare_site_list"
// @Success 200 {object} OperationResponse{data=WAFFlowList}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"waf list","scope":"security:read","effect":"read","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/waf/flows [get]
func listWAFFlows(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	result := WAFFlowList{Results: make([]WAFFlow, 0), BoundFlowIDs: make([]string, 0)}
	if _, supplied := input["site_id"]; supplied {
		siteID, err := recordID(input, "site_id")
		if err != nil {
			return nil, invalidArguments(err)
		}
		if _, err = repository.GetProxyRouteByID(ctx, siteID); err != nil {
			return nil, resourceFailure(ctx, "list site WAF flows", err)
		}
		ids, err := waf.ListSiteRuleGroupIDs(ctx, siteID)
		if err != nil {
			return nil, resourceFailure(ctx, "list site WAF flows", err)
		}
		for _, id := range ids {
			result.BoundFlowIDs = append(result.BoundFlowIDs, decimalID(id))
		}
	}
	flows, err := repository.ListOpenFlareWAFRuleGroups(ctx)
	if err != nil {
		return nil, resourceFailure(ctx, "list WAF flows", err)
	}
	for _, flow := range flows {
		result.Results = append(result.Results, WAFFlow{FlowID: decimalID(flow.ID), Name: flow.Name, Enabled: flow.Enabled, Global: flow.IsGlobal})
	}
	return result, nil
}

// @ID openflare_site_set_waf
// @Summary Bind or unbind one WAF flow on a website
// @Description Append one existing non-global WAF flow or remove its binding. Other flows retain their order. A disabled flow remains disabled. Pass flow_id from openflare_waf_list, not an attack protection group ID.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Param flow_id path string true "WAF flow ID from openflare_waf_list"
// @Param body body SiteWAFBody true "Whether the flow should be bound"
// @Success 200 {object} OperationResponse{data=SiteSaved}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"site set-waf","scope":"security:write","effect":"write","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"},"flow_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/sites/{site_id}/waf/{flow_id} [patch]
func setSiteWAF(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	siteID, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	flowID, err := recordID(input, "flow_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	err = repository.SetOpenFlareWAFSiteRuleGroup(ctx, siteID, flowID, input["bound"] == "true")
	if errors.Is(err, repository.ErrWAFGroupGlobal) {
		return nil, &operationError{http.StatusConflict, "GLOBAL_WAF_FLOW", err.Error()}
	}
	return siteSaved(ctx, siteID, "set site WAF flow", err)
}

// AttackGroup describes the category levels and rule overrides used by a WAF flow.
type AttackGroup struct {
	GroupID       string            `json:"group_id"`
	Name          string            `json:"name"`
	Levels        map[string]string `json:"levels"`
	RuleOverrides map[string]string `json:"rule_overrides"`
}

// AttackGroupList contains existing attack protection groups.
type AttackGroupList struct {
	Results []AttackGroup `json:"results"`
}

// AttackGroupSaved confirms the configuration stored on the controller.
type AttackGroupSaved struct {
	Group AttackGroup `json:"group"`
	State string      `json:"state"`
}

// AttackGroupDeleted identifies the group removed from the controller.
type AttackGroupDeleted struct {
	GroupID string `json:"group_id"`
	State   string `json:"state"`
}

func summarizeAttackGroup(item model.OpenFlareAttackGroup) AttackGroup {
	return AttackGroup{GroupID: decimalID(item.ID), Name: item.Name, Levels: item.Levels, RuleOverrides: item.RuleOverrides}
}

// @ID openflare_attack_group_list
// @Summary List attack protection rule groups
// @Description Read group IDs, category levels, and per-rule overrides. WAF flows reference these groups through security check nodes; website bindings use flow IDs.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Success 200 {object} OperationResponse{data=AttackGroupList}
// @Failure 401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"attack-group list","scope":"security:read","effect":"read"}
// @Router /api/v1/automation/waf/attack-groups [get]
func listAttackGroups(ctx context.Context, _ identity, _ map[string]string) (any, *operationError) {
	items, err := repository.ListOpenFlareAttackGroups(ctx, nil)
	if err != nil {
		return nil, operationFailed(ctx, "list attack protection groups", err)
	}
	result := AttackGroupList{Results: make([]AttackGroup, 0, len(items))}
	for _, item := range items {
		result.Results = append(result.Results, summarizeAttackGroup(item))
	}
	return result, nil
}

// @ID openflare_attack_group_create
// @Summary Create an attack protection rule group
// @Description Create a group for use in WAF security check nodes. This operation does not attach the group to a flow or a site.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param body body waf.AttackGroupInput true "Attack protection group"
// @Success 200 {object} OperationResponse{data=AttackGroupSaved}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"attack-group create","scope":"security:write","effect":"write"}
// @Router /api/v1/automation/waf/attack-groups [post]
func createAttackGroup(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	return writeAttackGroup(ctx, "openflare_attack_group_create", 0, input)
}

// @ID openflare_attack_group_update
// @Summary Update an attack protection rule group
// @Description Replace the name and category levels. Omitted categories are disabled. Omit rule_overrides to keep existing per-rule settings; an empty object clears them. Changes affect all WAF flows that use this group.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param group_id path string true "Attack protection group ID from openflare_attack_group_list"
// @Param body body waf.AttackGroupInput true "Replacement group settings"
// @Success 200 {object} OperationResponse{data=AttackGroupSaved}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"attack-group update","scope":"security:write","effect":"write","parameters":{"group_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/waf/attack-groups/{group_id} [patch]
func updateAttackGroup(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "group_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	return writeAttackGroup(ctx, "openflare_attack_group_update", id, input)
}

func writeAttackGroup(ctx context.Context, operation string, id uint, input map[string]string) (any, *operationError) {
	var body waf.AttackGroupInput
	if err := decodeBodyFields(operation, input, &body); err != nil {
		return nil, invalidArguments(err)
	}
	item, err := waf.SaveAttackGroup(ctx, id, body)
	if err != nil {
		var validation *waf.RuleValidationError
		if errors.As(err, &validation) {
			return nil, invalidArguments(validation)
		}
		return nil, resourceFailure(ctx, operation, err)
	}
	return AttackGroupSaved{Group: summarizeAttackGroup(*item), State: savedState}, nil
}

// @ID openflare_attack_group_delete
// @Summary Delete an unused attack protection rule group
// @Description Delete the selected group. Deletion is rejected while any WAF flow references it, including disabled and global flows.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param group_id path string true "Attack protection group ID from openflare_attack_group_list"
// @Success 200 {object} OperationResponse{data=AttackGroupDeleted}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"attack-group delete","scope":"security:write","effect":"write","parameters":{"group_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/waf/attack-groups/{group_id} [delete]
func deleteAttackGroup(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "group_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	if err := waf.DeleteAttackGroup(ctx, id); err != nil {
		if errors.Is(err, repository.ErrAttackGroupReferenced) {
			return nil, &operationError{http.StatusConflict, "ATTACK_GROUP_REFERENCED", "Remove the group's references from WAF flows before deleting it."}
		}
		return nil, resourceFailure(ctx, "delete attack protection group", err)
	}
	return AttackGroupDeleted{GroupID: decimalID(id), State: savedState}, nil
}
