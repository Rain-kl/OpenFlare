// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pow"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/proxy_route"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
)

// SiteSaved confirms a site update saved on the controller.
type SiteSaved struct {
	SiteID string `json:"site_id"`
	State  string `json:"state"`
}

// SiteSettings describes current proxy settings without authentication or header secrets.
type SiteSettings struct {
	Site              SiteSummary                 `json:"site"`
	HTTP2Enabled      bool                        `json:"http2_enabled"`
	RedirectHTTP      bool                        `json:"redirect_http"`
	CacheEnabled      bool                        `json:"cache_enabled"`
	CachePolicy       string                      `json:"cache_policy"`
	CacheRules        []string                    `json:"cache_rules"`
	CustomHeaderNames []string                    `json:"custom_header_names"`
	Limits            proxy_route.SiteLimitsPatch `json:"limits"`
	AuthMode          string                      `json:"auth_mode"`
	AuthTTLHours      int                         `json:"auth_ttl_hours,omitempty"`
}

// SiteAuthBody sets authentication mode and optional credentials.
type SiteAuthBody struct {
	// Authentication mode. Site login uses a browser session and requires HTTPS.
	Mode string `json:"mode" binding:"required" enums:"off,basic,site"`
	// Username. Omit to preserve an existing credential.
	Username *string `json:"username,omitempty" maxLength:"255"`
	// Password. Omit to preserve an existing credential. Never returned by tools.
	Password *string `json:"password,omitempty" maxLength:"255"`
	// Session lifetime in hours, for site mode only. Omit to retain an existing lifetime.
	TTLHours *int `json:"ttl_hours,omitempty" minimum:"1" maximum:"720"`
}

// SitePoWBody toggles independent PoW and optionally replaces its parameters.
type SitePoWBody struct {
	// Whether to enable the site's independent proof-of-work challenge.
	Enabled bool `json:"enabled" binding:"required"`
	// Complete replacement of challenge settings. Omit to retain settings or use defaults on first configuration.
	Config *edgeconfig.PoWConfig `json:"config,omitempty"`
}

// SitePoW reports a site's independent PoW settings.
type SitePoW struct {
	SiteID  string               `json:"site_id"`
	Enabled bool                 `json:"enabled"`
	Config  edgeconfig.PoWConfig `json:"config"`
}

// @ID openflare_site_get_settings
// @Summary Read website reverse proxy and authentication settings
// @Description Read the selected site's proxy settings, configured limits, and authentication mode. Header values, credentials, and signing keys are omitted. Use openflare_site_list to resolve a site ID.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Success 200 {object} OperationResponse{data=SiteSettings}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"site get-settings","scope":"sites:read","effect":"read","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/sites/{site_id}/settings [get]
func getSiteSettings(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	view, err := proxy_route.GetProxyRoute(ctx, id)
	if err != nil {
		return nil, resourceFailure(ctx, "get site settings", err)
	}
	result := SiteSettings{
		Site: summarizeSite(view), HTTP2Enabled: view.EnableHTTP2, RedirectHTTP: view.RedirectHTTP,
		CacheEnabled: view.CacheEnabled, CachePolicy: view.CachePolicy, CacheRules: view.CacheRuleList,
		CustomHeaderNames: make([]string, 0, len(view.CustomHeaderList)), AuthMode: "off",
		Limits: proxy_route.SiteLimitsPatch{Connections: &view.LimitConnPerServer, ConnectionsPerIP: &view.LimitConnPerIP, Rate: &view.LimitRate, RequestsPerIP: &view.LimitReqPerIP},
	}
	for _, header := range view.CustomHeaderList {
		result.CustomHeaderNames = append(result.CustomHeaderNames, header.Key)
	}
	if view.SiteAuthTTLHours > 0 {
		result.AuthMode, result.AuthTTLHours = "site", view.SiteAuthTTLHours
	} else if view.BasicAuthEnabled {
		result.AuthMode = "basic"
	}
	return result, nil
}

