// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package edgeconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const swDirPrefix = "sw/"

const defaultSWOfflineTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Website unavailable | Contact the administrator</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #ffffff; color: #333333; height: 100vh; display: flex; flex-direction: column; justify-content: center; align-items: center; text-align: center; padding: 48px 24px; }
  h1 { font-size: 28px; font-weight: 700; margin-bottom: 16px; }
  p { font-size: 16px; line-height: 1.7; color: #666666; max-width: 520px; }
  footer { display: flex; align-items: center; gap: 8px; margin-top: 32px; color: #000; font-size: 14px; }
</style>
</head>
<body>
<h1>Website temporarily unavailable</h1>
<p>This domain is temporarily unreachable. Contact the website administrator through another channel for the latest access address.</p>
<footer><!-- openflare-brand -->OpenFlare</footer>
</body>
</html>
`

// DefaultSWOfflineHTML is the built-in contact page shown when the domain is blocked.
var DefaultSWOfflineHTML = withBrandMark(defaultSWOfflineTemplate)

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
