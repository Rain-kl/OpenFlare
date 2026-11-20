// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package edgeconfig

import "strings"

// OriginErrorPageSupportPath identifies the published origin error HTML asset.
const OriginErrorPageSupportPath = "error_pages/origin_error.html.tmpl"

// DefaultOriginErrorPageHTML is the built-in default (aligned with frontend minimalist).
// Placeholders {{status}} and {{host}} are substituted by the edge runtime.
const DefaultOriginErrorPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{status}} | OpenFlare</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: #ffffff;
      color: #333333;
      height: 100vh;
      display: flex;
      flex-direction: column;
      justify-content: center;
      align-items: center;
      text-align: center;
      padding: 48px 24px;
      -webkit-font-smoothing: antialiased;
    }
    .container {
      max-width: 600px;
      width: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 24px;
    }
    .error-code {
      font-size: 48px;
      font-weight: 700;
      color: #333333;
      line-height: 1.2;
      letter-spacing: -0.02em;
    }
    .error-description {
      font-size: 20px;
      line-height: 1.6;
      color: #666666;
      max-width: 480px;
    }
    .host {
      font-size: 14px;
      color: #999999;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      word-break: break-all;
    }
    .footer {
      margin-top: 48px;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      color: #999999;
      font-size: 14px;
      font-weight: 500;
    }
    .brand-icon { width: 24px; height: 24px; fill: currentColor; display: block; }
    @media (max-width: 480px) {
      .error-code { font-size: 36px; }
      .error-description { font-size: 18px; }
    }
  </style>
</head>
<body>
<div class="container">
  <h1 class="error-code" aria-label="HTTP status">{{status}}</h1>
  <p class="error-description">
    The upstream server is unreachable. Please try again later or contact the site administrator if the problem persists.
  </p>
  <p class="host">{{host}}</p>
  <div class="footer">
    <svg class="brand-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M13 2L3 14H12L11 22L21 10H12L13 2Z" />
    </svg>
    <span>OpenFlare</span>
  </div>
</div>
</body>
</html>
`

// EffectiveOriginErrorPageHTML returns custom HTML when set, otherwise the built-in default.
func EffectiveOriginErrorPageHTML(cfg ConfigSnapshot) string {
	if strings.TrimSpace(cfg.OriginErrorPageHTML) == "" {
		return DefaultOriginErrorPageHTML
	}
	return cfg.OriginErrorPageHTML
}
