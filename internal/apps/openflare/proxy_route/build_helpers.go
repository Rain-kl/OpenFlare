// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package proxy_route provides helpers for building proxy route configurations.
package proxy_route

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

type proxyRouteJSONFields struct {
	cacheRulesJSON    string
	upstreamsJSON     string
	customHeadersJSON string
}

func resolveProxyRouteUpstreams(ctx context.Context, upstreamType string, input Input) (string, *uint, []string, error) {
	switch upstreamType {
	case proxyRouteUpstreamTypeTunnel, proxyRouteUpstreamTypePages:
		if upstreamType == proxyRouteUpstreamTypePages {
			if err := validatePagesRouteInput(ctx, input.PagesProjectID); err != nil {
				return "", nil, nil, err
			}
		}
		originURL := "http://127.0.0.1"
		return originURL, nil, []string{originURL}, nil
	default:
		originURL, originID, err := resolveProxyRoutePrimaryOrigin(ctx, input)
		if err != nil {
			return "", nil, nil, err
		}
		upstreams, err := normalizeUpstreams(originURL, input.Upstreams)
		if err != nil {
			return "", nil, nil, err
		}
		return originURL, originID, upstreams, nil
	}
}

func marshalProxyRouteJSONFields(
	upstreams []string,
	cacheRules []string,
	customHeaders []CustomHeaderInput,
) (*proxyRouteJSONFields, error) {
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
	return &proxyRouteJSONFields{
		cacheRulesJSON:    string(cacheRulesJSON),
		upstreamsJSON:     string(upstreamsJSON),
		customHeadersJSON: string(customHeadersJSON),
	}, nil
}

func normalizeProxyRouteBasicAuth(input *Input) error {
	if !input.BasicAuthEnabled {
		input.BasicAuthUsername = ""
		input.BasicAuthPassword = ""
		return nil
	}
	input.BasicAuthUsername = strings.TrimSpace(input.BasicAuthUsername)
	input.BasicAuthPassword = strings.TrimSpace(input.BasicAuthPassword)
	if input.BasicAuthUsername == "" || input.BasicAuthPassword == "" {
		return errors.New(errProxyRouteBasicAuth)
	}
	return nil
}

func populateProxyRouteFields(
	route *model.ProxyRoute,
	input Input,
	siteName string,
	jsonFields *proxyRouteJSONFields,
	originID *uint,
	upstreams []string,
	originHost, cachePolicy string,
	limitConnPerServer, limitConnPerIP int,
	limitRate, limitReqPerIP, upstreamType string,
) {
	route.SiteName = siteName
	route.OriginID = originID
	route.OriginURL = upstreams[0]
	route.OriginHost = originHost
	route.Upstreams = jsonFields.upstreamsJSON
	route.Enabled = input.Enabled
	route.EnableHTTPS = input.EnableHTTPS
	route.RedirectHTTP = input.RedirectHTTP
	route.LimitConnPerServer = limitConnPerServer
	route.LimitConnPerIP = limitConnPerIP
	route.LimitRate = limitRate
	route.LimitReqPerIP = limitReqPerIP
	route.CacheEnabled = input.CacheEnabled
	route.CachePolicy = normalizeCachePolicy(input.CacheEnabled, cachePolicy)
	route.CacheRules = jsonFields.cacheRulesJSON
	route.CustomHeaders = jsonFields.customHeadersJSON
	route.BasicAuthEnabled = input.BasicAuthEnabled
	route.BasicAuthUsername = input.BasicAuthUsername
	route.BasicAuthPassword = input.BasicAuthPassword
	route.UpstreamType = upstreamType
}

func applyProxyRouteUpstreamType(ctx context.Context, route *model.ProxyRoute, upstreamType string, input Input) error {
	switch upstreamType {
	case proxyRouteUpstreamTypeTunnel:
		tunnelNodeID, err := normalizeTunnelNodeID(input.TunnelNodeID, input.TunnelID)
		if err != nil {
			return err
		}
		if err := validateTunnelRouteInput(ctx, tunnelNodeID, input.TunnelTargetAddr, input.TunnelTargetProtocol); err != nil {
			return err
		}
		route.TunnelNodeID = tunnelNodeID
		route.TunnelTargetAddr = strings.TrimSpace(input.TunnelTargetAddr)
		route.TunnelTargetProtocol = normalizeTunnelTargetProtocol(input.TunnelTargetProtocol)
		route.PagesProjectID = nil
	case proxyRouteUpstreamTypePages:
		route.TunnelNodeID = nil
		route.TunnelTargetAddr = ""
		route.TunnelTargetProtocol = ""
		route.PagesProjectID = input.PagesProjectID
	default:
		route.TunnelNodeID = nil
		route.TunnelTargetAddr = ""
		route.TunnelTargetProtocol = ""
		route.PagesProjectID = nil
	}
	return nil
}
