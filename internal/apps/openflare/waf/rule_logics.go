// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// CreateRuleInput is the minimal payload used to create an orchestrated rule.
type CreateRuleInput struct {
	Name string `json:"name"`
}

// SaveRuleGraphInput atomically replaces a rule graph at the supplied revision.
type SaveRuleGraphInput struct {
	Revision uint64    `json:"revision"`
	Graph    RuleGraph `json:"graph"`
}

// UpdateRuleMetaInput updates metadata without replacing the graph.
type UpdateRuleMetaInput struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// RuleValidationError represents a safe user-facing validation failure.
type RuleValidationError struct{ Err error }

func (err *RuleValidationError) Error() string { return err.Err.Error() }
func (err *RuleValidationError) Unwrap() error { return err.Err }

// RuleView is the API representation of an orchestrated WAF rule.
type RuleView struct {
	ID               uint      `json:"id"`
	Name             string    `json:"name"`
	Enabled          bool      `json:"enabled"`
	IsGlobal         bool      `json:"is_global"`
	Graph            RuleGraph `json:"graph"`
	Revision         uint64    `json:"revision"`
	AppliedSiteIDs   []uint    `json:"applied_site_ids"`
	AppliedSiteCount int       `json:"applied_site_count"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}

// RuleGroupSite identifies a proxy route and its bound domains.
type RuleGroupSite struct {
	ID       uint     `json:"id"`
	SiteName string   `json:"site_name"`
	Domains  []string `json:"domains"`
}

// RemoveRuleGroupSitesResult reports how many bindings were removed.
type RemoveRuleGroupSitesResult struct {
	RemovedCount int64 `json:"removed_count"`
}

// ListRuleGroupSites lists explicitly bound sites, or every site for the global rule.
func ListRuleGroupSites(ctx context.Context, id uint) ([]RuleGroupSite, error) {
	group, err := repository.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	routes, err := repository.ListOpenFlareWAFRuleGroupSites(ctx, id, group.IsGlobal)
	if err != nil {
		return nil, err
	}
	sites := make([]RuleGroupSite, 0, len(routes))
	for _, route := range routes {
		domains := make([]string, 0, len(route.ZoneDomains))
		for _, domain := range route.ZoneDomains {
			domains = append(domains, domain.Domain)
		}
		sites = append(sites, RuleGroupSite{ID: route.ID, SiteName: route.SiteName, Domains: domains})
	}
	return sites, nil
}

// RemoveRuleGroupSites removes selected custom rule bindings without rewriting site rules.
func RemoveRuleGroupSites(ctx context.Context, id uint, routeIDs []uint) (*RemoveRuleGroupSitesResult, error) {
	if len(routeIDs) == 0 || slices.Contains(routeIDs, 0) {
		return nil, &RuleValidationError{Err: errors.New(errInvalidRuleSiteIDs)}
	}
	group, err := repository.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if group.IsGlobal {
		return nil, &RuleValidationError{Err: errors.New(errGlobalRuleSites)}
	}
	ids := slices.Clone(routeIDs)
	slices.Sort(ids)
	removed, err := repository.DeleteOpenFlareWAFRuleGroupSites(ctx, id, slices.Compact(ids))
	if err != nil {
		return nil, err
	}
	return &RemoveRuleGroupSitesResult{RemovedCount: removed}, nil
}

// ListRules returns all orchestrated WAF rules.
func ListRules(ctx context.Context) ([]RuleView, error) {
	if err := EnsureDefaultRuleGroup(ctx); err != nil {
		return nil, err
	}
	groups, err := repository.ListOpenFlareWAFRuleGroups(ctx)
	if err != nil {
		return nil, err
	}
	bindings, err := loadRuleGroupBindings(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]RuleView, 0, len(groups))
	for _, group := range groups {
		view, buildErr := buildRuleView(group, bindings[group.ID])
		if buildErr != nil {
			return nil, buildErr
		}
		views = append(views, view)
	}
	return views, nil
}

// GetRule returns one orchestrated WAF rule.
func GetRule(ctx context.Context, id uint) (*RuleView, error) {
	group, err := repository.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	bindings, err := loadRuleGroupBindings(ctx)
	if err != nil {
		return nil, err
	}
	view, err := buildRuleView(group, bindings[group.ID])
	return &view, err
}

// CreateRule creates a disabled custom rule with the safe default graph.
func CreateRule(ctx context.Context, input CreateRuleInput) (*RuleView, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, &RuleValidationError{Err: errors.New("WAF rule name must not be empty")}
	}
	raw, err := json.Marshal(DefaultRuleGraph())
	if err != nil {
		return nil, err
	}
	group := &model.OpenFlareWAFRuleGroup{Name: name, Enabled: false, IsGlobal: false, Graph: string(raw), Revision: 1}
	if err = repository.CreateOpenFlareWAFRuleGroup(ctx, group); err != nil {
		return nil, err
	}
	// GORM applies the model's database default to a false bool on Create, so
	// explicitly persist the safe disabled state after the row has an ID.
	group.Enabled = false
	if err = repository.UpdateOpenFlareWAFRuleGroup(ctx, group); err != nil {
		return nil, err
	}
	return GetRule(ctx, group.ID)
}

// UpdateRuleMeta updates rule metadata without touching its graph revision.
func UpdateRuleMeta(ctx context.Context, id uint, input UpdateRuleMetaInput) (*RuleView, error) {
	group, err := repository.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, &RuleValidationError{Err: errors.New("WAF rule name must not be empty")}
	}
	group.Name, group.Enabled = name, input.Enabled
	if err = repository.UpdateOpenFlareWAFRuleGroup(ctx, group); err != nil {
		return nil, err
	}
	return GetRule(ctx, id)
}

// DeleteRuleGroup deletes a non-global orchestrated WAF rule.
func DeleteRuleGroup(ctx context.Context, id uint) error {
	group, err := repository.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return err
	}
	if group.IsGlobal {
		return &RuleValidationError{Err: errors.New("global WAF rule cannot be deleted")}
	}
	return repository.DeleteOpenFlareWAFRuleGroupWithBindings(ctx, id)
}

// SaveRuleGraph validates and atomically replaces a rule graph.
func SaveRuleGraph(ctx context.Context, id uint, input SaveRuleGraphInput) (*RuleView, error) {
	if _, err := repository.GetOpenFlareWAFRuleGroupByID(ctx, id); err != nil {
		return nil, err
	}
	if err := ValidateRuleGraph(ctx, input.Graph, ruleIPGroupExists); err != nil {
		return nil, &RuleValidationError{Err: fmt.Errorf("invalid rule graph: %w", err)}
	}
	graph, err := CompileRuleGraph(input.Graph)
	if err != nil {
		return nil, &RuleValidationError{Err: err}
	}
	if _, err = loadAttackGroupReferences(ctx, graph); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(input.Graph)
	if err != nil {
		return nil, err
	}
	if _, err = repository.UpdateOpenFlareWAFRuleGraph(ctx, id, input.Revision, string(raw)); err != nil {
		return nil, err
	}
	return GetRule(ctx, id)
}

func ruleIPGroupExists(ctx context.Context, id uint) (bool, error) {
	_, err := repository.GetOpenFlareWAFIPGroupByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func buildRuleView(group *model.OpenFlareWAFRuleGroup, appliedSiteIDs []uint) (RuleView, error) {
	if group == nil {
		return RuleView{}, errors.New("waf rule is nil")
	}
	graph := DefaultRuleGraph()
	if strings.TrimSpace(group.Graph) != "" {
		if err := json.Unmarshal([]byte(group.Graph), &graph); err != nil {
			return RuleView{}, err
		}
	}
	ids := append([]uint(nil), appliedSiteIDs...)
	return RuleView{ID: group.ID, Name: group.Name, Enabled: group.Enabled, IsGlobal: group.IsGlobal,
		Graph: graph, Revision: group.Revision, AppliedSiteIDs: ids, AppliedSiteCount: len(ids),
		CreatedAt: group.CreatedAt.Format(time.RFC3339), UpdatedAt: group.UpdatedAt.Format(time.RFC3339)}, nil
}
