// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

// PolicyInput is the admin create/update payload for a SmartDNS routing policy.
type PolicyInput struct {
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	ScopeType        string       `json:"scope_type"`
	ScopeDomains     []string     `json:"scope_domains"`
	StrategyType     string       `json:"strategy_type"`
	Priority         int          `json:"priority"`
	TTL              int          `json:"ttl"`
	MaxAnswers       int          `json:"max_answers"`
	Enabled          *bool        `json:"enabled"`
	IsDefault        *bool        `json:"is_default"`
	FallbackStrategy string       `json:"fallback_strategy"`
	FallbackNodeIDs  []string     `json:"fallback_node_ids"`
	Config           PolicyConfig `json:"config"`
	JDMJSON          string       `json:"jdm_json"`
}

// PolicyView is the admin-facing SmartDNS policy representation.
type PolicyView struct {
	ID               uint         `json:"id"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	ScopeType        string       `json:"scope_type"`
	ScopeDomains     []string     `json:"scope_domains"`
	StrategyType     string       `json:"strategy_type"`
	Priority         int          `json:"priority"`
	TTL              int          `json:"ttl"`
	MaxAnswers       int          `json:"max_answers"`
	Enabled          bool         `json:"enabled"`
	IsDefault        bool         `json:"is_default"`
	FallbackStrategy string       `json:"fallback_strategy"`
	FallbackNodeIDs  []string     `json:"fallback_node_ids"`
	Config           PolicyConfig `json:"config"`
	JDMJSON          string       `json:"jdm_json"`
	Revision         int          `json:"revision"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

// PolicyValidateInput validates a visual flow graph or raw JDM JSON payload.
type PolicyValidateInput struct {
	StrategyType string       `json:"strategy_type"`
	Config       PolicyConfig `json:"config"`
	JDMJSON      string       `json:"jdm_json"`
}

// PolicyValidateResult reports compilation/validation results and normalized JDM JSON.
type PolicyValidateResult struct {
	Valid       bool   `json:"valid"`
	CompiledJDM string `json:"compiled_jdm"`
}

func toPolicyView(p *model.OpenFlareDNSPolicy) *PolicyView {
	if p == nil {
		return nil
	}
	var scopeDomains, fallbackNodeIDs []string
	_ = json.Unmarshal([]byte(p.ScopeDomainsJSON), &scopeDomains)
	_ = json.Unmarshal([]byte(p.FallbackNodeIDsJSON), &fallbackNodeIDs)
	if scopeDomains == nil {
		scopeDomains = []string{}
	}
	if fallbackNodeIDs == nil {
		fallbackNodeIDs = []string{}
	}
	cfg := defaultPolicyConfig(p.StrategyType)
	if strings.TrimSpace(p.ConfigJSON) != "" && p.ConfigJSON != "{}" {
		_ = json.Unmarshal([]byte(p.ConfigJSON), &cfg)
	}
	return &PolicyView{
		ID:               p.ID,
		Name:             p.Name,
		Description:      p.Description,
		ScopeType:        p.ScopeType,
		ScopeDomains:     scopeDomains,
		StrategyType:     p.StrategyType,
		Priority:         p.Priority,
		TTL:              p.TTL,
		MaxAnswers:       p.MaxAnswers,
		Enabled:          p.Enabled,
		IsDefault:        p.IsDefault,
		FallbackStrategy: p.FallbackStrategy,
		FallbackNodeIDs:  fallbackNodeIDs,
		Config:           cfg,
		JDMJSON:          p.JDMJSON,
		Revision:         p.Revision,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

// ListPolicies returns all configured SmartDNS policies.
func ListPolicies(ctx context.Context) ([]PolicyView, error) {
	rows, err := repository.ListDNSPolicies(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]PolicyView, 0, len(rows))
	for i := range rows {
		views = append(views, *toPolicyView(&rows[i]))
	}
	return views, nil
}

// GetPolicy returns a single SmartDNS policy by ID.
func GetPolicy(ctx context.Context, id uint) (*PolicyView, error) {
	row, err := repository.GetDNSPolicyByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newLogicError(http.StatusNotFound, errPolicyNotFound)
		}
		return nil, err
	}
	return toPolicyView(row), nil
}

// CreatePolicy validates, compiles, and persists a new SmartDNS routing policy.
func CreatePolicy(ctx context.Context, input PolicyInput) (*PolicyView, error) {
	item, err := buildPolicyModel(nil, input)
	if err != nil {
		return nil, err
	}
	if err := repository.CreateDNSPolicy(ctx, item); err != nil {
		return nil, err
	}
	NotifySnapshotDirty(ctx)
	return toPolicyView(item), nil
}

