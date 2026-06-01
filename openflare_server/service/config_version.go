package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"openflare/common"
	"openflare/model"
	openrestyrender "openflare/utils/render/openresty"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ReleaseResult struct {
	Version *model.ConfigVersion `json:"version"`
	Routes  []*model.ProxyRoute  `json:"routes"`
}

type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ConfigPreviewResult struct {
	SnapshotJSON   string        `json:"snapshot_json"`
	MainConfig     string        `json:"main_config"`
	RouteConfig    string        `json:"route_config"`
	RenderedConfig string        `json:"rendered_config"`
	SupportFiles   []SupportFile `json:"support_files"`
	Checksum       string        `json:"checksum"`
	RouteCount     int           `json:"route_count"`
	WebsiteCount   int           `json:"website_count"`
}

type ConfigVersionSummary = model.ConfigVersionSummary

type ConfigVersionDetail = model.ConfigVersion

type ConfigDiffResult struct {
	ActiveVersion        string                 `json:"active_version,omitempty"`
	AddedSites           []string               `json:"added_sites"`
	RemovedSites         []string               `json:"removed_sites"`
	ModifiedSites        []string               `json:"modified_sites"`
	AddedDomains         []string               `json:"added_domains"`
	RemovedDomains       []string               `json:"removed_domains"`
	ModifiedDomains      []string               `json:"modified_domains"`
	MainConfigChanged    bool                   `json:"main_config_changed"`
	WAFConfigChanged     bool                   `json:"waf_config_changed"`
	ChangedOptionKeys    []string               `json:"changed_option_keys"`
	ChangedOptionDetails []ConfigOptionDiffItem `json:"changed_option_details"`
	CurrentWebsiteCount  int                    `json:"current_website_count"`
	ActiveWebsiteCount   int                    `json:"active_website_count"`
}

type ConfigOptionDiffItem struct {
	Key           string `json:"key"`
	PreviousValue string `json:"previous_value"`
	CurrentValue  string `json:"current_value"`
}

type snapshotRoute struct {
	ID                 uint                          `json:"id,omitempty"`
	SiteName           string                        `json:"site_name,omitempty"`
	Domain             string                        `json:"domain"`
	Domains            []string                      `json:"domains,omitempty"`
	OriginURL          string                        `json:"origin_url"`
	OriginHost         string                        `json:"origin_host,omitempty"`
	Upstreams          []string                      `json:"upstreams,omitempty"`
	Enabled            bool                          `json:"enabled"`
	EnableHTTPS        bool                          `json:"enable_https"`
	CertID             *uint                         `json:"cert_id,omitempty"`
	CertIDs            []uint                        `json:"cert_ids,omitempty"`
	DomainCertIDs      []uint                        `json:"domain_cert_ids,omitempty"`
	RedirectHTTP       bool                          `json:"redirect_http"`
	LimitConnPerServer int                           `json:"limit_conn_per_server,omitempty"`
	LimitConnPerIP     int                           `json:"limit_conn_per_ip,omitempty"`
	LimitRate          string                        `json:"limit_rate,omitempty"`
	CacheEnabled       bool                          `json:"cache_enabled"`
	CachePolicy        string                        `json:"cache_policy,omitempty"`
	CacheRules         []string                      `json:"cache_rules,omitempty"`
	CustomHeaders      []ProxyRouteCustomHeaderInput `json:"custom_headers,omitempty"`
	PoWEnabled         bool                          `json:"pow_enabled,omitempty"`
	PoWConfig          *ProxyRoutePoWConfig          `json:"pow_config,omitempty"`
	BasicAuthEnabled   bool                          `json:"basic_auth_enabled,omitempty"`
	BasicAuthUsername  string                        `json:"basic_auth_username,omitempty"`
	BasicAuthPassword  string                        `json:"basic_auth_password,omitempty"`
	Remark             string                        `json:"remark,omitempty"`
	UpstreamType       string                        `json:"upstream_type,omitempty"`
	TunnelNodeID       *uint                         `json:"tunnel_node_id,omitempty"`
	TunnelTargetAddr   string                        `json:"tunnel_target_addr,omitempty"`
	TunnelTargetProto  string                        `json:"tunnel_target_protocol,omitempty"`
}

type snapshotWAFRuleGroup struct {
	ID                uint                 `json:"id"`
	Name              string               `json:"name"`
	Enabled           bool                 `json:"enabled"`
	IsGlobal          bool                 `json:"is_global"`
	BlockStatusCode   int                  `json:"block_status_code"`
	BlockResponseBody string               `json:"block_response_body,omitempty"`
	IPWhitelist       []string             `json:"ip_whitelist,omitempty"`
	IPBlacklist       []string             `json:"ip_blacklist,omitempty"`
	IPWhitelistGroups []uint               `json:"ip_whitelist_group_ids,omitempty"`
	IPBlacklistGroups []uint               `json:"ip_blacklist_group_ids,omitempty"`
	CountryWhitelist  []string             `json:"country_whitelist,omitempty"`
	CountryBlacklist  []string             `json:"country_blacklist,omitempty"`
	RegionWhitelist   []string             `json:"region_whitelist,omitempty"`
	RegionBlacklist   []string             `json:"region_blacklist,omitempty"`
	PoWEnabled        bool                 `json:"pow_enabled,omitempty"`
	PoWConfig         *ProxyRoutePoWConfig `json:"pow_config,omitempty"`
}

