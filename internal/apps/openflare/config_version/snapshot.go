// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	oftls "github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/protocol"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
	"gorm.io/gorm"
)

const (
	supportFilesPerCertificate  = 2
	wafIPGroupChecksumHexLength = 64

	// OpenResty 默认配置值
	defaultOpenRestyReturnStatus     = 421
	defaultOpenRestyWorkerConns      = 4096
	defaultOpenRestyRlimitNofile     = 65535
	defaultOpenRestyKeepaliveTimeout = 20
	defaultOpenRestyKeepaliveReqs    = 1000
	defaultOpenRestyHeaderTimeout    = 15
	defaultOpenRestyBodyTimeout      = 15
	defaultOpenRestySendTimeout      = 30
	defaultOpenRestyConnectTimeout   = 3
	defaultOpenRestyProxyTimeout     = 60
	defaultOpenRestyGzipMinLen       = 1024
	defaultOpenRestyGzipLevel        = 5
)

type snapshotRoute struct {
	ID                 uint                             `json:"id,omitempty"`
	SiteName           string                           `json:"site_name,omitempty"`
	Domains            []string                         `json:"domains,omitempty"`
	OriginURL          string                           `json:"origin_url"`
	OriginHost         string                           `json:"origin_host,omitempty"`
	Upstreams          []string                         `json:"upstreams,omitempty"`
	Enabled            bool                             `json:"enabled"`
	EnableHTTPS        bool                             `json:"enable_https"`
	DomainCertIDs      []uint                           `json:"domain_cert_ids,omitempty"`
	RedirectHTTP       bool                             `json:"redirect_http"`
	LimitConnPerServer int                              `json:"limit_conn_per_server,omitempty"`
	LimitConnPerIP     int                              `json:"limit_conn_per_ip,omitempty"`
	LimitRate          string                           `json:"limit_rate,omitempty"`
	CacheEnabled       bool                             `json:"cache_enabled"`
	CachePolicy        string                           `json:"cache_policy,omitempty"`
	CacheRules         []string                         `json:"cache_rules,omitempty"`
	CustomHeaders      []customHeaderInput              `json:"custom_headers,omitempty"`
	BasicAuthEnabled   bool                             `json:"basic_auth_enabled,omitempty"`
	BasicAuthUsername  string                           `json:"basic_auth_username,omitempty"`
	BasicAuthPassword  string                           `json:"basic_auth_password,omitempty"`
	UpstreamType       string                           `json:"upstream_type,omitempty"`
	TunnelNodeID       *uint                            `json:"tunnel_node_id,omitempty"`
	TunnelTargetAddr   string                           `json:"tunnel_target_addr,omitempty"`
	TunnelTargetProto  string                           `json:"tunnel_target_protocol,omitempty"`
	PagesProjectID     *uint                            `json:"pages_project_id,omitempty"`
	PagesDeployment    *openrestyrender.PagesDeployment `json:"pages_deployment,omitempty"`
}

