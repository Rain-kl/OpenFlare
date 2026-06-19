// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Rain-kl/Wavelet/internal/model"

	exprlang "github.com/expr-lang/expr"
	"gorm.io/gorm"
)

const (
	defaultWAFBlockStatusCode = 418
	maxWAFBlockBodyBytes      = 16 * 1024

	wafIPGroupTypeManual       = "manual"
	wafIPGroupTypeAutomatic    = "automatic"
	wafIPGroupTypeSubscription = "subscription"

	wafIPGroupSubscriptionFormatText = "text"
	wafIPGroupSubscriptionFormatJSON = "json"

	defaultWAFIPGroupSyncIntervalMinutes = 1440
	defaultWAFIPGroupAutoLookbackMinutes = 60
	minWAFIPGroupSyncIntervalMinutes     = 5
	maxWAFIPGroupSyncIntervalMinutes     = 43200
)

// RuleGroupInput is the create/update payload for WAF rule groups.
type RuleGroupInput struct {
	Name              string          `json:"name"`
	Enabled           bool            `json:"enabled"`
	BlockStatusCode   int             `json:"block_status_code"`
	BlockResponseBody string          `json:"block_response_body"`
	IPWhitelist       []string        `json:"ip_whitelist"`
	IPBlacklist       []string        `json:"ip_blacklist"`
	IPWhitelistGroups []uint          `json:"ip_whitelist_group_ids"`
	IPBlacklistGroups []uint          `json:"ip_blacklist_group_ids"`
	CountryWhitelist  []string        `json:"country_whitelist"`
	CountryBlacklist  []string        `json:"country_blacklist"`
	RegionWhitelist   []string        `json:"region_whitelist"`
	RegionBlacklist   []string        `json:"region_blacklist"`
	Remark            string          `json:"remark"`
	PoWEnabled        bool            `json:"pow_enabled"`
	PoWConfig         json.RawMessage `json:"pow_config"`
}

// PoWListConfig stores PoW whitelist/blacklist dimensions.
type PoWListConfig struct {
	IPs         []string `json:"ips"`
	IPCidrs     []string `json:"ip_cidrs"`
	Paths       []string `json:"paths"`
	PathRegexes []string `json:"path_regexes"`
	UserAgents  []string `json:"user_agents"`
}

// PoWConfig stores proof-of-work settings for a rule group.
type PoWConfig struct {
	Difficulty   int           `json:"difficulty"`
	Algorithm    string        `json:"algorithm"`
	SessionTTL   int           `json:"session_ttl"`
	ChallengeTTL int           `json:"challenge_ttl"`
	Whitelist    PoWListConfig `json:"whitelist"`
	Blacklist    PoWListConfig `json:"blacklist"`
}

// RuleGroupView is the API view for a WAF rule group.
type RuleGroupView struct {
	ID                uint       `json:"id"`
	Name              string     `json:"name"`
	Enabled           bool       `json:"enabled"`
	IsGlobal          bool       `json:"is_global"`
	BlockStatusCode   int        `json:"block_status_code"`
	BlockResponseBody string     `json:"block_response_body"`
	IPWhitelist       []string   `json:"ip_whitelist"`
	IPBlacklist       []string   `json:"ip_blacklist"`
	IPWhitelistGroups []uint     `json:"ip_whitelist_group_ids"`
	IPBlacklistGroups []uint     `json:"ip_blacklist_group_ids"`
	CountryWhitelist  []string   `json:"country_whitelist"`
	CountryBlacklist  []string   `json:"country_blacklist"`
	RegionWhitelist   []string   `json:"region_whitelist"`
	RegionBlacklist   []string   `json:"region_blacklist"`
	Remark            string     `json:"remark"`
	PoWEnabled        bool       `json:"pow_enabled"`
	PoWConfig         *PoWConfig `json:"pow_config"`
	AppliedSiteIDs    []uint     `json:"applied_site_ids"`
	AppliedSiteCount  int        `json:"applied_site_count"`
	CreatedAt         string     `json:"created_at"`
	UpdatedAt         string     `json:"updated_at"`
}

