package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"openflare/model"
	"sort"
	"strings"
	"time"

	exprlang "github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"gorm.io/gorm"
)

const (
	WAFIPGroupTypeManual       = "manual"
	WAFIPGroupTypeAutomatic    = "automatic"
	WAFIPGroupTypeSubscription = "subscription"

	WAFIPGroupSubscriptionFormatText = "text"
	WAFIPGroupSubscriptionFormatJSON = "json"

	defaultWAFIPGroupSyncIntervalMinutes = 1440
	defaultWAFIPGroupAutoLookbackMinutes = 60
	minWAFIPGroupSyncIntervalMinutes     = 5
	maxWAFIPGroupSyncIntervalMinutes     = 43200
	maxWAFIPGroupSubscriptionBytes       = 2 * 1024 * 1024
)

type wafIPGroupAutoConfig struct {
	LookbackMinutes int                  `json:"lookback_minutes"`
	TTL             int                  `json:"ttl"` // in seconds, default -1 (permanent)
	Rules           []wafIPGroupAutoRule `json:"rules"`
}

type WAFIPGroupExtIP struct {
	IP         string    `json:"ip"`
	CapturedAt time.Time `json:"captured_at"`
}

type WAFIPGroupExtIPView struct {
	IP         string `json:"ip"`
	CapturedAt string `json:"captured_at"`
}

type wafIPGroupAutoRule struct {
	Name string `json:"name"`
	Expr string `json:"expr"`
}

type wafIPGroupAutoRuleEnv struct {
	IP               string  `expr:"ip"`
	RequestCount     int     `expr:"request_count"`
	Status404Count   int     `expr:"status_404_count"`
	Status404Ratio   float64 `expr:"status_404_ratio"`
	IPHostCount      int     `expr:"ip_host_count"`
	IPHostRatio      float64 `expr:"ip_host_ratio"`
	ClientErrorCount int     `expr:"client_error_count"`
	ServerErrorCount int     `expr:"server_error_count"`
	LastSeenUnix     int64   `expr:"last_seen_unix"`
}

type wafIPGroupAutoAccumulator struct {
	ip               string
	requestCount     int
	status404Count   int
	ipHostCount      int
	clientErrorCount int
	serverErrorCount int
	lastSeen         time.Time
}