type snapshotWAFRuleGroup struct {
	ID       uint                 `json:"id"`
	Name     string               `json:"name"`
	Enabled  bool                 `json:"enabled"`
	IsGlobal bool                 `json:"is_global"`
	Graph    waf.RuntimeRuleGraph `json:"graph"`
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
	DefaultServerReturnStatus int    `json:"default_server_return_status"`
	WorkerProcesses           string `json:"worker_processes"`
	WorkerConnections         int    `json:"worker_connections"`
	WorkerRlimitNofile        int    `json:"worker_rlimit_nofile"`
	EventsUse                 string `json:"events_use,omitempty"`
	EventsMultiAcceptEnabled  bool   `json:"events_multi_accept_enabled"`
	KeepaliveTimeout          int    `json:"keepalive_timeout"`
	KeepaliveRequests         int    `json:"keepalive_requests"`
	ClientHeaderTimeout       int    `json:"client_header_timeout"`
	ClientBodyTimeout         int    `json:"client_body_timeout"`
	ClientMaxBodySize         string `json:"client_max_body_size"`
	LargeClientHeaderBuffers  string `json:"large_client_header_buffers"`
	SendTimeout               int    `json:"send_timeout"`
	ProxyConnectTimeout       int    `json:"proxy_connect_timeout"`
	ProxySendTimeout          int    `json:"proxy_send_timeout"`
	ProxyReadTimeout          int    `json:"proxy_read_timeout"`
	WebsocketEnabled          bool   `json:"websocket_enabled"`
	HTTP3Enabled              bool   `json:"http3_enabled"`
	ProxyRequestBuffering     bool   `json:"proxy_request_buffering"`
	ProxyBufferingEnabled     bool   `json:"proxy_buffering_enabled"`
	ProxyBuffers              string `json:"proxy_buffers"`
	ProxyBufferSize           string `json:"proxy_buffer_size"`
	ProxyBusyBuffersSize      string `json:"proxy_busy_buffers_size"`
	GzipEnabled               bool   `json:"gzip_enabled"`
	GzipMinLength             int    `json:"gzip_min_length"`
	GzipCompLevel             int    `json:"gzip_comp_level"`
	Resolvers                 string `json:"resolvers,omitempty"`
	CacheEnabled              bool   `json:"cache_enabled"`
	CachePath                 string `json:"cache_path,omitempty"`
	CacheLevels               string `json:"cache_levels"`
	CacheInactive             string `json:"cache_inactive"`
	CacheMaxSize              string `json:"cache_max_size"`
	CacheKeyTemplate          string `json:"cache_key_template"`
	CacheLockEnabled          bool   `json:"cache_lock_enabled"`
	CacheLockTimeout          string `json:"cache_lock_timeout"`
	CacheUseStale             string `json:"cache_use_stale"`
	MainConfigTemplate        string `json:"main_config_template,omitempty"`
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

func buildCurrentConfigBundle(ctx context.Context, requireRoutes bool) (*configBundle, error) {
	routes, err := model.ListEnabledProxyRoutes(ctx)
	if err != nil {
		return nil, err
	}
	if requireRoutes && len(routes) == 0 {
		return nil, errors.New(errNoEnabledRoutes)
	}
	snapshotRoutes, err := buildSnapshotRoutes(ctx, routes)
	if err != nil {
		return nil, err
	}
	wafSnapshot, err := buildSnapshotWAFDocument(ctx, routes)
	if err != nil {
		return nil, err
	}
	openRestyConfig := buildOpenRestyConfigSnapshot(ctx)
	snapshotDoc := snapshotDocument{
		Routes:          snapshotRoutes,
		OpenRestyConfig: openRestyConfig,
		WAF:             wafSnapshot,
	}
	snapshotJSON, err := json.Marshal(snapshotDoc)
	if err != nil {
		return nil, err
	}
	certificateFiles, err := buildCertificateSupportFiles(ctx, snapshotRoutes)
	if err != nil {
		return nil, err
	}

	mainConfig := ""
	routeConfig := ""
	checksum := ""
	supportFiles := []SupportFile(nil)

	rendered, renderErr := renderSnapshotConfig(string(snapshotJSON), certificateFiles)
	if renderErr == nil {
		mainConfig = rendered.MainConfig
		routeConfig = rendered.RouteConfig
		checksum = rendered.Checksum
		supportFiles = fromOpenRestySupportFiles(rendered.SupportFiles)
	} else {
		mainConfig, routeConfig, checksum = renderPlaceholderConfig(string(snapshotJSON))
	}

	return &configBundle{
		Routes:            routes,
		SnapshotRoutes:    snapshotRoutes,
		WAFSnapshot:       wafSnapshot,
		OpenRestyConfig:   openRestyConfig,
		SnapshotJSON:      string(snapshotJSON),
		MainConfig:        mainConfig,
		RouteConfig:       routeConfig,
		SupportFiles:      supportFiles,
		Checksum:          checksum,
		ChangedOptionKeys: openRestyOptionKeys(),
	}, nil
}

func buildSnapshotRoutes(ctx context.Context, routes []*model.ProxyRoute) ([]snapshotRoute, error) {
	items := make([]snapshotRoute, 0, len(routes))
	for _, route := range routes {
		zoneDomains, err := model.ListZoneDomainsByRouteID(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		if len(zoneDomains) == 0 {
			return nil, fmt.Errorf("route %s has no zone domains", route.SiteName)
		}
		domains := make([]string, 0, len(zoneDomains))
		domainCertIDs := make([]uint, 0, len(zoneDomains))
		for _, zoneDomain := range zoneDomains {
			domains = append(domains, zoneDomain.Domain)
			if zoneDomain.CertID == nil {
				domainCertIDs = append(domainCertIDs, 0)
				continue
			}
			domainCertIDs = append(domainCertIDs, *zoneDomain.CertID)
		}
		customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 自定义请求头无效", route.SiteName)
		}
		upstreamType := normalizeUpstreamType(route.UpstreamType)
		originURL := route.OriginURL
		upstreams, err := decodeStoredUpstreams(route.Upstreams, route.OriginURL)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 上游配置无效", route.SiteName)
		}
		var tunnelNodeID *uint
		var tunnelTargetAddr string
		var tunnelTargetProtocol string
		var pagesProjectID *uint
		var pagesDeployment *openrestyrender.PagesDeployment
		switch upstreamType {
		case "tunnel":
			originURL = resolveTunnelOpenRestyUpstreamURL(ctx)
			upstreams = []string{originURL}
			tunnelNodeID = route.TunnelNodeID
			tunnelTargetAddr = strings.TrimSpace(route.TunnelTargetAddr)
			tunnelTargetProtocol = normalizeTunnelTargetProtocol(route.TunnelTargetProtocol)
		case "pages":
			originURL, upstreams, pagesProjectID, pagesDeployment, err = buildPagesRouteSnapshot(ctx, route)
			if err != nil {
				return nil, err
			}
		}
		cacheRules, err := decodeStoredCacheRules(route.CacheRules)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 缓存规则无效", route.SiteName)
		}
		items = append(items, snapshotRoute{
			ID:                 route.ID,
			SiteName:           route.SiteName,
			Domains:            domains,
			OriginURL:          originURL,
			OriginHost:         route.OriginHost,
			Upstreams:          upstreams,
			Enabled:            route.Enabled,
			EnableHTTPS:        route.EnableHTTPS,
			DomainCertIDs:      domainCertIDs,
			RedirectHTTP:       route.RedirectHTTP,
			LimitConnPerServer: route.LimitConnPerServer,
			LimitConnPerIP:     route.LimitConnPerIP,
			LimitRate:          route.LimitRate,
			CacheEnabled:       route.CacheEnabled,
			CachePolicy:        route.CachePolicy,
			CacheRules:         cacheRules,
			CustomHeaders:      customHeaders,
			BasicAuthEnabled:   route.BasicAuthEnabled,
			BasicAuthUsername:  route.BasicAuthUsername,
			BasicAuthPassword:  route.BasicAuthPassword,
			UpstreamType:       upstreamType,
			TunnelNodeID:       tunnelNodeID,
			TunnelTargetAddr:   tunnelTargetAddr,
			TunnelTargetProto:  tunnelTargetProtocol,
			PagesProjectID:     pagesProjectID,
			PagesDeployment:    pagesDeployment,
		})
	}
	return items, nil
}

