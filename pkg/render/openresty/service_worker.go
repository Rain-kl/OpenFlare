// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SW location strings and Lua module paths used by the Service Worker offline fallback.
const (
	SWJSLocation      = "location = /sw.js"
	SWOfflineLocation = "location = /offline.html"
	SWChallengeLua    = "sw/challenge.lua"
	SWRuntimeLua      = "sw/runtime.lua"
	swDirPrefix       = "sw/"
)

// DefaultSWOfflineHTML is the built-in contact page shown when the domain is blocked.
const DefaultSWOfflineHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>网站暂时无法访问 | 联系站长</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #ffffff; color: #333333; height: 100vh; display: flex; flex-direction: column; justify-content: center; align-items: center; text-align: center; padding: 48px 24px; }
  h1 { font-size: 28px; font-weight: 700; margin-bottom: 16px; }
  p { font-size: 16px; line-height: 1.7; color: #666666; max-width: 520px; }
</style>
</head>
<body>
<h1>网站暂时无法访问</h1>
<p>当前域名暂时无法从网络访问。请通过其他方式联系网站管理员获取最新访问入口。</p>
</body>
</html>
`

// EffectiveSWOfflineHTML returns custom HTML when set, otherwise the built-in default.
func EffectiveSWOfflineHTML(cfg ConfigSnapshot) string {
	if strings.TrimSpace(cfg.SWOfflineHTML) == "" {
		return DefaultSWOfflineHTML
	}
	return cfg.SWOfflineHTML
}

// ServiceWorkerSupportFiles returns the sw.js script and offline contact page.
// The sw.js content is derived from the offline HTML (see defaultSWJS) so that
// HTML-only edits change the script, forcing browsers to re-install the worker
// and re-cache the updated page.
func ServiceWorkerSupportFiles(cfg ConfigSnapshot) []SupportFile {
	if !cfg.SWOfflineEnabled {
		return nil
	}
	html := EffectiveSWOfflineHTML(cfg)
	return []SupportFile{
		{Path: swDirPrefix + "sw.js", Content: defaultSWJS(html)},
		{Path: swDirPrefix + "offline.html", Content: html},
	}
}

// swJSTemplate is the service worker body. The cache name is replaced with a
// version derived from the offline HTML: editing the HTML changes the cache
// name, which changes the sw.js bytes, which makes the browser re-install the
// worker (sw.js is served with Cache-Control: no-cache) and fetch the new
// /offline.html into the fresh cache during install.
const swJSTemplate = `var CACHE = "__CACHE_NAME__";
var OFFLINE = "/offline.html";
self.addEventListener("install", function (e) {
  e.waitUntil(caches.open(CACHE).then(function (c) { return c.addAll([OFFLINE]); }));
  self.skipWaiting();
});
self.addEventListener("activate", function (e) {
  e.waitUntil(caches.keys().then(function (keys) {
    return Promise.all(keys.filter(function (k) { return k.indexOf("openflare-offline-") === 0 && k !== CACHE; }).map(function (k) { return caches.delete(k); }));
  }));
  self.clients.claim();
});
self.addEventListener("fetch", function (e) {
  if (e.request.method !== "GET" || e.request.mode !== "navigate") { return; }
  e.respondWith(
    fetch(e.request).catch(function () {
      return caches.match(e.request).then(function (r) { return r || caches.match(OFFLINE); });
    })
  );
});
`

func defaultSWJS(offlineHTML string) string {
	sum := sha256.Sum256([]byte(offlineHTML))
	version := hex.EncodeToString(sum[:])[:12]
	return strings.ReplaceAll(swJSTemplate, "__CACHE_NAME__", "openflare-offline-"+version)
}

// routeSWEnabled returns true when SW offline fallback applies to this route.
func routeSWEnabled(routeDomains []string, cfg ConfigSnapshot) bool {
	if !cfg.SWOfflineEnabled || len(cfg.SWOfflineDomains) == 0 {
		return false
	}
	scope := make(map[string]struct{}, len(cfg.SWOfflineDomains))
	for _, d := range cfg.SWOfflineDomains {
		scope[d] = struct{}{}
	}
	for _, d := range routeDomains {
		if _, ok := scope[d]; ok {
			return true
		}
	}
	return false
}

// renderServiceWorkerChallenger emits SW static locations and the homepage
// challenge intercept for HTTPS server blocks.
func renderServiceWorkerChallenger(_ ConfigSnapshot) string {
	var builder strings.Builder
	builder.WriteString("\n    location = /sw.js {\n")
	builder.WriteString("        alias " + SWDirPlaceholder + "/sw.js;\n")
	builder.WriteString("        default_type application/javascript;\n")
	builder.WriteString("        add_header Service-Worker-Allowed /;\n")
	builder.WriteString("        add_header Cache-Control \"no-cache\";\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    location = /offline.html {\n")
	builder.WriteString("        alias " + SWDirPlaceholder + "/offline.html;\n")
	builder.WriteString("        default_type text/html;\n")
	builder.WriteString("        add_header Cache-Control \"no-cache\";\n")
	builder.WriteString("    }\n\n")
	builder.WriteString("    location = /__openflare_sw_challenge {\n")
	builder.WriteString("        internal;\n")
	builder.WriteString("        # hit when sw.runtime.check() intercepts the homepage in the access phase\n")
	builder.WriteString("        content_by_lua_file " + SWDirPlaceholder + "/challenge.lua;\n")
	builder.WriteString("    }\n")
	return builder.String()
}
