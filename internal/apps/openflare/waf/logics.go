// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"

	exprlang "github.com/expr-lang/expr"
	"gorm.io/gorm"
)

const (
	maxWAFBlockBodyBytes = 16 * 1024

	wafIPGroupTypeManual       = "manual"
	wafIPGroupTypeAutomatic    = "automatic"
	wafIPGroupTypeSubscription = "subscription"

	wafIPGroupSubscriptionFormatText = "text"
	wafIPGroupSubscriptionFormatJSON = "json"

	defaultWAFIPGroupSyncIntervalMinutes = 1440
	defaultWAFIPGroupAutoLookback        = "1h"
	defaultWAFIPGroupAutoLookbackDur     = time.Hour
	minWAFIPGroupSyncIntervalMinutes     = 1
	maxWAFIPGroupSyncIntervalMinutes     = 43200
	maxWAFIPGroupAutoLookback            = 30 * 24 * time.Hour
	minPoWSessionTTLSeconds              = 60
	minPoWChallengeTTLSeconds            = 30
	powAlgorithmFast                     = "fast"
	powAlgorithmSlow                     = "slow"
)

// SiteRuleGroupsView is the site-level WAF binding view.
type SiteRuleGroupsView struct {
	RouteID           uint       `json:"route_id"`
	GlobalRuleGroup   *RuleView  `json:"global_rule_group"`
	RuleGroups        []RuleView `json:"rule_groups"`
	AppliedRuleGroups []RuleView `json:"applied_rule_groups"`
	AppliedIDs        []uint     `json:"applied_ids"`
}

// IDsRequest carries a list of numeric ids.
type IDsRequest struct {
	IDs []uint `json:"ids"`
}

// IPGroupInput is the create/update payload for WAF IP groups.
type IPGroupInput struct {
	Name                    string          `json:"name"`
	Type                    string          `json:"type"`
	Enabled                 bool            `json:"enabled"`
	IPList                  []string        `json:"ip_list"`
	AutoConfig              json.RawMessage `json:"auto_config"`
	SubscriptionURL         string          `json:"subscription_url"`
	SubscriptionFormat      string          `json:"subscription_format"`
	SubscriptionMappingRule string          `json:"subscription_mapping_rule"`
	SyncIntervalMinutes     int             `json:"sync_interval_minutes"`
}

// IPGroupExtIPView is an external IP entry in API responses.
type IPGroupExtIPView struct {
	IP         string `json:"ip"`
	CapturedAt string `json:"captured_at"`
}

// IPGroupView is the API view for a WAF IP group.
type IPGroupView struct {
	ID                      uint               `json:"id"`
	Name                    string             `json:"name"`
	Type                    string             `json:"type"`
	Enabled                 bool               `json:"enabled"`
	IPList                  []string           `json:"ip_list"`
	AutoConfig              json.RawMessage    `json:"auto_config"`
	ExtIPs                  []IPGroupExtIPView `json:"ext_ips"`
	SubscriptionURL         string             `json:"subscription_url"`
	SubscriptionFormat      string             `json:"subscription_format"`
	SubscriptionMappingRule string             `json:"subscription_mapping_rule"`
	SyncIntervalMinutes     int                `json:"sync_interval_minutes"`
	LastSyncedAt            string             `json:"last_synced_at,omitempty"`
	NextSyncAt              string             `json:"next_sync_at,omitempty"`
	LastSyncStatus          string             `json:"last_sync_status"`
	LastSyncMessage         string             `json:"last_sync_message"`
	ReferencedByRuleCount   int                `json:"referenced_by_rule_count"`
	CreatedAt               string             `json:"created_at"`
	UpdatedAt               string             `json:"updated_at"`
}

// IPGroupSyncResult is the response for manual IP group sync.
type IPGroupSyncResult struct {
	Group      IPGroupView `json:"group"`
	IPCount    int         `json:"ip_count"`
	SyncedAt   string      `json:"synced_at"`
	NextSyncAt string      `json:"next_sync_at"`
	Status     string      `json:"status"`
	Message    string      `json:"message"`
}