// SiteRuleGroupsView is the site-level WAF binding view.
type SiteRuleGroupsView struct {
	RouteID           uint            `json:"route_id"`
	GlobalRuleGroup   *RuleGroupView  `json:"global_rule_group"`
	RuleGroups        []RuleGroupView `json:"rule_groups"`
	AppliedRuleGroups []RuleGroupView `json:"applied_rule_groups"`
	AppliedIDs        []uint          `json:"applied_ids"`
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
	Remark                  string          `json:"remark"`
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
	Remark                  string             `json:"remark"`
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
	MatchedIPs      []string `json:"matched_ips"`
	MatchedCount    int      `json:"matched_count"`
	LookbackMinutes int      `json:"lookback_minutes"`
	RuleCount       int      `json:"rule_count"`
	TestedAt        string   `json:"tested_at"`
}

type ipGroupAutoConfig struct {
	LookbackMinutes int               `json:"lookback_minutes"`
	TTL             int               `json:"ttl"`
	Rules           []ipGroupAutoRule `json:"rules"`
}

type ipGroupAutoRule struct {
	Name string `json:"name"`
	Expr string `json:"expr"`
}

type ipGroupExtIP struct {
	IP         string    `json:"ip"`
	CapturedAt time.Time `json:"captured_at"`
}

var powAlgorithmValues = map[string]bool{"fast": true, "slow": true}

// ListRuleGroups returns all WAF rule groups.
func ListRuleGroups(ctx context.Context) ([]RuleGroupView, error) {
	if err := EnsureDefaultRuleGroup(ctx); err != nil {
		return nil, err
	}
	groups, err := model.ListOpenFlareWAFRuleGroups(ctx)
	if err != nil {
		return nil, err
	}
	bindings, err := loadRuleGroupBindings(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]RuleGroupView, 0, len(groups))
	for _, group := range groups {
		view, buildErr := buildRuleGroupView(group, bindings[group.ID])
		if buildErr != nil {
			return nil, buildErr
		}
		views = append(views, view)
	}
	return views, nil
}

