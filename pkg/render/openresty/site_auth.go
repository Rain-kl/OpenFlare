// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const siteAuthPath = "/.openflare/site-auth"
const siteAuthPagePath = "site-auth/index.html"
const siteAuthRuntimePath = "site-auth/runtime.lua"

//go:embed site_auth.html
var defaultSiteAuthPage string

//go:embed site_auth.lua
var siteAuthRuntime string

func siteAuthEnabled(routes []Route) bool {
	for _, route := range routes {
		if route.SiteAuthTTLHours != 0 {
			return true
		}
	}
	return false
}

func validateSiteAuth(route Route) error {
	if route.SiteAuthTTLHours == 0 {
		return nil
	}
	key, err := hex.DecodeString(route.SiteAuthKey)
	if route.SiteAuthTTLHours < 1 || route.SiteAuthTTLHours > 720 || !route.EnableHTTPS ||
		route.BasicAuthEnabled || route.BasicAuthUsername == "" || route.BasicAuthPassword == "" ||
		err != nil || len(key) != 32 {
		return fmt.Errorf("site %s has invalid site authentication configuration", route.SiteName)
	}
	return nil
}

func siteAuthSupportFiles(html string) []SupportFile {
	if strings.TrimSpace(html) == "" {
		html = defaultSiteAuthPage
	}
	return []SupportFile{
		{Path: siteAuthPagePath, Content: html},
		{Path: siteAuthRuntimePath, Content: siteAuthRuntime},
	}
}

// The Agent installs both files through the existing response-page bundle.
// Lua caches the module and HTML per worker, so the request path needs no file I/O after warmup.
func renderSiteAuthServer(route Route) string {
	if route.SiteAuthTTLHours == 0 {
		return ""
	}
	credentials := siteAuthCredentialDigest(route)
	return fmt.Sprintf(`
    add_header Cache-Control "private, no-store" always;
    server_rewrite_by_lua_block {
        local auth = package.loaded["openflare.site_auth"]
        if not auth then
            auth = dofile("%s/%s")
            package.loaded["openflare.site_auth"] = auth
        end
        auth.check("%s")
    }
    location = %s {
        limit_req zone=openflare_site_auth burst=10 nodelay;
        limit_req_status 429;
        client_max_body_size 8k;
        client_body_buffer_size 8k;
        access_by_lua_file %s/waf/check.lua;
        content_by_lua_block {
            package.loaded["openflare.site_auth"].login("%s", %d, "%s", "%s/%s")
        }
    }
`, CertDirPlaceholder, siteAuthRuntimePath, route.SiteAuthKey, siteAuthPath, LuaDirPlaceholder,
		route.SiteAuthKey, int((time.Duration(route.SiteAuthTTLHours)*time.Hour)/time.Second), credentials, CertDirPlaceholder, siteAuthPagePath)
}

func siteAuthCredentialDigest(route Route) string {
	digest := sha256.Sum256([]byte(route.SiteAuthKey + "\x00" + route.BasicAuthUsername + "\x00" + route.BasicAuthPassword))
	return hex.EncodeToString(digest[:])
}

func renderSiteAuthForwardCookie(route Route, headers []CustomHeader) string {
	if route.SiteAuthTTLHours == 0 {
		return ""
	}
	for _, header := range headers {
		if strings.EqualFold(header.Key, "Cookie") {
			return ""
		}
	}
	return siteAuthForwardCookie
}

const siteAuthForwardCookie = `        set_by_lua_block $openflare_forward_cookie {
            return package.loaded["openflare.site_auth"].forward_cookie()
        }
        proxy_set_header Cookie $openflare_forward_cookie;
`
