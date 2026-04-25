package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"openflare/common"
	"openflare/model"
	"regexp"
	"sort"
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
	Remark             string                        `json:"remark,omitempty"`
}

type routeCacheConfig struct {
	Enabled bool
	Policy  string
	Rules   []string
}

type routeLimitConfig struct {
	LimitConnPerServer int
	LimitConnPerIP     int
	LimitRate          string
}

type routeUpstreamConfig struct {
	Name              string
	Scheme            string
	ProxyPassURI      string
	Servers           []string
	UsesNamedUpstream bool
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
	CacheEnabled             bool   `json:"cache_enabled"`
	CachePath                string `json:"cache_path,omitempty"`
	CacheLevels              string `json:"cache_levels"`
	CacheInactive            string `json:"cache_inactive"`
	CacheMaxSize             string `json:"cache_max_size"`
	CacheKeyTemplate         string `json:"cache_key_template"`
	CacheLockEnabled         bool   `json:"cache_lock_enabled"`
	CacheLockTimeout         string `json:"cache_lock_timeout"`
	CacheUseStale            string `json:"cache_use_stale"`
}

type snapshotDocument struct {
	Routes          []snapshotRoute         `json:"routes"`
	OpenRestyConfig openRestyConfigSnapshot `json:"openresty_config"`
}

type configBundle struct {
	Routes            []*model.ProxyRoute
	SnapshotRoutes    []snapshotRoute
	OpenRestyConfig   openRestyConfigSnapshot
	SnapshotJSON      string
	MainConfig        string
	RouteConfig       string
	SupportFiles      []SupportFile
	Checksum          string
	ChangedOptionKeys []string
}

const (
	nginxCertDirPlaceholder             = "__OPENFLARE_CERT_DIR__"
	nginxRouteConfigPlaceholder         = "__OPENFLARE_ROUTE_CONFIG__"
	nginxAccessLogPlaceholder           = "__OPENFLARE_ACCESS_LOG__"
	nginxLuaDirPlaceholder              = "__OPENFLARE_LUA_DIR__"
	nginxObservabilityListenPlaceholder = "__OPENFLARE_OBSERVABILITY_LISTEN__"
	nginxObservabilityPortPlaceholder   = "__OPENFLARE_OBSERVABILITY_PORT__"
)

var requiredMainConfigTemplatePlaceholders = []string{
	"{{OpenRestyWorkerProcesses}}",
	"{{OpenRestyWorkerConnections}}",
	"{{OpenRestyWorkerRlimitNofile}}",
	"{{OpenRestyConnectionUpgradeMap}}",
	"{{OpenRestyDefaultServerBlock}}",
	"{{OpenRestyAccessLogPath}}",
	"{{OpenRestyEventsUseDirective}}",
	"{{OpenRestyEventsMultiAcceptDirective}}",
	"{{OpenRestyKeepaliveTimeout}}",
	"{{OpenRestyKeepaliveRequests}}",
	"{{OpenRestyClientHeaderTimeout}}",
	"{{OpenRestyClientBodyTimeout}}",
	"{{OpenRestyClientMaxBodySize}}",
	"{{OpenRestyLargeClientHeaderBuffers}}",
	"{{OpenRestySendTimeout}}",
	"{{OpenRestyProxyConnectTimeout}}",
	"{{OpenRestyProxySendTimeout}}",
	"{{OpenRestyProxyReadTimeout}}",
	"{{OpenRestyProxyRequestBuffering}}",
	"{{OpenRestyProxyBuffering}}",
	"{{OpenRestyProxyBuffers}}",
	"{{OpenRestyProxyBufferSize}}",
	"{{OpenRestyProxyBusyBuffersSize}}",
	"{{OpenRestyGzip}}",
	"{{OpenRestyGzipMinLength}}",
	"{{OpenRestyGzipCompLevel}}",
	"{{OpenRestyCacheBlock}}",
	"{{OpenRestyRouteConfigInclude}}",
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

func HasConfigChanges() (bool, error) {
	bundle, err := buildCurrentConfigBundle(false)
	if err != nil {
		return false, err
	}
	activeVersion, err := model.GetActiveConfigVersion()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return len(bundle.Routes) > 0, nil
		}
		return false, err
	}
	return activeVersion.Checksum != bundle.Checksum, nil
}

