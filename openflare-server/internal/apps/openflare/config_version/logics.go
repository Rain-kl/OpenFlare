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
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// ConfigPreviewResult is the preview response payload.
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

// ConfigDiffResult is the diff response payload.
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

// ConfigOptionDiffItem describes a changed OpenResty option.
type ConfigOptionDiffItem struct {
	Key           string `json:"key"`
	PreviousValue string `json:"previous_value"`
	CurrentValue  string `json:"current_value"`
}

// CleanupInput is the cleanup request payload.
type CleanupInput struct {
	KeepCount int `json:"keep_count"`
}

// CleanupResult is the cleanup response payload.
type CleanupResult struct {
	DeletedCount int64  `json:"deleted_count"`
	Message      string `json:"message"`
}

// ListConfigVersions returns all config version summaries.
func ListConfigVersions(ctx context.Context) ([]*model.ConfigVersionSummary, error) {
	return model.ListConfigVersionSummaries(ctx)
}

// GetConfigVersionDetail returns a config version by id.
func GetConfigVersionDetail(ctx context.Context, id uint) (*model.ConfigVersion, error) {
	return model.GetConfigVersionByID(ctx, id)
}

// GetActiveConfigVersion returns the active config version.
func GetActiveConfigVersion(ctx context.Context) (*model.ConfigVersion, error) {
	return model.GetActiveConfigVersion(ctx)
}

// PreviewConfigVersion renders the current draft configuration.
func PreviewConfigVersion(ctx context.Context) (*ConfigPreviewResult, error) {
	bundle, err := buildCurrentConfigBundle(ctx, false)
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

// DiffConfigVersion compares the current draft against the active version.
func DiffConfigVersion(ctx context.Context) (*ConfigDiffResult, error) {
	bundle, err := buildCurrentConfigBundle(ctx, false)
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
	activeVersion, err := model.GetActiveConfigVersion(ctx)
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

// PublishConfigVersion publishes the current draft as a new active version.
func PublishConfigVersion(ctx context.Context, createdBy string, force bool) (*model.ConfigVersion, error) {
	bundle, err := buildCurrentConfigBundle(ctx, true)
	if err != nil {
		return nil, err
	}
	if len(bundle.Routes) == 0 {
		return nil, errors.New(errNoEnabledRoutes)
	}
	activeVersion, err := model.GetActiveConfigVersion(ctx)
	if !force && err == nil && activeVersion.Checksum == bundle.Checksum {
		return nil, errors.New(errNoChangesToPublish)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	supportFilesJSON, err := json.Marshal(bundle.SupportFiles)
	if err != nil {
		return nil, err
	}
	version, err := nextVersionNumber(ctx, time.Now())
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
	if err = model.PublishConfigVersionTx(ctx, record); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errVersionConflict)
		}
		return nil, err
	}
	return record, nil
}

// ActivateConfigVersion activates an existing config version.
func ActivateConfigVersion(ctx context.Context, id uint) (*model.ConfigVersion, error) {
	version, err := model.GetConfigVersionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = model.ActivateConfigVersionTx(ctx, id); err != nil {
		return nil, err
	}
	version.IsActive = true
	return version, nil
}

// CleanupConfigVersions removes old inactive config versions.
func CleanupConfigVersions(ctx context.Context, keepCount int) (*CleanupResult, error) {
	if keepCount < 3 {
		keepCount = 3
	}
	versions, err := model.ListConfigVersionSummaries(ctx)
	if err != nil {
		return nil, err
	}
	if len(versions) <= keepCount {
		return &CleanupResult{DeletedCount: 0, Message: "清理成功"}, nil
	}
	var deleteIDs []uint
	for index, version := range versions {
		if index < keepCount {
			continue
		}
		if version.IsActive {
			continue
		}
		deleteIDs = append(deleteIDs, version.ID)
	}
	if len(deleteIDs) == 0 {
		return &CleanupResult{DeletedCount: 0, Message: "清理成功"}, nil
	}
	deletedCount, err := model.DeleteConfigVersionsByIDs(ctx, deleteIDs)
	if err != nil {
		return nil, err
	}
	return &CleanupResult{DeletedCount: deletedCount, Message: "清理成功"}, nil
}