// GetRuleGroup returns a WAF rule group by id.
func GetRuleGroup(ctx context.Context, id uint) (*RuleGroupView, error) {
	group, err := model.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	bindings, err := loadRuleGroupBindings(ctx)
	if err != nil {
		return nil, err
	}
	view, err := buildRuleGroupView(group, bindings[group.ID])
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// CreateRuleGroup creates a custom WAF rule group.
func CreateRuleGroup(ctx context.Context, input RuleGroupInput) (*RuleGroupView, error) {
	group, err := buildRuleGroup(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	group.IsGlobal = false
	if err = model.CreateOpenFlareWAFRuleGroup(ctx, group); err != nil {
		return nil, err
	}
	return GetRuleGroup(ctx, group.ID)
}

// UpdateRuleGroup updates a WAF rule group.
func UpdateRuleGroup(ctx context.Context, id uint, input RuleGroupInput) (*RuleGroupView, error) {
	group, err := model.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	isGlobal := group.IsGlobal
	group, err = buildRuleGroup(ctx, group, input)
	if err != nil {
		return nil, err
	}
	group.IsGlobal = isGlobal
	if isGlobal && strings.TrimSpace(group.Name) == "" {
		group.Name = "全局规则组"
	}
	if err = model.UpdateOpenFlareWAFRuleGroup(ctx, group); err != nil {
		return nil, err
	}
	return GetRuleGroup(ctx, group.ID)
}

// DeleteRuleGroup deletes a non-global WAF rule group.
func DeleteRuleGroup(ctx context.Context, id uint) error {
	group, err := model.GetOpenFlareWAFRuleGroupByID(ctx, id)
	if err != nil {
		return err
	}
	if group.IsGlobal {
		return errors.New("全局 WAF 规则组不能删除")
	}
	return model.DeleteOpenFlareWAFRuleGroupWithBindings(ctx, group.ID)
}

// ReplaceRuleGroupSites replaces site bindings for a rule group.
func ReplaceRuleGroupSites(ctx context.Context, groupID uint, routeIDs []uint) (*RuleGroupView, error) {
	group, err := model.GetOpenFlareWAFRuleGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group.IsGlobal {
		return nil, errors.New("全局 WAF 规则组默认应用到所有网站，不能手动绑定")
	}
	normalized, err := normalizeRouteIDs(ctx, routeIDs)
	if err != nil {
		return nil, err
	}
	if err = model.ReplaceOpenFlareWAFRuleGroupBindings(ctx, groupID, normalized); err != nil {
		return nil, err
	}
	return GetRuleGroup(ctx, groupID)
}

// GetSiteRuleGroups returns WAF rule groups for a proxy route.
func GetSiteRuleGroups(ctx context.Context, routeID uint) (*SiteRuleGroupsView, error) {
	if _, err := model.GetOpenFlareProxyRouteByID(ctx, routeID); err != nil {
		return nil, err
	}
	groups, err := ListRuleGroups(ctx)
	if err != nil {
		return nil, err
	}
	appliedIDs, err := ListSiteRuleGroupIDs(ctx, routeID)
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[uint]struct{}, len(appliedIDs))
	for _, id := range appliedIDs {
		appliedSet[id] = struct{}{}
	}
	var global *RuleGroupView
	custom := make([]RuleGroupView, 0, len(groups))
	applied := make([]RuleGroupView, 0, len(appliedIDs))
	for index := range groups {
		group := groups[index]
		if group.IsGlobal {
			item := group
			global = &item
			continue
		}
		custom = append(custom, group)
		if _, ok := appliedSet[group.ID]; ok {
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
		return nil, err
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
	group := &model.OpenFlareWAFRuleGroup{
		Name:              "全局规则组",
		Enabled:           true,
		IsGlobal:          true,
		BlockStatusCode:   defaultWAFBlockStatusCode,
		IPWhitelist:       "[]",
		IPBlacklist:       "[]",
		IPWhitelistGroups: "[]",
		IPBlacklistGroups: "[]",
		CountryWhitelist:  "[]",
		CountryBlacklist:  "[]",
		RegionWhitelist:   "[]",
		RegionBlacklist:   "[]",
		PoWEnabled:        false,
		PoWConfig:         "{}",
		BlockResponseBody: "",
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
		return nil, err
	}
	if err = model.CreateOpenFlareWAFIPGroup(ctx, group); err != nil {
		return nil, err
	}
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
		return nil, err
	}
	if err = model.UpdateOpenFlareWAFIPGroup(ctx, group); err != nil {
		return nil, err
	}
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
		return errors.New("IP 组已被 WAF 规则组引用，请先移除引用")
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
		return nil, err
	}
	now := time.Now().UTC()
	ips, err := evaluateParsedIPGroupAutoConfig(ctx, config, now)
	if err != nil {
		return nil, err
	}
	return &IPGroupAutoTestResult{
		MatchedIPs:      ips,
		MatchedCount:    len(ips),
		LookbackMinutes: config.LookbackMinutes,
		RuleCount:       len(config.Rules),
		TestedAt:        now.Format(time.RFC3339),
	}, nil
}

func buildRuleGroup(ctx context.Context, group *model.OpenFlareWAFRuleGroup, input RuleGroupInput) (*model.OpenFlareWAFRuleGroup, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("规则组名称不能为空")
	}
	statusCode := input.BlockStatusCode
	if statusCode == 0 {
		statusCode = defaultWAFBlockStatusCode
	}
	if statusCode < 400 || statusCode > 599 {
		return nil, errors.New("拦截状态码必须在 400-599 之间")
	}
	if len([]byte(input.BlockResponseBody)) > maxWAFBlockBodyBytes {
		return nil, fmt.Errorf("拦截页面内容不能超过 %d 字节", maxWAFBlockBodyBytes)
	}
	ipWhitelist, err := normalizeIPList(input.IPWhitelist)
	if err != nil {
		return nil, fmt.Errorf("IP 白名单无效: %w", err)
	}
	ipBlacklist, err := normalizeIPList(input.IPBlacklist)
	if err != nil {
		return nil, fmt.Errorf("IP 黑名单无效: %w", err)
	}
	ipWhitelistGroups, err := normalizeIPGroupIDs(ctx, input.IPWhitelistGroups)
	if err != nil {
		return nil, fmt.Errorf("IP 白名单引用无效: %w", err)
	}
	ipBlacklistGroups, err := normalizeIPGroupIDs(ctx, input.IPBlacklistGroups)
	if err != nil {
		return nil, fmt.Errorf("IP 黑名单引用无效: %w", err)
	}
	countryWhitelist, err := normalizeCountryList(input.CountryWhitelist)
	if err != nil {
		return nil, fmt.Errorf("地域白名单无效: %w", err)
	}
	countryBlacklist, err := normalizeCountryList(input.CountryBlacklist)
	if err != nil {
		return nil, fmt.Errorf("地域黑名单无效: %w", err)
	}
	regionWhitelist := normalizeStringList(input.RegionWhitelist)
	regionBlacklist := normalizeStringList(input.RegionBlacklist)
	powConfigRaw := strings.TrimSpace(string(input.PoWConfig))
	if powConfigRaw == "" {
		powConfigRaw = "{}"
	}
	powConfig, err := normalizePoWConfig(input.PoWEnabled, powConfigRaw)
	if err != nil {
		return nil, err
	}
	powConfigJSON, _ := json.Marshal(powConfig)

	ipWhitelistJSON, _ := json.Marshal(ipWhitelist)
	ipBlacklistJSON, _ := json.Marshal(ipBlacklist)
	ipWhitelistGroupsJSON, _ := json.Marshal(ipWhitelistGroups)
	ipBlacklistGroupsJSON, _ := json.Marshal(ipBlacklistGroups)
	countryWhitelistJSON, _ := json.Marshal(countryWhitelist)
	countryBlacklistJSON, _ := json.Marshal(countryBlacklist)
	regionWhitelistJSON, _ := json.Marshal(regionWhitelist)
	regionBlacklistJSON, _ := json.Marshal(regionBlacklist)

	if group == nil {
		group = &model.OpenFlareWAFRuleGroup{}
	}
	group.Name = name
	group.Enabled = input.Enabled
	group.BlockStatusCode = statusCode
	group.BlockResponseBody = input.BlockResponseBody
	group.IPWhitelist = string(ipWhitelistJSON)
	group.IPBlacklist = string(ipBlacklistJSON)
	group.IPWhitelistGroups = string(ipWhitelistGroupsJSON)
	group.IPBlacklistGroups = string(ipBlacklistGroupsJSON)
	group.CountryWhitelist = string(countryWhitelistJSON)
	group.CountryBlacklist = string(countryBlacklistJSON)
	group.RegionWhitelist = string(regionWhitelistJSON)
	group.RegionBlacklist = string(regionBlacklistJSON)
	group.PoWEnabled = input.PoWEnabled
	group.PoWConfig = string(powConfigJSON)
	group.Remark = strings.TrimSpace(input.Remark)
	return group, nil
}