func PublishConfigVersion(createdBy string) (*ReleaseResult, error) {
	bundle, err := buildCurrentConfigBundle(true)
	if err != nil {
		return nil, err
	}
	if len(bundle.Routes) == 0 {
		return nil, errors.New("没有可发布的启用规则")
	}
	activeVersion, err := model.GetActiveConfigVersion()
	if err == nil && activeVersion.Checksum == bundle.Checksum {
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
		if isUniqueConstraintError(err) {
			return nil, errors.New("版本号生成冲突，请重试")
		}
		return nil, err
	}
	return &ReleaseResult{
		Version: record,
		Routes:  bundle.Routes,
	}, nil
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
	return version, nil
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
	openRestyConfig := buildOpenRestyConfigSnapshot()
	snapshotDoc := snapshotDocument{
		Routes:          snapshotRoutes,
		OpenRestyConfig: openRestyConfig,
	}
	snapshotJSON, err := json.Marshal(snapshotDoc)
	if err != nil {
		return nil, err
	}
	routeConfig, supportFiles, err := renderRouteConfig(routes, openRestyConfig)
	if err != nil {
		return nil, err
	}
	powConfigJSON, powSupportFiles, err := renderPowConfigBundle(routes)
	if err != nil {
		return nil, err
	}
	supportFiles = append(supportFiles, powSupportFiles...)
	mainConfig := renderMainConfig(openRestyConfig)
	supportFiles = append(supportFiles, SupportFile{Path: "pow_config.json", Content: powConfigJSON})
	return &configBundle{
		Routes:            routes,
		SnapshotRoutes:    snapshotRoutes,
		OpenRestyConfig:   openRestyConfig,
		SnapshotJSON:      string(snapshotJSON),
		MainConfig:        mainConfig,
		RouteConfig:       routeConfig,
		SupportFiles:      supportFiles,
		Checksum:          checksumBundle(mainConfig, routeConfig, supportFiles),
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
		upstreams, err := decodeStoredUpstreams(route.Upstreams, route.OriginURL)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 上游配置无效", route.Domain)
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
			SiteName:           normalizeProxyRouteSiteNameInput(route, route.SiteName, domains[0]),
			Domain:             domains[0],
			Domains:            domains,
			OriginURL:          route.OriginURL,
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
			Remark:             route.Remark,
		})
	}
	return items, nil
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
	if left.SiteName != right.SiteName || left.Domain != right.Domain || left.OriginURL != right.OriginURL || left.OriginHost != right.OriginHost || left.EnableHTTPS != right.EnableHTTPS || left.RedirectHTTP != right.RedirectHTTP || left.LimitConnPerServer != right.LimitConnPerServer || left.LimitConnPerIP != right.LimitConnPerIP || left.LimitRate != right.LimitRate || left.CacheEnabled != right.CacheEnabled || left.CachePolicy != right.CachePolicy || left.PoWEnabled != right.PoWEnabled || !uintSliceEqual(left.CertIDs, right.CertIDs) || !uintSliceEqual(left.DomainCertIDs, right.DomainCertIDs) {
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
		CacheEnabled:             common.OpenRestyCacheEnabled,
		CachePath:                common.OpenRestyCachePath,
		CacheLevels:              common.OpenRestyCacheLevels,
		CacheInactive:            common.OpenRestyCacheInactive,
		CacheMaxSize:             common.OpenRestyCacheMaxSize,
		CacheKeyTemplate:         common.OpenRestyCacheKeyTemplate,
		CacheLockEnabled:         common.OpenRestyCacheLockEnabled,
		CacheLockTimeout:         common.OpenRestyCacheLockTimeout,
		CacheUseStale:            common.OpenRestyCacheUseStale,
	}
}

