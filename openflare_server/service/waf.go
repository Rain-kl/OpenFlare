package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"openflare/model"
	"openflare/utils"
	"sort"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

const (
	defaultWAFBlockStatusCode = 418
	maxWAFBlockBodyBytes      = 16 * 1024
)

type WAFRuleGroupInput struct {
	Name              string          `json:"name"`
	Enabled           bool            `json:"enabled"`
	BlockStatusCode   int             `json:"block_status_code"`
	BlockResponseBody string          `json:"block_response_body"`
	IPWhitelist       []string        `json:"ip_whitelist"`
	IPBlacklist       []string        `json:"ip_blacklist"`
	CountryWhitelist  []string        `json:"country_whitelist"`
	CountryBlacklist  []string        `json:"country_blacklist"`
	RegionWhitelist   []string        `json:"region_whitelist"`
	RegionBlacklist   []string        `json:"region_blacklist"`
	Remark            string          `json:"remark"`
	PoWEnabled        bool            `json:"pow_enabled"`
	PoWConfig         json.RawMessage `json:"pow_config"`
}

type WAFRuleGroupView struct {
	ID                uint                 `json:"id"`
	Name              string               `json:"name"`
	Enabled           bool                 `json:"enabled"`
	IsGlobal          bool                 `json:"is_global"`
	BlockStatusCode   int                  `json:"block_status_code"`
	BlockResponseBody string               `json:"block_response_body"`
	IPWhitelist       []string             `json:"ip_whitelist"`
	IPBlacklist       []string             `json:"ip_blacklist"`
	CountryWhitelist  []string             `json:"country_whitelist"`
	CountryBlacklist  []string             `json:"country_blacklist"`
	RegionWhitelist   []string             `json:"region_whitelist"`
	RegionBlacklist   []string             `json:"region_blacklist"`
	Remark            string               `json:"remark"`
	PoWEnabled        bool                 `json:"pow_enabled"`
	PoWConfig         *ProxyRoutePoWConfig `json:"pow_config"`
	AppliedSiteIDs    []uint               `json:"applied_site_ids"`
	AppliedSiteCount  int                  `json:"applied_site_count"`
	CreatedAt         string               `json:"created_at"`
	UpdatedAt         string               `json:"updated_at"`
}

type WAFSiteRuleGroupsView struct {
	RouteID           uint               `json:"route_id"`
	GlobalRuleGroup   *WAFRuleGroupView  `json:"global_rule_group"`
	RuleGroups        []WAFRuleGroupView `json:"rule_groups"`
	AppliedRuleGroups []WAFRuleGroupView `json:"applied_rule_groups"`
	AppliedIDs        []uint             `json:"applied_ids"`
}