func nextVersionNumber(ctx context.Context, now time.Time) (string, error) {
	prefix := now.Format("20060102")
	latest, err := model.GetLatestConfigVersionByPrefix(ctx, prefix)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Sprintf("%s-%03d", prefix, 1), nil
	}
	if err != nil {
		return "", err
	}
	suffix := strings.TrimPrefix(latest, prefix+"-")
	sequence, err := strconv.Atoi(suffix)
	if err != nil {
		return "", fmt.Errorf("invalid config version sequence %q: %w", latest, err)
	}
	return fmt.Sprintf("%s-%03d", prefix, sequence+1), nil
}

func parseSnapshotDocument(snapshotJSON string) (*snapshotDocument, error) {
	text := strings.TrimSpace(snapshotJSON)
	if text == "" {
		return &snapshotDocument{Routes: []snapshotRoute{}}, nil
	}
	if strings.HasPrefix(text, "[") {
		var routes []snapshotRoute
		if err := json.Unmarshal([]byte(text), &routes); err != nil {
			return nil, errors.New(errInvalidSnapshotFormat)
		}
		return &snapshotDocument{Routes: normalizeSnapshotRoutes(routes)}, nil
	}
	var snapshot snapshotDocument
	if err := json.Unmarshal([]byte(text), &snapshot); err != nil {
		return nil, errors.New(errInvalidSnapshotFormat)
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
			routes[index].SiteName = normalizeProxyRouteSiteName(nil, routes[index].SiteName, normalizedDomains[0])
		}
		normalizedCertIDs, primaryCertID, certErr := normalizeSnapshotCertificateIDs(routes[index].CertID, routes[index].CertIDs)
		if certErr == nil {
			routes[index].CertID = primaryCertID
			routes[index].CertIDs = normalizedCertIDs
		}
		normalizedDomainCertIDs, domainCertErr := resolveDomainCertIDs(routes[index].Domains, routes[index].CertIDs, "")
		if domainCertErr == nil && len(routes[index].DomainCertIDs) == 0 {
			routes[index].DomainCertIDs = normalizedDomainCertIDs
		}
		normalizedUpstreams, upstreamErr := normalizeUpstreams(routes[index].OriginURL, routes[index].Upstreams)
		if upstreamErr == nil {
			routes[index].OriginURL = normalizedUpstreams[0]
			routes[index].Upstreams = normalizedUpstreams
		}
		if !routes[index].BasicAuthEnabled {
			routes[index].BasicAuthUsername = ""
			routes[index].BasicAuthPassword = ""
		}
		routes[index].UpstreamType = normalizeUpstreamType(routes[index].UpstreamType)
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
	if left.SiteName != right.SiteName || left.Domain != right.Domain || left.OriginURL != right.OriginURL ||
		left.OriginHost != right.OriginHost || left.EnableHTTPS != right.EnableHTTPS || left.RedirectHTTP != right.RedirectHTTP ||
		left.LimitConnPerServer != right.LimitConnPerServer || left.LimitConnPerIP != right.LimitConnPerIP ||
		left.LimitRate != right.LimitRate || left.CacheEnabled != right.CacheEnabled || left.CachePolicy != right.CachePolicy ||
		left.BasicAuthEnabled != right.BasicAuthEnabled || left.BasicAuthUsername != right.BasicAuthUsername ||
		left.BasicAuthPassword != right.BasicAuthPassword || left.UpstreamType != right.UpstreamType ||
		!uintPtrEqual(left.TunnelNodeID, right.TunnelNodeID) || left.TunnelTargetAddr != right.TunnelTargetAddr ||
		left.TunnelTargetProto != right.TunnelTargetProto || !uintPtrEqual(left.PagesProjectID, right.PagesProjectID) ||
		!uintSliceEqual(left.CertIDs, right.CertIDs) || !uintSliceEqual(left.DomainCertIDs, right.DomainCertIDs) {
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
	appendIfChanged("OpenRestyDefaultServerReturnStatus", fmt.Sprintf("%d", left.DefaultServerReturnStatus), fmt.Sprintf("%d", right.DefaultServerReturnStatus))
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
	appendIfChanged("OpenRestyHTTP3Enabled", fmt.Sprintf("%t", left.HTTP3Enabled), fmt.Sprintf("%t", right.HTTP3Enabled))
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
		"OpenRestyDefaultServerReturnStatus",
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
		"OpenRestyHTTP3Enabled",
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