func diffOpenRestyOptionKeys(left openRestyConfigSnapshot, right openRestyConfigSnapshot) []string {
	details := diffOpenRestyOptionDetails(left, right)
	return extractOptionDiffKeys(details)
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

func renderRouteConfig(routes []*model.ProxyRoute, cfg openRestyConfigSnapshot) (string, []SupportFile, error) {
	var builder strings.Builder
	builder.WriteString("# This file is generated by OpenFlare. Do not edit manually.\n")
	supportFiles := make([]SupportFile, 0)
	for _, route := range routes {
		domains, err := decodeStoredDomains(route.Domains, route.Domain)
		if err != nil {
			return "", nil, fmt.Errorf("route %s domains are invalid", route.Domain)
		}
		serverNames := renderServerNames(domains)
		displayName := route.SiteName
		if strings.TrimSpace(displayName) == "" {
			displayName = domains[0]
		}
		customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
		if err != nil {
			return "", nil, fmt.Errorf("路由 %s 自定义请求头无效", route.Domain)
		}
		upstreams, err := decodeStoredUpstreams(route.Upstreams, route.OriginURL)
		if err != nil {
			return "", nil, fmt.Errorf("路由 %s 上游配置无效", route.Domain)
		}
		cacheRules, err := decodeStoredCacheRules(route.CacheRules)
		if err != nil {
			return "", nil, fmt.Errorf("路由 %s 缓存规则无效", route.Domain)
		}
		cacheConfig := routeCacheConfig{
			Enabled: route.CacheEnabled,
			Policy:  route.CachePolicy,
			Rules:   cacheRules,
		}
		limitConfig := routeLimitConfig{
			LimitConnPerServer: route.LimitConnPerServer,
			LimitConnPerIP:     route.LimitConnPerIP,
			LimitRate:          route.LimitRate,
		}
		upstreamConfig := buildRouteUpstreamConfig(route, upstreams)
		if upstreamConfig.UsesNamedUpstream {
			builder.WriteString(renderNamedUpstreamBlock(upstreamConfig))
		}
		if !route.EnableHTTPS {
			builder.WriteString(renderHTTPProxyServer(serverNames, route.OriginURL, route.OriginHost, customHeaders, cacheConfig, limitConfig, upstreamConfig, route.PoWEnabled, cfg))
			continue
		}
		certIDs, err := decodeStoredCertIDs(route.CertIDs, route.CertID)
		if err != nil {
			return "", nil, fmt.Errorf("route %s cert_ids are invalid: %w", route.Domain, err)
		}
		domainCertIDs, err := resolveProxyRouteDomainCertIDs(route, domains, certIDs)
		if err != nil {
			return "", nil, fmt.Errorf("route %s domain_cert_ids are invalid: %w", route.Domain, err)
		}
		if route.CertID == nil || *route.CertID == 0 {
			return "", nil, fmt.Errorf("路由 %s 未配置证书", route.Domain)
		}
		if len(certIDs) == 0 {
			return "", nil, fmt.Errorf("路由 %s 未配置证书", route.Domain)
		}
		certificates, err := loadTLSCertificates(certIDs)
		if err != nil {
			return "", nil, fmt.Errorf("route %s certificate lookup failed: %w", route.Domain, err)
		}
		certificateByID := make(map[uint]*model.TLSCertificate, len(certificates))
		for _, certificate := range certificates {
			if certificate == nil {
				continue
			}
			certificateByID[certificate.ID] = certificate
			supportFiles = append(supportFiles,
				SupportFile{Path: certificateCertFileName(certificate.ID), Content: normalizePEM(certificate.CertPEM)},
				SupportFile{Path: certificateKeyFileName(certificate.ID), Content: normalizePEM(certificate.KeyPEM)},
			)
		}

		httpOnlyDomains := make([]string, 0, len(domains))
		domainsByCertID := make(map[uint][]string, len(certIDs))
		for index, domain := range domains {
			if index >= len(domainCertIDs) || domainCertIDs[index] == 0 {
				httpOnlyDomains = append(httpOnlyDomains, domain)
				continue
			}
			domainsByCertID[domainCertIDs[index]] = append(
				domainsByCertID[domainCertIDs[index]],
				domain,
			)
		}
		for _, certID := range certIDs {
			assignedDomains := domainsByCertID[certID]
			if len(assignedDomains) == 0 {
				continue
			}
			certificate := certificateByID[certID]
			if certificate == nil {
				return "", nil, fmt.Errorf("route %s certificate %d does not exist", route.Domain, certID)
			}
			if err := validateCertificateCoverage(certificate, assignedDomains); err != nil {
				return "", nil, fmt.Errorf("site %s certificate validation failed: %w", displayName, err)
			}
		}

		if route.RedirectHTTP {
			if len(httpOnlyDomains) > 0 {
				builder.WriteString(renderHTTPProxyServer(renderServerNames(httpOnlyDomains), route.OriginURL, route.OriginHost, customHeaders, cacheConfig, limitConfig, upstreamConfig, route.PoWEnabled, cfg))
			}
			for _, certID := range certIDs {
				assignedDomains := domainsByCertID[certID]
				if len(assignedDomains) == 0 {
					continue
				}
				builder.WriteString(renderHTTPRedirectServer(renderServerNames(assignedDomains)))
			}
		} else {
			builder.WriteString(renderHTTPProxyServer(serverNames, route.OriginURL, route.OriginHost, customHeaders, cacheConfig, limitConfig, upstreamConfig, route.PoWEnabled, cfg))
		}
		for _, certID := range certIDs {
			assignedDomains := domainsByCertID[certID]
			if len(assignedDomains) == 0 {
				continue
			}
			builder.WriteString(renderHTTPSServer(renderServerNames(assignedDomains), route.OriginURL, route.OriginHost, certID, customHeaders, cacheConfig, limitConfig, upstreamConfig, route.PoWEnabled, cfg))
		}
	}
	return builder.String(), dedupeSupportFiles(supportFiles), nil
}

func renderMainConfig(cfg openRestyConfigSnapshot) string {
	templateText := common.OpenRestyMainConfigTemplate
	if strings.TrimSpace(templateText) == "" {
		templateText = defaultOpenRestyMainConfigTemplate()
	}
	return renderMainConfigTemplate(templateText, cfg)
}

func ValidateOpenRestyMainConfigTemplate(templateText string) error {
	trimmed := strings.TrimSpace(templateText)
	if trimmed == "" {
		return errors.New("OpenRestyMainConfigTemplate 不能为空")
	}
	for _, placeholder := range requiredMainConfigTemplatePlaceholders {
		if !strings.Contains(trimmed, placeholder) {
			return fmt.Errorf("OpenRestyMainConfigTemplate 必须保留占位符 %s", placeholder)
		}
	}
	return nil
}

func defaultOpenRestyMainConfigTemplate() string {
	return common.OpenRestyMainConfigTemplate
}

func renderMainConfigTemplate(templateText string, cfg openRestyConfigSnapshot) string {
	replacer := strings.NewReplacer(
		"{{OpenRestyWorkerProcesses}}", cfg.WorkerProcesses,
		"{{OpenRestyWorkerConnections}}", fmt.Sprintf("%d", cfg.WorkerConnections),
		"{{OpenRestyWorkerRlimitNofile}}", fmt.Sprintf("%d", cfg.WorkerRlimitNofile),
		"{{OpenRestyConnectionUpgradeMap}}", renderConnectionUpgradeMap(),
		"{{OpenRestyDefaultServerBlock}}", renderDefaultServerBlock(),
		"{{OpenRestyAccessLogPath}}", nginxAccessLogPlaceholder,
		"{{OpenRestyEventsUseDirective}}", renderTemplateDirective(cfg.EventsUse != "", fmt.Sprintf("use %s;", cfg.EventsUse)),
		"{{OpenRestyEventsMultiAcceptDirective}}", renderTemplateDirective(cfg.EventsMultiAcceptEnabled, "multi_accept on;"),
		"{{OpenRestyKeepaliveTimeout}}", fmt.Sprintf("%d", cfg.KeepaliveTimeout),
		"{{OpenRestyKeepaliveRequests}}", fmt.Sprintf("%d", cfg.KeepaliveRequests),
		"{{OpenRestyClientHeaderTimeout}}", fmt.Sprintf("%d", cfg.ClientHeaderTimeout),
		"{{OpenRestyClientBodyTimeout}}", fmt.Sprintf("%d", cfg.ClientBodyTimeout),
		"{{OpenRestyClientMaxBodySize}}", cfg.ClientMaxBodySize,
		"{{OpenRestyLargeClientHeaderBuffers}}", cfg.LargeClientHeaderBuffers,
		"{{OpenRestySendTimeout}}", fmt.Sprintf("%d", cfg.SendTimeout),
		"{{OpenRestyProxyConnectTimeout}}", fmt.Sprintf("%d", cfg.ProxyConnectTimeout),
		"{{OpenRestyProxySendTimeout}}", fmt.Sprintf("%d", cfg.ProxySendTimeout),
		"{{OpenRestyProxyReadTimeout}}", fmt.Sprintf("%d", cfg.ProxyReadTimeout),
		"{{OpenRestyProxyRequestBuffering}}", onOff(cfg.ProxyRequestBuffering),
		"{{OpenRestyProxyBuffering}}", onOff(cfg.ProxyBufferingEnabled),
		"{{OpenRestyProxyBuffers}}", cfg.ProxyBuffers,
		"{{OpenRestyProxyBufferSize}}", cfg.ProxyBufferSize,
		"{{OpenRestyProxyBusyBuffersSize}}", cfg.ProxyBusyBuffersSize,
		"{{OpenRestyGzip}}", onOff(cfg.GzipEnabled),
		"{{OpenRestyGzipMinLength}}", fmt.Sprintf("%d", cfg.GzipMinLength),
		"{{OpenRestyGzipCompLevel}}", fmt.Sprintf("%d", cfg.GzipCompLevel),
		"{{OpenRestyResolverDirective}}", "",
		"{{OpenRestyCacheBlock}}", renderOpenRestyCacheTemplateBlock(cfg),
		"{{OpenRestyRouteConfigInclude}}", nginxRouteConfigPlaceholder,
	)
	return replacer.Replace(templateText)
}

func renderTemplateDirective(enabled bool, statement string) string {
	if !enabled {
		return ""
	}
	return fmt.Sprintf("    %s\n", statement)
}

func renderOpenRestyCacheTemplateBlock(cfg openRestyConfigSnapshot) string {
	lines := make([]string, 0, 12)
	lines = append(lines, renderOpenRestyLimitZoneBlock())
	if !cfg.CacheEnabled {
		lines = append(lines, renderOpenRestyObservabilityTemplateBlock())
		return strings.Join(lines, "")
	}
	lines = append(lines, strings.Join([]string{
		fmt.Sprintf("    proxy_cache_path %s levels=%s keys_zone=openflare_cache:10m inactive=%s max_size=%s;", cfg.CachePath, cfg.CacheLevels, cfg.CacheInactive, cfg.CacheMaxSize),
		fmt.Sprintf("    proxy_cache_key \"%s\";", cfg.CacheKeyTemplate),
		fmt.Sprintf("    proxy_cache_lock %s;", onOff(cfg.CacheLockEnabled)),
		fmt.Sprintf("    proxy_cache_lock_timeout %s;", cfg.CacheLockTimeout),
		fmt.Sprintf("    proxy_cache_use_stale %s;", cfg.CacheUseStale),
		"",
	}, "\n"))
	lines = append(lines, renderOpenRestyObservabilityTemplateBlock())
	return strings.Join(lines, "")
}

func renderOpenRestyLimitZoneBlock() string {
	return strings.Join([]string{
		"    limit_conn_zone $server_name zone=openflare_conn_per_server:10m;",
		"    limit_conn_zone $binary_remote_addr zone=openflare_conn_per_ip:10m;",
		"",
	}, "\n")
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

const nginxPowStaticDirPlaceholder = "__OPENFLARE_POW_STATIC_DIR__"

func renderPowAccessBlock(powEnabled bool) string {
	if !powEnabled {
		return ""
	}
	return fmt.Sprintf("        access_by_lua_file %s/pow/check.lua;\n", nginxLuaDirPlaceholder)
}

func renderPowLocationBlocks(powEnabled bool) string {
	if !powEnabled {
		return ""
	}
	return fmt.Sprintf("\n    location = %spass-challenge {\n        content_by_lua_file %s/pow/verify.lua;\n    }\n\n    location = %smake-challenge {\n        content_by_lua_file %s/pow/challenge.lua;\n    }\n\n", anubisAPIPrefix, nginxLuaDirPlaceholder, anubisAPIPrefix, nginxLuaDirPlaceholder)
}

func renderPowStaticLocationBlock(powEnabled bool) string {
	if !powEnabled {
		return ""
	}
	return fmt.Sprintf("    location %s {\n        alias %s/;\n        types {\n            text/css css;\n            application/javascript js mjs;\n            application/json json;\n            image/webp webp;\n            font/woff2 woff2;\n        }\n    }\n\n", anubisStaticPrefix, nginxPowStaticDirPlaceholder)
}

const anubisStaticPrefix = "/.within.website/x/cmd/anubis/static/"
const anubisAPIPrefix = "/.within.website/x/cmd/anubis/api/"

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

func uintPointerEqual(left *uint, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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

func checksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func checksumBundle(mainConfig string, routeConfig string, supportFiles []SupportFile) string {
	var builder strings.Builder
	builder.WriteString(mainConfig)
	builder.WriteString("\n--route-config--\n")
	builder.WriteString(routeConfig)
	builder.WriteString("\n--support-files--\n")
	files := dedupeSupportFiles(supportFiles)
	sort.Slice(files, func(i int, j int) bool {
		return files[i].Path < files[j].Path
	})
	for _, file := range files {
		builder.WriteString(file.Path)
		builder.WriteString("\n")
		builder.WriteString(file.Content)
		builder.WriteString("\n")
	}
	return checksum(builder.String())
}

func nextVersionNumber(now time.Time) (string, error) {
	prefix := now.Format("20060102")
	var count int64
	if err := model.DB.Model(&model.ConfigVersion{}).Where("version LIKE ?", prefix+"-%").Count(&count).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%03d", prefix, count+1), nil
}

func renderHTTPProxyServer(serverNames string, originURL string, originHost string, customHeaders []ProxyRouteCustomHeaderInput, cacheConfig routeCacheConfig, limitConfig routeLimitConfig, upstreamConfig routeUpstreamConfig, powEnabled bool, cfg openRestyConfigSnapshot) string {
	return fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n%s    location / {\n%s%s%s%s%s    }\n%s}\n\n", serverNames, renderPowLocationBlocks(powEnabled), renderPowAccessBlock(powEnabled), renderProxyHeaderBlock(originURL, originHost, customHeaders, upstreamConfig), renderRouteLimitBlock(limitConfig), renderRouteCacheBlock(cacheConfig, cfg), renderProxyPassBlock(originURL, upstreamConfig), renderPowStaticLocationBlock(powEnabled))
}

func renderHTTPRedirectServer(serverNames string) string {
	return fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n\n    return 301 https://$host$request_uri;\n}\n\n", serverNames)
}

func renderHTTPSServer(serverNames string, originURL string, originHost string, certificateID uint, customHeaders []ProxyRouteCustomHeaderInput, cacheConfig routeCacheConfig, limitConfig routeLimitConfig, upstreamConfig routeUpstreamConfig, powEnabled bool, cfg openRestyConfigSnapshot) string {
	certPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateCertFileName(certificateID))
	keyPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateKeyFileName(certificateID))
	return fmt.Sprintf("server {\n    listen 443 ssl;\n    http2 on;\n    server_name %s;\n    ssl_certificate %s;\n    ssl_certificate_key %s;\n%s    location / {\n%s%s%s%s%s    }\n%s}\n\n", serverNames, certPath, keyPath, renderPowLocationBlocks(powEnabled), renderPowAccessBlock(powEnabled), renderProxyHeaderBlock(originURL, originHost, customHeaders, upstreamConfig), renderRouteLimitBlock(limitConfig), renderRouteCacheBlock(cacheConfig, cfg), renderProxyPassBlock(originURL, upstreamConfig), renderPowStaticLocationBlock(powEnabled))
}