// IPGroupAutoTestInput tests automatic IP group configuration.
type IPGroupAutoTestInput struct {
	AutoConfig json.RawMessage `json:"auto_config"`
}

// IPGroupAutoTestResult is the response for automatic IP group test.
type IPGroupAutoTestResult struct {
	MatchedIPs   []string `json:"matched_ips"`
	MatchedCount int      `json:"matched_count"`
	Lookback     string   `json:"lookback"`
	RuleCount    int      `json:"rule_count"`
	TestedAt     string   `json:"tested_at"`
}

type ipGroupAutoConfig struct {
	Lookback string            `json:"lookback"`
	TTL      int               `json:"ttl"`
	Rules    []ipGroupAutoRule `json:"rules"`
	// lookbackDuration is resolved from Lookback (and legacy lookback_minutes) for runtime queries.
	lookbackDuration time.Duration `json:"-"`
}

type ipGroupAutoRule struct {
	Name string `json:"name"`
	Expr string `json:"expr"`
}

type ipGroupExtIP struct {
	IP         string    `json:"ip"`
	CapturedAt time.Time `json:"captured_at"`
}

// GetSiteRuleGroups returns WAF rule groups for a proxy route.
func GetSiteRuleGroups(ctx context.Context, routeID uint) (*SiteRuleGroupsView, error) {
	if _, err := model.GetOpenFlareProxyRouteByID(ctx, routeID); err != nil {
		return nil, err
	}
	groups, err := ListRules(ctx)
	if err != nil {
		return nil, err
	}
	appliedIDs, err := ListSiteRuleGroupIDs(ctx, routeID)
	if err != nil {
		return nil, err
	}
	var global *RuleView
	custom := make([]RuleView, 0, len(groups))
	applied := make([]RuleView, 0, len(appliedIDs))
	groupByID := make(map[uint]RuleView, len(groups))
	for index := range groups {
		group := groups[index]
		if group.IsGlobal {
			item := group
			global = &item
			continue
		}
		custom = append(custom, group)
		groupByID[group.ID] = group
	}
	for _, id := range appliedIDs {
		if group, ok := groupByID[id]; ok {
			applied = append(applied, group)
		}
	}
	return &SiteRuleGroupsView{
		RouteID:           routeID,
		GlobalRuleGroup:   global,
		RuleGroups:        custom,
		AppliedRuleGroups: applied,
		AppliedIDs:        appliedIDs,
	}, nil
}

// ReplaceSiteRuleGroups replaces rule group bindings for a proxy route.
func ReplaceSiteRuleGroups(ctx context.Context, routeID uint, groupIDs []uint) (*SiteRuleGroupsView, error) {
	if _, err := model.GetOpenFlareProxyRouteByID(ctx, routeID); err != nil {
		return nil, err
	}
	normalized, err := normalizeRuleGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, &RuleValidationError{Err: err}
	}
	if err = model.ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx, routeID, normalized); err != nil {
		return nil, err
	}
	return GetSiteRuleGroups(ctx, routeID)
}

// ListSiteRuleGroupIDs returns rule group ids bound to a proxy route.
func ListSiteRuleGroupIDs(ctx context.Context, routeID uint) ([]uint, error) {
	bindings, err := model.ListOpenFlareWAFRuleGroupBindingsByRouteID(ctx, routeID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.RuleGroupID)
	}
	return ids, nil
}

