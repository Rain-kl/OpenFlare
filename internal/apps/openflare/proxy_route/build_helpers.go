// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package proxy_route provides helpers for building proxy route configurations.
package proxy_route

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

func applyProxyRouteAuth(route *model.ProxyRoute, input *Input) error {
	ttl := 0
	if input.SiteAuthTTLHours != nil {
		ttl = *input.SiteAuthTTLHours
	} else if !input.BasicAuthEnabled && route.SiteAuthTTLHours > 0 {
		// Older clients omit the field when editing other route settings.
		ttl = route.SiteAuthTTLHours
		input.BasicAuthUsername = route.BasicAuthUsername
		input.BasicAuthPassword = route.BasicAuthPassword
	}
	input.BasicAuthUsername = strings.TrimSpace(input.BasicAuthUsername)
	input.BasicAuthPassword = strings.TrimSpace(input.BasicAuthPassword)
	if err := validateProxyRouteAuth(input, ttl); err != nil {
		return err
	}
	if ttl > 0 {
		if route.SiteAuthKey == "" || route.SiteAuthTTLHours != ttl ||
			route.BasicAuthUsername != input.BasicAuthUsername || route.BasicAuthPassword != input.BasicAuthPassword {
			var key [32]byte
			rand.Read(key[:])
			route.SiteAuthKey = hex.EncodeToString(key[:])
		}
	} else {
		route.SiteAuthKey = ""
	}
	route.SiteAuthTTLHours = ttl
	if ttl == 0 && !input.BasicAuthEnabled {
		input.BasicAuthUsername = ""
		input.BasicAuthPassword = ""
	}
	return nil
}

func validateProxyRouteAuth(input *Input, ttl int) error {
	if ttl < 0 || ttl > 720 {
		return errors.New(errProxyRouteSiteAuthTTL)
	}
	if ttl > 0 && input.BasicAuthEnabled {
		return errors.New(errProxyRouteSiteAuthMode)
	}
	if ttl > 0 && !input.EnableHTTPS {
		return errors.New(errProxyRouteSiteAuthHTTPS)
	}
	if ttl > 0 && (input.BasicAuthUsername == "" || input.BasicAuthPassword == "" ||
		len(input.BasicAuthUsername) > 255 || len(input.BasicAuthPassword) > 255 ||
		strings.ContainsRune(input.BasicAuthUsername, 0) || strings.ContainsRune(input.BasicAuthPassword, 0)) {
		return errors.New(errProxyRouteSiteAuthCredentials)
	}
	if input.BasicAuthEnabled && (input.BasicAuthUsername == "" || input.BasicAuthPassword == "") {
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

func applyProxyRouteHTTP2(route *model.ProxyRoute, input Input) {
	if input.EnableHTTP2 != nil {
		route.EnableHTTP2 = *input.EnableHTTP2
	} else if route.ID == 0 {
		route.EnableHTTP2 = true
	}
}

func applyProxyRouteUpstreamType(ctx context.Context, route *model.ProxyRoute, upstreamType string, input Input) error {
	switch upstreamType {
	case proxyRouteUpstreamTypeTunnel:
		tunnelID, err := normalizeTunnelID(input.TunnelID)
		if err != nil {
			return err
		}
		optionsJSON, err := normalizeTunnelOptionsJSON(input.TunnelOptionsJSON)
		if err != nil {
			return err
		}
		if err := validateTunnelRouteInput(ctx, tunnelID, input.TunnelTargetAddr, input.TunnelTargetProtocol); err != nil {
			return err
		}
		route.TunnelID = tunnelID
		route.TunnelTargetAddr = strings.TrimSpace(input.TunnelTargetAddr)
		route.TunnelTargetProtocol = normalizeTunnelTargetProtocol(input.TunnelTargetProtocol)
		route.TunnelOptionsJSON = optionsJSON
		route.PagesProjectID = nil
	case proxyRouteUpstreamTypePages:
		route.TunnelID = nil
		route.TunnelTargetAddr = ""
		route.TunnelTargetProtocol = ""
		route.TunnelOptionsJSON = "{}"
		route.PagesProjectID = input.PagesProjectID
	default:
		route.TunnelID = nil
		route.TunnelTargetAddr = ""
		route.TunnelTargetProtocol = ""
		route.TunnelOptionsJSON = "{}"
		route.PagesProjectID = nil
	}
	return nil
}