func buildRuleGroupView(group *model.OpenFlareWAFRuleGroup, appliedSiteIDs []uint) (RuleGroupView, error) {
	if group == nil {
		return RuleGroupView{}, errors.New("waf rule group is nil")
	}
	sort.Slice(appliedSiteIDs, func(i, j int) bool { return appliedSiteIDs[i] < appliedSiteIDs[j] })
	view := RuleGroupView{
		ID:                group.ID,
		Name:              group.Name,
		Enabled:           group.Enabled,
		IsGlobal:          group.IsGlobal,
		BlockStatusCode:   group.BlockStatusCode,
		BlockResponseBody: group.BlockResponseBody,
		Remark:            group.Remark,
		PoWEnabled:        group.PoWEnabled,
		AppliedSiteIDs:    appliedSiteIDs,
		AppliedSiteCount:  len(appliedSiteIDs),
		CreatedAt:         group.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         group.UpdatedAt.Format(time.RFC3339),
	}
	var err error
	if view.IPWhitelist, err = decodeStringList(group.IPWhitelist); err != nil {
		return view, err
	}
	if view.IPBlacklist, err = decodeStringList(group.IPBlacklist); err != nil {
		return view, err
	}
	view.IPWhitelistGroups = mustDecodeUintList(group.IPWhitelistGroups)
	view.IPBlacklistGroups = mustDecodeUintList(group.IPBlacklistGroups)
	if view.CountryWhitelist, err = decodeStringList(group.CountryWhitelist); err != nil {
		return view, err
	}
	if view.CountryBlacklist, err = decodeStringList(group.CountryBlacklist); err != nil {
		return view, err
	}
	if view.RegionWhitelist, err = decodeStringList(group.RegionWhitelist); err != nil {
		return view, err
	}
	if view.RegionBlacklist, err = decodeStringList(group.RegionBlacklist); err != nil {
		return view, err
	}
	if view.PoWConfig, err = decodeStoredPoWConfig(group.PoWEnabled, group.PoWConfig); err != nil {
		return view, err
	}
	return view, nil
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
	group.Remark = strings.TrimSpace(input.Remark)
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
		Remark:                  group.Remark,
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
		for _, id := range mustDecodeUintList(group.IPWhitelistGroups) {
			counts[id]++
		}
		for _, id := range mustDecodeUintList(group.IPBlacklistGroups) {
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

func normalizeCountryList(items []string) ([]string, error) {
	normalized := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.ToUpper(strings.TrimSpace(raw))
		if item == "" {
			continue
		}
		if len(item) != 2 || !unicode.IsLetter(rune(item[0])) || !unicode.IsLetter(rune(item[1])) {
			return nil, fmt.Errorf("%s 不是合法国家代码", item)
		}
		normalized = append(normalized, item)
	}
	normalized = uniqueStrings(normalized)
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeStringList(items []string) []string {
	normalized := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	normalized = uniqueStrings(normalized)
	sort.Strings(normalized)
	return normalized
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

func normalizeRouteIDs(ctx context.Context, routeIDs []uint) ([]uint, error) {
	normalized := uniqueUintIDs(routeIDs)
	for _, routeID := range normalized {
		if _, err := model.GetOpenFlareProxyRouteByID(ctx, routeID); err != nil {
			return nil, fmt.Errorf("网站 %d 不存在", routeID)
		}
	}
	return normalized, nil
}

func normalizeRuleGroupIDs(ctx context.Context, groupIDs []uint) ([]uint, error) {
	normalized := uniqueUintIDs(groupIDs)
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

func normalizeIPGroupIDs(ctx context.Context, ids []uint) ([]uint, error) {
	normalized := uniqueUintIDs(ids)
	for _, id := range normalized {
		if _, err := model.GetOpenFlareWAFIPGroupByID(ctx, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("IP 组 %d 不存在", id)
			}
			return nil, err
		}
	}
	return normalized, nil
}

func uniqueUintIDs(ids []uint) []uint {
	normalized := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		normalized = append(normalized, id)
	}
	normalized = uniqueUints(normalized)
	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })
	return normalized
}