// EnsureDefaultRuleGroup ensures the global WAF rule group exists.
func EnsureDefaultRuleGroup(ctx context.Context) error {
	_, err := model.GetGlobalOpenFlareWAFRuleGroup(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	graph, marshalErr := json.Marshal(DefaultRuleGraph())
	if marshalErr != nil {
		return marshalErr
	}
	group := &model.OpenFlareWAFRuleGroup{
		Name: "全局规则组", Enabled: true, IsGlobal: true, Graph: string(graph), Revision: 1,
	}
	return model.CreateOpenFlareWAFRuleGroup(ctx, group)
}

// ListIPGroups returns all WAF IP groups.
func ListIPGroups(ctx context.Context) ([]IPGroupView, error) {
	groups, err := model.ListOpenFlareWAFIPGroups(ctx)
	if err != nil {
		return nil, err
	}
	referenceCounts, err := loadIPGroupReferenceCounts(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]IPGroupView, 0, len(groups))
	for _, group := range groups {
		view, buildErr := buildIPGroupView(group, referenceCounts[group.ID])
		if buildErr != nil {
			return nil, buildErr
		}
		views = append(views, view)
	}
	return views, nil
}

// GetIPGroup returns a WAF IP group by id.
func GetIPGroup(ctx context.Context, id uint) (*IPGroupView, error) {
	group, err := model.GetOpenFlareWAFIPGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	referenceCounts, err := loadIPGroupReferenceCounts(ctx)
	if err != nil {
		return nil, err
	}
	view, err := buildIPGroupView(group, referenceCounts[group.ID])
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// CreateIPGroup creates a WAF IP group.
func CreateIPGroup(ctx context.Context, input IPGroupInput) (*IPGroupView, error) {
	group, err := buildIPGroup(nil, input)
	if err != nil {
		return nil, &RuleValidationError{Err: err}
	}
	if err = model.CreateOpenFlareWAFIPGroup(ctx, group); err != nil {
		return nil, err
	}
	broadcastIPGroupToAgents(ctx, group.ID)
	return GetIPGroup(ctx, group.ID)
}

// UpdateIPGroup updates a WAF IP group.
func UpdateIPGroup(ctx context.Context, id uint, input IPGroupInput) (*IPGroupView, error) {
	group, err := model.GetOpenFlareWAFIPGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	group, err = buildIPGroup(group, input)
	if err != nil {
		return nil, &RuleValidationError{Err: err}
	}
	if err = model.UpdateOpenFlareWAFIPGroup(ctx, group); err != nil {
		return nil, err
	}
	broadcastIPGroupToAgents(ctx, group.ID)
	return GetIPGroup(ctx, group.ID)
}

// DeleteIPGroup deletes a WAF IP group when not referenced.
func DeleteIPGroup(ctx context.Context, id uint) error {
	group, err := model.GetOpenFlareWAFIPGroupByID(ctx, id)
	if err != nil {
		return err
	}
	counts, err := loadIPGroupReferenceCounts(ctx)
	if err != nil {
		return err
	}
	if counts[group.ID] > 0 {
		return &RuleValidationError{Err: errors.New("IP 组已被 WAF 规则引用，请先移除引用")}
	}
	return model.DeleteOpenFlareWAFIPGroup(ctx, group.ID)
}

// SyncIPGroup synchronizes a subscription or automatic WAF IP group.
func SyncIPGroup(ctx context.Context, id uint) (*IPGroupSyncResult, error) {
	group, err := model.GetOpenFlareWAFIPGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return syncOpenFlareWAFIPGroup(ctx, group, time.Now().UTC())
}

// TestIPGroupAutoConfig evaluates automatic IP group rules against recent access logs.
func TestIPGroupAutoConfig(ctx context.Context, input IPGroupAutoTestInput) (*IPGroupAutoTestResult, error) {
	config, err := parseIPGroupAutoConfig(input.AutoConfig)
	if err != nil {
		return nil, &RuleValidationError{Err: err}
	}
	now := time.Now().UTC()
	ips, err := evaluateParsedIPGroupAutoConfig(ctx, config, now)
	if err != nil {
		return nil, err
	}
	return &IPGroupAutoTestResult{
		MatchedIPs:   ips,
		MatchedCount: len(ips),
		Lookback:     config.Lookback,
		RuleCount:    len(config.Rules),
		TestedAt:     now.Format(time.RFC3339),
	}, nil
}

func loadRuleGroupBindings(ctx context.Context) (map[uint][]uint, error) {
	bindings, err := model.ListOpenFlareWAFRuleGroupBindings(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[uint][]uint, len(bindings))
	for _, binding := range bindings {
		result[binding.RuleGroupID] = append(result[binding.RuleGroupID], binding.ProxyRouteID)
	}
	return result, nil
}

func loadIPGroupReferenceCounts(ctx context.Context) (map[uint]int, error) {
	groups, err := model.ListOpenFlareWAFRuleGroups(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int)
	for _, group := range groups {
		graph := DefaultRuleGraph()
		if strings.TrimSpace(group.Graph) != "" {
			if err = json.Unmarshal([]byte(group.Graph), &graph); err != nil {
				return nil, fmt.Errorf("decode WAF rule %d graph: %w", group.ID, err)
			}
		}
		for _, id := range ReferencedIPGroupIDs(graph) {
			counts[id]++
		}
	}
	return counts, nil
}

func pruneIPGroupExtIPs(group *model.OpenFlareWAFIPGroup, ipList []string) error {
	if group == nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(ipList))
	for _, ip := range ipList {
		allowed[ip] = struct{}{}
	}
	var extIPs []ipGroupExtIP
	if group.ExtIPs != "" && group.ExtIPs != "[]" {
		if err := json.Unmarshal([]byte(group.ExtIPs), &extIPs); err != nil {
			return err
		}
	}
	pruned := make([]ipGroupExtIP, 0, len(extIPs))
	for _, extIP := range extIPs {
		if _, ok := allowed[extIP.IP]; ok {
			pruned = append(pruned, extIP)
		}
	}
	extIPsJSON, err := json.Marshal(pruned)
	if err != nil {
		return err
	}
	group.ExtIPs = string(extIPsJSON)
	return nil
}

