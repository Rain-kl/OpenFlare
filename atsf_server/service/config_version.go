package service

import (
	"atsflare/common"
	"atsflare/model"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
}

type ConfigDiffResult struct {
	ActiveVersion        string                 `json:"active_version,omitempty"`
	AddedDomains         []string               `json:"added_domains"`
	RemovedDomains       []string               `json:"removed_domains"`
	ModifiedDomains      []string               `json:"modified_domains"`
	MainConfigChanged    bool                   `json:"main_config_changed"`
	ChangedOptionKeys    []string               `json:"changed_option_keys"`
	ChangedOptionDetails []ConfigOptionDiffItem `json:"changed_option_details"`
}

type ConfigOptionDiffItem struct {
	Key           string `json:"key"`
	PreviousValue string `json:"previous_value"`
	CurrentValue  string `json:"current_value"`
}

type snapshotRoute struct {
	Domain        string                        `json:"domain"`
	OriginURL     string                        `json:"origin_url"`
	Enabled       bool                          `json:"enabled"`
	EnableHTTPS   bool                          `json:"enable_https"`
	CertID        *uint                         `json:"cert_id,omitempty"`
	RedirectHTTP  bool                          `json:"redirect_http"`
	CustomHeaders []ProxyRouteCustomHeaderInput `json:"custom_headers,omitempty"`
	Remark        string                        `json:"remark,omitempty"`
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
	nginxCertDirPlaceholder             = "__ATSF_CERT_DIR__"
	nginxRouteConfigPlaceholder         = "__ATSF_ROUTE_CONFIG__"
	nginxAccessLogPlaceholder           = "__ATSF_ACCESS_LOG__"
	nginxLuaDirPlaceholder              = "__ATSF_LUA_DIR__"
	nginxObservabilityListenPlaceholder = "__ATSF_OBSERVABILITY_LISTEN__"
	nginxObservabilityPortPlaceholder   = "__ATSF_OBSERVABILITY_PORT__"
)

var requiredMainConfigTemplatePlaceholders = []string{
	"{{OpenRestyWorkerProcesses}}",
	"{{OpenRestyWorkerConnections}}",
	"{{OpenRestyWorkerRlimitNofile}}",
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

func ListConfigVersions() ([]*model.ConfigVersion, error) {
	return model.ListConfigVersions()
}

func GetActiveConfigVersion() (*model.ConfigVersion, error) {
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
	}, nil
}

func DiffConfigVersion() (*ConfigDiffResult, error) {
	bundle, err := buildCurrentConfigBundle(false)
	if err != nil {
		return nil, err
	}
	result := &ConfigDiffResult{
		AddedDomains:         []string{},
		RemovedDomains:       []string{},
		ModifiedDomains:      []string{},
		ChangedOptionKeys:    []string{},
		ChangedOptionDetails: []ConfigOptionDiffItem{},
	}
	activeVersion, err := model.GetActiveConfigVersion()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			for _, route := range bundle.SnapshotRoutes {
				result.AddedDomains = append(result.AddedDomains, route.Domain)
			}
			result.MainConfigChanged = true
			result.ChangedOptionKeys = openRestyOptionKeys()
			result.ChangedOptionDetails = buildInitialOpenRestyOptionDiffs(bundle.OpenRestyConfig)
			return result, nil
		}
		return nil, err
	}
	result.ActiveVersion = activeVersion.Version
	activeSnapshot, err := parseSnapshotDocument(activeVersion.SnapshotJSON)
	if err != nil {
		return nil, err
	}
	currentMap := make(map[string]snapshotRoute, len(bundle.SnapshotRoutes))
	for _, route := range bundle.SnapshotRoutes {
		currentMap[route.Domain] = route
	}
	activeMap := make(map[string]snapshotRoute, len(activeSnapshot.Routes))
	for _, route := range activeSnapshot.Routes {
		activeMap[route.Domain] = route
	}
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
	routeConfig, supportFiles, err := renderRouteConfig(routes)
	if err != nil {
		return nil, err
	}
	mainConfig := renderMainConfig(openRestyConfig)
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
		customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
		if err != nil {
			return nil, fmt.Errorf("路由 %s 自定义请求头无效", route.Domain)
		}
		items = append(items, snapshotRoute{
			Domain:        route.Domain,
			OriginURL:     route.OriginURL,
			Enabled:       route.Enabled,
			EnableHTTPS:   route.EnableHTTPS,
			CertID:        route.CertID,
			RedirectHTTP:  route.RedirectHTTP,
			CustomHeaders: customHeaders,
			Remark:        route.Remark,
		})
	}
	return items, nil
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
		normalizedHeaders, err := normalizeCustomHeaders(routes[index].CustomHeaders)
		if err == nil {
			routes[index].CustomHeaders = normalizedHeaders
		}
	}
	return routes
}

