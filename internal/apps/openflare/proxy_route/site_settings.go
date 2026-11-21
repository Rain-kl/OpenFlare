// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

// SettingsPatch changes only fields explicitly supplied by the caller.
type SettingsPatch struct {
	// Whether the site serves traffic.
	Enabled *bool `json:"enabled,omitempty"`
	// Whether the site serves HTTPS. Each bound domain needs a covering certificate.
	EnableHTTPS *bool `json:"enable_https,omitempty"`
	// Whether HTTP/2 is enabled on the HTTPS listener.
	EnableHTTP2 *bool `json:"enable_http2,omitempty"`
	// Whether to redirect HTTP to HTTPS; requires HTTPS.
	RedirectHTTP *bool `json:"redirect_http,omitempty"`
	// Whether edge caching is enabled.
	CacheEnabled *bool `json:"cache_enabled,omitempty"`
	// Cache policy: static, all, suffix, path_prefix, or path_exact.
	CachePolicy *string `json:"cache_policy,omitempty" enums:"static,all,suffix,path_prefix,path_exact"`
	// Replace custom cache rules; an empty array clears the rules when permitted by the cache policy.
	CacheRules *[]string `json:"cache_rules,omitempty"`
	// Replace custom response headers; an empty array clears them.
	CustomHeaders *[]CustomHeaderInput `json:"custom_headers,omitempty"`
	// Update site-specific connection, bandwidth, and request limits. Omitted limits are preserved.
	Limits *SiteLimitsPatch `json:"limits,omitempty"`
}

// SiteLimitsPatch changes per-site connection, bandwidth, or request limits.
type SiteLimitsPatch struct {
	// Zero inherits the default. Minus one disables the limit.
	Connections *int `json:"limit_conn_per_server,omitempty" minimum:"-1"`
	// Zero inherits the default. Minus one disables the limit.
	ConnectionsPerIP *int `json:"limit_conn_per_ip,omitempty" minimum:"-1"`
	// Response rate, such as 512k or 2m. Empty inherits. Minus one disables.
	Rate *string `json:"limit_rate,omitempty"`
	// Request rate per IP, such as 10r/s or 100r/m. Empty inherits. Minus one disables.
	RequestsPerIP *string `json:"limit_req_per_ip,omitempty"`
}

// SiteAuthInput changes authentication without exposing or rewriting other settings.
type SiteAuthInput struct {
	Mode     string
	Username *string
	Password *string
	TTLHours *int
}

// ErrInvalidSiteSettings classifies validation errors safe to return to operators.
var ErrInvalidSiteSettings = errors.New("invalid site settings")

func invalidSiteSettings(err error) error { return fmt.Errorf("%w: %s", ErrInvalidSiteSettings, err) }

// PatchSettings validates a partial settings update against the locked site.
func PatchSettings(ctx context.Context, id uint, patch SettingsPatch) error {
	return repository.WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		route, err := repository.LockProxyRouteTx(tx, id)
		if err != nil {
			return err
		}
		fields, err := applySettingsPatch(tx, route, patch)
		if err != nil {
			return err
		}
		if len(fields) == 0 {
			return invalidSiteSettings(errors.New("supply at least one setting to update"))
		}
		return repository.UpdateProxyRouteFieldsTx(tx, route, fields)
	})
}

func applySettingsPatch(tx *gorm.DB, route *model.ProxyRoute, patch SettingsPatch) ([]string, error) {
	var fields []string
	setBool := func(name string, dst *bool, input *bool) {
		if input != nil {
			*dst = *input
			fields = append(fields, name)
		}
	}
	setBool("Enabled", &route.Enabled, patch.Enabled)
	setBool("EnableHTTPS", &route.EnableHTTPS, patch.EnableHTTPS)
	setBool("EnableHTTP2", &route.EnableHTTP2, patch.EnableHTTP2)
	setBool("RedirectHTTP", &route.RedirectHTTP, patch.RedirectHTTP)
	if err := validateSettingsTLS(tx, route, patch.EnableHTTPS); err != nil {
		return nil, err
	}
	if patch.CacheEnabled != nil || patch.CachePolicy != nil || patch.CacheRules != nil {
		if err := applyCachePatch(route, patch); err != nil {
			return nil, err
		}
		fields = append(fields, "CacheEnabled", "CachePolicy", "CacheRules")
	}
	if patch.CustomHeaders != nil {
		headers, err := normalizeCustomHeaders(*patch.CustomHeaders)
		if err != nil {
			return nil, invalidSiteSettings(err)
		}
		raw, err := json.Marshal(headers)
		if err != nil {
			return nil, err
		}
		route.CustomHeaders = string(raw)
		fields = append(fields, "CustomHeaders")
	}
	if patch.Limits != nil {
		limits, err := applySiteLimits(route, *patch.Limits)
		if err != nil {
			return nil, invalidSiteSettings(err)
		}
		fields = append(fields, limits...)
	}
	return fields, nil
}