func normalizeIPList(items []string) ([]string, error) {
	normalized := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				return nil, fmt.Errorf("%s 不是合法 IP 段", item)
			}
			item = prefix.Masked().String()
		} else {
			addr, err := netip.ParseAddr(item)
			if err != nil {
				return nil, fmt.Errorf("%s 不是合法 IP", item)
			}
			item = addr.String()
		}
		normalized = append(normalized, item)
	}
	normalized = uniqueStrings(normalized)
	sort.Strings(normalized)
	return normalized, nil
}

func decodeStringList(raw string) ([]string, error) {
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

func normalizeRuleGroupIDs(ctx context.Context, groupIDs []uint) ([]uint, error) {
	normalized := uniqueUintIDsInOrder(groupIDs)
	for _, groupID := range normalized {
		group, err := model.GetOpenFlareWAFRuleGroupByID(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("WAF 规则组 %d 不存在", groupID)
		}
		if group.IsGlobal {
			return nil, errors.New("全局 WAF 规则组不需要手动绑定")
		}
	}
	return normalized, nil
}

func uniqueUintIDsInOrder(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func normalizeIPGroupAutoConfig(raw json.RawMessage) (string, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		text = "{}"
	}
	config, err := parseIPGroupAutoConfig(json.RawMessage(text))
	if err != nil {
		return "", err
	}
	normalized, _ := json.Marshal(config)
	return string(normalized), nil
}

func parseIPGroupAutoConfig(raw json.RawMessage) (ipGroupAutoConfig, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		text = "{}"
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(text), &object); err != nil || object == nil {
		return ipGroupAutoConfig{}, errors.New("自动 IP 组配置必须是 JSON 对象")
	}
	var config ipGroupAutoConfig
	if err := json.Unmarshal([]byte(text), &config); err != nil {
		return ipGroupAutoConfig{}, errors.New("自动 IP 组配置必须是 JSON 对象")
	}
	lookbackDur, lookbackText, err := resolveIPGroupAutoLookback(object)
	if err != nil {
		return ipGroupAutoConfig{}, err
	}
	config.Lookback = lookbackText
	config.lookbackDuration = lookbackDur
	if config.TTL == 0 {
		config.TTL = -1
	}
	if config.Rules == nil {
		config.Rules = []ipGroupAutoRule{}
	}
	for i, rule := range config.Rules {
		rule.Name = strings.TrimSpace(rule.Name)
		rule.Expr = strings.TrimSpace(rule.Expr)
		if rule.Expr == "" {
			return ipGroupAutoConfig{}, fmt.Errorf("自动规则 %d 的 Expr 表达式不能为空", i+1)
		}
		if _, err := exprlang.Compile(rule.Expr, exprlang.Env(ipGroupAutoRuleEnv{}), exprlang.AsBool()); err != nil {
			return ipGroupAutoConfig{}, fmt.Errorf("自动规则 %s Expr 无效: %w", displayIPGroupAutoRuleName(rule, i), err)
		}
		config.Rules[i] = rule
	}
	return config, nil
}

