// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

const (
	supportFilesPerCertificate  = 1
	wafIPGroupChecksumHexLength = 64
)

type snapshotRoute struct {
	ID                 uint                        `json:"id,omitempty"`
	SiteName           string                      `json:"site_name,omitempty"`
	Domains            []string                    `json:"domains,omitempty"`
	OriginURL          string                      `json:"origin_url"`
	OriginHost         string                      `json:"origin_host,omitempty"`
	Upstreams          []string                    `json:"upstreams,omitempty"`
	Enabled            bool                        `json:"enabled"`
	EnableHTTPS        bool                        `json:"enable_https"`
	EnableHTTP2        bool                        `json:"enable_http2"`
	DomainCertIDs      []uint                      `json:"domain_cert_ids,omitempty"`
	RedirectHTTP       bool                        `json:"redirect_http"`
	LimitConnPerServer int                         `json:"limit_conn_per_server,omitempty"`
	LimitConnPerIP     int                         `json:"limit_conn_per_ip,omitempty"`
	LimitRate          string                      `json:"limit_rate,omitempty"`
	LimitReqPerIP      string                      `json:"limit_req_per_ip,omitempty"`
	CacheEnabled       bool                        `json:"cache_enabled"`
	CachePolicy        string                      `json:"cache_policy,omitempty"`
	CacheRules         []string                    `json:"cache_rules,omitempty"`
	CustomHeaders      []customHeaderInput         `json:"custom_headers,omitempty"`
	PoWEnabled         bool                        `json:"pow_enabled,omitempty"`
	PoWConfig          *edgeconfig.PoWConfig       `json:"pow_config,omitempty"`
	BasicAuthEnabled   bool                        `json:"basic_auth_enabled,omitempty"`
	BasicAuthUsername  string                      `json:"basic_auth_username,omitempty"`
	BasicAuthPassword  string                      `json:"basic_auth_password,omitempty"`
	SiteAuthTTLHours   int                         `json:"site_auth_ttl_hours,omitempty"`
	SiteAuthKey        string                      `json:"site_auth_key,omitempty"`
	UpstreamType       string                      `json:"upstream_type,omitempty"`
	TunnelNodeID       *uint                       `json:"tunnel_node_id,omitempty"`
	TunnelTargetAddr   string                      `json:"tunnel_target_addr,omitempty"`
	TunnelTargetProto  string                      `json:"tunnel_target_protocol,omitempty"`
	PagesProjectID     *uint                       `json:"pages_project_id,omitempty"`
	PagesDeployment    *edgeconfig.PagesDeployment `json:"pages_deployment,omitempty"`
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
	RuleGroups       []snapshotWAFRuleGroup         `json:"rule_groups"`
	IPGroups         []snapshotWAFIPGroup           `json:"ip_groups,omitempty"`
	Bindings         []snapshotWAFBinding           `json:"bindings"`
	RequestWhitelist []edgeconfig.WAFWhitelistEntry `json:"request_whitelist,omitempty"`
}

type edgeSettingsSnapshot = edgeconfig.ConfigSnapshot

type snapshotDocument struct {
	Routes       []snapshotRoute      `json:"routes"`
	EdgeSettings edgeSettingsSnapshot `json:"edge_config"`
	WAF          snapshotWAFDocument  `json:"waf"`
}

type configBundle struct {
	SnapshotRoutes   []snapshotRoute
	WAFSnapshot      snapshotWAFDocument
	SnapshotJSON     string
	SupportFilesJSON string
	Checksum         string
}

func buildCurrentConfigBundle(ctx context.Context) (*configBundle, error) {
	routes, err := repository.ListEnabledProxyRoutes(ctx)
	if err != nil {
		return nil, err
	}
	snapshotRoutes, err := buildSnapshotRoutes(ctx, routes)
	if err != nil {
		return nil, err
	}
	if err := attachSnapshotPoWPolicies(ctx, snapshotRoutes); err != nil {
		return nil, err
	}
	wafSnapshot, err := buildSnapshotWAFDocument(ctx, snapshotRoutes)
	if err != nil {
		return nil, err
	}
	edgeSettings, err := buildEdgeSettingsSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	snapshotDoc := snapshotDocument{
		Routes:       snapshotRoutes,
		EdgeSettings: edgeSettings,
		WAF:          wafSnapshot,
	}
	snapshotJSON, err := json.Marshal(snapshotDoc)
	if err != nil {
		return nil, err
	}
	certificateFiles, err := buildCertificateSupportFiles(ctx, snapshotRoutes)
	if err != nil {
		return nil, err
	}

	rendered, err := renderSnapshotConfig(string(snapshotJSON), certificateFiles)
	if err != nil {
		return nil, err
	}

	return &configBundle{
		SnapshotRoutes:   snapshotRoutes,
		WAFSnapshot:      wafSnapshot,
		SnapshotJSON:     string(snapshotJSON),
		SupportFilesJSON: rendered.SupportFilesJSON,
		Checksum:         rendered.Checksum,
	}, nil
}

