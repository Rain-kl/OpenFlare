// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

// CustomHeaderInput 自定义响应头。
type CustomHeaderInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Input 代理规则创建/更新请求。
type Input struct {
	SiteName             string              `json:"site_name"`
	ZoneDomainIDs        []uint              `json:"zone_domain_ids"`
	OriginID             *uint               `json:"origin_id"`
	OriginURL            string              `json:"origin_url"`
	OriginScheme         string              `json:"origin_scheme"`
	OriginAddress        string              `json:"origin_address"`
	OriginPort           string              `json:"origin_port"`
	OriginURI            string              `json:"origin_uri"`
	OriginHost           string              `json:"origin_host"`
	Upstreams            []string            `json:"upstreams"`
	Enabled              bool                `json:"enabled"`
	EnableHTTPS          bool                `json:"enable_https"`
	EnableHTTP2          *bool               `json:"enable_http2"`
	RedirectHTTP         bool                `json:"redirect_http"`
	LimitConnPerServer   int                 `json:"limit_conn_per_server"`
	LimitConnPerIP       int                 `json:"limit_conn_per_ip"`
	LimitRate            string              `json:"limit_rate"`
	LimitReqPerIP        string              `json:"limit_req_per_ip"`
	CacheEnabled         bool                `json:"cache_enabled"`
	CachePolicy          string              `json:"cache_policy"`
	CacheRules           []string            `json:"cache_rules"`
	CustomHeaders        []CustomHeaderInput `json:"custom_headers"`
	BasicAuthEnabled     bool                `json:"basic_auth_enabled"`
	BasicAuthUsername    string              `json:"basic_auth_username"`
	BasicAuthPassword    string              `json:"basic_auth_password"`
	SiteAuthTTLHours     *int                `json:"site_auth_ttl_hours"`
	UpstreamType         string              `json:"upstream_type"`
	TunnelID             *uint               `json:"tunnel_id"`
	TunnelTargetAddr     string              `json:"tunnel_target_addr"`
	TunnelTargetProtocol string              `json:"tunnel_target_protocol"`
	TunnelOptionsJSON    string              `json:"tunnel_options_json"`
	PagesProjectID       *uint               `json:"pages_project_id"`
}