func ListWAFRuleGroups() ([]WAFRuleGroupView, error) {
	if err := EnsureDefaultWAFRuleGroup(); err != nil {
		return nil, err
	}
	groups, err := model.ListWAFRuleGroups()
	if err != nil {
		return nil, err
	}
	bindings, err := loadWAFBindings()
	if err != nil {
		return nil, err
	}
	views := make([]WAFRuleGroupView, 0, len(groups))
	for _, group := range groups {
		view, err := buildWAFRuleGroupView(group, bindings[group.ID])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func GetWAFRuleGroup(id uint) (*WAFRuleGroupView, error) {
	group, err := model.GetWAFRuleGroupByID(id)
	if err != nil {
		return nil, err
	}
	bindings, err := loadWAFBindings()
	if err != nil {
		return nil, err
	}
	view, err := buildWAFRuleGroupView(group, bindings[group.ID])
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func CreateWAFRuleGroup(input WAFRuleGroupInput) (*WAFRuleGroupView, error) {
	group, err := buildWAFRuleGroup(nil, input)
	if err != nil {
		return nil, err
	}
	group.IsGlobal = false
	if err := group.Insert(); err != nil {
		return nil, err
	}
	return GetWAFRuleGroup(group.ID)
}

func UpdateWAFRuleGroup(id uint, input WAFRuleGroupInput) (*WAFRuleGroupView, error) {
	group, err := model.GetWAFRuleGroupByID(id)
	if err != nil {
		return nil, err
	}
	isGlobal := group.IsGlobal
	group, err = buildWAFRuleGroup(group, input)
	if err != nil {
		return nil, err
	}
	group.IsGlobal = isGlobal
	if isGlobal && strings.TrimSpace(group.Name) == "" {
		group.Name = "全局规则组"
	}
	if err := group.Update(); err != nil {
		return nil, err
	}
	return GetWAFRuleGroup(group.ID)
}

func DeleteWAFRuleGroup(id uint) error {
	group, err := model.GetWAFRuleGroupByID(id)
	if err != nil {
		return err
	}
	if group.IsGlobal {
		return errors.New("全局 WAF 规则组不能删除")
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_group_id = ?", group.ID).Delete(&model.WAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(group).Error
	})
}

func ReplaceWAFRuleGroupSites(groupID uint, routeIDs []uint) (*WAFRuleGroupView, error) {
	group, err := model.GetWAFRuleGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group.IsGlobal {
		return nil, errors.New("全局 WAF 规则组默认应用到所有网站，不能手动绑定")
	}
	normalized, err := normalizeWAFRouteIDs(routeIDs)
	if err != nil {
		return nil, err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_group_id = ?", groupID).Delete(&model.WAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		for _, routeID := range normalized {
			binding := model.WAFRuleGroupBinding{RuleGroupID: groupID, ProxyRouteID: routeID}
			if err := tx.Create(&binding).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetWAFRuleGroup(groupID)
}

func GetWAFSiteRuleGroups(routeID uint) (*WAFSiteRuleGroupsView, error) {
	if _, err := model.GetProxyRouteByID(routeID); err != nil {
		return nil, err
	}
	groups, err := ListWAFRuleGroups()
	if err != nil {
		return nil, err
	}
	appliedIDs, err := ListWAFSiteRuleGroupIDs(routeID)
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[uint]struct{}, len(appliedIDs))
	for _, id := range appliedIDs {
		appliedSet[id] = struct{}{}
	}
	var global *WAFRuleGroupView
	custom := make([]WAFRuleGroupView, 0, len(groups))
	applied := make([]WAFRuleGroupView, 0, len(appliedIDs))
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
	return &WAFSiteRuleGroupsView{
		RouteID:           routeID,
		GlobalRuleGroup:   global,
		RuleGroups:        custom,
		AppliedRuleGroups: applied,
		AppliedIDs:        appliedIDs,
	}, nil
}

func ReplaceWAFSiteRuleGroups(routeID uint, groupIDs []uint) (*WAFSiteRuleGroupsView, error) {
	if _, err := model.GetProxyRouteByID(routeID); err != nil {
		return nil, err
	}
	normalized, err := normalizeWAFRuleGroupIDs(groupIDs)
	if err != nil {
		return nil, err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("proxy_route_id = ?", routeID).Delete(&model.WAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		for _, groupID := range normalized {
			binding := model.WAFRuleGroupBinding{RuleGroupID: groupID, ProxyRouteID: routeID}
			if err := tx.Create(&binding).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetWAFSiteRuleGroups(routeID)
}

func ListWAFSiteRuleGroupIDs(routeID uint) ([]uint, error) {
	var bindings []model.WAFRuleGroupBinding
	if err := model.DB.Where("proxy_route_id = ?", routeID).Order("rule_group_id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.RuleGroupID)
	}
	return ids, nil
}

func EnsureDefaultWAFRuleGroup() error {
	_, err := model.GetGlobalWAFRuleGroup()
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	group := &model.WAFRuleGroup{
		Name:              "全局规则组",
		Enabled:           true,
		IsGlobal:          true,
		BlockStatusCode:   defaultWAFBlockStatusCode,
		IPWhitelist:       "[]",
		IPBlacklist:       "[]",
		CountryWhitelist:  "[]",
		CountryBlacklist:  "[]",
		RegionWhitelist:   "[]",
		RegionBlacklist:   "[]",
		PoWEnabled:        false,
		PoWConfig:         "{}",
		BlockResponseBody: "",
	}
	return group.Insert()
}

func buildWAFRuleGroup(group *model.WAFRuleGroup, input WAFRuleGroupInput) (*model.WAFRuleGroup, error) {
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
	ipWhitelist, err := normalizeWAFIPList(input.IPWhitelist)
	if err != nil {
		return nil, fmt.Errorf("IP 白名单无效: %w", err)
	}
	ipBlacklist, err := normalizeWAFIPList(input.IPBlacklist)
	if err != nil {
		return nil, fmt.Errorf("IP 黑名单无效: %w", err)
	}
	countryWhitelist, err := normalizeWAFCountryList(input.CountryWhitelist)
	if err != nil {
		return nil, fmt.Errorf("地域白名单无效: %w", err)
	}
	countryBlacklist, err := normalizeWAFCountryList(input.CountryBlacklist)
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
	countryWhitelistJSON, _ := json.Marshal(countryWhitelist)
	countryBlacklistJSON, _ := json.Marshal(countryBlacklist)
	regionWhitelistJSON, _ := json.Marshal(regionWhitelist)
	regionBlacklistJSON, _ := json.Marshal(regionBlacklist)

	if group == nil {
		group = &model.WAFRuleGroup{}
	}
	group.Name = name
	group.Enabled = input.Enabled
	group.BlockStatusCode = statusCode
	group.BlockResponseBody = input.BlockResponseBody
	group.IPWhitelist = string(ipWhitelistJSON)
	group.IPBlacklist = string(ipBlacklistJSON)
	group.CountryWhitelist = string(countryWhitelistJSON)
	group.CountryBlacklist = string(countryBlacklistJSON)
	group.RegionWhitelist = string(regionWhitelistJSON)
	group.RegionBlacklist = string(regionBlacklistJSON)
	group.PoWEnabled = input.PoWEnabled
	group.PoWConfig = string(powConfigJSON)
	group.Remark = strings.TrimSpace(input.Remark)
	return group, nil
}

func buildWAFRuleGroupView(group *model.WAFRuleGroup, appliedSiteIDs []uint) (WAFRuleGroupView, error) {
	if group == nil {
		return WAFRuleGroupView{}, errors.New("waf rule group is nil")
	}
	sort.Slice(appliedSiteIDs, func(i, j int) bool { return appliedSiteIDs[i] < appliedSiteIDs[j] })
	view := WAFRuleGroupView{
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

func loadWAFBindings() (map[uint][]uint, error) {
	var bindings []model.WAFRuleGroupBinding
	if err := model.DB.Order("rule_group_id asc").Order("proxy_route_id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	result := make(map[uint][]uint, len(bindings))
	for _, binding := range bindings {
		result[binding.RuleGroupID] = append(result[binding.RuleGroupID], binding.ProxyRouteID)
	}
	return result, nil
}

func normalizeWAFIPList(items []string) ([]string, error) {
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
	normalized = utils.Unique(normalized)
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeWAFCountryList(items []string) ([]string, error) {
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
	normalized = utils.Unique(normalized)
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
	normalized = utils.Unique(normalized)
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

func normalizeWAFRouteIDs(routeIDs []uint) ([]uint, error) {
	normalized := uniqueUintIDs(routeIDs)
	for _, routeID := range normalized {
		if _, err := model.GetProxyRouteByID(routeID); err != nil {
			return nil, fmt.Errorf("网站 %d 不存在", routeID)
		}
	}
	return normalized, nil
}

func normalizeWAFRuleGroupIDs(groupIDs []uint) ([]uint, error) {
	normalized := uniqueUintIDs(groupIDs)
	for _, groupID := range normalized {
		group, err := model.GetWAFRuleGroupByID(groupID)
		if err != nil {
			return nil, fmt.Errorf("WAF 规则组 %d 不存在", groupID)
		}
		if group.IsGlobal {
			return nil, errors.New("全局 WAF 规则组不需要手动绑定")
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
	normalized = utils.Unique(normalized)
	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })
	return normalized
}