func snapshotRouteConfigEqual(left snapshotRoute, right snapshotRoute) bool {
	if left.Domain != right.Domain || left.OriginURL != right.OriginURL || left.EnableHTTPS != right.EnableHTTPS || left.RedirectHTTP != right.RedirectHTTP || !uintPointerEqual(left.CertID, right.CertID) {
		return false
	}
	if len(left.CustomHeaders) != len(right.CustomHeaders) {
		return false
	}
	for index := range left.CustomHeaders {
		if left.CustomHeaders[index] != right.CustomHeaders[index] {
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

func renderRouteConfig(routes []*model.ProxyRoute) (string, []SupportFile, error) {
	var builder strings.Builder
	builder.WriteString("# This file is generated by ATSFlare. Do not edit manually.\n")
	supportFiles := make([]SupportFile, 0)
	for _, route := range routes {
		customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
		if err != nil {
			return "", nil, fmt.Errorf("路由 %s 自定义请求头无效", route.Domain)
		}
		if !route.EnableHTTPS {
			builder.WriteString(renderHTTPProxyServer(route.Domain, route.OriginURL, customHeaders))
			continue
		}
		if route.CertID == nil || *route.CertID == 0 {
			return "", nil, fmt.Errorf("路由 %s 未配置证书", route.Domain)
		}
		certificate, err := model.GetTLSCertificateByID(*route.CertID)
		if err != nil {
			return "", nil, fmt.Errorf("路由 %s 关联证书不存在", route.Domain)
		}
		supportFiles = append(supportFiles,
			SupportFile{Path: certificateCertFileName(certificate.ID), Content: normalizePEM(certificate.CertPEM)},
			SupportFile{Path: certificateKeyFileName(certificate.ID), Content: normalizePEM(certificate.KeyPEM)},
		)
		if route.RedirectHTTP {
			builder.WriteString(renderHTTPRedirectServer(route.Domain))
		} else {
			builder.WriteString(renderHTTPProxyServer(route.Domain, route.OriginURL, customHeaders))
		}
		builder.WriteString(renderHTTPSServer(route.Domain, route.OriginURL, certificate.ID, customHeaders))
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
	lines := make([]string, 0, 8)
	if !cfg.CacheEnabled {
		lines = append(lines, renderOpenRestyObservabilityTemplateBlock())
		return strings.Join(lines, "")
	}
	lines = append(lines, strings.Join([]string{
		fmt.Sprintf("    proxy_cache_path %s levels=%s keys_zone=atsflare_cache:10m inactive=%s max_size=%s;", cfg.CachePath, cfg.CacheLevels, cfg.CacheInactive, cfg.CacheMaxSize),
		fmt.Sprintf("    proxy_cache_key \"%s\";", cfg.CacheKeyTemplate),
		fmt.Sprintf("    proxy_cache_lock %s;", onOff(cfg.CacheLockEnabled)),
		fmt.Sprintf("    proxy_cache_lock_timeout %s;", cfg.CacheLockTimeout),
		fmt.Sprintf("    proxy_cache_use_stale %s;", cfg.CacheUseStale),
		"",
	}, "\n"))
	lines = append(lines, renderOpenRestyObservabilityTemplateBlock())
	return strings.Join(lines, "")
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func uintPointerEqual(left *uint, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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

func renderHTTPProxyServer(domain string, originURL string, customHeaders []ProxyRouteCustomHeaderInput) string {
	return fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n\n    location / {\n%s        proxy_pass %s;\n    }\n}\n\n", domain, renderProxyHeaderBlock(customHeaders), originURL)
}

func renderHTTPRedirectServer(domain string) string {
	return fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n\n    return 301 https://$host$request_uri;\n}\n\n", domain)
}

func renderHTTPSServer(domain string, originURL string, certificateID uint, customHeaders []ProxyRouteCustomHeaderInput) string {
	certPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateCertFileName(certificateID))
	keyPath := fmt.Sprintf("%s/%s", nginxCertDirPlaceholder, certificateKeyFileName(certificateID))
	return fmt.Sprintf("server {\n    listen 443 ssl;\n    server_name %s;\n    ssl_certificate %s;\n    ssl_certificate_key %s;\n\n    location / {\n%s        proxy_pass %s;\n    }\n}\n\n", domain, certPath, keyPath, renderProxyHeaderBlock(customHeaders), originURL)
}

func renderProxyHeaderBlock(customHeaders []ProxyRouteCustomHeaderInput) string {
	var builder strings.Builder
	builder.WriteString("        proxy_set_header Host $host;\n")
	builder.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	builder.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
	if common.OpenRestyWebsocketEnabled {
		builder.WriteString("        proxy_http_version 1.1;\n")
		builder.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
		builder.WriteString("        proxy_set_header Connection $http_connection;\n")
	}
	for _, header := range customHeaders {
		builder.WriteString(fmt.Sprintf("        proxy_set_header %s %s;\n", header.Key, quoteNginxHeaderValue(header.Value)))
	}
	if common.OpenRestyCacheEnabled {
		builder.WriteString("        proxy_cache atsflare_cache;\n")
	}
	return builder.String()
}

func quoteNginxHeaderValue(value string) string {
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
