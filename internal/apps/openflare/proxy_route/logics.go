// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	UpstreamType         string              `json:"upstream_type"`
	TunnelNodeID         *uint               `json:"tunnel_node_id"`
	TunnelID             *uint               `json:"tunnel_id"`
	TunnelTargetAddr     string              `json:"tunnel_target_addr"`
	TunnelTargetProtocol string              `json:"tunnel_target_protocol"`
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
	UpstreamType         string              `json:"upstream_type"`
	TunnelNodeID         *uint               `json:"tunnel_node_id"`
	TunnelID             *uint               `json:"tunnel_id"`
	TunnelTargetAddr     string              `json:"tunnel_target_addr"`
	TunnelTargetProtocol string              `json:"tunnel_target_protocol"`
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
	routes, err := model.ListProxyRoutes(ctx)
	if err != nil {
		return nil, err
	}
	return buildProxyRouteViews(ctx, routes)
}

// GetProxyRoute 获取代理规则详情。
func GetProxyRoute(ctx context.Context, id uint) (*View, error) {
	route, err := model.GetProxyRouteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildProxyRouteView(ctx, route)
}

// CreateProxyRoute 创建代理规则。
func CreateProxyRoute(ctx context.Context, input Input) (*View, error) {
	route, _, err := buildProxyRoute(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	if err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPagesProjectsForRouteMutation(tx, 0, route); err != nil {
			return err
		}
		if err := tx.Create(route).Error; err != nil {
			return err
		}
		return replaceZoneDomainRouteBindings(tx, route.ID, input.ZoneDomainIDs)
	}); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errProxyRouteIdentityExists)
		}
		return nil, err
	}
	return buildProxyRouteView(ctx, route)
}

// UpdateProxyRoute 更新代理规则。
func UpdateProxyRoute(ctx context.Context, id uint, input Input) (*View, error) {
	route, err := model.GetProxyRouteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	previousPagesProjectID := pagesProjectIDForRoute(route)
	route, _, err = buildProxyRoute(ctx, route, input)
	if err != nil {
		return nil, err
	}
	if err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPagesProjectsForRouteMutation(tx, previousPagesProjectID, route); err != nil {
			return err
		}
		if err := updateProxyRouteRecord(tx, route); err != nil {
			return err
		}
		return replaceZoneDomainRouteBindings(tx, route.ID, input.ZoneDomainIDs)
	}); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errProxyRouteIdentityExists)
		}
		return nil, err
	}
	return buildProxyRouteView(ctx, route)
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
	sort.Slice(projectIDs, func(i int, j int) bool { return projectIDs[i] < projectIDs[j] })

	for _, projectID := range projectIDs {
		var project model.PagesProject
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, projectID).Error
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
			if err := validateLockedPagesRouteProject(&project); err != nil {
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
	if _, err := model.GetProxyRouteByID(ctx, id); err != nil {
		return err
	}
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ZoneDomain{}).Where("proxy_route_id = ?", id).Update("proxy_route_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&model.ProxyRoute{}, id).Error
	})
}

func buildProxyRoute(ctx context.Context, route *model.ProxyRoute, input Input) (*model.ProxyRoute, []model.ZoneDomain, error) {
	domains, err := loadProxyRouteZoneDomains(ctx, input.ZoneDomainIDs)
	if err != nil {
		return nil, nil, err
	}
	siteName := strings.TrimSpace(input.SiteName)

	upstreamType := normalizeUpstreamType(input.UpstreamType)
	_, originID, upstreams, err := resolveProxyRouteUpstreams(ctx, upstreamType, input)
	if err != nil {
		return nil, nil, err
	}
	originHost := strings.TrimSpace(input.OriginHost)
	cachePolicy := strings.TrimSpace(input.CachePolicy)
	cacheRules, err := normalizeCacheRules(input.CacheEnabled, cachePolicy, input.CacheRules)
	if err != nil {
		return nil, nil, err
	}
	customHeaders, err := normalizeCustomHeaders(input.CustomHeaders)
	if err != nil {
		return nil, nil, err
	}
	limitConnPerServer, err := normalizeProxyRouteLimitConnValue(input.LimitConnPerServer, "limit_conn_per_server")
	if err != nil {
		return nil, nil, err
	}
	limitConnPerIP, err := normalizeProxyRouteLimitConnValue(input.LimitConnPerIP, "limit_conn_per_ip")
	if err != nil {
		return nil, nil, err
	}
	limitRate, err := normalizeProxyRouteLimitRate(input.LimitRate)
	if err != nil {
		return nil, nil, err
	}
	limitReqPerIP, err := normalizeProxyRouteLimitReqPerIP(input.LimitReqPerIP)
	if err != nil {
		return nil, nil, err
	}
	if err := validateProxyRouteZoneDomainCertificates(ctx, domains, input.EnableHTTPS); err != nil {
		return nil, nil, err
	}
	jsonFields, err := marshalProxyRouteJSONFields(upstreams, cacheRules, customHeaders)
	if err != nil {
		return nil, nil, err
	}

	if err := validateProxyRouteSiteName(siteName); err != nil {
		return nil, nil, err
	}
	if err := validateProxyRouteSiteNameUniqueness(ctx, route, siteName); err != nil {
		return nil, nil, err
	}
	if err := validateOriginHost(originHost); err != nil {
		return nil, nil, err
	}
	if input.RedirectHTTP && !input.EnableHTTPS {
		return nil, nil, errors.New(errProxyRouteRedirectHTTP)
	}

	if err := normalizeProxyRouteBasicAuth(&input); err != nil {
		return nil, nil, err
	}

	if route == nil {
		route = &model.ProxyRoute{}
	}
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
		return nil, nil, err
	}
	return route, domains, nil
}

func buildProxyRouteViews(ctx context.Context, routes []*model.ProxyRoute) ([]*View, error) {
	views := make([]*View, 0, len(routes))
	for _, route := range routes {
		view, err := buildProxyRouteView(ctx, route)
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
	domains, err := model.ListZoneDomainsByRouteID(ctx, route.ID)
	if err != nil {
		return nil, err
	}
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
		UpstreamType:         route.UpstreamType,
		TunnelNodeID:         route.TunnelNodeID,
		TunnelID:             route.TunnelNodeID,
		TunnelTargetAddr:     route.TunnelTargetAddr,
		TunnelTargetProtocol: route.TunnelTargetProtocol,
		PagesProjectID:       route.PagesProjectID,
		CreatedAt:            route.CreatedAt,
		UpdatedAt:            route.UpdatedAt,
	}, nil
}