type WAFIPGroupInput struct {
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

type WAFIPGroupView struct {
	ID                      uint                  `json:"id"`
	Name                    string                `json:"name"`
	Type                    string                `json:"type"`
	Enabled                 bool                  `json:"enabled"`
	IPList                  []string              `json:"ip_list"`
	AutoConfig              json.RawMessage       `json:"auto_config"`
	ExtIPs                  []WAFIPGroupExtIPView `json:"ext_ips"`
	SubscriptionURL         string                `json:"subscription_url"`
	SubscriptionFormat      string                `json:"subscription_format"`
	SubscriptionMappingRule string                `json:"subscription_mapping_rule"`
	SyncIntervalMinutes     int                   `json:"sync_interval_minutes"`
	LastSyncedAt            string                `json:"last_synced_at,omitempty"`
	NextSyncAt              string                `json:"next_sync_at,omitempty"`
	LastSyncStatus          string                `json:"last_sync_status"`
	LastSyncMessage         string                `json:"last_sync_message"`
	Remark                  string                `json:"remark"`
	ReferencedByRuleCount   int                   `json:"referenced_by_rule_count"`
	CreatedAt               string                `json:"created_at"`
	UpdatedAt               string                `json:"updated_at"`
}

type WAFIPGroupSyncResult struct {
	Group      WAFIPGroupView `json:"group"`
	IPCount    int            `json:"ip_count"`
	SyncedAt   string         `json:"synced_at"`
	NextSyncAt string         `json:"next_sync_at"`
	Status     string         `json:"status"`
	Message    string         `json:"message"`
}

type WAFIPGroupAutoTestInput struct {
	AutoConfig json.RawMessage `json:"auto_config"`
}

type WAFIPGroupAutoTestResult struct {
	MatchedIPs      []string `json:"matched_ips"`
	MatchedCount    int      `json:"matched_count"`
	LookbackMinutes int      `json:"lookback_minutes"`
	RuleCount       int      `json:"rule_count"`
	TestedAt        string   `json:"tested_at"`
}

func ListWAFIPGroups() ([]WAFIPGroupView, error) {
	groups, err := model.ListWAFIPGroups()
	if err != nil {
		return nil, err
	}
	referenceCounts, err := loadWAFIPGroupReferenceCounts()
	if err != nil {
		return nil, err
	}
	views := make([]WAFIPGroupView, 0, len(groups))
	for _, group := range groups {
		view, err := buildWAFIPGroupView(group, referenceCounts[group.ID])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func GetWAFIPGroup(id uint) (*WAFIPGroupView, error) {
	group, err := model.GetWAFIPGroupByID(id)
	if err != nil {
		return nil, err
	}
	referenceCounts, err := loadWAFIPGroupReferenceCounts()
	if err != nil {
		return nil, err
	}
	view, err := buildWAFIPGroupView(group, referenceCounts[group.ID])
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func CreateWAFIPGroup(input WAFIPGroupInput) (*WAFIPGroupView, error) {
	group, err := buildWAFIPGroup(nil, input)
	if err != nil {
		return nil, err
	}
	if err := group.Insert(); err != nil {
		return nil, err
	}
	view, err := GetWAFIPGroup(group.ID)
	if err == nil {
		broadcastWAFIPGroupToAgents(group.ID)
	}
	return view, err
}

func UpdateWAFIPGroup(id uint, input WAFIPGroupInput) (*WAFIPGroupView, error) {
	group, err := model.GetWAFIPGroupByID(id)
	if err != nil {
		return nil, err
	}
	group, err = buildWAFIPGroup(group, input)
	if err != nil {
		return nil, err
	}
	if err := group.Update(); err != nil {
		return nil, err
	}
	view, err := GetWAFIPGroup(group.ID)
	if err == nil {
		broadcastWAFIPGroupToAgents(group.ID)
	}
	return view, err
}

func DeleteWAFIPGroup(id uint) error {
	group, err := model.GetWAFIPGroupByID(id)
	if err != nil {
		return err
	}
	counts, err := loadWAFIPGroupReferenceCounts()
	if err != nil {
		return err
	}
	if counts[group.ID] > 0 {
		return errors.New("IP 组已被 WAF 规则组引用，请先移除引用")
	}
	return group.Delete()
}

func SyncWAFIPGroup(id uint) (*WAFIPGroupSyncResult, error) {
	group, err := model.GetWAFIPGroupByID(id)
	if err != nil {
		return nil, err
	}
	return syncWAFIPGroup(group, time.Now().UTC())
}

func TestWAFIPGroupAutoConfig(input WAFIPGroupAutoTestInput) (*WAFIPGroupAutoTestResult, error) {
	config, err := parseWAFIPGroupAutoConfig(input.AutoConfig)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	ips, err := evaluateParsedWAFIPGroupAutoConfig(config, now)
	if err != nil {
		return nil, err
	}
	return &WAFIPGroupAutoTestResult{
		MatchedIPs:      ips,
		MatchedCount:    len(ips),
		LookbackMinutes: config.LookbackMinutes,
		RuleCount:       len(config.Rules),
		TestedAt:        now.Format(time.RFC3339),
	}, nil
}

func SyncDueWAFIPGroups() error {
	now := time.Now().UTC()
	groups, err := model.ListDueWAFIPGroups(now)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if _, err := syncWAFIPGroup(group, now); err != nil {
			continue
		}
	}
	return nil
}

func buildWAFIPGroup(group *model.WAFIPGroup, input WAFIPGroupInput) (*model.WAFIPGroup, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("IP 组名称不能为空")
	}
	groupType := normalizeWAFIPGroupType(input.Type)
	if groupType == "" {
		return nil, errors.New("IP 组类型无效")
	}
	ipList := input.IPList
	subscriptionURL := ""
	subscriptionFormat := normalizeWAFIPGroupSubscriptionFormat(input.SubscriptionFormat)
	mappingRule := strings.TrimSpace(input.SubscriptionMappingRule)
	syncInterval := normalizeWAFIPGroupSyncInterval(input.SyncIntervalMinutes)
	autoConfig := "{}"

	switch groupType {
	case WAFIPGroupTypeManual:
		subscriptionFormat = WAFIPGroupSubscriptionFormatText
		mappingRule = ""
	case WAFIPGroupTypeAutomatic:
		normalizedConfig, err := normalizeWAFIPGroupAutoConfig(input.AutoConfig)
		if err != nil {
			return nil, err
		}
		autoConfig = normalizedConfig
		subscriptionFormat = WAFIPGroupSubscriptionFormatText
		mappingRule = ""
	case WAFIPGroupTypeSubscription:
		subscriptionURL = strings.TrimSpace(input.SubscriptionURL)
		if err := validateSubscriptionURL(subscriptionURL); err != nil {
			return nil, err
		}
		if subscriptionFormat == "" {
			subscriptionFormat = WAFIPGroupSubscriptionFormatText
		}
	}

	normalizedIPs, err := normalizeWAFIPList(ipList)
	if err != nil {
		return nil, err
	}
	ipListJSON, _ := json.Marshal(normalizedIPs)
	if group == nil {
		group = &model.WAFIPGroup{}
		group.ExtIPs = "[]"
	}
	group.Name = name
	group.Type = groupType
	group.Enabled = input.Enabled
	group.IPList = string(ipListJSON)
	group.AutoConfig = autoConfig
	group.SubscriptionURL = subscriptionURL
	group.SubscriptionFormat = subscriptionFormat
	group.SubscriptionMappingRule = mappingRule
	group.SyncIntervalMinutes = syncInterval
	group.NextSyncAt = nextWAFIPGroupSyncAt(group.Type, group.Enabled, syncInterval, group.NextSyncAt)
	group.Remark = strings.TrimSpace(input.Remark)
	return group, nil
}

func buildWAFIPGroupView(group *model.WAFIPGroup, referenceCount int) (WAFIPGroupView, error) {
	if group == nil {
		return WAFIPGroupView{}, errors.New("waf ip group is nil")
	}
	ips, err := decodeStringList(group.IPList)
	if err != nil {
		return WAFIPGroupView{}, err
	}
	autoConfig := json.RawMessage(strings.TrimSpace(group.AutoConfig))
	if len(autoConfig) == 0 {
		autoConfig = json.RawMessage("{}")
	}
	var extIPs []WAFIPGroupExtIP
	if group.ExtIPs != "" && group.ExtIPs != "[]" {
		_ = json.Unmarshal([]byte(group.ExtIPs), &extIPs)
	}
	viewExtIPs := make([]WAFIPGroupExtIPView, 0, len(extIPs))
	for _, extIP := range extIPs {
		viewExtIPs = append(viewExtIPs, WAFIPGroupExtIPView{
			IP:         extIP.IP,
			CapturedAt: extIP.CapturedAt.Format(time.RFC3339),
		})
	}
	view := WAFIPGroupView{
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

func ChangedWAFIPGroupsForAgent(ids []uint, checksums map[string]string) ([]AgentWAFIPGroup, error) {
	targetIDs := uniqueUintIDs(ids)
	if len(targetIDs) == 0 {
		activeIDs, err := activeConfigWAFIPGroupIDs()
		if err != nil {
			return nil, err
		}
		targetIDs = activeIDs
	}
	if len(targetIDs) == 0 {
		return []AgentWAFIPGroup{}, nil
	}
	groups, err := buildAgentWAFIPGroups(targetIDs)
	if err != nil {
		return nil, err
	}
	changed := make([]AgentWAFIPGroup, 0, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(checksums[fmt.Sprintf("%d", group.ID)]) == group.Checksum {
			continue
		}
		changed = append(changed, group)
	}
	return changed, nil
}

func SyncWAFIPGroupsForAgent(input AgentWAFIPGroupSyncInput) (*AgentWAFIPGroupSyncResult, error) {
	groups, err := ChangedWAFIPGroupsForAgent(input.IDs, input.Checksums)
	if err != nil {
		return nil, err
	}
	return &AgentWAFIPGroupSyncResult{Groups: groups}, nil
}

func buildAgentWAFIPGroups(ids []uint) ([]AgentWAFIPGroup, error) {
	ids = uniqueUintIDs(ids)
	if len(ids) == 0 {
		return []AgentWAFIPGroup{}, nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	groups, err := model.ListWAFIPGroupsByIDs(ids)
	if err != nil {
		return nil, err
	}
	groupByID := make(map[uint]*model.WAFIPGroup, len(groups))
	for _, group := range groups {
		groupByID[group.ID] = group
	}
	result := make([]AgentWAFIPGroup, 0, len(ids))
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

func buildAgentWAFIPGroup(group *model.WAFIPGroup) (AgentWAFIPGroup, error) {
	if group == nil {
		return AgentWAFIPGroup{}, errors.New("IP 组不存在")
	}
	ips, err := decodeStringList(group.IPList)
	if err != nil {
		return AgentWAFIPGroup{}, err
	}
	if !group.Enabled {
		ips = []string{}
	}
	agentGroup := AgentWAFIPGroup{
		ID:      group.ID,
		Name:    group.Name,
		Type:    group.Type,
		Enabled: group.Enabled,
		IPList:  ips,
	}
	agentGroup.Checksum = checksumAgentWAFIPGroup(agentGroup)
	return agentGroup, nil
}

func checksumAgentWAFIPGroup(group AgentWAFIPGroup) string {
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

func activeConfigWAFIPGroupIDs() ([]uint, error) {
	version, err := model.GetActiveConfigVersion()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []uint{}, nil
		}
		return nil, err
	}
	snapshot, err := parseSnapshotDocument(version.SnapshotJSON)
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

func broadcastWAFIPGroupToAgents(id uint) {
	groups, err := buildAgentWAFIPGroups([]uint{id})
	if err != nil || len(groups) == 0 {
		if err != nil {
			slog.Debug("build waf ip group broadcast payload failed", "id", id, "error", err)
		}
		return
	}
	BroadcastAgentWSWAFIPGroups(groups)
}

func syncWAFIPGroup(group *model.WAFIPGroup, now time.Time) (*WAFIPGroupSyncResult, error) {
	if group == nil {
		return nil, errors.New("IP 组不存在")
	}
	switch group.Type {
	case WAFIPGroupTypeSubscription:
		return syncWAFIPGroupSubscription(group, now)
	case WAFIPGroupTypeAutomatic:
		return syncWAFIPGroupAutomatic(group, now)
	default:
		return nil, errors.New("只有自动和订阅类型 IP 组支持同步")
	}
}

func syncWAFIPGroupSubscription(group *model.WAFIPGroup, now time.Time) (*WAFIPGroupSyncResult, error) {
	content, err := downloadWAFIPGroupSubscription(group.SubscriptionURL)
	if err != nil {
		recordWAFIPGroupSyncFailure(group, now, err)
		return nil, err
	}
	ips, err := parseWAFIPGroupSubscription(content, group.SubscriptionFormat, group.SubscriptionMappingRule)
	if err != nil {
		recordWAFIPGroupSyncFailure(group, now, err)
		return nil, err
	}
	ipListJSON, _ := json.Marshal(ips)
	nextSyncAt := now.Add(time.Duration(group.SyncIntervalMinutes) * time.Minute)
	group.IPList = string(ipListJSON)
	group.LastSyncedAt = &now
	group.NextSyncAt = &nextSyncAt
	group.LastSyncStatus = "success"
	group.LastSyncMessage = fmt.Sprintf("同步成功，共 %d 条 IP/IP 段", len(ips))
	if err := group.UpdateSyncResult(); err != nil {
		return nil, err
	}
	broadcastWAFIPGroupToAgents(group.ID)
	view, err := GetWAFIPGroup(group.ID)
	if err != nil {
		return nil, err
	}
	return &WAFIPGroupSyncResult{
		Group:      *view,
		IPCount:    len(ips),
		SyncedAt:   now.Format(time.RFC3339),
		NextSyncAt: nextSyncAt.Format(time.RFC3339),
		Status:     group.LastSyncStatus,
		Message:    group.LastSyncMessage,
	}, nil
}

func syncWAFIPGroupAutomatic(group *model.WAFIPGroup, now time.Time) (*WAFIPGroupSyncResult, error) {
	config, err := parseWAFIPGroupAutoConfig(json.RawMessage(group.AutoConfig))
	if err != nil {
		recordWAFIPGroupSyncFailure(group, now, err)
		return nil, err
	}

	var existingExtIPs []WAFIPGroupExtIP
	if group.ExtIPs != "" && group.ExtIPs != "[]" {
		_ = json.Unmarshal([]byte(group.ExtIPs), &existingExtIPs)
	}

	activeExtIPs := make([]WAFIPGroupExtIP, 0, len(existingExtIPs))
	for _, extIP := range existingExtIPs {
		if config.TTL > 0 {
			expirationTime := extIP.CapturedAt.Add(time.Duration(config.TTL) * time.Second)
			if expirationTime.Before(now) {
				continue
			}
		}
		activeExtIPs = append(activeExtIPs, extIP)
	}

	ips, err := evaluateParsedWAFIPGroupAutoConfig(config, now)
	if err != nil {
		recordWAFIPGroupSyncFailure(group, now, err)
		return nil, err
	}

	extIPMap := make(map[string]int)
	for idx, extIP := range activeExtIPs {
		extIPMap[extIP.IP] = idx
	}

	for _, ip := range ips {
		if idx, ok := extIPMap[ip]; ok {
			activeExtIPs[idx].CapturedAt = now
		} else {
			activeExtIPs = append(activeExtIPs, WAFIPGroupExtIP{
				IP:         ip,
				CapturedAt: now,
			})
		}
	}

	finalIPs := make([]string, 0, len(activeExtIPs))
	for _, extIP := range activeExtIPs {
		finalIPs = append(finalIPs, extIP.IP)
	}
	finalIPs, err = normalizeWAFIPList(finalIPs)
	if err != nil {
		recordWAFIPGroupSyncFailure(group, now, err)
		return nil, err
	}

	extIPsJSON, _ := json.Marshal(activeExtIPs)
	ipListJSON, _ := json.Marshal(finalIPs)

	nextSyncAt := now.Add(time.Duration(normalizeWAFIPGroupSyncInterval(group.SyncIntervalMinutes)) * time.Minute)
	group.IPList = string(ipListJSON)
	group.ExtIPs = string(extIPsJSON)
	group.LastSyncedAt = &now
	group.NextSyncAt = &nextSyncAt
	group.LastSyncStatus = "success"
	group.LastSyncMessage = fmt.Sprintf("自动规则执行成功，共命中 %d 个 IP，当前生效 %d 个 IP", len(ips), len(finalIPs))
	if err := group.UpdateSyncResult(); err != nil {
		return nil, err
	}
	broadcastWAFIPGroupToAgents(group.ID)
	view, err := GetWAFIPGroup(group.ID)
	if err != nil {
		return nil, err
	}
	return &WAFIPGroupSyncResult{
		Group:      *view,
		IPCount:    len(finalIPs),
		SyncedAt:   now.Format(time.RFC3339),
		NextSyncAt: nextSyncAt.Format(time.RFC3339),
		Status:     group.LastSyncStatus,
		Message:    group.LastSyncMessage,
	}, nil
}

func recordWAFIPGroupSyncFailure(group *model.WAFIPGroup, now time.Time, syncErr error) {
	nextSyncAt := now.Add(time.Duration(normalizeWAFIPGroupSyncInterval(group.SyncIntervalMinutes)) * time.Minute)
	group.LastSyncedAt = &now
	group.NextSyncAt = &nextSyncAt
	group.LastSyncStatus = "failed"
	group.LastSyncMessage = syncErr.Error()
	_ = group.UpdateSyncResult()
}

func normalizeWAFIPGroupAutoConfig(raw json.RawMessage) (string, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		text = "{}"
	}
	config, err := parseWAFIPGroupAutoConfig(json.RawMessage(text))
	if err != nil {
		return "", err
	}
	normalized, _ := json.Marshal(config)
	return string(normalized), nil
}

func evaluateWAFIPGroupAutoConfig(raw string, now time.Time) ([]string, error) {
	config, err := parseWAFIPGroupAutoConfig(json.RawMessage(raw))
	if err != nil {
		return nil, err
	}
	return evaluateParsedWAFIPGroupAutoConfig(config, now)
}

func parseWAFIPGroupAutoConfig(raw json.RawMessage) (wafIPGroupAutoConfig, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		text = "{}"
	}
	var config wafIPGroupAutoConfig
	if err := json.Unmarshal([]byte(text), &config); err != nil {
		return wafIPGroupAutoConfig{}, errors.New("自动 IP 组配置必须是 JSON 对象")
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(text), &object); err != nil || object == nil {
		return wafIPGroupAutoConfig{}, errors.New("自动 IP 组配置必须是 JSON 对象")
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
		config.Rules = []wafIPGroupAutoRule{}
	}
	for i, rule := range config.Rules {
		rule.Name = strings.TrimSpace(rule.Name)
		rule.Expr = strings.TrimSpace(rule.Expr)
		if rule.Expr == "" {
			return wafIPGroupAutoConfig{}, fmt.Errorf("自动规则 %d 的 Expr 表达式不能为空", i+1)
		}
		if _, err := exprlang.Compile(rule.Expr, exprlang.Env(wafIPGroupAutoRuleEnv{}), exprlang.AsBool()); err != nil {
			return wafIPGroupAutoConfig{}, fmt.Errorf("自动规则 %s Expr 无效: %w", displayWAFIPGroupAutoRuleName(rule, i), err)
		}
		config.Rules[i] = rule
	}
	return config, nil
}

func evaluateParsedWAFIPGroupAutoConfig(config wafIPGroupAutoConfig, now time.Time) ([]string, error) {
	if len(config.Rules) == 0 {
		return []string{}, nil
	}
	programs := make([]*vm.Program, 0, len(config.Rules))
	for i, rule := range config.Rules {
		program, err := exprlang.Compile(rule.Expr, exprlang.Env(wafIPGroupAutoRuleEnv{}), exprlang.AsBool())
		if err != nil {
			return nil, fmt.Errorf("自动规则 %s Expr 无效: %w", displayWAFIPGroupAutoRuleName(rule, i), err)
		}
		programs = append(programs, program)
	}
	logs, err := model.ListNodeAccessLogsForWAFIPGroup(model.NodeAccessLogQuery{
		Since: now.Add(-time.Duration(config.LookbackMinutes) * time.Minute),
		Until: now,
	})
	if err != nil {
		return nil, err
	}
	accumulators := make(map[string]*wafIPGroupAutoAccumulator)
	for _, item := range logs {
		if item == nil {
			continue
		}
		ip, ok := normalizeIPLiteral(item.RemoteAddr)
		if !ok {
			continue
		}
		acc := accumulators[ip]
		if acc == nil {
			acc = &wafIPGroupAutoAccumulator{ip: ip}
			accumulators[ip] = acc
		}
		acc.requestCount++
		if item.StatusCode == http.StatusNotFound {
			acc.status404Count++
		}
		if item.StatusCode >= 400 && item.StatusCode < 500 {
			acc.clientErrorCount++
		}
		if item.StatusCode >= 500 {
			acc.serverErrorCount++
		}
		if hostIsIPLiteral(item.Host) {
			acc.ipHostCount++
		}
		if item.LoggedAt.After(acc.lastSeen) {
			acc.lastSeen = item.LoggedAt
		}
	}
	matched := make([]string, 0)
	for _, acc := range accumulators {
		env := acc.toExprEnv()
		for _, program := range programs {
			output, err := exprlang.Run(program, env)
			if err != nil {
				return nil, fmt.Errorf("执行自动规则失败: %w", err)
			}
			if matchedRule, ok := output.(bool); ok && matchedRule {
				matched = append(matched, acc.ip)
				break
			}
		}
	}
	return normalizeWAFIPList(matched)
}

func (acc *wafIPGroupAutoAccumulator) toExprEnv() wafIPGroupAutoRuleEnv {
	env := wafIPGroupAutoRuleEnv{
		IP:               acc.ip,
		RequestCount:     acc.requestCount,
		Status404Count:   acc.status404Count,
		IPHostCount:      acc.ipHostCount,
		ClientErrorCount: acc.clientErrorCount,
		ServerErrorCount: acc.serverErrorCount,
	}
	if acc.requestCount > 0 {
		env.Status404Ratio = float64(acc.status404Count) / float64(acc.requestCount)
		env.IPHostRatio = float64(acc.ipHostCount) / float64(acc.requestCount)
	}
	if !acc.lastSeen.IsZero() {
		env.LastSeenUnix = acc.lastSeen.Unix()
	}
	return env
}

func displayWAFIPGroupAutoRuleName(rule wafIPGroupAutoRule, index int) string {
	if rule.Name != "" {
		return rule.Name
	}
	return fmt.Sprintf("#%d", index+1)
}

func normalizeIPLiteral(value string) (string, bool) {
	host := strings.TrimSpace(value)
	if host == "" {
		return "", false
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	return addr.String(), true
}

func hostIsIPLiteral(value string) bool {
	_, ok := normalizeIPLiteral(value)
	return ok
}

func downloadWAFIPGroupSubscription(rawURL string) ([]byte, error) {
	if err := validateSubscriptionURL(rawURL); err != nil {
		return nil, err
	}
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("下载订阅失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("订阅返回状态码 %d", resp.StatusCode)
	}
	var buffer bytes.Buffer
	reader := io.LimitReader(resp.Body, maxWAFIPGroupSubscriptionBytes+1)
	if _, err := buffer.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("读取订阅内容失败: %w", err)
	}
	if buffer.Len() > maxWAFIPGroupSubscriptionBytes {
		return nil, fmt.Errorf("订阅内容不能超过 %d 字节", maxWAFIPGroupSubscriptionBytes)
	}
	return buffer.Bytes(), nil
}

func parseWAFIPGroupSubscription(content []byte, format string, mappingRule string) ([]string, error) {
	switch normalizeWAFIPGroupSubscriptionFormat(format) {
	case WAFIPGroupSubscriptionFormatJSON:
		items, err := parseWAFIPGroupJSONSubscription(content, mappingRule)
		if err != nil {
			return nil, err
		}
		return normalizeWAFIPList(items)
	default:
		return normalizeWAFIPList(parseWAFIPGroupTextSubscription(string(content)))
	}
}

func parseWAFIPGroupTextSubscription(text string) []string {
	lines := strings.Split(text, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item == "" || strings.HasPrefix(item, "#") {
			continue
		}
		items = append(items, item)
	}
	return items
}

func parseWAFIPGroupJSONSubscription(content []byte, mappingRule string) ([]string, error) {
	var payload any
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, fmt.Errorf("JSON 订阅解析失败: %w", err)
	}
	nodes, err := selectJSONMappingNodes(payload, mappingRule)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(nodes))
	for _, node := range nodes {
		collectJSONStrings(node, &items)
	}
	if len(items) == 0 {
		return nil, errors.New("JSON 订阅没有解析到 IP/IP 段")
	}
	return items, nil
}

func selectJSONMappingNodes(payload any, mappingRule string) ([]any, error) {
	rule := strings.TrimSpace(mappingRule)
	if rule == "" || rule == "$" {
		return []any{payload}, nil
	}
	rule = strings.TrimPrefix(rule, "$.")
	nodes := []any{payload}
	for _, rawSegment := range strings.Split(rule, ".") {
		segment := strings.TrimSpace(rawSegment)
		if segment == "" {
			continue
		}
		expandArray := strings.HasSuffix(segment, "[]")
		segment = strings.TrimSuffix(segment, "[]")
		next := make([]any, 0)
		for _, node := range nodes {
			object, ok := node.(map[string]any)
			if !ok {
				continue
			}
			value, ok := object[segment]
			if !ok {
				continue
			}
			if expandArray {
				array, ok := value.([]any)
				if !ok {
					continue
				}
				next = append(next, array...)
			} else {
				next = append(next, value)
			}
		}
		nodes = next
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("JSON 映射规则 %q 未匹配到内容", mappingRule)
	}
	return nodes, nil
}

func collectJSONStrings(node any, items *[]string) {
	switch value := node.(type) {
	case string:
		*items = append(*items, value)
	case []any:
		for _, item := range value {
			collectJSONStrings(item, items)
		}
	}
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

func normalizeWAFIPGroupType(value string) string {
	switch strings.TrimSpace(value) {
	case WAFIPGroupTypeManual, "":
		return WAFIPGroupTypeManual
	case WAFIPGroupTypeAutomatic:
		return WAFIPGroupTypeAutomatic
	case WAFIPGroupTypeSubscription:
		return WAFIPGroupTypeSubscription
	default:
		return ""
	}
}

func normalizeWAFIPGroupSubscriptionFormat(value string) string {
	switch strings.TrimSpace(value) {
	case WAFIPGroupSubscriptionFormatJSON:
		return WAFIPGroupSubscriptionFormatJSON
	default:
		return WAFIPGroupSubscriptionFormatText
	}
}

func normalizeWAFIPGroupSyncInterval(value int) int {
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

func nextWAFIPGroupSyncAt(groupType string, enabled bool, interval int, current *time.Time) *time.Time {
	if (groupType != WAFIPGroupTypeSubscription && groupType != WAFIPGroupTypeAutomatic) || !enabled {
		return nil
	}
	if current != nil && current.After(time.Now().UTC()) {
		return current
	}
	next := time.Now().UTC().Add(time.Duration(normalizeWAFIPGroupSyncInterval(interval)) * time.Minute)
	return &next
}

func loadWAFIPGroupReferenceCounts() (map[uint]int, error) {
	var groups []model.WAFRuleGroup
	if err := model.DB.Select("ip_whitelist_groups", "ip_blacklist_groups").Find(&groups).Error; err != nil {
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

func normalizeWAFIPGroupIDs(ids []uint) ([]uint, error) {
	normalized := uniqueUintIDs(ids)
	for _, id := range normalized {
		if _, err := model.GetWAFIPGroupByID(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("IP 组 %d 不存在", id)
			}
			return nil, err
		}
	}
	return normalized, nil
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