func uniqueUints(items []uint) []uint {
	seen := make(map[uint]struct{}, len(items))
	result := make([]uint, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
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

func mustDecodeUintList(raw string) []uint {
	var values []uint
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &values); err != nil {
		return []uint{}
	}
	values = uniqueUintIDs(values)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values
}

func defaultPoWConfig() PoWConfig {
	return PoWConfig{
		Difficulty:   4,
		Algorithm:    "fast",
		SessionTTL:   600,
		ChallengeTTL: 300,
		Whitelist:    PoWListConfig{IPs: []string{}, IPCidrs: []string{}, Paths: []string{}, PathRegexes: []string{}, UserAgents: []string{}},
		Blacklist:    PoWListConfig{IPs: []string{}, IPCidrs: []string{}, Paths: []string{}, PathRegexes: []string{}, UserAgents: []string{}},
	}
}

func normalizePoWConfig(enabled bool, raw string) (PoWConfig, error) {
	if !enabled {
		return defaultPoWConfig(), nil
	}

	cfg := defaultPoWConfig()
	text := strings.TrimSpace(raw)
	if text != "" && text != "{}" {
		if err := json.Unmarshal([]byte(text), &cfg); err != nil {
			return cfg, errors.New("pow_config 格式无效")
		}
	}

	if cfg.Difficulty < 1 || cfg.Difficulty > 16 {
		return cfg, errors.New("pow_config.difficulty 必须在 1-16 之间")
	}
	if !powAlgorithmValues[cfg.Algorithm] {
		return cfg, errors.New("pow_config.algorithm 必须为 fast 或 slow")
	}
	if cfg.SessionTTL < 60 {
		return cfg, errors.New("pow_config.session_ttl 不能小于 60 秒")
	}
	if cfg.ChallengeTTL < 30 {
		return cfg, errors.New("pow_config.challenge_ttl 不能小于 30 秒")
	}

	for _, cidr := range cfg.Whitelist.IPCidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return cfg, fmt.Errorf("pow_config 白名单 IP CIDR 格式无效: %s", cidr)
		}
	}
	for _, cidr := range cfg.Blacklist.IPCidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return cfg, fmt.Errorf("pow_config 黑名单 IP CIDR 格式无效: %s", cidr)
		}
	}

	for _, re := range cfg.Whitelist.PathRegexes {
		if _, err := regexp.Compile(re); err != nil {
			return cfg, fmt.Errorf("pow_config 白名单路径正则格式无效: %s", re)
		}
	}
	for _, re := range cfg.Blacklist.PathRegexes {
		if _, err := regexp.Compile(re); err != nil {
			return cfg, fmt.Errorf("pow_config 黑名单路径正则格式无效: %s", re)
		}
	}

	for _, ip := range cfg.Whitelist.IPs {
		if net.ParseIP(ip) == nil {
			return cfg, fmt.Errorf("pow_config 白名单 IP 格式无效: %s", ip)
		}
	}
	for _, ip := range cfg.Blacklist.IPs {
		if net.ParseIP(ip) == nil {
			return cfg, fmt.Errorf("pow_config 黑名单 IP 格式无效: %s", ip)
		}
	}

	type dimension struct {
		name string
		wl   []string
		bl   []string
	}
	dimensions := []dimension{
		{"IP", cfg.Whitelist.IPs, cfg.Blacklist.IPs},
		{"IP CIDR", cfg.Whitelist.IPCidrs, cfg.Blacklist.IPCidrs},
		{"路径", cfg.Whitelist.Paths, cfg.Blacklist.Paths},
		{"路径正则", cfg.Whitelist.PathRegexes, cfg.Blacklist.PathRegexes},
		{"User-Agent", cfg.Whitelist.UserAgents, cfg.Blacklist.UserAgents},
	}
	for _, dim := range dimensions {
		if len(dim.wl) > 0 && len(dim.bl) > 0 {
			return cfg, fmt.Errorf("pow_config %s 不能同时配置白名单和黑名单", dim.name)
		}
	}

	return cfg, nil
}

func decodeStoredPoWConfig(enabled bool, raw string) (*PoWConfig, error) {
	if !enabled {
		cfg := defaultPoWConfig()
		return &cfg, nil
	}
	text := strings.TrimSpace(raw)
	if text == "" || text == "{}" {
		cfg := defaultPoWConfig()
		return &cfg, nil
	}
	var cfg PoWConfig
	if err := json.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, errors.New("pow_config 格式无效")
	}
	return &cfg, nil
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
	var config ipGroupAutoConfig
	if err := json.Unmarshal([]byte(text), &config); err != nil {
		return ipGroupAutoConfig{}, errors.New("自动 IP 组配置必须是 JSON 对象")
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(text), &object); err != nil || object == nil {
		return ipGroupAutoConfig{}, errors.New("自动 IP 组配置必须是 JSON 对象")
	}
	if config.LookbackMinutes <= 0 {
		config.LookbackMinutes = defaultWAFIPGroupAutoLookbackMinutes
	}
	if config.LookbackMinutes < 5 {
		config.LookbackMinutes = 5
	}
	if config.LookbackMinutes > 43200 {
		config.LookbackMinutes = 43200
	}
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