// resolveIPGroupAutoLookback accepts lookback as duration string (60m/1h) or legacy lookback_minutes number.
func resolveIPGroupAutoLookback(object map[string]any) (time.Duration, string, error) {
	if raw, ok := object["lookback"]; ok && raw != nil {
		dur, text, err := parseIPGroupLookbackValue(raw)
		if err != nil {
			return 0, "", err
		}
		return dur, text, nil
	}
	if raw, ok := object["lookback_minutes"]; ok && raw != nil {
		// legacy: minutes as number or numeric string
		minutes, err := parsePositiveNumber(raw)
		if err != nil {
			return 0, "", fmt.Errorf("lookback_minutes 无效: %w", err)
		}
		if minutes <= 0 {
			return defaultWAFIPGroupAutoLookbackDur, defaultWAFIPGroupAutoLookback, nil
		}
		dur := time.Duration(minutes) * time.Minute
		if dur > maxWAFIPGroupAutoLookback {
			return 0, "", fmt.Errorf("回看窗口不能超过 %s", formatLookbackDuration(maxWAFIPGroupAutoLookback))
		}
		return dur, formatLookbackDuration(dur), nil
	}
	return defaultWAFIPGroupAutoLookbackDur, defaultWAFIPGroupAutoLookback, nil
}

func parseIPGroupLookbackValue(raw any) (time.Duration, string, error) {
	switch v := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return defaultWAFIPGroupAutoLookbackDur, defaultWAFIPGroupAutoLookback, nil
		}
		// bare integer string → minutes
		if isAllDigits(trimmed) {
			minutes, err := parsePositiveNumber(trimmed)
			if err != nil || minutes <= 0 {
				return 0, "", errors.New("lookback 格式不合法，请使用 60m、1h 等时长")
			}
			dur := time.Duration(minutes) * time.Minute
			if dur > maxWAFIPGroupAutoLookback {
				return 0, "", fmt.Errorf("回看窗口不能超过 %s", formatLookbackDuration(maxWAFIPGroupAutoLookback))
			}
			return dur, formatLookbackDuration(dur), nil
		}
		dur, err := time.ParseDuration(strings.ToLower(trimmed))
		if err != nil || dur <= 0 {
			return 0, "", errors.New("lookback 格式不合法，请使用 60m、1h 等时长")
		}
		if dur > maxWAFIPGroupAutoLookback {
			return 0, "", fmt.Errorf("回看窗口不能超过 %s", formatLookbackDuration(maxWAFIPGroupAutoLookback))
		}
		return dur, formatLookbackDuration(dur), nil
	case float64:
		if v <= 0 {
			return defaultWAFIPGroupAutoLookbackDur, defaultWAFIPGroupAutoLookback, nil
		}
		if v != float64(int64(v)) {
			return 0, "", errors.New("lookback 数值必须为整数分钟")
		}
		dur := time.Duration(int64(v)) * time.Minute
		if dur > maxWAFIPGroupAutoLookback {
			return 0, "", fmt.Errorf("回看窗口不能超过 %s", formatLookbackDuration(maxWAFIPGroupAutoLookback))
		}
		return dur, formatLookbackDuration(dur), nil
	case json.Number:
		return parseIPGroupLookbackValue(string(v))
	default:
		return 0, "", errors.New("lookback 格式不合法，请使用 60m、1h 等时长")
	}
}