func buildSnapshotWAFDocument(ctx context.Context, routes []*model.ProxyRoute) (snapshotWAFDocument, error) {
	if err := waf.EnsureDefaultRuleGroup(ctx); err != nil {
		return snapshotWAFDocument{}, err
	}
	groups, err := model.ListOpenFlareWAFRuleGroups(ctx)
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	ruleGroups := make([]snapshotWAFRuleGroup, 0, len(groups))
	referencedIPGroupIDs := make(map[uint]struct{})
	enabledRuleIDs := make(map[uint]struct{})
	for _, group := range groups {
		if !group.Enabled {
			continue
		}
		var editorGraph waf.RuleGraph
		if err = json.Unmarshal([]byte(group.Graph), &editorGraph); err != nil {
			return snapshotWAFDocument{}, fmt.Errorf("WAF 规则 %s 的图数据无效: %w", group.Name, err)
		}
		if err = waf.ValidateRuleGraph(ctx, editorGraph, snapshotWAFIPGroupExists); err != nil {
			return snapshotWAFDocument{}, fmt.Errorf("WAF 规则 %s 的图无效: %w", group.Name, err)
		}
		runtimeGraph, compileErr := waf.CompileRuleGraph(editorGraph)
		if compileErr != nil {
			return snapshotWAFDocument{}, fmt.Errorf("WAF 规则 %s 编译失败: %w", group.Name, compileErr)
		}
		ruleGroups = append(ruleGroups, snapshotWAFRuleGroup{
			ID: group.ID, Name: group.Name, Enabled: group.Enabled, IsGlobal: group.IsGlobal, Graph: runtimeGraph,
		})
		enabledRuleIDs[group.ID] = struct{}{}
		for _, id := range waf.ReferencedIPGroupIDs(editorGraph) {
			referencedIPGroupIDs[id] = struct{}{}
		}
	}
	ipGroups, err := buildSnapshotWAFIPGroups(ctx, referencedIPGroupIDs)
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	enabledRouteSiteNames := make(map[uint]string, len(routes))
	for _, route := range routes {
		if route == nil {
			continue
		}
		domains, domainErr := model.ListZoneDomainsByRouteID(ctx, route.ID)
		if domainErr != nil {
			return snapshotWAFDocument{}, domainErr
		}
		if len(domains) == 0 {
			return snapshotWAFDocument{}, fmt.Errorf("route %s has no zone domains", route.SiteName)
		}
		enabledRouteSiteNames[route.ID] = route.SiteName
	}
	rawBindings, err := model.ListOpenFlareWAFRuleGroupBindings(ctx)
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	groupIDsByRoute := make(map[uint][]uint, len(rawBindings))
	for _, binding := range rawBindings {
		if _, ok := enabledRouteSiteNames[binding.ProxyRouteID]; !ok {
			continue
		}
		if _, enabled := enabledRuleIDs[binding.RuleGroupID]; enabled {
			groupIDsByRoute[binding.ProxyRouteID] = append(groupIDsByRoute[binding.ProxyRouteID], binding.RuleGroupID)
		}
	}
	bindings := make([]snapshotWAFBinding, 0, len(enabledRouteSiteNames))
	for routeID, siteName := range enabledRouteSiteNames {
		bindings = append(bindings, snapshotWAFBinding{
			RouteID:      routeID,
			SiteName:     siteName,
			RuleGroupIDs: nonNilUintSlice(groupIDsByRoute[routeID]),
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

func nonNilUintSlice(values []uint) []uint {
	if values == nil {
		return make([]uint, 0)
	}
	return values
}

func validateSnapshotWAFIPGroupSize(groups []snapshotWAFIPGroup) error {
	runtimeGroups := make(map[string]protocol.WAFIPGroup, len(groups))
	for _, group := range groups {
		ipList := group.IPList
		if !group.Enabled {
			ipList = []string{}
		}
		runtimeGroups[strconv.FormatUint(uint64(group.ID), 10)] = protocol.WAFIPGroup{
			ID:       group.ID,
			Name:     group.Name,
			Type:     group.Type,
			Enabled:  group.Enabled,
			IPList:   ipList,
			Checksum: strings.Repeat("0", wafIPGroupChecksumHexLength),
		}
	}
	return protocol.ValidateWAFIPGroupSnapshotSize(runtimeGroups)
}

func buildSnapshotWAFIPGroups(ctx context.Context, idSet map[uint]struct{}) ([]snapshotWAFIPGroup, error) {
	if len(idSet) == 0 {
		return []snapshotWAFIPGroup{}, nil
	}
	ids := make([]uint, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	groups, err := listWAFIPGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	groupByID := make(map[uint]*model.OpenFlareWAFIPGroup, len(groups))
	for _, group := range groups {
		groupByID[group.ID] = group
	}
	snapshots := make([]snapshotWAFIPGroup, 0, len(ids))
	for _, id := range ids {
		group := groupByID[id]
		if group == nil {
			return nil, fmt.Errorf("IP 组 %d 不存在", id)
		}
		ipList, decodeErr := decodeIPList(group.IPList)
		if decodeErr != nil {
			return nil, decodeErr
		}
		snapshots = append(snapshots, snapshotWAFIPGroup{
			ID:      group.ID,
			Name:    group.Name,
			Type:    group.Type,
			Enabled: group.Enabled,
			IPList:  ipList,
		})
	}
	if err = validateSnapshotWAFIPGroupSize(snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func snapshotWAFIPGroupExists(ctx context.Context, id uint) (bool, error) {
	group, err := model.GetOpenFlareWAFIPGroupByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return group != nil, err
}

func decodeIPList(raw string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []string{}, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		return nil, fmt.Errorf("ip_list payload is invalid")
	}
	return items, nil
}

func buildOpenRestyConfigSnapshot(ctx context.Context) openRestyConfigSnapshot {
	// 读取所有 OpenResty 配置，使用默认值作为降级
	getIntConfig := func(key string, defaultVal int) int {
		val, err := repository.GetIntByKey(ctx, key)
		if err != nil || val <= 0 {
			return defaultVal
		}
		return val
	}

	getBoolConfig := func(key string, defaultVal bool) bool {
		val, err := repository.GetBoolByKey(ctx, key)
		if err != nil {
			return defaultVal
		}
		return val
	}

	getStringConfig := func(key string, defaultVal string) string {
		config, err := repository.GetSystemConfigByKey(ctx, key)
		if err != nil {
			return defaultVal
		}
		return config.Value
	}

	snapshot := openRestyConfigSnapshot{
		DefaultServerReturnStatus: getIntConfig(model.ConfigKeyOpenRestyDefaultServerReturnStatus, defaultOpenRestyReturnStatus),
		WorkerProcesses:           getStringConfig(model.ConfigKeyOpenRestyWorkerProcesses, "auto"),
		WorkerConnections:         getIntConfig(model.ConfigKeyOpenRestyWorkerConnections, defaultOpenRestyWorkerConns),
		WorkerRlimitNofile:        getIntConfig(model.ConfigKeyOpenRestyWorkerRlimitNofile, defaultOpenRestyRlimitNofile),
		EventsUse:                 getStringConfig(model.ConfigKeyOpenRestyEventsUse, "epoll"),
		EventsMultiAcceptEnabled:  getBoolConfig(model.ConfigKeyOpenRestyEventsMultiAcceptEnabled, true),
		KeepaliveTimeout:          getIntConfig(model.ConfigKeyOpenRestyKeepaliveTimeout, defaultOpenRestyKeepaliveTimeout),
		KeepaliveRequests:         getIntConfig(model.ConfigKeyOpenRestyKeepaliveRequests, defaultOpenRestyKeepaliveReqs),
		ClientHeaderTimeout:       getIntConfig(model.ConfigKeyOpenRestyClientHeaderTimeout, defaultOpenRestyHeaderTimeout),
		ClientBodyTimeout:         getIntConfig(model.ConfigKeyOpenRestyClientBodyTimeout, defaultOpenRestyBodyTimeout),
		ClientMaxBodySize:         getStringConfig(model.ConfigKeyOpenRestyClientMaxBodySize, "64m"),
		LargeClientHeaderBuffers:  getStringConfig(model.ConfigKeyOpenRestyLargeClientHeaderBuffers, "4 16k"),
		SendTimeout:               getIntConfig(model.ConfigKeyOpenRestySendTimeout, defaultOpenRestySendTimeout),
		ProxyConnectTimeout:       getIntConfig(model.ConfigKeyOpenRestyProxyConnectTimeout, defaultOpenRestyConnectTimeout),
		ProxySendTimeout:          getIntConfig(model.ConfigKeyOpenRestyProxySendTimeout, defaultOpenRestyProxyTimeout),
		ProxyReadTimeout:          getIntConfig(model.ConfigKeyOpenRestyProxyReadTimeout, defaultOpenRestyProxyTimeout),
		WebsocketEnabled:          getBoolConfig(model.ConfigKeyOpenRestyWebsocketEnabled, true),
		HTTP3Enabled:              getBoolConfig(model.ConfigKeyOpenRestyHTTP3Enabled, true),
		ProxyRequestBuffering:     getBoolConfig(model.ConfigKeyOpenRestyProxyRequestBufferingEnabled, false),
		ProxyBufferingEnabled:     getBoolConfig(model.ConfigKeyOpenRestyProxyBufferingEnabled, true),
		ProxyBuffers:              getStringConfig(model.ConfigKeyOpenRestyProxyBuffers, "16 16k"),
		ProxyBufferSize:           getStringConfig(model.ConfigKeyOpenRestyProxyBufferSize, "8k"),
		ProxyBusyBuffersSize:      getStringConfig(model.ConfigKeyOpenRestyProxyBusyBuffersSize, "64k"),
		GzipEnabled:               getBoolConfig(model.ConfigKeyOpenRestyGzipEnabled, true),
		GzipMinLength:             getIntConfig(model.ConfigKeyOpenRestyGzipMinLength, defaultOpenRestyGzipMinLen),
		GzipCompLevel:             getIntConfig(model.ConfigKeyOpenRestyGzipCompLevel, defaultOpenRestyGzipLevel),
		Resolvers:                 getStringConfig(model.ConfigKeyOpenRestyResolvers, ""),
		CacheEnabled:              getBoolConfig(model.ConfigKeyOpenRestyCacheEnabled, false),
		CachePath:                 getStringConfig(model.ConfigKeyOpenRestyCachePath, ""),
		CacheLevels:               getStringConfig(model.ConfigKeyOpenRestyCacheLevels, "1:2"),
		CacheInactive:             getStringConfig(model.ConfigKeyOpenRestyCacheInactive, "30m"),
		CacheMaxSize:              getStringConfig(model.ConfigKeyOpenRestyCacheMaxSize, "1g"),
		CacheKeyTemplate:          getStringConfig(model.ConfigKeyOpenRestyCacheKeyTemplate, "$scheme$host$request_uri"),
		CacheLockEnabled:          getBoolConfig(model.ConfigKeyOpenRestyCacheLockEnabled, true),
		CacheLockTimeout:          getStringConfig(model.ConfigKeyOpenRestyCacheLockTimeout, "5s"),
		CacheUseStale:             getStringConfig(model.ConfigKeyOpenRestyCacheUseStale, "error timeout updating http_500 http_502 http_503 http_504"),
		MainConfigTemplate:        getStringConfig(model.ConfigKeyOpenRestyMainConfigTemplate, model.DefaultOpenRestyMainConfigTemplate),
	}
	snapshot.CachePath = normalizeProxyCachePathForSnapshot(snapshot.CacheEnabled, snapshot.CachePath)
	return snapshot
}

func normalizeProxyCachePathForSnapshot(cacheEnabled bool, cachePath string) string {
	if !cacheEnabled {
		return strings.TrimSpace(cachePath)
	}
	trimmed := strings.TrimSpace(cachePath)
	if trimmed == "" || strings.HasPrefix(trimmed, "/var/") {
		return openrestyrender.ProxyCachePathPlaceholder
	}
	return trimmed
}

func buildCertificateSupportFiles(ctx context.Context, routes []snapshotRoute) ([]SupportFile, error) {
	certIDSet := make(map[uint]struct{})
	for _, route := range routes {
		for _, certID := range route.DomainCertIDs {
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
	files := make([]SupportFile, 0, len(certIDs)*supportFilesPerCertificate)
	for _, certID := range certIDs {
		certificate, err := model.GetTLSCertificateByID(ctx, certID)
		if err != nil {
			return nil, err
		}
		keyPEM, err := oftls.OpenKeyPEM(certificate.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("certificate %d private key: %w", certificate.ID, err)
		}
		if strings.TrimSpace(keyPEM) == "" {
			return nil, fmt.Errorf("certificate %d has no private key", certificate.ID)
		}
		files = append(files,
			SupportFile{Path: certificateCertFileName(certificate.ID), Content: normalizePEM(certificate.CertPEM)},
			SupportFile{Path: certificateKeyFileName(certificate.ID), Content: normalizePEM(keyPEM)},
		)
	}
	return dedupeSupportFiles(files), nil
}