func buildSnapshotRoutes(ctx context.Context, routes []*model.ProxyRoute) ([]snapshotRoute, error) {
	items := make([]snapshotRoute, 0, len(routes))
	var relayURL string
	for _, route := range routes {
		zoneDomains, err := repository.ListZoneDomainsByRouteID(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		if len(zoneDomains) == 0 {
			continue
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
			return nil, fmt.Errorf("invalid custom request headers for route %s", route.SiteName)
		}
		upstreamType := normalizeUpstreamType(route.UpstreamType)
		originURL := route.OriginURL
		upstreams, err := decodeStoredUpstreams(route.Upstreams, route.OriginURL)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream configuration for route %s", route.SiteName)
		}
		var tunnelNodeID *uint
		var tunnelTargetAddr string
		var tunnelTargetProtocol string
		var pagesProjectID *uint
		var pagesDeployment *edgeconfig.PagesDeployment
		switch upstreamType {
		case "tunnel":
			if relayURL == "" {
				relayURL, err = resolveTunnelEdgeUpstreamURL(ctx)
				if err != nil {
					return nil, err
				}
			}
			originURL = relayURL
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
			return nil, fmt.Errorf("invalid cache rules for route %s", route.SiteName)
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
			EnableHTTP2:        route.EnableHTTP2,
			DomainCertIDs:      domainCertIDs,
			RedirectHTTP:       route.RedirectHTTP,
			LimitConnPerServer: route.LimitConnPerServer,
			LimitConnPerIP:     route.LimitConnPerIP,
			LimitRate:          route.LimitRate,
			LimitReqPerIP:      route.LimitReqPerIP,
			CacheEnabled:       route.CacheEnabled,
			CachePolicy:        route.CachePolicy,
			CacheRules:         cacheRules,
			CustomHeaders:      customHeaders,
			BasicAuthEnabled:   route.BasicAuthEnabled,
			BasicAuthUsername:  route.BasicAuthUsername,
			BasicAuthPassword:  route.BasicAuthPassword,
			SiteAuthTTLHours:   route.SiteAuthTTLHours,
			SiteAuthKey:        route.SiteAuthKey,
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

func attachSnapshotPoWPolicies(ctx context.Context, routes []snapshotRoute) error {
	policies, err := repository.ListOpenFlarePoWPolicies(ctx)
	if err != nil {
		return err
	}
	byRoute := make(map[uint]model.OpenFlarePoWPolicy, len(policies))
	for _, policy := range policies {
		byRoute[policy.RouteID] = policy
	}
	for index := range routes {
		policy, ok := byRoute[routes[index].ID]
		if !ok {
			continue
		}
		var config edgeconfig.PoWConfig
		if err := json.Unmarshal([]byte(policy.Config), &config); err != nil {
			return fmt.Errorf("invalid PoW policy for route %d: %w", policy.RouteID, err)
		}
		if err := edgeconfig.ValidatePoWConfig(config); err != nil {
			return fmt.Errorf("invalid PoW policy for route %d: %w", policy.RouteID, err)
		}
		routes[index].PoWEnabled, routes[index].PoWConfig = policy.Enabled, &config
	}
	return nil
}

func buildSnapshotWAFDocument(ctx context.Context, routes []snapshotRoute) (snapshotWAFDocument, error) {
	groups, err := repository.ListOpenFlareWAFRuleGroups(ctx)
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	ruleGroups := make([]snapshotWAFRuleGroup, 0, len(groups))
	graphs := make([]waf.RuntimeRuleGraph, 0, len(groups))
	referencedIPGroupIDs := make(map[uint]struct{})
	enabledRuleIDs := make(map[uint]struct{})
	for _, group := range groups {
		if !group.Enabled {
			continue
		}
		var editorGraph waf.RuleGraph
		if err = json.Unmarshal([]byte(group.Graph), &editorGraph); err != nil {
			return snapshotWAFDocument{}, fmt.Errorf("invalid graph data for WAF rule %s: %w", group.Name, err)
		}
		if err = waf.ValidateRuleGraph(ctx, editorGraph, snapshotWAFIPGroupExists); err != nil {
			return snapshotWAFDocument{}, fmt.Errorf("invalid graph for WAF rule %s: %w", group.Name, err)
		}
		runtimeGraph, compileErr := waf.CompileRuleGraph(editorGraph)
		if compileErr != nil {
			return snapshotWAFDocument{}, fmt.Errorf("failed to compile WAF rule %s: %w", group.Name, compileErr)
		}
		graphs = append(graphs, runtimeGraph)
		ruleGroups = append(ruleGroups, snapshotWAFRuleGroup{
			ID: group.ID, Name: group.Name, Enabled: group.Enabled, IsGlobal: group.IsGlobal, Graph: runtimeGraph,
		})
		enabledRuleIDs[group.ID] = struct{}{}
		for _, id := range waf.ReferencedIPGroupIDs(editorGraph) {
			referencedIPGroupIDs[id] = struct{}{}
		}
	}
	if err = waf.ResolveAttackGroups(ctx, graphs...); err != nil {
		return snapshotWAFDocument{}, err
	}
	ipGroups, err := buildSnapshotWAFIPGroups(ctx, referencedIPGroupIDs)
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	enabledRouteSiteNames := make(map[uint]string, len(routes))
	for _, route := range routes {
		enabledRouteSiteNames[route.ID] = route.SiteName
	}
	rawBindings, err := repository.ListOpenFlareWAFRuleGroupBindings(ctx)
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
	whitelist, err := waf.SnapshotWAFWhitelist(ctx)
	if err != nil {
		return snapshotWAFDocument{}, err
	}
	return snapshotWAFDocument{RuleGroups: ruleGroups, IPGroups: ipGroups, Bindings: bindings, RequestWhitelist: whitelist}, nil
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
	slices.Sort(ids)
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
			return nil, fmt.Errorf("IP group %d was not found", id)
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
	group, err := repository.GetOpenFlareWAFIPGroupByID(ctx, id)
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
		return nil, errors.New("ip_list payload is invalid")
	}
	return items, nil
}

func buildEdgeSettingsSnapshot(ctx context.Context) (edgeSettingsSnapshot, error) {
	values, err := repository.LoadEdgeSnapshotSettings(ctx, []string{
		model.ConfigKeyEdgeDefaultLimitConnPerServer,
		model.ConfigKeyEdgeDefaultLimitConnPerIP,
		model.ConfigKeyEdgeDefaultLimitRate,
		model.ConfigKeyEdgeDefaultLimitReqPerIP,
		model.ConfigKeyEdgeTrustedProxyCIDRs,
		model.ConfigKeyOriginErrorPageEnabled,
		model.ConfigKeyOriginErrorPageStatusCodes,
		model.ConfigKeyOriginErrorPageHTML,
		model.ConfigKeyOriginErrorPageGetOnly,
		model.ConfigKeyPoWPage,
		model.ConfigKeySiteAuthHTML,
		model.ConfigKeySWOfflineEnabled,
		model.ConfigKeySWOfflineHTML,
		model.ConfigKeySWOfflineDomains,
	})
	if err != nil {
		return edgeSettingsSnapshot{}, err
	}
	getNonNegIntConfig := func(key string, defaultVal int) int {
		val, err := strconv.Atoi(values[key])
		if err != nil || val < 0 {
			return defaultVal
		}
		return val
	}

	getBoolConfig := func(key string, defaultVal bool) bool {
		val, err := strconv.ParseBool(values[key])
		if err != nil {
			return defaultVal
		}
		return val
	}

	getStringConfig := func(key string, defaultVal string) string {
		if value, ok := values[key]; ok {
			return value
		}
		return defaultVal
	}

	snapshot := edgeSettingsSnapshot{CacheStaticExtensions: edgeconfig.DefaultStaticCacheExtensions}
	snapshot.TrustedProxyCIDRs = edgeconfig.DefaultTrustedProxyCIDRs()
	if value, ok := values[model.ConfigKeyEdgeTrustedProxyCIDRs]; ok {
		snapshot.TrustedProxyCIDRs = parseTrustedProxyCIDRs(value)
	}
	snapshot.DefaultLimitConnPerServer = getNonNegIntConfig(model.ConfigKeyEdgeDefaultLimitConnPerServer, 0)
	snapshot.DefaultLimitConnPerIP = getNonNegIntConfig(model.ConfigKeyEdgeDefaultLimitConnPerIP, 0)
	snapshot.DefaultLimitRate = strings.ToLower(strings.TrimSpace(getStringConfig(model.ConfigKeyEdgeDefaultLimitRate, "")))
	snapshot.DefaultLimitReqPerIP = strings.ToLower(strings.TrimSpace(getStringConfig(model.ConfigKeyEdgeDefaultLimitReqPerIP, "")))
	snapshot.OriginErrorPageEnabled = getBoolConfig(model.ConfigKeyOriginErrorPageEnabled, true)
	snapshot.OriginErrorPageStatusCodes = parseOriginErrorPageStatusCodes(getStringConfig(model.ConfigKeyOriginErrorPageStatusCodes, `["500-599"]`))
	snapshot.OriginErrorPageHTML = getStringConfig(model.ConfigKeyOriginErrorPageHTML, "")
	snapshot.PoWPage = parsePoWPageConfig(ctx, getStringConfig(model.ConfigKeyPoWPage, "{}"))
	snapshot.SiteAuthHTML = getStringConfig(model.ConfigKeySiteAuthHTML, "")
	snapshot.OriginErrorPageGetOnly = getBoolConfig(model.ConfigKeyOriginErrorPageGetOnly, false)
	snapshot.SWOfflineEnabled = getBoolConfig(model.ConfigKeySWOfflineEnabled, false)
	snapshot.SWOfflineHTML = getStringConfig(model.ConfigKeySWOfflineHTML, "")
	var offlineDomains []string
	if json.Unmarshal([]byte(values[model.ConfigKeySWOfflineDomains]), &offlineDomains) == nil {
		snapshot.SWOfflineDomains = offlineDomains
	}
	if snapshot.DefaultLimitRate == "0" {
		snapshot.DefaultLimitRate = ""
	}
	if snapshot.DefaultLimitReqPerIP == "0" {
		snapshot.DefaultLimitReqPerIP = ""
	}
	return snapshot, nil
}

func parseTrustedProxyCIDRs(value string) []string {
	var cidrs []string
	if err := json.Unmarshal([]byte(value), &cidrs); err != nil || cidrs == nil {
		return []string{}
	}
	return cidrs
}

func parsePoWPageConfig(ctx context.Context, raw string) edgeconfig.PoWPageConfig {
	var cfg edgeconfig.PoWPageConfig
	err := json.Unmarshal([]byte(raw), &cfg)
	if err == nil {
		err = edgeconfig.ValidatePoWPageConfig(cfg)
	}
	if err != nil {
		logger.ErrorF(ctx, "[OpenFlareConfig] decode PoW page configuration failed: error=%v", err)
		return edgeconfig.PoWPageConfig{}
	}
	return cfg
}

func parseOriginErrorPageStatusCodes(raw string) []string {
	const defaultTag = "500-599"
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{defaultTag}
	}
	var tags []string
	if err := json.Unmarshal([]byte(trimmed), &tags); err != nil || len(tags) == 0 {
		return []string{defaultTag}
	}
	return tags
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
	slices.Sort(certIDs)
	files := make([]SupportFile, 0, len(certIDs)*supportFilesPerCertificate)
	for _, certID := range certIDs {
		certificate, err := repository.GetTLSCertificateByID(ctx, certID)
		if err != nil {
			return nil, err
		}
		keyPEM, err := credential.Open(certificate.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("certificate %d private key: %w", certificate.ID, err)
		}
		if strings.TrimSpace(keyPEM) == "" {
			return nil, fmt.Errorf("certificate %d has no private key", certificate.ID)
		}
		certPEM := normalizePEM(certificate.CertPEM)
		if _, err := tls.X509KeyPair([]byte(certPEM), []byte(normalizePEM(keyPEM))); err != nil {
			return nil, fmt.Errorf("certificate %d key pair mismatch: %w", certificate.ID, err)
		}
		files = append(files,
			SupportFile{Path: certificateCertFileName(certificate.ID), Content: certPEM},
		)
	}
	return dedupeSupportFiles(files), nil
}