func parsePositiveNumber(raw any) (int, error) {
	switch v := raw.(type) {
	case float64:
		if v != float64(int(v)) {
			return 0, errors.New("必须为整数")
		}
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, err
		}
		return int(i), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || !isAllDigits(trimmed) {
			return 0, errors.New("必须为整数")
		}
		n := 0
		for _, ch := range trimmed {
			n = n*10 + int(ch-'0')
		}
		return n, nil
	default:
		return 0, errors.New("必须为整数")
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func formatLookbackDuration(d time.Duration) string {
	if d <= 0 {
		return defaultWAFIPGroupAutoLookback
	}
	// Prefer compact human units used in config examples.
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return d.String()
}

func validateSubscriptionURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return errors.New("订阅 URL 无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("订阅 URL 仅支持 http 或 https")
	}
	return nil
}

func normalizeIPGroupType(value string) string {
	switch strings.TrimSpace(value) {
	case wafIPGroupTypeManual, "":
		return wafIPGroupTypeManual
	case wafIPGroupTypeAutomatic:
		return wafIPGroupTypeAutomatic
	case wafIPGroupTypeSubscription:
		return wafIPGroupTypeSubscription
	default:
		return ""
	}
}

func normalizeIPGroupSubscriptionFormat(value string) string {
	switch strings.TrimSpace(value) {
	case wafIPGroupSubscriptionFormatJSON:
		return wafIPGroupSubscriptionFormatJSON
	default:
		return wafIPGroupSubscriptionFormatText
	}
}

func normalizeIPGroupSyncInterval(value int) int {
	if value <= 0 {
		return defaultWAFIPGroupSyncIntervalMinutes
	}
	if value < minWAFIPGroupSyncIntervalMinutes {
		return minWAFIPGroupSyncIntervalMinutes
	}
	if value > maxWAFIPGroupSyncIntervalMinutes {
		return maxWAFIPGroupSyncIntervalMinutes
	}
	return value
}

