// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

// CustomHeaderInput 自定义响应头。
type CustomHeaderInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Input 代理规则创建/更新请求。
type Input struct {
	SiteName             string              `json:"site_name"`
	Domain               string              `json:"domain"`
	Domains              []string            `json:"domains"`
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
	CertID               *uint               `json:"cert_id"`
	CertIDs              []uint              `json:"cert_ids"`
	DomainCertIDs        []uint              `json:"domain_cert_ids"`
	RedirectHTTP         bool                `json:"redirect_http"`
	LimitConnPerServer   int                 `json:"limit_conn_per_server"`
	LimitConnPerIP       int                 `json:"limit_conn_per_ip"`
	LimitRate            string              `json:"limit_rate"`
	CacheEnabled         bool                `json:"cache_enabled"`
	CachePolicy          string              `json:"cache_policy"`
	CacheRules           []string            `json:"cache_rules"`
	CustomHeaders        []CustomHeaderInput `json:"custom_headers"`
	BasicAuthEnabled     bool                `json:"basic_auth_enabled"`
	BasicAuthUsername    string              `json:"basic_auth_username"`
	BasicAuthPassword    string              `json:"basic_auth_password"`
	Remark               string              `json:"remark"`
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
	Domain               string              `json:"domain"`
	Domains              []string            `json:"domains"`
	PrimaryDomain        string              `json:"primary_domain"`
	DomainCount          int                 `json:"domain_count"`
	OriginID             *uint               `json:"origin_id"`
	OriginURL            string              `json:"origin_url"`
	OriginHost           string              `json:"origin_host"`
	Upstreams            string              `json:"upstreams"`
	UpstreamList         []string            `json:"upstream_list"`
	Enabled              bool                `json:"enabled"`
	EnableHTTPS          bool                `json:"enable_https"`
	CertID               *uint               `json:"cert_id"`
	CertIDs              []uint              `json:"cert_ids"`
	DomainCertIDs        []uint              `json:"domain_cert_ids"`
	RedirectHTTP         bool                `json:"redirect_http"`
	LimitConnPerServer   int                 `json:"limit_conn_per_server"`
	LimitConnPerIP       int                 `json:"limit_conn_per_ip"`
	LimitRate            string              `json:"limit_rate"`
	CacheEnabled         bool                `json:"cache_enabled"`
	CachePolicy          string              `json:"cache_policy"`
	CacheRules           string              `json:"cache_rules"`
	CacheRuleList        []string            `json:"cache_rule_list"`
	CustomHeaders        string              `json:"custom_headers"`
	CustomHeaderList     []CustomHeaderInput `json:"custom_header_list"`
	BasicAuthEnabled     bool                `json:"basic_auth_enabled"`
	BasicAuthUsername    string              `json:"basic_auth_username"`
	BasicAuthPassword    string              `json:"basic_auth_password"`
	Remark               string              `json:"remark"`
	UpstreamType         string              `json:"upstream_type"`
	TunnelNodeID         *uint               `json:"tunnel_node_id"`
	TunnelID             *uint               `json:"tunnel_id"`
	TunnelTargetAddr     string              `json:"tunnel_target_addr"`
	TunnelTargetProtocol string              `json:"tunnel_target_protocol"`
	PagesProjectID       *uint               `json:"pages_project_id"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
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
	route, err := buildProxyRoute(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	if err = model.CreateProxyRouteRecord(ctx, route); err != nil {
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
	route, err = buildProxyRoute(ctx, route, input)
	if err != nil {
		return nil, err
	}
	if err = model.UpdateProxyRouteRecord(ctx, route); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errProxyRouteIdentityExists)
		}
		return nil, err
	}
	return buildProxyRouteView(ctx, route)
}

// DeleteProxyRoute 删除代理规则。
func DeleteProxyRoute(ctx context.Context, id uint) error {
	if _, err := model.GetProxyRouteByID(ctx, id); err != nil {
		return err
	}
	return model.DeleteProxyRouteRecord(ctx, id)
}

func buildProxyRoute(ctx context.Context, route *model.ProxyRoute, input Input) (*model.ProxyRoute, error) {
	domains, err := normalizeProxyRouteDomainsInput(route, input.Domain, input.Domains)
	if err != nil {
		return nil, err
	}
	domain := domains[0]
	siteName := normalizeProxyRouteSiteNameInput(route, input.SiteName, domain)

	upstreamType := normalizeUpstreamType(input.UpstreamType)
	var originURL string
	var originID *uint
	var upstreams []string

	if upstreamType == "tunnel" {
		originURL = "http://127.0.0.1"
		upstreams = []string{originURL}
	} else if upstreamType == "pages" {
		if err := validatePagesRouteInput(ctx, input.PagesProjectID); err != nil {
			return nil, err
		}
		originURL = "http://127.0.0.1"
		upstreams = []string{originURL}
	} else {
		originURL, originID, err = resolveProxyRoutePrimaryOrigin(ctx, input)
		if err != nil {
			return nil, err
		}
		upstreams, err = normalizeUpstreams(originURL, input.Upstreams)
		if err != nil {
			return nil, err
		}
	}
	originHost := strings.TrimSpace(input.OriginHost)
	remark := strings.TrimSpace(input.Remark)
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

	cacheRulesJSON, err := json.Marshal(cacheRules)
	if err != nil {
		return nil, err
	}
	upstreamsJSON, err := json.Marshal(upstreams)
	if err != nil {
		return nil, err
	}
	customHeadersJSON, err := json.Marshal(customHeaders)
	if err != nil {
		return nil, err
	}

	if !input.EnableHTTPS {
		input.RedirectHTTP = false
		input.CertID = nil
		input.CertIDs = nil
		input.DomainCertIDs = nil
	}
	domainCertIDs, certIDs, primaryCertID, err := normalizeProxyRouteDomainCertificateIDs(
		ctx,
		domains,
		input.EnableHTTPS,
		input.DomainCertIDs,
		input.CertID,
		input.CertIDs,
	)
	if err != nil {
		return nil, err
	}
	if err := validateProxyRouteDomainCertificateCoverage(ctx, domains, domainCertIDs); err != nil {
		return nil, err
	}
	certIDsJSON, err := json.Marshal(certIDs)
	if err != nil {
		return nil, err
	}
	domainCertIDsJSON, err := json.Marshal(domainCertIDs)
	if err != nil {
		return nil, err
	}
	domainsJSON, err := json.Marshal(domains)
	if err != nil {
		return nil, err
	}

	if err := validateProxyRouteSiteName(siteName); err != nil {
		return nil, err
	}
	if err := validateProxyRouteIdentityUniqueness(ctx, route, siteName, domains); err != nil {
		return nil, err
	}
	if err := validateOriginHost(originHost); err != nil {
		return nil, err
	}
	input.DomainCertIDs = domainCertIDs
	input.CertIDs = certIDs
	input.CertID = primaryCertID
	if input.RedirectHTTP && !input.EnableHTTPS {
		return nil, errors.New(errProxyRouteRedirectHTTP)
	}

	if input.BasicAuthEnabled {
		input.BasicAuthUsername = strings.TrimSpace(input.BasicAuthUsername)
		input.BasicAuthPassword = strings.TrimSpace(input.BasicAuthPassword)
		if input.BasicAuthUsername == "" || input.BasicAuthPassword == "" {
			return nil, errors.New(errProxyRouteBasicAuth)
		}
	} else {
		input.BasicAuthUsername = ""
		input.BasicAuthPassword = ""
	}

	if route == nil {
		route = &model.ProxyRoute{}
	}
	route.SiteName = siteName
	route.Domain = domain
	route.Domains = string(domainsJSON)
	route.OriginID = originID
	route.OriginURL = upstreams[0]
	route.OriginHost = originHost
	route.Upstreams = string(upstreamsJSON)
	route.Enabled = input.Enabled
	route.EnableHTTPS = input.EnableHTTPS
	route.CertID = input.CertID
	route.CertIDs = string(certIDsJSON)
	route.DomainCertIDs = string(domainCertIDsJSON)
	route.RedirectHTTP = input.RedirectHTTP
	route.LimitConnPerServer = limitConnPerServer
	route.LimitConnPerIP = limitConnPerIP
	route.LimitRate = limitRate
	route.CacheEnabled = input.CacheEnabled
	route.CachePolicy = normalizeCachePolicy(input.CacheEnabled, cachePolicy)
	route.CacheRules = string(cacheRulesJSON)
	route.CustomHeaders = string(customHeadersJSON)
	route.BasicAuthEnabled = input.BasicAuthEnabled
	route.BasicAuthUsername = input.BasicAuthUsername
	route.BasicAuthPassword = input.BasicAuthPassword
	route.Remark = remark
	route.UpstreamType = upstreamType
	if upstreamType == "tunnel" {
		tunnelNodeID, err := normalizeTunnelNodeID(input.TunnelNodeID, input.TunnelID)
		if err != nil {
			return nil, err
		}
		if err := validateTunnelRouteInput(ctx, tunnelNodeID, input.TunnelTargetAddr, input.TunnelTargetProtocol); err != nil {
			return nil, err
		}
		route.TunnelNodeID = tunnelNodeID
		route.TunnelTargetAddr = strings.TrimSpace(input.TunnelTargetAddr)
		route.TunnelTargetProtocol = normalizeTunnelTargetProtocol(input.TunnelTargetProtocol)
		route.PagesProjectID = nil
	} else if upstreamType == "pages" {
		route.TunnelNodeID = nil
		route.TunnelTargetAddr = ""
		route.TunnelTargetProtocol = ""
		route.PagesProjectID = input.PagesProjectID
	} else {
		route.TunnelNodeID = nil
		route.TunnelTargetAddr = ""
		route.TunnelTargetProtocol = ""
		route.PagesProjectID = nil
	}
	return route, nil
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
	domains, err := decodeStoredDomains(route.Domains, route.Domain)
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
	certIDs, err := decodeStoredCertIDs(route.CertIDs, route.CertID)
	if err != nil {
		return nil, err
	}
	domainCertIDs, err := resolveProxyRouteDomainCertIDs(ctx, route, domains, certIDs)
	if err != nil {
		return nil, err
	}
	var certID *uint
	if len(certIDs) > 0 {
		certID = &certIDs[0]
	}
	primaryDomain := domains[0]
	return &View{
		ID:                   route.ID,
		SiteName:             normalizeProxyRouteSiteNameInput(route, route.SiteName, primaryDomain),
		Domain:               primaryDomain,
		Domains:              domains,
		PrimaryDomain:        primaryDomain,
		DomainCount:          len(domains),
		OriginID:             route.OriginID,
		OriginURL:            route.OriginURL,
		OriginHost:           route.OriginHost,
		Upstreams:            route.Upstreams,
		UpstreamList:         upstreams,
		Enabled:              route.Enabled,
		EnableHTTPS:          route.EnableHTTPS,
		CertID:               certID,
		CertIDs:              certIDs,
		DomainCertIDs:        domainCertIDs,
		RedirectHTTP:         route.RedirectHTTP,
		LimitConnPerServer:   route.LimitConnPerServer,
		LimitConnPerIP:       route.LimitConnPerIP,
		LimitRate:            route.LimitRate,
		CacheEnabled:         route.CacheEnabled,
		CachePolicy:          route.CachePolicy,
		CacheRules:           route.CacheRules,
		CacheRuleList:        cacheRules,
		CustomHeaders:        route.CustomHeaders,
		CustomHeaderList:     customHeaders,
		BasicAuthEnabled:     route.BasicAuthEnabled,
		BasicAuthUsername:    route.BasicAuthUsername,
		BasicAuthPassword:    route.BasicAuthPassword,
		Remark:               route.Remark,
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