// View 代理规则视图。
type View struct {
	ID                   uint                `json:"id"`
	SiteName             string              `json:"site_name"`
	ZoneDomainIDs        []uint              `json:"zone_domain_ids"`
	ZoneDomains          []ZoneDomainView    `json:"zone_domains"`
	OriginID             *uint               `json:"origin_id"`
	OriginURL            string              `json:"origin_url"`
	OriginHost           string              `json:"origin_host"`
	Upstreams            string              `json:"upstreams"`
	UpstreamList         []string            `json:"upstream_list"`
	Enabled              bool                `json:"enabled"`
	EnableHTTPS          bool                `json:"enable_https"`
	EnableHTTP2          bool                `json:"enable_http2"`
	RedirectHTTP         bool                `json:"redirect_http"`
	LimitConnPerServer   int                 `json:"limit_conn_per_server"`
	LimitConnPerIP       int                 `json:"limit_conn_per_ip"`
	LimitRate            string              `json:"limit_rate"`
	LimitReqPerIP        string              `json:"limit_req_per_ip"`
	CacheEnabled         bool                `json:"cache_enabled"`
	CachePolicy          string              `json:"cache_policy"`
	CacheRules           string              `json:"cache_rules"`
	CacheRuleList        []string            `json:"cache_rule_list"`
	CustomHeaders        string              `json:"custom_headers"`
	CustomHeaderList     []CustomHeaderInput `json:"custom_header_list"`
	BasicAuthEnabled     bool                `json:"basic_auth_enabled"`
	BasicAuthUsername    string              `json:"basic_auth_username"`
	BasicAuthPassword    string              `json:"basic_auth_password"`
	SiteAuthTTLHours     int                 `json:"site_auth_ttl_hours"`
	UpstreamType         string              `json:"upstream_type"`
	TunnelID             *uint               `json:"tunnel_id"`
	TunnelTargetAddr     string              `json:"tunnel_target_addr"`
	TunnelTargetProtocol string              `json:"tunnel_target_protocol"`
	TunnelOptionsJSON    string              `json:"tunnel_options_json"`
	PagesProjectID       *uint               `json:"pages_project_id"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

// ZoneDomainView is the route-safe representation of a bound Zone domain.
type ZoneDomainView struct {
	ID     uint   `json:"id"`
	ZoneID uint   `json:"zone_id"`
	Domain string `json:"domain"`
	CertID *uint  `json:"cert_id"`
}

// ListProxyRoutes 列出全部代理规则。
func ListProxyRoutes(ctx context.Context) ([]*View, error) {
	routes, err := repository.ListProxyRoutesWithDomains(ctx)
	if err != nil {
		return nil, err
	}
	return buildProxyRouteViews(routes)
}

// GetProxyRoute 获取代理规则详情。
func GetProxyRoute(ctx context.Context, id uint) (*View, error) {
	route, err := repository.GetProxyRouteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildProxyRouteView(ctx, route)
}

// CreateProxyRoute 创建代理规则。
func CreateProxyRoute(ctx context.Context, input Input) (*View, error) {
	route, err := buildProxyRoute(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	if err = repository.WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		if err := lockPagesProjectsForRouteMutation(tx, 0, route); err != nil {
			return err
		}
		if err := repository.CreateProxyRouteRecordTx(tx, route); err != nil {
			return err
		}
		return repository.ReplaceZoneDomainRouteBindingsTx(tx, route.ID, input.ZoneDomainIDs)
	}); err != nil {
		if mapped := mapProxyRoutePersistError(err); mapped != nil {
			return nil, mapped
		}
		return nil, err
	}
	notifyAffectedTunnels(ctx, 0, tunnelIDForRoute(route))
	return buildProxyRouteView(ctx, route)
}

// UpdateProxyRoute 更新代理规则。
func UpdateProxyRoute(ctx context.Context, id uint, input Input) (*View, error) {
	route, err := repository.GetProxyRouteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	previousPagesProjectID := pagesProjectIDForRoute(route)
	previousTunnelID := tunnelIDForRoute(route)
	route, err = buildProxyRoute(ctx, route, input)
	if err != nil {
		return nil, err
	}
	if err = repository.WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		if err := lockPagesProjectsForRouteMutation(tx, previousPagesProjectID, route); err != nil {
			return err
		}
		if err := repository.UpdateProxyRouteRecordTx(tx, route); err != nil {
			return err
		}
		return repository.ReplaceZoneDomainRouteBindingsTx(tx, route.ID, input.ZoneDomainIDs)
	}); err != nil {
		if mapped := mapProxyRoutePersistError(err); mapped != nil {
			return nil, mapped
		}
		return nil, err
	}
	notifyAffectedTunnels(ctx, previousTunnelID, tunnelIDForRoute(route))
	return buildProxyRouteView(ctx, route)
}

func mapProxyRoutePersistError(err error) error {
	if err == nil {
		return nil
	}
	if isUniqueConstraintError(err) {
		return errors.New(errProxyRouteIdentityExists)
	}
	if errors.Is(err, repository.ErrZoneDomainBoundToAnotherRoute) {
		return errors.New(errProxyRouteZoneDomainBound)
	}
	if errors.Is(err, repository.ErrZoneDomainNotFound) {
		return errors.New(errProxyRouteZoneDomainNotFound)
	}
	return nil
}

func pagesProjectIDForRoute(route *model.ProxyRoute) uint {
	if route == nil || route.UpstreamType != proxyRouteUpstreamTypePages || route.PagesProjectID == nil {
		return 0
	}
	return *route.PagesProjectID
}

func lockPagesProjectsForRouteMutation(tx *gorm.DB, previousProjectID uint, route *model.ProxyRoute) error {
	nextProjectID := pagesProjectIDForRoute(route)
	var projectIDs []uint
	if previousProjectID != 0 {
		projectIDs = append(projectIDs, previousProjectID)
	}
	if nextProjectID != 0 && nextProjectID != previousProjectID {
		projectIDs = append(projectIDs, nextProjectID)
	}
	slices.Sort(projectIDs)

	for _, projectID := range projectIDs {
		project, err := repository.LockPagesProjectByIDTx(tx, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) && projectID != nextProjectID {
			continue
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errProxyRoutePagesNotFound)
		}
		if err != nil {
			return err
		}
		if projectID == nextProjectID {
			if err := validateLockedPagesRouteProject(project); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLockedPagesRouteProject(project *model.PagesProject) error {
	if project == nil {
		return errors.New(errProxyRoutePagesNotFound)
	}
	if !project.Enabled {
		return errors.New(errProxyRoutePagesDisabled)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID == 0 {
		return errors.New(errProxyRoutePagesNoDeploy)
	}
	return nil
}

// DeleteProxyRoute 删除代理规则。
func DeleteProxyRoute(ctx context.Context, id uint) error {
	route, err := repository.GetProxyRouteByID(ctx, id)
	if err != nil {
		return err
	}
	previousTunnelID := tunnelIDForRoute(route)
	if err := repository.DeleteProxyRouteAndUnbind(ctx, id); err != nil {
		return err
	}
	notifyAffectedTunnels(ctx, previousTunnelID, 0)
	return nil
}

func tunnelIDForRoute(route *model.ProxyRoute) uint {
	if route == nil || route.UpstreamType != proxyRouteUpstreamTypeTunnel || route.TunnelID == nil {
		return 0
	}
	return *route.TunnelID
}

func notifyAffectedTunnels(ctx context.Context, prevID, nextID uint) {
	for _, tid := range []uint{prevID, nextID} {
		if tid == 0 {
			continue
		}
		if err := repository.IncrementTunnelConfigRevision(ctx, tid); err == nil {
			websocket.NotifyTunnelChanged(ctx, tid)
		}
	}
	if prevID > 0 && nextID > 0 && prevID == nextID {
		return
	}
}

func buildProxyRoute(ctx context.Context, route *model.ProxyRoute, input Input) (*model.ProxyRoute, error) {
	domains, err := loadProxyRouteZoneDomains(ctx, input.ZoneDomainIDs)
	if err != nil {
		return nil, err
	}
	siteName := strings.TrimSpace(input.SiteName)

	upstreamType := normalizeUpstreamType(input.UpstreamType)
	_, originID, upstreams, err := resolveProxyRouteUpstreams(ctx, upstreamType, input)
	if err != nil {
		return nil, err
	}
	originHost := strings.TrimSpace(input.OriginHost)
	cachePolicy := strings.TrimSpace(input.CachePolicy)
	cacheRules, err := normalizeCacheRules(input.CacheEnabled, cachePolicy, input.CacheRules)
	if err != nil {
		return nil, err
	}
	customHeaders, err := normalizeCustomHeaders(input.CustomHeaders)
	if err != nil {
		return nil, err
	}
	limitConnPerServer, err := normalizeProxyRouteLimitConnValue(input.LimitConnPerServer, "limit_conn_per_server")
	if err != nil {
		return nil, err
	}
	limitConnPerIP, err := normalizeProxyRouteLimitConnValue(input.LimitConnPerIP, "limit_conn_per_ip")
	if err != nil {
		return nil, err
	}
	limitRate, err := normalizeProxyRouteLimitRate(input.LimitRate)
	if err != nil {
		return nil, err
	}
	limitReqPerIP, err := normalizeProxyRouteLimitReqPerIP(input.LimitReqPerIP)
	if err != nil {
		return nil, err
	}
	if err := validateProxyRouteZoneDomainCertificates(ctx, domains, input.EnableHTTPS); err != nil {
		return nil, err
	}
	jsonFields, err := marshalProxyRouteJSONFields(upstreams, cacheRules, customHeaders)
	if err != nil {
		return nil, err
	}

	if err := validateProxyRouteSiteName(siteName); err != nil {
		return nil, err
	}
	if err := validateProxyRouteSiteNameUniqueness(ctx, route, siteName); err != nil {
		return nil, err
	}
	if err := validateOriginHost(originHost); err != nil {
		return nil, err
	}
	if input.RedirectHTTP && !input.EnableHTTPS {
		return nil, errors.New(errProxyRouteRedirectHTTP)
	}

	if route == nil {
		route = &model.ProxyRoute{}
	}
	if err := applyProxyRouteAuth(route, &input); err != nil {
		return nil, err
	}
	applyProxyRouteHTTP2(route, input)
	populateProxyRouteFields(
		route,
		input,
		siteName,
		jsonFields,
		originID,
		upstreams,
		originHost,
		cachePolicy,
		limitConnPerServer,
		limitConnPerIP,
		limitRate,
		limitReqPerIP,
		upstreamType,
	)
	if err := applyProxyRouteUpstreamType(ctx, route, upstreamType, input); err != nil {
		return nil, err
	}
	return route, nil
}

func buildProxyRouteViews(routes []*model.ProxyRoute) ([]*View, error) {
	views := make([]*View, 0, len(routes))
	for _, route := range routes {
		view, err := proxyRouteViewWithDomains(route, route.ZoneDomains)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func buildProxyRouteView(ctx context.Context, route *model.ProxyRoute) (*View, error) {
	if route == nil {
		return nil, errors.New("proxy route is nil")
	}
	domains, err := repository.ListZoneDomainsByRouteID(ctx, route.ID)
	if err != nil {
		return nil, err
	}
	return proxyRouteViewWithDomains(route, domains)
}

func proxyRouteViewWithDomains(route *model.ProxyRoute, domains []model.ZoneDomain) (*View, error) {
	upstreams, err := decodeStoredUpstreams(route.Upstreams, route.OriginURL)
	if err != nil {
		return nil, err
	}
	cacheRules, err := decodeStoredCacheRules(route.CacheRules)
	if err != nil {
		return nil, err
	}
	customHeaders, err := decodeStoredCustomHeaders(route.CustomHeaders)
	if err != nil {
		return nil, err
	}
	zoneDomainIDs := make([]uint, 0, len(domains))
	zoneDomains := make([]ZoneDomainView, 0, len(domains))
	for _, domain := range domains {
		zoneDomainIDs = append(zoneDomainIDs, domain.ID)
		zoneDomains = append(zoneDomains, ZoneDomainView{ID: domain.ID, ZoneID: domain.ZoneID, Domain: domain.Domain, CertID: domain.CertID})
	}
	return &View{
		ID:                   route.ID,
		SiteName:             route.SiteName,
		ZoneDomainIDs:        zoneDomainIDs,
		ZoneDomains:          zoneDomains,
		OriginID:             route.OriginID,
		OriginURL:            route.OriginURL,
		OriginHost:           route.OriginHost,
		Upstreams:            route.Upstreams,
		UpstreamList:         upstreams,
		Enabled:              route.Enabled,
		EnableHTTPS:          route.EnableHTTPS,
		EnableHTTP2:          route.EnableHTTP2,
		RedirectHTTP:         route.RedirectHTTP,
		LimitConnPerServer:   route.LimitConnPerServer,
		LimitConnPerIP:       route.LimitConnPerIP,
		LimitRate:            route.LimitRate,
		LimitReqPerIP:        route.LimitReqPerIP,
		CacheEnabled:         route.CacheEnabled,
		CachePolicy:          displayCachePolicy(route.CacheEnabled, route.CachePolicy),
		CacheRules:           route.CacheRules,
		CacheRuleList:        cacheRules,
		CustomHeaders:        route.CustomHeaders,
		CustomHeaderList:     customHeaders,
		BasicAuthEnabled:     route.BasicAuthEnabled,
		BasicAuthUsername:    route.BasicAuthUsername,
		BasicAuthPassword:    route.BasicAuthPassword,
		SiteAuthTTLHours:     route.SiteAuthTTLHours,
		UpstreamType:         route.UpstreamType,
		TunnelID:             route.TunnelID,
		TunnelTargetAddr:     route.TunnelTargetAddr,
		TunnelTargetProtocol: route.TunnelTargetProtocol,
		TunnelOptionsJSON:    route.TunnelOptionsJSON,
		PagesProjectID:       route.PagesProjectID,
		CreatedAt:            route.CreatedAt,
		UpdatedAt:            route.UpdatedAt,
	}, nil
}