func validateSettingsTLS(tx *gorm.DB, route *model.ProxyRoute, https *bool) error {
	if !route.EnableHTTPS && route.SiteAuthTTLHours > 0 {
		return invalidSiteSettings(errors.New(errProxyRouteSiteAuthHTTPS))
	}
	if !route.EnableHTTPS && route.RedirectHTTP {
		return invalidSiteSettings(errors.New(errProxyRouteRedirectHTTP))
	}
	if https == nil || !*https {
		return nil
	}
	domains, err := repository.ListZoneDomainsByRouteIDTx(tx, route.ID)
	if err != nil {
		return err
	}
	if len(domains) == 0 {
		return invalidSiteSettings(errors.New(errProxyRouteZoneDomainsRequired))
	}
	var lookupErr error
	if err := validateBoundCertificates(domains, func(id uint) (*model.TLSCertificate, error) {
		certificate, err := repository.GetTLSCertificateByIDTx(tx, id)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			lookupErr = err
		}
		return certificate, err
	}); err != nil {
		if lookupErr != nil {
			return lookupErr
		}
		return invalidSiteSettings(err)
	}
	return nil
}

func applyCachePatch(route *model.ProxyRoute, patch SettingsPatch) error {
	if patch.CacheEnabled != nil {
		route.CacheEnabled = *patch.CacheEnabled
	}
	if patch.CachePolicy != nil {
		route.CachePolicy = *patch.CachePolicy
	}
	rules, err := decodeStoredCacheRules(route.CacheRules)
	if err != nil && patch.CacheRules == nil {
		return err
	}
	if patch.CacheRules != nil {
		rules = *patch.CacheRules
	}
	rules, err = normalizeCacheRules(route.CacheEnabled, route.CachePolicy, rules)
	if err != nil {
		return invalidSiteSettings(err)
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	route.CacheRules = string(raw)
	route.CachePolicy = normalizeCachePolicy(route.CacheEnabled, route.CachePolicy)
	return nil
}

func applySiteLimits(route *model.ProxyRoute, limits SiteLimitsPatch) ([]string, error) {
	var fields []string
	for _, limit := range []struct {
		field, label string
		dst, input   *int
	}{
		{"LimitConnPerServer", "limit_conn_per_server", &route.LimitConnPerServer, limits.Connections},
		{"LimitConnPerIP", "limit_conn_per_ip", &route.LimitConnPerIP, limits.ConnectionsPerIP},
	} {
		if limit.input == nil {
			continue
		}
		value, err := normalizeProxyRouteLimitConnValue(*limit.input, limit.label)
		if err != nil {
			return nil, err
		}
		*limit.dst = value
		fields = append(fields, limit.field)
	}
	for _, limit := range []struct {
		field      string
		dst, input *string
		normalize  func(string) (string, error)
	}{
		{"LimitRate", &route.LimitRate, limits.Rate, normalizeProxyRouteLimitRate},
		{"LimitReqPerIP", &route.LimitReqPerIP, limits.RequestsPerIP, normalizeProxyRouteLimitReqPerIP},
	} {
		if limit.input == nil {
			continue
		}
		value, err := limit.normalize(*limit.input)
		if err != nil {
			return nil, err
		}
		*limit.dst = value
		fields = append(fields, limit.field)
	}
	return fields, nil
}

// SetSiteAuth updates only the authentication fields of an existing site.
func SetSiteAuth(ctx context.Context, id uint, auth SiteAuthInput) error {
	return repository.WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		route, err := repository.LockProxyRouteTx(tx, id)
		if err != nil {
			return err
		}
		if err := applySiteAuth(route, auth); err != nil {
			return invalidSiteSettings(err)
		}
		fields := []string{"BasicAuthEnabled", "BasicAuthUsername", "BasicAuthPassword", "SiteAuthTTLHours", "SiteAuthKey"}
		return repository.UpdateProxyRouteFieldsTx(tx, route, fields)
	})
}

func applySiteAuth(route *model.ProxyRoute, auth SiteAuthInput) error {
	ttl := 0
	input := Input{EnableHTTPS: route.EnableHTTPS, SiteAuthTTLHours: &ttl}
	switch auth.Mode {
	case "off":
		if auth.Username != nil || auth.Password != nil || auth.TTLHours != nil {
			return errors.New("off mode does not accept credentials or a session TTL")
		}
	case "basic":
		if auth.TTLHours != nil {
			return errors.New("session TTL is only supported in site mode")
		}
		input.BasicAuthEnabled = true
	case "site":
		ttl = route.SiteAuthTTLHours
		if auth.TTLHours != nil {
			ttl = *auth.TTLHours
		}
		if ttl <= 0 {
			return errors.New("site mode requires a positive session TTL")
		}
	default:
		return errors.New("authentication mode must be off, basic, or site")
	}
	if auth.Mode != "off" {
		if err := setSiteAuthCredentials(route, auth, &input); err != nil {
			return err
		}
	}
	if err := applyProxyRouteAuth(route, &input); err != nil {
		return err
	}
	route.BasicAuthEnabled = input.BasicAuthEnabled
	route.BasicAuthUsername, route.BasicAuthPassword = input.BasicAuthUsername, input.BasicAuthPassword
	return nil
}

func setSiteAuthCredentials(route *model.ProxyRoute, auth SiteAuthInput, input *Input) error {
	if route.BasicAuthEnabled || route.SiteAuthTTLHours > 0 {
		input.BasicAuthUsername, input.BasicAuthPassword = route.BasicAuthUsername, route.BasicAuthPassword
	}
	if auth.Username != nil {
		input.BasicAuthUsername = *auth.Username
	}
	if auth.Password != nil {
		input.BasicAuthPassword = *auth.Password
	}
	if strings.ContainsAny(input.BasicAuthUsername, "\x00\r\n") || strings.ContainsAny(input.BasicAuthPassword, "\x00\r\n") ||
		(input.BasicAuthEnabled && strings.Contains(input.BasicAuthUsername, ":")) {
		return errors.New("invalid authentication credentials")
	}
	return nil
}