// UpdatePolicy validates, recompiles, and saves updates to an existing SmartDNS policy.
func UpdatePolicy(ctx context.Context, id uint, input PolicyInput) (*PolicyView, error) {
	existing, err := repository.GetDNSPolicyByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newLogicError(http.StatusNotFound, errPolicyNotFound)
		}
		return nil, err
	}
	item, err := buildPolicyModel(existing, input)
	if err != nil {
		return nil, err
	}
	item.Revision = existing.Revision + 1
	if err := repository.SaveDNSPolicy(ctx, item); err != nil {
		return nil, err
	}
	NotifySnapshotDirty(ctx)
	return toPolicyView(item), nil
}

// DeletePolicy deletes a SmartDNS policy after ensuring it is neither default nor referenced by Cloudflare groups.
func DeletePolicy(ctx context.Context, id uint) error {
	existing, err := repository.GetDNSPolicyByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newLogicError(http.StatusNotFound, errPolicyNotFound)
		}
		return err
	}
	if existing.IsDefault {
		return newLogicError(http.StatusBadRequest, errPolicyDefaultLocked)
	}
	refs, err := repository.CountPointingGroupsBySmartDNSPolicyID(ctx, id)
	if err != nil {
		return err
	}
	if refs > 0 {
		return newLogicError(http.StatusConflict, errPolicyInUse)
	}
	if err := repository.DeleteDNSPolicy(ctx, id); err != nil {
		return err
	}
	NotifySnapshotDirty(ctx)
	return nil
}

// SetDefaultPolicy marks a policy as the default enabled fallback policy.
func SetDefaultPolicy(ctx context.Context, id uint) error {
	if err := repository.SetDefaultDNSPolicy(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newLogicError(http.StatusNotFound, errPolicyNotFound)
		}
		return err
	}
	NotifySnapshotDirty(ctx)
	return nil
}

// ValidatePolicyGraph compiles and validates a policy config / visual flow graph / JDM JSON without persisting it.
func ValidatePolicyGraph(input PolicyValidateInput) (*PolicyValidateResult, error) {
	jdm, err := resolveAndCompilePolicyJDM(input.StrategyType, input.Config, input.JDMJSON)
	if err != nil {
		return nil, err
	}
	return &PolicyValidateResult{
		Valid:       true,
		CompiledJDM: jdm,
	}, nil
}

func buildPolicyModel(existing *model.OpenFlareDNSPolicy, input PolicyInput) (*model.OpenFlareDNSPolicy, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 128 {
		return nil, newLogicError(http.StatusBadRequest, errPolicyNameRequired)
	}
	scopeType := strings.ToLower(strings.TrimSpace(input.ScopeType))
	if scopeType == "" {
		scopeType = model.DNSPolicyScopeAll
	}
	if scopeType != model.DNSPolicyScopeAll && scopeType != model.DNSPolicyScopeDomains {
		return nil, newLogicError(http.StatusBadRequest, errPolicyScopeInvalid)
	}
	scopeDomains := normalizeDomainList(input.ScopeDomains)
	if scopeType == model.DNSPolicyScopeDomains && len(scopeDomains) == 0 {
		return nil, newLogicError(http.StatusBadRequest, errPolicyScopeDomainEmpty)
	}
	strategyType := strings.ToLower(strings.TrimSpace(input.StrategyType))
	if strategyType == "" {
		strategyType = model.DNSPolicyStrategySmart
	}
	switch strategyType {
	case model.DNSPolicyStrategySmart, model.DNSPolicyStrategyLoadBalance, model.DNSPolicyStrategyCustom:
	default:
		return nil, newLogicError(http.StatusBadRequest, errPolicyStrategyInvalid)
	}
	fallback := strings.ToLower(strings.TrimSpace(input.FallbackStrategy))
	if fallback == "" {
		fallback = model.DNSPolicyFallbackSmart
	}
	switch fallback {
	case model.DNSPolicyFallbackSmart, model.DNSPolicyFallbackLoadBalance, model.DNSPolicyFallbackStaticNodes, model.DNSPolicyFallbackAllHealthy:
	default:
		return nil, newLogicError(http.StatusBadRequest, errPolicyFallbackInvalid)
	}
	ttl := input.TTL
	if ttl == 0 {
		ttl = defaultSmartDNSTTL
	}
	if ttl < 10 || ttl > 86400 {
		return nil, newLogicError(http.StatusBadRequest, errPolicyTTLInvalid)
	}
	maxAnswers := input.MaxAnswers
	if maxAnswers == 0 {
		maxAnswers = 1
	}
	if maxAnswers < 1 || maxAnswers > 16 {
		return nil, newLogicError(http.StatusBadRequest, errPolicyMaxAnswersInvalid)
	}
	priority := input.Priority
	if priority <= 0 {
		priority = 100
	}

	jdmJSON, err := resolveAndCompilePolicyJDM(strategyType, input.Config, input.JDMJSON)
	if err != nil {
		return nil, err
	}

	cfg := input.Config
	if strings.TrimSpace(cfg.Mode) == "" {
		if strategyType == model.DNSPolicyStrategyLoadBalance {
			cfg.Mode = model.DNSPolicyStrategyLoadBalance
		} else {
			cfg.Mode = model.DNSPolicyStrategySmart
		}
	}
	if cfg.Weights.Latency == 0 && cfg.Weights.Load == 0 && cfg.Weights.Geo == 0 {
		cfg.Weights = defaultPolicyConfig(strategyType).Weights
	}
	if cfg.Filters.NodeIDs == nil {
		cfg.Filters.NodeIDs = []string{}
	}
	if cfg.Filters.Countries == nil {
		cfg.Filters.Countries = []string{}
	}
	if cfg.Filters.Tags == nil {
		cfg.Filters.Tags = []string{}
	}
	if cfg.Filters.NetworkGroups == nil {
		cfg.Filters.NetworkGroups = []string{}
	}

	scopeJSON, _ := json.Marshal(scopeDomains)
	fbIDs := normalizeStringSlice(input.FallbackNodeIDs)
	fbJSON, _ := json.Marshal(fbIDs)
	cfgJSON, _ := json.Marshal(cfg)

	item := &model.OpenFlareDNSPolicy{}
	if existing != nil {
		*item = *existing
	}
	item.Name = name
	item.Description = strings.TrimSpace(input.Description)
	item.ScopeType = scopeType
	item.ScopeDomainsJSON = string(scopeJSON)
	item.StrategyType = strategyType
	item.Priority = priority
	item.TTL = ttl
	item.MaxAnswers = maxAnswers
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	} else if existing == nil {
		item.Enabled = true
	}
	if input.IsDefault != nil {
		item.IsDefault = *input.IsDefault
	}
	if item.IsDefault {
		item.Enabled = true
	}
	item.FallbackStrategy = fallback
	item.FallbackNodeIDsJSON = string(fbJSON)
	item.ConfigJSON = string(cfgJSON)
	item.JDMJSON = jdmJSON
	if item.Revision <= 0 {
		item.Revision = 1
	}
	return item, nil
}