func renderHTTPSServerWithCertificates(serverNames string, originURL string, originHost string, certificateIDs []uint, customHeaders []ProxyRouteCustomHeaderInput, cacheConfig routeCacheConfig, limitConfig routeLimitConfig, upstreamConfig routeUpstreamConfig, cfg openRestyConfigSnapshot) string {
	var certificateBlock strings.Builder
	for _, certificateID := range certificateIDs {
		certPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateCertFileName(certificateID))
		keyPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateKeyFileName(certificateID))
		certificateBlock.WriteString(fmt.Sprintf("    ssl_certificate %s;\n", certPath))
		certificateBlock.WriteString(fmt.Sprintf("    ssl_certificate_key %s;\n", keyPath))
	}
	return fmt.Sprintf("server {\n    listen 443 ssl;\n    http2 on;\n    server_name %s;\n%s\n    location / {\n%s%s%s%s    }\n}\n\n", serverNames, certificateBlock.String(), renderProxyHeaderBlock(originURL, originHost, customHeaders, upstreamConfig), renderRouteLimitBlock(limitConfig), renderRouteCacheBlock(cacheConfig, cfg), renderProxyPassBlock(originURL, upstreamConfig))
}

func renderServerNames(domains []string) string {
	return strings.Join(domains, " ")
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

func validateCertificateCoverageSet(certificates []*model.TLSCertificate, domains []string) error {
	if len(certificates) == 0 {
		return errors.New("certificate set is empty")
	}
	leaves := make([]interface{ VerifyHostname(string) error }, 0, len(certificates))
	for _, certificate := range certificates {
		if certificate == nil {
			return errors.New("certificate is nil")
		}
		leaf, err := parseLeafCertificate(certificate.CertPEM)
		if err != nil {
			return err
		}
		leaves = append(leaves, leaf)
	}
	for _, domain := range domains {
		covered := false
		for _, leaf := range leaves {
			if leaf.VerifyHostname(domain) == nil {
				covered = true
				break
			}
		}
		if !covered {
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

func renderConnectionUpgradeMap() string {
	return "    map $http_upgrade $connection_upgrade {\n        default upgrade;\n        ''      \"\";\n    }\n\n"
}

func renderDefaultServerBlock() string {
	return strings.Join([]string{
		"    server {",
		"        listen 80 default_server;",
		"        server_name _;",
		"",
		"        return 404;",
		"    }",
		"",
		"    server {",
		"        listen 443 ssl default_server;",
		"        server_name _;",
		"",
		"        ssl_reject_handshake on;",
		"    }",
		"",
	}, "\n")
}

func renderProxyHeaderBlock(originURL string, originHost string, customHeaders []ProxyRouteCustomHeaderInput, upstreamConfig routeUpstreamConfig) string {
	var builder strings.Builder
	if strings.TrimSpace(originHost) != "" {
		builder.WriteString(fmt.Sprintf("        proxy_set_header Host %s;\n", quoteNginxHeaderValue(originHost)))
	} else {
		builder.WriteString("        proxy_set_header Host $host;\n")
	}
	if upstreamServerName := resolveUpstreamServerName(originURL, originHost); upstreamServerName != "" {
		builder.WriteString("        proxy_ssl_server_name on;\n")
		builder.WriteString(fmt.Sprintf("        proxy_ssl_name %s;\n", quoteNginxHeaderValue(upstreamServerName)))
	}
	builder.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
	if common.OpenRestyWebsocketEnabled {
		builder.WriteString("        proxy_http_version 1.1;\n")
		builder.WriteString("        proxy_set_header Connection $connection_upgrade;\n")
		builder.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
	} else if upstreamConfig.UsesNamedUpstream {
		builder.WriteString("        proxy_http_version 1.1;\n")
		builder.WriteString("        proxy_set_header Connection \"\";\n")
	}
	for _, header := range customHeaders {
		builder.WriteString(fmt.Sprintf("        proxy_set_header %s %s;\n", header.Key, quoteNginxHeaderValue(header.Value)))
	}
	return builder.String()
}

func renderRouteCacheBlock(cacheConfig routeCacheConfig, cfg openRestyConfigSnapshot) string {
	if !cfg.CacheEnabled || !cacheConfig.Enabled {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("        set $openflare_skip_cache 0;\n")
	builder.WriteString("        if ($request_method != GET) {\n            set $openflare_skip_cache 1;\n        }\n")
	builder.WriteString("        if ($http_authorization != \"\") {\n            set $openflare_skip_cache 1;\n        }\n")
	builder.WriteString("        if ($http_cookie ~* \"(session|sess|token|auth|jwt|logged_in|remember|laravel_session|connect\\\\.sid|_session)\") {\n            set $openflare_skip_cache 1;\n        }\n")
	builder.WriteString("        if ($http_cache_control ~* \"(no-cache|no-store|private)\") {\n            set $openflare_skip_cache 1;\n        }\n")
	if policyCondition := renderRouteCachePolicyCondition(cacheConfig); policyCondition != "" {
		builder.WriteString(policyCondition)
	}
	builder.WriteString("        proxy_cache openflare_cache;\n")
	builder.WriteString("        proxy_cache_methods GET;\n")
	builder.WriteString("        proxy_cache_bypass $openflare_skip_cache;\n")
	builder.WriteString("        proxy_no_cache $openflare_skip_cache;\n")
	return builder.String()
}

func renderRouteLimitBlock(limitConfig routeLimitConfig) string {
	if limitConfig.LimitConnPerServer <= 0 && limitConfig.LimitConnPerIP <= 0 && strings.TrimSpace(limitConfig.LimitRate) == "" {
		return ""
	}
	var builder strings.Builder
	if limitConfig.LimitConnPerServer > 0 {
		builder.WriteString(fmt.Sprintf("        limit_conn openflare_conn_per_server %d;\n", limitConfig.LimitConnPerServer))
	}
	if limitConfig.LimitConnPerIP > 0 {
		builder.WriteString(fmt.Sprintf("        limit_conn openflare_conn_per_ip %d;\n", limitConfig.LimitConnPerIP))
	}
	if strings.TrimSpace(limitConfig.LimitRate) != "" {
		builder.WriteString(fmt.Sprintf("        limit_rate %s;\n", limitConfig.LimitRate))
	}
	return builder.String()
}

func renderRouteCachePolicyCondition(cacheConfig routeCacheConfig) string {
	switch cacheConfig.Policy {
	case proxyRouteCachePolicySuffix:
		return fmt.Sprintf("        if ($uri !~* %s) {\n            set $openflare_skip_cache 1;\n        }\n", quoteNginxStringLiteral(buildSuffixMatchPattern(cacheConfig.Rules)))
	case proxyRouteCachePolicyPathPrefix:
		return fmt.Sprintf("        if ($uri !~ %s) {\n            set $openflare_skip_cache 1;\n        }\n", quoteNginxStringLiteral(buildPathPrefixMatchPattern(cacheConfig.Rules)))
	case proxyRouteCachePolicyPathExact:
		return fmt.Sprintf("        if ($uri !~ %s) {\n            set $openflare_skip_cache 1;\n        }\n", quoteNginxStringLiteral(buildPathExactMatchPattern(cacheConfig.Rules)))
	default:
		return ""
	}
}

func buildSuffixMatchPattern(rules []string) string {
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		parts = append(parts, regexp.QuoteMeta(rule))
	}
	return fmt.Sprintf("\\.(?:%s)$", strings.Join(parts, "|"))
}

func buildPathPrefixMatchPattern(rules []string) string {
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		trimmed := strings.TrimRight(rule, "/")
		if trimmed == "" {
			trimmed = "/"
		}
		if trimmed == "/" {
			parts = append(parts, "/")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s(?:/|$)", regexp.QuoteMeta(trimmed)))
	}
	return fmt.Sprintf("^(?:%s)", strings.Join(parts, "|"))
}

func buildPathExactMatchPattern(rules []string) string {
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		parts = append(parts, regexp.QuoteMeta(rule))
	}
	return fmt.Sprintf("^(?:%s)$", strings.Join(parts, "|"))
}

func renderProxyPassBlock(originURL string, upstreamConfig routeUpstreamConfig) string {
	parsed, err := url.Parse(originURL)
	if err != nil || parsed.Host == "" || parsed.Scheme == "" {
		return fmt.Sprintf("        proxy_pass %s;\n", originURL)
	}
	if upstreamConfig.UsesNamedUpstream {
		return fmt.Sprintf("        proxy_pass %s://%s%s;\n", upstreamConfig.Scheme, upstreamConfig.Name, upstreamConfig.ProxyPassURI)
	}
	return fmt.Sprintf("        proxy_pass %s;\n", originURL)
}

func buildRouteUpstreamConfig(route *model.ProxyRoute, upstreams []string) routeUpstreamConfig {
	if len(upstreams) == 0 {
		return routeUpstreamConfig{}
	}
	if len(upstreams) == 1 {
		parsed, err := url.Parse(strings.TrimSpace(upstreams[0]))
		if err != nil || parsed.Host == "" || parsed.Scheme == "" {
			return routeUpstreamConfig{}
		}
		return routeUpstreamConfig{
			Name:              buildRouteUpstreamName(route),
			Scheme:            parsed.Scheme,
			ProxyPassURI:      buildUpstreamProxyPassURI(parsed),
			Servers:           []string{parsed.Host},
			UsesNamedUpstream: true,
		}
	}
	servers := make([]string, 0, len(upstreams))
	var scheme string
	for _, upstream := range upstreams {
		parsed, err := url.Parse(strings.TrimSpace(upstream))
		if err != nil || parsed.Host == "" || parsed.Scheme == "" {
			return routeUpstreamConfig{}
		}
		if strings.TrimSpace(parsed.EscapedPath()) != "" && strings.TrimSpace(parsed.EscapedPath()) != "/" {
			return routeUpstreamConfig{}
		}
		if parsed.RawQuery != "" {
			return routeUpstreamConfig{}
		}
		if scheme == "" {
			scheme = parsed.Scheme
		} else if scheme != parsed.Scheme {
			return routeUpstreamConfig{}
		}
		servers = append(servers, parsed.Host)
	}
	return routeUpstreamConfig{
		Name:              buildRouteUpstreamName(route),
		Scheme:            scheme,
		Servers:           servers,
		UsesNamedUpstream: true,
	}
}

func buildUpstreamProxyPassURI(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	path := parsed.EscapedPath()
	if path == "/" {
		path = ""
	}
	if parsed.RawQuery == "" {
		return path
	}
	return fmt.Sprintf("%s?%s", path, parsed.RawQuery)
}

func buildRouteUpstreamName(route *model.ProxyRoute) string {
	identity := strings.TrimSpace(route.SiteName)
	if identity == "" {
		identity = route.Domain
	}
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, identity)
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		sanitized = "backend"
	}
	return fmt.Sprintf("backend_%s_%d", sanitized, route.ID)
}

func renderNamedUpstreamBlock(upstreamConfig routeUpstreamConfig) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("upstream %s {\n", upstreamConfig.Name))
	for _, server := range upstreamConfig.Servers {
		builder.WriteString(fmt.Sprintf("    server %s max_fails=3 fail_timeout=10s;\n", server))
	}
	builder.WriteString("    keepalive 128;\n}\n\n")
	return builder.String()
}