// @ID openflare_site_patch_settings
// @Summary Update reverse proxy settings and limits for one website
// @Description Change only the supplied fields, including HTTPS, HTTP/2, redirection, caching, response headers and rate limits. Use openflare_site_set_upstreams for upstream changes. The result confirms controller storage.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Param body body proxy_route.SettingsPatch true "Partial reverse proxy settings"
// @Success 200 {object} OperationResponse{data=SiteSaved}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"site patch-settings","scope":"sites:write","effect":"write","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"},"cache_rules":{"minItems":0,"maxItems":256},"custom_headers":{"minItems":0,"maxItems":64}}}
// @Router /api/v1/automation/sites/{site_id}/settings [patch]
func patchSiteSettings(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	var patch proxy_route.SettingsPatch
	if err := decodeBodyFields("openflare_site_patch_settings", input, &patch); err != nil {
		return nil, invalidArguments(err)
	}
	return siteSaved(ctx, id, "patch site settings", proxy_route.PatchSettings(ctx, id, patch))
}

// @ID openflare_site_set_auth
// @Summary Set a website's authentication mode
// @Description Configure HTTP Basic authentication, HTTPS site login, or disable authentication. For enabled modes, omit credentials to keep them or supply both for first setup. Disabling clears the stored credentials. Credentials are not returned.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Param body body SiteAuthBody true "Authentication settings"
// @Success 200 {object} OperationResponse{data=SiteSaved}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"site set-auth","scope":"security:write","effect":"write","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/sites/{site_id}/auth [patch]
func setSiteAuth(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	var body SiteAuthBody
	if err := decodeBodyFields("openflare_site_set_auth", input, &body); err != nil {
		return nil, invalidArguments(err)
	}
	update := proxy_route.SiteAuthInput{Mode: body.Mode, Username: body.Username, Password: body.Password, TTLHours: body.TTLHours}
	return siteSaved(ctx, id, "set site authentication", proxy_route.SetSiteAuth(ctx, id, update))
}

// @ID openflare_site_get_pow
// @Summary Read a website's independent PoW policy
// @Description Read challenge settings for the selected website. An unconfigured website returns disabled defaults. WAF flow PoW nodes are configured separately.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Success 200 {object} OperationResponse{data=SitePoW}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"site get-pow","scope":"security:read","effect":"read","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/sites/{site_id}/pow [get]
func getSitePoW(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	policy, err := pow.GetPolicy(ctx, id)
	if err != nil {
		return nil, resourceFailure(ctx, "get site PoW", err)
	}
	return SitePoW{SiteID: decimalID(id), Enabled: policy.Enabled, Config: policy.Config}, nil
}

// @ID openflare_site_set_pow
// @Summary Configure independent PoW for one website
// @Description Enable or disable the website's challenge. Omit config to keep the existing settings or use defaults on first setup. To edit parameters, read the current config and submit a full replacement. WAF flow bindings are preserved.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param site_id path string true "Site record ID from openflare_site_list"
// @Param body body SitePoWBody true "Independent proof-of-work settings"
// @Success 200 {object} OperationResponse{data=SiteSaved}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"site set-pow","scope":"security:write","effect":"write","parameters":{"site_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/sites/{site_id}/pow [patch]
func setSitePoW(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "site_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	var body SitePoWBody
	if err := decodeBodyFields("openflare_site_set_pow", input, &body); err != nil {
		return nil, invalidArguments(err)
	}
	err = pow.SetPolicy(ctx, id, body.Enabled, body.Config)
	if validation := new(pow.ValidationError); errors.As(err, &validation) {
		return nil, invalidArguments(validation)
	}
	return siteSaved(ctx, id, "set site PoW", err)
}

func siteSaved(ctx context.Context, id uint, operation string, err error) (any, *operationError) {
	if err != nil {
		if errors.Is(err, proxy_route.ErrInvalidSiteSettings) {
			return nil, invalidArguments(err)
		}
		return nil, siteFailure(ctx, operation, err)
	}
	return SiteSaved{SiteID: decimalID(id), State: savedState}, nil
}