type snapshotWAFIPGroup struct {
	ID      uint     `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Enabled bool     `json:"enabled"`
	IPList  []string `json:"ip_list,omitempty"`
}

type snapshotWAFBinding struct {
	RouteID      uint   `json:"route_id"`
	SiteName     string `json:"site_name"`
	RuleGroupIDs []uint `json:"rule_group_ids"`
}

type snapshotWAFDocument struct {
	RuleGroups []snapshotWAFRuleGroup `json:"rule_groups"`
	IPGroups   []snapshotWAFIPGroup   `json:"ip_groups,omitempty"`
	Bindings   []snapshotWAFBinding   `json:"bindings"`
}

type openRestyConfigSnapshot struct {
	WorkerProcesses          string `json:"worker_processes"`
	WorkerConnections        int    `json:"worker_connections"`
	WorkerRlimitNofile       int    `json:"worker_rlimit_nofile"`
	EventsUse                string `json:"events_use,omitempty"`
	EventsMultiAcceptEnabled bool   `json:"events_multi_accept_enabled"`
	KeepaliveTimeout         int    `json:"keepalive_timeout"`
	KeepaliveRequests        int    `json:"keepalive_requests"`
	ClientHeaderTimeout      int    `json:"client_header_timeout"`
	ClientBodyTimeout        int    `json:"client_body_timeout"`
	ClientMaxBodySize        string `json:"client_max_body_size"`
	LargeClientHeaderBuffers string `json:"large_client_header_buffers"`
	SendTimeout              int    `json:"send_timeout"`
	ProxyConnectTimeout      int    `json:"proxy_connect_timeout"`
	ProxySendTimeout         int    `json:"proxy_send_timeout"`
	ProxyReadTimeout         int    `json:"proxy_read_timeout"`
	WebsocketEnabled         bool   `json:"websocket_enabled"`
	ProxyRequestBuffering    bool   `json:"proxy_request_buffering"`
	ProxyBufferingEnabled    bool   `json:"proxy_buffering_enabled"`
	ProxyBuffers             string `json:"proxy_buffers"`
	ProxyBufferSize          string `json:"proxy_buffer_size"`
	ProxyBusyBuffersSize     string `json:"proxy_busy_buffers_size"`
	GzipEnabled              bool   `json:"gzip_enabled"`
	GzipMinLength            int    `json:"gzip_min_length"`
	GzipCompLevel            int    `json:"gzip_comp_level"`
	Resolvers                string `json:"resolvers,omitempty"`
	CacheEnabled             bool   `json:"cache_enabled"`
	CachePath                string `json:"cache_path,omitempty"`
	CacheLevels              string `json:"cache_levels"`
	CacheInactive            string `json:"cache_inactive"`
	CacheMaxSize             string `json:"cache_max_size"`
	CacheKeyTemplate         string `json:"cache_key_template"`
	CacheLockEnabled         bool   `json:"cache_lock_enabled"`
	CacheLockTimeout         string `json:"cache_lock_timeout"`
	CacheUseStale            string `json:"cache_use_stale"`
	MainConfigTemplate       string `json:"main_config_template,omitempty"`
}

type snapshotDocument struct {
	Routes          []snapshotRoute         `json:"routes"`
	OpenRestyConfig openRestyConfigSnapshot `json:"openresty_config"`
	WAF             snapshotWAFDocument     `json:"waf"`
}

type configBundle struct {
	Routes            []*model.ProxyRoute
	SnapshotRoutes    []snapshotRoute
	WAFSnapshot       snapshotWAFDocument
	OpenRestyConfig   openRestyConfigSnapshot
	SnapshotJSON      string
	MainConfig        string
	RouteConfig       string
	SupportFiles      []SupportFile
	Checksum          string
	ChangedOptionKeys []string
}

func ListConfigVersions() ([]*ConfigVersionSummary, error) {
	return model.ListConfigVersionSummaries()
}

func GetConfigVersionDetail(id uint) (*ConfigVersionDetail, error) {
	return model.GetConfigVersionByID(id)
}

func GetActiveConfigVersion() (*ConfigVersionDetail, error) {
	return model.GetActiveConfigVersion()
}

func PreviewConfigVersion() (*ConfigPreviewResult, error) {
	bundle, err := buildCurrentConfigBundle(false)
	if err != nil {
		return nil, err
	}
	return &ConfigPreviewResult{
		SnapshotJSON:   bundle.SnapshotJSON,
		MainConfig:     bundle.MainConfig,
		RouteConfig:    bundle.RouteConfig,
		RenderedConfig: bundle.RouteConfig,
		SupportFiles:   bundle.SupportFiles,
		Checksum:       bundle.Checksum,
		RouteCount:     len(bundle.Routes),
		WebsiteCount:   len(bundle.SnapshotRoutes),
	}, nil
}

func DiffConfigVersion() (*ConfigDiffResult, error) {
	bundle, err := buildCurrentConfigBundle(false)
	if err != nil {
		return nil, err
	}
	result := &ConfigDiffResult{
		AddedSites:           []string{},
		RemovedSites:         []string{},
		ModifiedSites:        []string{},
		AddedDomains:         []string{},
		RemovedDomains:       []string{},
		ModifiedDomains:      []string{},
		ChangedOptionKeys:    []string{},
		ChangedOptionDetails: []ConfigOptionDiffItem{},
		CurrentWebsiteCount:  len(bundle.SnapshotRoutes),
	}
	activeVersion, err := model.GetActiveConfigVersion()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			for _, route := range bundle.SnapshotRoutes {
				result.AddedSites = append(result.AddedSites, route.SiteName)
				result.AddedDomains = append(result.AddedDomains, route.Domains...)
			}
			result.MainConfigChanged = true
			result.ChangedOptionKeys = openRestyOptionKeys()
			result.ChangedOptionDetails = buildInitialOpenRestyOptionDiffs(bundle.OpenRestyConfig)
			sort.Strings(result.AddedSites)
			sort.Strings(result.AddedDomains)
			sort.Strings(result.ChangedOptionKeys)
			return result, nil
		}
		return nil, err
	}
	result.ActiveVersion = activeVersion.Version
	activeSnapshot, err := parseSnapshotDocument(activeVersion.SnapshotJSON)
	if err != nil {
		return nil, err
	}
	result.ActiveWebsiteCount = len(activeSnapshot.Routes)
	currentSiteMap := flattenSnapshotRoutesBySite(bundle.SnapshotRoutes)
	activeSiteMap := flattenSnapshotRoutesBySite(activeSnapshot.Routes)
	for siteName, currentRoute := range currentSiteMap {
		activeRoute, ok := activeSiteMap[siteName]
		if !ok {
			result.AddedSites = append(result.AddedSites, siteName)
			continue
		}
		if !snapshotRouteConfigEqual(activeRoute, currentRoute) {
			result.ModifiedSites = append(result.ModifiedSites, siteName)
		}
	}
	for siteName := range activeSiteMap {
		if _, ok := currentSiteMap[siteName]; !ok {
			result.RemovedSites = append(result.RemovedSites, siteName)
		}
	}
	currentMap := flattenSnapshotRoutesByDomain(bundle.SnapshotRoutes)
	activeMap := flattenSnapshotRoutesByDomain(activeSnapshot.Routes)
	for domain, currentRoute := range currentMap {
		activeRoute, ok := activeMap[domain]
		if !ok {
			result.AddedDomains = append(result.AddedDomains, domain)
			continue
		}
		if !snapshotRouteConfigEqual(activeRoute, currentRoute) {
			result.ModifiedDomains = append(result.ModifiedDomains, domain)
		}
	}
	for domain := range activeMap {
		if _, ok := currentMap[domain]; !ok {
			result.RemovedDomains = append(result.RemovedDomains, domain)
		}
	}
	result.MainConfigChanged = activeVersion.MainConfig != bundle.MainConfig
	result.WAFConfigChanged = !snapshotWAFConfigEqual(activeSnapshot.WAF, bundle.WAFSnapshot)
	result.ChangedOptionDetails = diffOpenRestyOptionDetails(activeSnapshot.OpenRestyConfig, bundle.OpenRestyConfig)
	result.ChangedOptionKeys = extractOptionDiffKeys(result.ChangedOptionDetails)
	sort.Strings(result.AddedSites)
	sort.Strings(result.RemovedSites)
	sort.Strings(result.ModifiedSites)
	sort.Strings(result.AddedDomains)
	sort.Strings(result.RemovedDomains)
	sort.Strings(result.ModifiedDomains)
	sort.Strings(result.ChangedOptionKeys)
	return result, nil
}

func PublishConfigVersion(createdBy string, force bool) (*ReleaseResult, error) {
	bundle, err := buildCurrentConfigBundle(true)
	if err != nil {
		return nil, err
	}
	if len(bundle.Routes) == 0 {
		return nil, errors.New("没有可发布的启用规则")
	}
	activeVersion, err := model.GetActiveConfigVersion()
	if !force && err == nil && activeVersion.Checksum == bundle.Checksum {
		return nil, errors.New("当前规则没有变更，不能重复发布")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	supportFilesJSON, err := json.Marshal(bundle.SupportFiles)
	if err != nil {
		return nil, err
	}
	version, err := nextVersionNumber(time.Now())
	if err != nil {
		return nil, err
	}
	record := &model.ConfigVersion{
		Version:          version,
		SnapshotJSON:     bundle.SnapshotJSON,
		MainConfig:       bundle.MainConfig,
		RenderedConfig:   bundle.RouteConfig,
		SupportFilesJSON: string(supportFilesJSON),
		Checksum:         bundle.Checksum,
		IsActive:         true,
		CreatedBy:        createdBy,
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if model.IsUniqueConstraintError(err) {
			return nil, errors.New("版本号生成冲突，请重试")
		}
		return nil, err
	}
	activeConfig := &ActiveConfigMeta{
		Version:  record.Version,
		Checksum: record.Checksum,
	}
	BroadcastAgentWSActiveConfig(activeConfig)
	BroadcastFlaredWSActiveConfig(activeConfig)
	return &ReleaseResult{
		Version: record,
		Routes:  bundle.Routes,
	}, nil
}

func sourceSupportFiles(files []SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]SupportFile, 0, len(files))
	for _, file := range files {
		if isRuntimeGeneratedSupportFile(file.Path) {
			continue
		}
		result = append(result, file)
	}
	return result
}

func isRuntimeGeneratedSupportFile(path string) bool {
	switch strings.TrimSpace(path) {
	case "pow_config.json", "waf_config.json", openrestyrender.SourceConfigFileName:
		return true
	default:
		return false
	}
}

func ActivateConfigVersion(id uint) (*model.ConfigVersion, error) {
	version, err := model.GetConfigVersionByID(id)
	if err != nil {
		return nil, err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		if err := tx.Model(version).Update("is_active", true).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	version.IsActive = true
	activeConfig := &ActiveConfigMeta{
		Version:  version.Version,
		Checksum: version.Checksum,
	}
	BroadcastAgentWSActiveConfig(activeConfig)
	BroadcastFlaredWSActiveConfig(activeConfig)
	return version, nil
}

func CleanupConfigVersions(keepCount int) (int64, error) {
	if keepCount < 3 {
		keepCount = 3
	}
	var versions []model.ConfigVersion
	if err := model.DB.Select("id", "is_active").Order("id desc").Find(&versions).Error; err != nil {
		return 0, err
	}
	if len(versions) <= keepCount {
		return 0, nil
	}
	var deleteIDs []uint
	for i, v := range versions {
		if i < keepCount {
			continue
		}
		if v.IsActive {
			continue
		}
		deleteIDs = append(deleteIDs, v.ID)
	}
	if len(deleteIDs) == 0 {
		return 0, nil
	}
	result := model.DB.Where("id IN ?", deleteIDs).Delete(&model.ConfigVersion{})
	return result.RowsAffected, result.Error
}

func buildCurrentConfigBundle(requireRoutes bool) (*configBundle, error) {
	routes, err := model.GetEnabledProxyRoutes()
	if err != nil {
		return nil, err
	}
	if requireRoutes && len(routes) == 0 {
		return nil, errors.New("没有可发布的启用规则")
	}
	snapshotRoutes, err := buildSnapshotRoutes(routes)
	if err != nil {
		return nil, err
	}
	wafSnapshot, err := buildSnapshotWAFDocument(routes)
	if err != nil {
		return nil, err
	}
	openRestyConfig := buildOpenRestyConfigSnapshot()
	snapshotDoc := snapshotDocument{
		Routes:          snapshotRoutes,
		OpenRestyConfig: openRestyConfig,
		WAF:             wafSnapshot,
	}
	snapshotJSON, err := json.Marshal(snapshotDoc)
	if err != nil {
		return nil, err
	}
	certificateFiles, err := buildCertificateSupportFiles(snapshotRoutes)
	if err != nil {
		return nil, err
	}
	rendered, err := renderSnapshotConfig(string(snapshotJSON), certificateFiles)
	if err != nil {
		return nil, err
	}
	return &configBundle{
		Routes:            routes,
		SnapshotRoutes:    snapshotRoutes,
		WAFSnapshot:       wafSnapshot,
		OpenRestyConfig:   openRestyConfig,
		SnapshotJSON:      string(snapshotJSON),
		MainConfig:        rendered.MainConfig,
		RouteConfig:       rendered.RouteConfig,
		SupportFiles:      fromOpenRestySupportFiles(rendered.SupportFiles),
		Checksum:          rendered.Checksum,
		ChangedOptionKeys: openRestyOptionKeys(),
	}, nil
}

func buildSnapshotRoutes(routes []*model.ProxyRoute) ([]snapshotRoute, error) {
	items := make([]snapshotRoute, 0, len(routes))
	for _, route := range routes {
		domains, err := decodeStoredDomains(route.Domains, route.Domain)
		if err != nil {
			return nil, fmt.Errorf("route %s domains are invalid", route.Domain)
		}
		customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 自定义请求头无效", route.Domain)
		}
		upstreamType := normalizeUpstreamType(route.UpstreamType)
		originURL := route.OriginURL
		upstreams, err := decodeStoredUpstreams(route.Upstreams, route.OriginURL)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 上游配置无效", route.Domain)
		}
		var tunnelNodeID *uint
		var tunnelTargetAddr string
		var tunnelTargetProtocol string
		if upstreamType == "tunnel" {
			originURL = resolveTunnelOpenRestyUpstreamURL()
			upstreams = []string{originURL}
			tunnelNodeID = route.TunnelNodeID
			tunnelTargetAddr = strings.TrimSpace(route.TunnelTargetAddr)
			tunnelTargetProtocol = normalizeTunnelTargetProtocol(route.TunnelTargetProtocol)
		}
		cacheRules, err := decodeStoredCacheRules(route.CacheRules)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 缓存规则无效", route.Domain)
		}
		powConfig, err := decodeStoredPoWConfig(route.PoWEnabled, route.PoWConfig)
		if err != nil {
			return nil, fmt.Errorf("路由 %s PoW 配置无效", route.Domain)
		}
		if !route.PoWEnabled {
			powConfig = nil
		}
		items = append(items, snapshotRoute{
			ID:                 route.ID,
			SiteName:           normalizeProxyRouteSiteNameInput(route, route.SiteName, domains[0]),
			Domain:             domains[0],
			Domains:            domains,
			OriginURL:          originURL,
			OriginHost:         route.OriginHost,
			Upstreams:          upstreams,
			Enabled:            route.Enabled,
			EnableHTTPS:        route.EnableHTTPS,
			CertID:             route.CertID,
			CertIDs:            mustDecodeSnapshotCertIDs(route),
			DomainCertIDs:      mustDecodeSnapshotDomainCertIDs(route, domains),
			RedirectHTTP:       route.RedirectHTTP,
			LimitConnPerServer: route.LimitConnPerServer,
			LimitConnPerIP:     route.LimitConnPerIP,
			LimitRate:          route.LimitRate,
			CacheEnabled:       route.CacheEnabled,
			CachePolicy:        route.CachePolicy,
			CacheRules:         cacheRules,
			CustomHeaders:      customHeaders,
			PoWEnabled:         route.PoWEnabled,
			PoWConfig:          powConfig,
			BasicAuthEnabled:   route.BasicAuthEnabled,
			BasicAuthUsername:  route.BasicAuthUsername,
			BasicAuthPassword:  route.BasicAuthPassword,
			Remark:             route.Remark,
			UpstreamType:       upstreamType,
			TunnelNodeID:       tunnelNodeID,
			TunnelTargetAddr:   tunnelTargetAddr,
			TunnelTargetProto:  tunnelTargetProtocol,
		})
	}
	return items, nil
}

func resolveTunnelOpenRestyUpstreamURL() string {
	relayNodes, err := model.ListNodesByType("tunnel_relay")
	if err == nil && len(relayNodes) > 0 {
		for _, node := range relayNodes {
			if node != nil {
				addr := relayAgentAddress(node)
				if addr != "" {
					return "http://" + addr
				}
			}
		}
	}
	return "http://127.0.0.1:8080"
}

func buildSnapshotWAFDocument(routes []*model.ProxyRoute) (snapshotWAFDocument, error) {
	if err := EnsureDefaultWAFRuleGroup(); err != nil {
		return snapshotWAFDocument{}, err
	}
	views, err := ListWAFRuleGroups()
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	ruleGroups := make([]snapshotWAFRuleGroup, 0, len(views))
	for _, view := range views {
		if !view.Enabled {
			continue
		}
		ruleGroups = append(ruleGroups, snapshotWAFRuleGroup{
			ID:                view.ID,
			Name:              view.Name,
			Enabled:           view.Enabled,
			IsGlobal:          view.IsGlobal,
			BlockStatusCode:   view.BlockStatusCode,
			BlockResponseBody: view.BlockResponseBody,
			IPWhitelist:       view.IPWhitelist,
			IPBlacklist:       view.IPBlacklist,
			IPWhitelistGroups: view.IPWhitelistGroups,
			IPBlacklistGroups: view.IPBlacklistGroups,
			CountryWhitelist:  view.CountryWhitelist,
			CountryBlacklist:  view.CountryBlacklist,
			RegionWhitelist:   view.RegionWhitelist,
			RegionBlacklist:   view.RegionBlacklist,
			PoWEnabled:        view.PoWEnabled,
			PoWConfig:         view.PoWConfig,
		})
	}
	ipGroups, err := buildSnapshotWAFIPGroups(ruleGroups)
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	enabledRouteIDs := make(map[uint]string, len(routes))
	for _, route := range routes {
		if route == nil {
			continue
		}
		siteName := strings.TrimSpace(route.SiteName)
		if siteName == "" {
			siteName = route.Domain
		}
		enabledRouteIDs[route.ID] = siteName
	}
	var rawBindings []model.WAFRuleGroupBinding
	if err := model.DB.Order("proxy_route_id asc").Order("rule_group_id asc").Find(&rawBindings).Error; err != nil {
		return snapshotWAFDocument{}, err
	}
	groupIDsByRoute := make(map[uint][]uint, len(rawBindings))
	for _, binding := range rawBindings {
		if _, ok := enabledRouteIDs[binding.ProxyRouteID]; !ok {
			continue
		}
		groupIDsByRoute[binding.ProxyRouteID] = append(groupIDsByRoute[binding.ProxyRouteID], binding.RuleGroupID)
	}
	bindings := make([]snapshotWAFBinding, 0, len(groupIDsByRoute))
	for routeID, groupIDs := range groupIDsByRoute {
		sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
		bindings = append(bindings, snapshotWAFBinding{
			RouteID:      routeID,
			SiteName:     enabledRouteIDs[routeID],
			RuleGroupIDs: groupIDs,
		})
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].SiteName == bindings[j].SiteName {
			return bindings[i].RouteID < bindings[j].RouteID
		}
		return bindings[i].SiteName < bindings[j].SiteName
	})
	return snapshotWAFDocument{RuleGroups: ruleGroups, IPGroups: ipGroups, Bindings: bindings}, nil
}

func buildSnapshotWAFIPGroups(ruleGroups []snapshotWAFRuleGroup) ([]snapshotWAFIPGroup, error) {
	idSet := make(map[uint]struct{})
	for _, group := range ruleGroups {
		for _, id := range group.IPWhitelistGroups {
			idSet[id] = struct{}{}
		}
		for _, id := range group.IPBlacklistGroups {
			idSet[id] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return []snapshotWAFIPGroup{}, nil
	}
	ids := make([]uint, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
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
	snapshots := make([]snapshotWAFIPGroup, 0, len(ids))
	for _, id := range ids {
		group := groupByID[id]
		if group == nil {
			return nil, fmt.Errorf("IP 组 %d 不存在", id)
		}
		snapshots = append(snapshots, snapshotWAFIPGroup{
			ID:      group.ID,
			Name:    group.Name,
			Type:    group.Type,
			Enabled: group.Enabled,
		})
	}
	return snapshots, nil
}

func mustDecodeSnapshotCertIDs(route *model.ProxyRoute) []uint {
	if route == nil {
		return []uint{}
	}
	certIDs, err := decodeStoredCertIDs(route.CertIDs, route.CertID)
	if err != nil {
		return []uint{}
	}
	return certIDs
}

func mustDecodeSnapshotDomainCertIDs(
	route *model.ProxyRoute,
	domains []string,
) []uint {
	if route == nil {
		return []uint{}
	}
	certIDs, err := decodeStoredCertIDs(route.CertIDs, route.CertID)
	if err != nil {
		return []uint{}
	}
	domainCertIDs, err := resolveProxyRouteDomainCertIDs(route, domains, certIDs)
	if err != nil {
		return []uint{}
	}
	return domainCertIDs
}

func parseSnapshotDocument(snapshotJSON string) (*snapshotDocument, error) {
	text := strings.TrimSpace(snapshotJSON)
	if text == "" {
		return &snapshotDocument{Routes: []snapshotRoute{}}, nil
	}
	if strings.HasPrefix(text, "[") {
		var routes []snapshotRoute
		if err := json.Unmarshal([]byte(text), &routes); err != nil {
			return nil, errors.New("历史版本快照格式不合法")
		}
		return &snapshotDocument{Routes: normalizeSnapshotRoutes(routes)}, nil
	}
	var snapshot snapshotDocument
	if err := json.Unmarshal([]byte(text), &snapshot); err != nil {
		return nil, errors.New("历史版本快照格式不合法")
	}
	snapshot.Routes = normalizeSnapshotRoutes(snapshot.Routes)
	return &snapshot, nil
}

func normalizeSnapshotRoutes(routes []snapshotRoute) []snapshotRoute {
	if len(routes) == 0 {
		return []snapshotRoute{}
	}
	for index := range routes {
		normalizedDomains, err := decodeStoredDomains("", routes[index].Domain)
		if len(routes[index].Domains) > 0 {
			normalizedDomains, err = normalizeProxyRouteDomains(routes[index].Domains)
		}
		if err == nil && len(normalizedDomains) > 0 {
			routes[index].Domains = normalizedDomains
			routes[index].Domain = normalizedDomains[0]
			routes[index].SiteName = normalizeProxyRouteSiteNameInput(
				&model.ProxyRoute{SiteName: routes[index].SiteName},
				routes[index].SiteName,
				normalizedDomains[0],
			)
		}
		normalizedHeaders, err := normalizeCustomHeaders(routes[index].CustomHeaders)
		if err == nil {
			routes[index].CustomHeaders = normalizedHeaders
		}
		normalizedCertIDs, primaryCertID, err := normalizeSnapshotCertificateIDs(routes[index].CertID, routes[index].CertIDs)
		if err == nil {
			routes[index].CertID = primaryCertID
			routes[index].CertIDs = normalizedCertIDs
		}
		normalizedDomainCertIDs, err := normalizeSnapshotDomainCertificateIDs(
			routes[index].Domains,
			routes[index].CertIDs,
			routes[index].DomainCertIDs,
		)
		if err == nil {
			routes[index].DomainCertIDs = normalizedDomainCertIDs
		}
		normalizedUpstreams, err := normalizeUpstreams(routes[index].OriginURL, routes[index].Upstreams)
		if err == nil {
			routes[index].OriginURL = normalizedUpstreams[0]
			routes[index].Upstreams = normalizedUpstreams
		}
		normalizedCacheRules, err := normalizeCacheRules(routes[index].CacheEnabled, routes[index].CachePolicy, routes[index].CacheRules)
		if err == nil {
			routes[index].CachePolicy = normalizeCachePolicy(routes[index].CacheEnabled, routes[index].CachePolicy)
			routes[index].CacheRules = normalizedCacheRules
		}
		normalizedLimitRate, err := normalizeProxyRouteLimitRate(routes[index].LimitRate)
		if err == nil {
			routes[index].LimitRate = normalizedLimitRate
		}
		if routes[index].PoWEnabled {
			raw, err := json.Marshal(routes[index].PoWConfig)
			if err == nil {
				normalizedPoWConfig, err := normalizePoWConfig(true, string(raw))
				if err == nil {
					routes[index].PoWConfig = &normalizedPoWConfig
				}
			}
		} else {
			routes[index].PoWConfig = nil
		}
		if !routes[index].BasicAuthEnabled {
			routes[index].BasicAuthUsername = ""
			routes[index].BasicAuthPassword = ""
		}
		routes[index].UpstreamType = normalizeUpstreamType(routes[index].UpstreamType)
		if routes[index].UpstreamType == "tunnel" {
			routes[index].TunnelTargetAddr = strings.TrimSpace(routes[index].TunnelTargetAddr)
			routes[index].TunnelTargetProto = normalizeTunnelTargetProtocol(routes[index].TunnelTargetProto)
		} else {
			routes[index].TunnelNodeID = nil
			routes[index].TunnelTargetAddr = ""
			routes[index].TunnelTargetProto = ""
		}
	}
	return routes
}

func flattenSnapshotRoutesBySite(routes []snapshotRoute) map[string]snapshotRoute {
	siteMap := make(map[string]snapshotRoute)
	for _, route := range normalizeSnapshotRoutes(routes) {
		siteMap[route.SiteName] = route
	}
	return siteMap
}

func flattenSnapshotRoutesByDomain(routes []snapshotRoute) map[string]snapshotRoute {
	domainMap := make(map[string]snapshotRoute)
	for _, route := range normalizeSnapshotRoutes(routes) {
		for _, domain := range route.Domains {
			item := route
			item.Domain = domain
			domainMap[domain] = item
		}
	}
	return domainMap
}

func snapshotRouteConfigEqual(left snapshotRoute, right snapshotRoute) bool {
	if left.SiteName != right.SiteName || left.Domain != right.Domain || left.OriginURL != right.OriginURL || left.OriginHost != right.OriginHost || left.EnableHTTPS != right.EnableHTTPS || left.RedirectHTTP != right.RedirectHTTP || left.LimitConnPerServer != right.LimitConnPerServer || left.LimitConnPerIP != right.LimitConnPerIP || left.LimitRate != right.LimitRate || left.CacheEnabled != right.CacheEnabled || left.CachePolicy != right.CachePolicy || left.PoWEnabled != right.PoWEnabled || left.BasicAuthEnabled != right.BasicAuthEnabled || left.BasicAuthUsername != right.BasicAuthUsername || left.BasicAuthPassword != right.BasicAuthPassword || left.UpstreamType != right.UpstreamType || !uintPtrEqual(left.TunnelNodeID, right.TunnelNodeID) || left.TunnelTargetAddr != right.TunnelTargetAddr || left.TunnelTargetProto != right.TunnelTargetProto || !uintSliceEqual(left.CertIDs, right.CertIDs) || !uintSliceEqual(left.DomainCertIDs, right.DomainCertIDs) {
		return false
	}
	if len(left.Domains) != len(right.Domains) {
		return false
	}
	for index := range left.Domains {
		if left.Domains[index] != right.Domains[index] {
			return false
		}
	}
	if len(left.Upstreams) != len(right.Upstreams) {
		return false
	}
	for index := range left.Upstreams {
		if left.Upstreams[index] != right.Upstreams[index] {
			return false
		}
	}
	if len(left.CacheRules) != len(right.CacheRules) {
		return false
	}
	for index := range left.CacheRules {
		if left.CacheRules[index] != right.CacheRules[index] {
			return false
		}
	}
	if len(left.CustomHeaders) != len(right.CustomHeaders) {
		return false
	}
	for index := range left.CustomHeaders {
		if left.CustomHeaders[index] != right.CustomHeaders[index] {
			return false
		}
	}
	if !snapshotPoWConfigEqual(left.PoWConfig, right.PoWConfig) {
		return false
	}
	return true
}

func snapshotWAFConfigEqual(left snapshotWAFDocument, right snapshotWAFDocument) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func snapshotPoWConfigEqual(left *ProxyRoutePoWConfig, right *ProxyRoutePoWConfig) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Difficulty == right.Difficulty &&
		left.Algorithm == right.Algorithm &&
		left.SessionTTL == right.SessionTTL &&
		left.ChallengeTTL == right.ChallengeTTL &&
		stringSliceEqual(left.Whitelist.IPs, right.Whitelist.IPs) &&
		stringSliceEqual(left.Whitelist.IPCidrs, right.Whitelist.IPCidrs) &&
		stringSliceEqual(left.Whitelist.Paths, right.Whitelist.Paths) &&
		stringSliceEqual(left.Whitelist.PathRegexes, right.Whitelist.PathRegexes) &&
		stringSliceEqual(left.Whitelist.UserAgents, right.Whitelist.UserAgents) &&
		stringSliceEqual(left.Blacklist.IPs, right.Blacklist.IPs) &&
		stringSliceEqual(left.Blacklist.IPCidrs, right.Blacklist.IPCidrs) &&
		stringSliceEqual(left.Blacklist.Paths, right.Blacklist.Paths) &&
		stringSliceEqual(left.Blacklist.PathRegexes, right.Blacklist.PathRegexes) &&
		stringSliceEqual(left.Blacklist.UserAgents, right.Blacklist.UserAgents)
}

func stringSliceEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func buildOpenRestyConfigSnapshot() openRestyConfigSnapshot {
	return openRestyConfigSnapshot{
		WorkerProcesses:          common.OpenRestyWorkerProcesses,
		WorkerConnections:        common.OpenRestyWorkerConnections,
		WorkerRlimitNofile:       common.OpenRestyWorkerRlimitNofile,
		EventsUse:                common.OpenRestyEventsUse,
		EventsMultiAcceptEnabled: common.OpenRestyEventsMultiAcceptEnabled,
		KeepaliveTimeout:         common.OpenRestyKeepaliveTimeout,
		KeepaliveRequests:        common.OpenRestyKeepaliveRequests,
		ClientHeaderTimeout:      common.OpenRestyClientHeaderTimeout,
		ClientBodyTimeout:        common.OpenRestyClientBodyTimeout,
		ClientMaxBodySize:        common.OpenRestyClientMaxBodySize,
		LargeClientHeaderBuffers: common.OpenRestyLargeClientHeaderBuffers,
		SendTimeout:              common.OpenRestySendTimeout,
		ProxyConnectTimeout:      common.OpenRestyProxyConnectTimeout,
		ProxySendTimeout:         common.OpenRestyProxySendTimeout,
		ProxyReadTimeout:         common.OpenRestyProxyReadTimeout,
		WebsocketEnabled:         common.OpenRestyWebsocketEnabled,
		ProxyRequestBuffering:    common.OpenRestyProxyRequestBufferingEnabled,
		ProxyBufferingEnabled:    common.OpenRestyProxyBufferingEnabled,
		ProxyBuffers:             common.OpenRestyProxyBuffers,
		ProxyBufferSize:          common.OpenRestyProxyBufferSize,
		ProxyBusyBuffersSize:     common.OpenRestyProxyBusyBuffersSize,
		GzipEnabled:              common.OpenRestyGzipEnabled,
		GzipMinLength:            common.OpenRestyGzipMinLength,
		GzipCompLevel:            common.OpenRestyGzipCompLevel,
		Resolvers:                common.OpenRestyResolvers,
		CacheEnabled:             common.OpenRestyCacheEnabled,
		CachePath:                common.OpenRestyCachePath,
		CacheLevels:              common.OpenRestyCacheLevels,
		CacheInactive:            common.OpenRestyCacheInactive,
		CacheMaxSize:             common.OpenRestyCacheMaxSize,
		CacheKeyTemplate:         common.OpenRestyCacheKeyTemplate,
		CacheLockEnabled:         common.OpenRestyCacheLockEnabled,
		CacheLockTimeout:         common.OpenRestyCacheLockTimeout,
		CacheUseStale:            common.OpenRestyCacheUseStale,
		MainConfigTemplate:       common.OpenRestyMainConfigTemplate,
	}
}

func renderSnapshotConfig(sourceJSON string, certificateFiles []SupportFile) (*openrestyrender.Result, error) {
	return openrestyrender.RenderJSON(sourceJSON, toOpenRestySupportFiles(certificateFiles))
}

func buildCertificateSupportFiles(routes []snapshotRoute) ([]SupportFile, error) {
	certIDSet := make(map[uint]struct{})
	for _, route := range routes {
		for _, certID := range route.CertIDs {
			if certID != 0 {
				certIDSet[certID] = struct{}{}
			}
		}
	}
	if len(certIDSet) == 0 {
		return nil, nil
	}
	certIDs := make([]uint, 0, len(certIDSet))
	for certID := range certIDSet {
		certIDs = append(certIDs, certID)
	}
	sort.Slice(certIDs, func(i, j int) bool { return certIDs[i] < certIDs[j] })
	certificates, err := loadTLSCertificates(certIDs)
	if err != nil {
		return nil, err
	}
	files := make([]SupportFile, 0, len(certificates)*2)
	for _, certificate := range certificates {
		if certificate == nil {
			continue
		}
		files = append(files,
			SupportFile{Path: certificateCertFileName(certificate.ID), Content: normalizePEM(certificate.CertPEM)},
			SupportFile{Path: certificateKeyFileName(certificate.ID), Content: normalizePEM(certificate.KeyPEM)},
		)
	}
	return dedupeSupportFiles(files), nil
}

func toOpenRestySupportFiles(files []SupportFile) []openrestyrender.SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]openrestyrender.SupportFile, 0, len(files))
	for _, file := range files {
		result = append(result, openrestyrender.SupportFile{Path: file.Path, Content: file.Content})
	}
	return result
}

func fromOpenRestySupportFiles(files []openrestyrender.SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]SupportFile, 0, len(files))
	for _, file := range files {
		result = append(result, SupportFile{Path: file.Path, Content: file.Content})
	}
	return result
}

func buildInitialOpenRestyOptionDiffs(current openRestyConfigSnapshot) []ConfigOptionDiffItem {
	details := diffOpenRestyOptionDetails(openRestyConfigSnapshot{}, current)
	for index := range details {
		details[index].PreviousValue = ""
	}
	return details
}

func diffOpenRestyOptionDetails(left openRestyConfigSnapshot, right openRestyConfigSnapshot) []ConfigOptionDiffItem {
	changes := make([]ConfigOptionDiffItem, 0)
	appendIfChanged := func(key string, previous string, current string) {
		if previous == current {
			return
		}
		changes = append(changes, ConfigOptionDiffItem{
			Key:           key,
			PreviousValue: previous,
			CurrentValue:  current,
		})
	}
	appendIfChanged("OpenRestyWorkerProcesses", left.WorkerProcesses, right.WorkerProcesses)
	appendIfChanged("OpenRestyWorkerConnections", fmt.Sprintf("%d", left.WorkerConnections), fmt.Sprintf("%d", right.WorkerConnections))
	appendIfChanged("OpenRestyWorkerRlimitNofile", fmt.Sprintf("%d", left.WorkerRlimitNofile), fmt.Sprintf("%d", right.WorkerRlimitNofile))
	appendIfChanged("OpenRestyEventsUse", left.EventsUse, right.EventsUse)
	appendIfChanged("OpenRestyEventsMultiAcceptEnabled", fmt.Sprintf("%t", left.EventsMultiAcceptEnabled), fmt.Sprintf("%t", right.EventsMultiAcceptEnabled))
	appendIfChanged("OpenRestyKeepaliveTimeout", fmt.Sprintf("%d", left.KeepaliveTimeout), fmt.Sprintf("%d", right.KeepaliveTimeout))
	appendIfChanged("OpenRestyKeepaliveRequests", fmt.Sprintf("%d", left.KeepaliveRequests), fmt.Sprintf("%d", right.KeepaliveRequests))
	appendIfChanged("OpenRestyClientHeaderTimeout", fmt.Sprintf("%d", left.ClientHeaderTimeout), fmt.Sprintf("%d", right.ClientHeaderTimeout))
	appendIfChanged("OpenRestyClientBodyTimeout", fmt.Sprintf("%d", left.ClientBodyTimeout), fmt.Sprintf("%d", right.ClientBodyTimeout))
	appendIfChanged("OpenRestyClientMaxBodySize", left.ClientMaxBodySize, right.ClientMaxBodySize)
	appendIfChanged("OpenRestyLargeClientHeaderBuffers", left.LargeClientHeaderBuffers, right.LargeClientHeaderBuffers)
	appendIfChanged("OpenRestySendTimeout", fmt.Sprintf("%d", left.SendTimeout), fmt.Sprintf("%d", right.SendTimeout))
	appendIfChanged("OpenRestyProxyConnectTimeout", fmt.Sprintf("%d", left.ProxyConnectTimeout), fmt.Sprintf("%d", right.ProxyConnectTimeout))
	appendIfChanged("OpenRestyProxySendTimeout", fmt.Sprintf("%d", left.ProxySendTimeout), fmt.Sprintf("%d", right.ProxySendTimeout))
	appendIfChanged("OpenRestyProxyReadTimeout", fmt.Sprintf("%d", left.ProxyReadTimeout), fmt.Sprintf("%d", right.ProxyReadTimeout))
	appendIfChanged("OpenRestyWebsocketEnabled", fmt.Sprintf("%t", left.WebsocketEnabled), fmt.Sprintf("%t", right.WebsocketEnabled))
	appendIfChanged("OpenRestyProxyRequestBufferingEnabled", fmt.Sprintf("%t", left.ProxyRequestBuffering), fmt.Sprintf("%t", right.ProxyRequestBuffering))
	appendIfChanged("OpenRestyProxyBufferingEnabled", fmt.Sprintf("%t", left.ProxyBufferingEnabled), fmt.Sprintf("%t", right.ProxyBufferingEnabled))
	appendIfChanged("OpenRestyProxyBuffers", left.ProxyBuffers, right.ProxyBuffers)
	appendIfChanged("OpenRestyProxyBufferSize", left.ProxyBufferSize, right.ProxyBufferSize)
	appendIfChanged("OpenRestyProxyBusyBuffersSize", left.ProxyBusyBuffersSize, right.ProxyBusyBuffersSize)
	appendIfChanged("OpenRestyGzipEnabled", fmt.Sprintf("%t", left.GzipEnabled), fmt.Sprintf("%t", right.GzipEnabled))
	appendIfChanged("OpenRestyGzipMinLength", fmt.Sprintf("%d", left.GzipMinLength), fmt.Sprintf("%d", right.GzipMinLength))
	appendIfChanged("OpenRestyGzipCompLevel", fmt.Sprintf("%d", left.GzipCompLevel), fmt.Sprintf("%d", right.GzipCompLevel))
	appendIfChanged("OpenRestyResolvers", left.Resolvers, right.Resolvers)
	appendIfChanged("OpenRestyCacheEnabled", fmt.Sprintf("%t", left.CacheEnabled), fmt.Sprintf("%t", right.CacheEnabled))
	appendIfChanged("OpenRestyCachePath", left.CachePath, right.CachePath)
	appendIfChanged("OpenRestyCacheLevels", left.CacheLevels, right.CacheLevels)
	appendIfChanged("OpenRestyCacheInactive", left.CacheInactive, right.CacheInactive)
	appendIfChanged("OpenRestyCacheMaxSize", left.CacheMaxSize, right.CacheMaxSize)
	appendIfChanged("OpenRestyCacheKeyTemplate", left.CacheKeyTemplate, right.CacheKeyTemplate)
	appendIfChanged("OpenRestyCacheLockEnabled", fmt.Sprintf("%t", left.CacheLockEnabled), fmt.Sprintf("%t", right.CacheLockEnabled))
	appendIfChanged("OpenRestyCacheLockTimeout", left.CacheLockTimeout, right.CacheLockTimeout)
	appendIfChanged("OpenRestyCacheUseStale", left.CacheUseStale, right.CacheUseStale)
	return changes
}

func extractOptionDiffKeys(details []ConfigOptionDiffItem) []string {
	keys := make([]string, 0, len(details))
	for _, item := range details {
		keys = append(keys, item.Key)
	}
	return keys
}

func openRestyOptionKeys() []string {
	return []string{
		"OpenRestyWorkerProcesses",
		"OpenRestyWorkerConnections",
		"OpenRestyWorkerRlimitNofile",
		"OpenRestyEventsUse",
		"OpenRestyEventsMultiAcceptEnabled",
		"OpenRestyKeepaliveTimeout",
		"OpenRestyKeepaliveRequests",
		"OpenRestyClientHeaderTimeout",
		"OpenRestyClientBodyTimeout",
		"OpenRestyClientMaxBodySize",
		"OpenRestyLargeClientHeaderBuffers",
		"OpenRestySendTimeout",
		"OpenRestyProxyConnectTimeout",
		"OpenRestyProxySendTimeout",
		"OpenRestyProxyReadTimeout",
		"OpenRestyWebsocketEnabled",
		"OpenRestyProxyRequestBufferingEnabled",
		"OpenRestyProxyBufferingEnabled",
		"OpenRestyProxyBuffers",
		"OpenRestyProxyBufferSize",
		"OpenRestyProxyBusyBuffersSize",
		"OpenRestyGzipEnabled",
		"OpenRestyGzipMinLength",
		"OpenRestyGzipCompLevel",
		"OpenRestyCacheEnabled",
		"OpenRestyCachePath",
		"OpenRestyCacheLevels",
		"OpenRestyCacheInactive",
		"OpenRestyCacheMaxSize",
		"OpenRestyCacheKeyTemplate",
		"OpenRestyCacheLockEnabled",
		"OpenRestyCacheLockTimeout",
		"OpenRestyCacheUseStale",
	}
}

func ValidateOpenRestyMainConfigTemplate(templateText string) error {
	return openrestyrender.ValidateMainConfigTemplate(templateText)
}

func normalizeSnapshotCertificateIDs(primaryCertID *uint, certIDs []uint) ([]uint, *uint, error) {
	candidates := make([]uint, 0, len(certIDs)+1)
	if primaryCertID != nil && *primaryCertID != 0 {
		candidates = append(candidates, *primaryCertID)
	}
	candidates = append(candidates, certIDs...)

	normalized := make([]uint, 0, len(candidates))
	seen := make(map[uint]struct{}, len(candidates))
	for _, certID := range candidates {
		if certID == 0 {
			continue
		}
		if _, ok := seen[certID]; ok {
			continue
		}
		seen[certID] = struct{}{}
		normalized = append(normalized, certID)
	}

	var normalizedPrimary *uint
	if len(normalized) > 0 {
		normalizedPrimary = &normalized[0]
	}
	return normalized, normalizedPrimary, nil
}

func normalizeSnapshotDomainCertificateIDs(
	domains []string,
	certIDs []uint,
	domainCertIDs []uint,
) ([]uint, error) {
	if len(domainCertIDs) > 0 {
		if len(domains) > 0 && len(domainCertIDs) != len(domains) {
			return nil, errors.New("snapshot domain_cert_ids length is invalid")
		}
		normalized := make([]uint, len(domainCertIDs))
		copy(normalized, domainCertIDs)
		return normalized, nil
	}
	if len(certIDs) == 0 {
		return []uint{}, nil
	}
	if len(certIDs) == 1 {
		normalized := make([]uint, len(domains))
		for index := range normalized {
			normalized[index] = certIDs[0]
		}
		return normalized, nil
	}
	if len(certIDs) == len(domains) {
		normalized := make([]uint, len(certIDs))
		copy(normalized, certIDs)
		return normalized, nil
	}
	return []uint{}, nil
}

func uintSliceEqual(left []uint, right []uint) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func uintPtrEqual(left *uint, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func nextVersionNumber(now time.Time) (string, error) {
	prefix := now.Format("20060102")
	var latest model.ConfigVersion
	err := model.DB.
		Select("version").
		Where("version LIKE ?", prefix+"-%").
		Order("version desc").
		First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Sprintf("%s-%03d", prefix, 1), nil
	}
	if err != nil {
		return "", err
	}
	suffix := strings.TrimPrefix(latest.Version, prefix+"-")
	sequence, err := strconv.Atoi(suffix)
	if err != nil {
		return "", fmt.Errorf("invalid config version sequence %q: %w", latest.Version, err)
	}
	return fmt.Sprintf("%s-%03d", prefix, sequence+1), nil
}

func validateCertificateCoverage(certificate *model.TLSCertificate, domains []string) error {
	if certificate == nil {
		return errors.New("certificate is nil")
	}
	leaf, err := parseLeafCertificate(certificate.CertPEM)
	if err != nil {
		return err
	}
	for _, domain := range domains {
		if err := leaf.VerifyHostname(domain); err != nil {
			return fmt.Errorf("certificate does not cover domain %s", domain)
		}
	}
	return nil
}

func loadTLSCertificates(certIDs []uint) ([]*model.TLSCertificate, error) {
	certificates := make([]*model.TLSCertificate, 0, len(certIDs))
	for _, certID := range certIDs {
		certificate, err := model.GetTLSCertificateByID(certID)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, certificate)
	}
	return certificates, nil
}

func certificateCertFileName(id uint) string {
	return fmt.Sprintf("%d.crt", id)
}

func certificateKeyFileName(id uint) string {
	return fmt.Sprintf("%d.key", id)
}

func normalizePEM(content string) string {
	return strings.TrimSpace(content) + "\n"
}

func dedupeSupportFiles(files []SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	unique := make(map[string]SupportFile, len(files))
	for _, file := range files {
		unique[file.Path] = file
	}
	result := make([]SupportFile, 0, len(unique))
	for _, file := range unique {
		result = append(result, file)
	}
	return result
}