func resolveUpstreamServerName(originURL string, originHost string) string {
	parsed, err := url.Parse(originURL)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return ""
	}
	if strings.TrimSpace(originHost) != "" {
		parsedHost, err := url.Parse("//" + originHost)
		if err == nil && parsedHost.Hostname() != "" {
			return parsedHost.Hostname()
		}
		return originHost
	}
	return parsed.Hostname()
}

func quoteNginxHeaderValue(value string) string {
	return quoteNginxStringLiteral(value)
}

func quoteNginxStringLiteral(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return fmt.Sprintf(`"%s"`, escaped)
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

func renderPowConfigBundle(routes []*model.ProxyRoute) (string, []SupportFile, error) {
	type domainEntry struct {
		Domains []string               `json:"domains"`
		Enabled bool                   `json:"enabled"`
		Config  map[string]interface{} `json:"config"`
	}
	entries := make([]domainEntry, 0)
	hasPow := false
	for _, route := range routes {
		if !route.PoWEnabled {
			continue
		}
		hasPow = true
		domains, err := decodeStoredDomains(route.Domains, route.Domain)
		if err != nil {
			return "", nil, err
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(route.PoWConfig), &cfg); err != nil {
			return "", nil, fmt.Errorf("route %s pow_config is invalid", route.Domain)
		}
		entries = append(entries, domainEntry{
			Domains: domains,
			Enabled: true,
			Config:  cfg,
		})
	}
	if !hasPow {
		return "{}", nil, nil
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", nil, err
	}
	return string(data), nil, nil
}