func nextIPGroupSyncAt(groupType string, enabled bool, interval int, current *time.Time) *time.Time {
	if (groupType != wafIPGroupTypeSubscription && groupType != wafIPGroupTypeAutomatic) || !enabled {
		return nil
	}
	if current != nil && current.After(time.Now().UTC()) {
		return current
	}
	next := time.Now().UTC().Add(time.Duration(normalizeIPGroupSyncInterval(interval)) * time.Minute)
	return &next
}
func buildIPGroup(group *model.OpenFlareWAFIPGroup, input IPGroupInput) (*model.OpenFlareWAFIPGroup, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("IP 组名称不能为空")
	}
	groupType := normalizeIPGroupType(input.Type)
	if groupType == "" {
		return nil, errors.New("IP 组类型无效")
	}
	ipList := input.IPList
	subscriptionURL := ""
	subscriptionFormat := normalizeIPGroupSubscriptionFormat(input.SubscriptionFormat)
	mappingRule := strings.TrimSpace(input.SubscriptionMappingRule)
	syncInterval := normalizeIPGroupSyncInterval(input.SyncIntervalMinutes)
	autoConfig := "{}"

	switch groupType {
	case wafIPGroupTypeManual:
		subscriptionFormat = wafIPGroupSubscriptionFormatText
		mappingRule = ""
	case wafIPGroupTypeAutomatic:
		normalizedConfig, err := normalizeIPGroupAutoConfig(input.AutoConfig)
		if err != nil {
			return nil, err
		}
		autoConfig = normalizedConfig
		subscriptionFormat = wafIPGroupSubscriptionFormatText
		mappingRule = ""
	case wafIPGroupTypeSubscription:
		subscriptionURL = strings.TrimSpace(input.SubscriptionURL)
		if err := validateSubscriptionURL(subscriptionURL); err != nil {
			return nil, err
		}
		if subscriptionFormat == "" {
			subscriptionFormat = wafIPGroupSubscriptionFormatText
		}
	}

	normalizedIPs, err := normalizeIPList(ipList)
	if err != nil {
		return nil, err
	}
	ipListJSON, _ := json.Marshal(normalizedIPs)
	if group == nil {
		group = &model.OpenFlareWAFIPGroup{}
		group.ExtIPs = "[]"
	}
	group.Name = name
	group.Type = groupType
	group.Enabled = input.Enabled
	group.IPList = string(ipListJSON)
	if groupType == wafIPGroupTypeAutomatic {
		if err := pruneIPGroupExtIPs(group, normalizedIPs); err != nil {
			return nil, err
		}
	}
	group.AutoConfig = autoConfig
	group.SubscriptionURL = subscriptionURL
	group.SubscriptionFormat = subscriptionFormat
	group.SubscriptionMappingRule = mappingRule
	group.SyncIntervalMinutes = syncInterval
	group.NextSyncAt = nextIPGroupSyncAt(group.Type, group.Enabled, syncInterval, group.NextSyncAt)
	return group, nil
}

func buildIPGroupView(group *model.OpenFlareWAFIPGroup, referenceCount int) (IPGroupView, error) {
	if group == nil {
		return IPGroupView{}, errors.New("waf ip group is nil")
	}
	ips, err := decodeStringList(group.IPList)
	if err != nil {
		return IPGroupView{}, err
	}
	autoConfig := json.RawMessage(strings.TrimSpace(group.AutoConfig))
	if len(autoConfig) == 0 {
		autoConfig = json.RawMessage("{}")
	}
	var extIPs []ipGroupExtIP
	if group.ExtIPs != "" && group.ExtIPs != "[]" {
		_ = json.Unmarshal([]byte(group.ExtIPs), &extIPs)
	}
	viewExtIPs := make([]IPGroupExtIPView, 0, len(extIPs))
	for _, extIP := range extIPs {
		viewExtIPs = append(viewExtIPs, IPGroupExtIPView{
			IP:         extIP.IP,
			CapturedAt: extIP.CapturedAt.Format(time.RFC3339),
		})
	}
	view := IPGroupView{
		ID:                      group.ID,
		Name:                    group.Name,
		Type:                    group.Type,
		Enabled:                 group.Enabled,
		IPList:                  ips,
		AutoConfig:              autoConfig,
		ExtIPs:                  viewExtIPs,
		SubscriptionURL:         group.SubscriptionURL,
		SubscriptionFormat:      group.SubscriptionFormat,
		SubscriptionMappingRule: group.SubscriptionMappingRule,
		SyncIntervalMinutes:     group.SyncIntervalMinutes,
		LastSyncStatus:          group.LastSyncStatus,
		LastSyncMessage:         group.LastSyncMessage,
		ReferencedByRuleCount:   referenceCount,
		CreatedAt:               group.CreatedAt.Format(time.RFC3339),
		UpdatedAt:               group.UpdatedAt.Format(time.RFC3339),
	}
	if group.LastSyncedAt != nil {
		view.LastSyncedAt = group.LastSyncedAt.Format(time.RFC3339)
	}
	if group.NextSyncAt != nil {
		view.NextSyncAt = group.NextSyncAt.Format(time.RFC3339)
	}
	return view, nil
}
