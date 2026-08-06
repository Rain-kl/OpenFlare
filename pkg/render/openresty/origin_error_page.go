package openresty

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// OriginErrorPageSupportPath is the SupportFile path for the origin error HTML template.
	OriginErrorPageSupportPath = "error_pages/origin_error.html.tmpl"

	// OriginErrorPageInternalLocation is the internal nginx location that serves the error body.
	OriginErrorPageInternalLocation = "/__openflare_origin_error"

	defaultOriginErrorPageStatusTag = "500-599"
)

// DefaultOriginErrorPageHTML is the built-in default (aligned with frontend minimalist).
// Placeholders {{status}} and {{host}} are substituted at request time by Lua.
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

func effectiveOriginErrorPageStatusTags(cfg ConfigSnapshot) []string {
	if len(cfg.OriginErrorPageStatusCodes) == 0 {
		return []string{defaultOriginErrorPageStatusTag}
	}
	return cfg.OriginErrorPageStatusCodes
}

func originErrorPageSupportFile(cfg ConfigSnapshot) SupportFile {
	return SupportFile{
		Path:    OriginErrorPageSupportPath,
		Content: EffectiveOriginErrorPageHTML(cfg),
	}
}

func renderOriginErrorPageIntercept(cfg ConfigSnapshot) string {
	if !cfg.OriginErrorPageEnabled {
		return ""
	}
	if _, err := ExpandStatusCodeTags(effectiveOriginErrorPageStatusTags(cfg)); err != nil {
		return ""
	}
	return "        proxy_intercept_errors on;\n"
}

// renderOriginErrorPageServerBits emits server-level error_page + internal location.
// Returns empty string when disabled, expand fails, or no codes remain.
func renderOriginErrorPageServerBits(cfg ConfigSnapshot) string {
	if !cfg.OriginErrorPageEnabled {
		return ""
	}
	codes, err := ExpandStatusCodeTags(effectiveOriginErrorPageStatusTags(cfg))
	if err != nil || len(codes) == 0 {
		return ""
	}
	parts := make([]string, len(codes))
	for i, code := range codes {
		parts[i] = strconv.Itoa(code)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "    error_page %s = %s;\n", strings.Join(parts, " "), OriginErrorPageInternalLocation)
	builder.WriteString(renderOriginErrorPageInternalLocation())
	return builder.String()
}

func renderOriginErrorPageInternalLocation() string {
	return fmt.Sprintf(`    location = %s {
        internal;
        default_type text/html;
        charset utf-8;
        content_by_lua_block {
            local f = io.open("%s", "r")
            if not f then
                ngx.status = ngx.status
                ngx.say("Error ", ngx.status)
                return
            end
            local body = f:read("*a")
            f:close()
            local status = tostring(ngx.status)
            local host = ngx.var.host or ""
            body = body:gsub("{{status}}", status, 1)
            body = body:gsub("{{host}}", host, 1)
            body = body:gsub("{{status}}", status)
            body = body:gsub("{{host}}", host)
            ngx.header["Content-Type"] = "text/html; charset=utf-8"
            ngx.say(body)
        }
    }
`, OriginErrorPageInternalLocation, ErrorPageTmplPlaceholder)
}