func resolveAndCompilePolicyJDM(strategyType string, cfg PolicyConfig, rawJDM string) (string, error) {
	if cfg.Graph != nil && len(cfg.Graph.Nodes) > 0 {
		return CompileFlowGraphToJDM(*cfg.Graph)
	}
	if trimmed := strings.TrimSpace(rawJDM); trimmed != "" && trimmed != "{}" {
		if _, err := ValidateJDMJSON(trimmed); err != nil {
			return "", err
		}
		return trimmed, nil
	}
	return CompilePresetJDM(strategyType, cfg)
}

func normalizeStringSlice(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, s := range items {
		t := strings.TrimSpace(s)
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// matchPolicyForDomain selects the active SmartDNS policy for a queried FQDN using:
// 1. Explicit Cloudflare SmartDNS pointing group domain/target_host route binding
// 2. Scope-matched enabled domain policy ordered by priority ASC
// 3. Default / catch-all `all`-scoped policy.
func matchPolicyForDomain(env *protocol.DNSSnapshotEnvelope, domain string) (protocol.DNSSnapshotPolicy, bool) {
	if env == nil || len(env.Policies) == 0 {
		return protocol.DNSSnapshotPolicy{}, false
	}
	qdomain := normalizeDomain(domain)

	byID := make(map[uint]protocol.DNSSnapshotPolicy, len(env.Policies))
	for _, p := range env.Policies {
		byID[p.ID] = p
	}

	for _, route := range env.DomainRoutes {
		if route.TargetHost == qdomain {
			if p, ok := byID[route.PolicyID]; ok {
				return p, true
			}
		}
		for _, member := range route.MemberDomains {
			if matchDomainPattern(member, qdomain) {
				if p, ok := byID[route.PolicyID]; ok {
					return p, true
				}
			}
		}
	}

	for _, p := range env.Policies {
		if p.ScopeType == model.DNSPolicyScopeDomains {
			for _, pat := range p.ScopeDomains {
				if matchDomainPattern(pat, qdomain) {
					return p, true
				}
			}
		}
	}

	var fallback *protocol.DNSSnapshotPolicy
	for i := range env.Policies {
		p := &env.Policies[i]
		if p.IsDefault {
			return *p, true
		}
		if fallback == nil && p.ScopeType == model.DNSPolicyScopeAll {
			fallback = p
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return env.Policies[0], true
}

func matchDomainPattern(pattern, domain string) bool {
	pat := normalizeDomain(pattern)
	dom := normalizeDomain(domain)
	if pat == "" || dom == "" {
		return false
	}
	if pat == dom {
		return true
	}
	if strings.HasPrefix(pat, "*.") {
		suffix := pat[2:]
		return dom == suffix || strings.HasSuffix(dom, "."+suffix)
	}
	return false
}
