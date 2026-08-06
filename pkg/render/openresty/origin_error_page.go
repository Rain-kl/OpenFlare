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

// DefaultOriginErrorPageHTML is the built-in Cloudflare-style origin error page.
// Placeholders {{status}} and {{host}} are substituted at request time by Lua.
const DefaultOriginErrorPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{status}} | 源站暂时无法提供服务</title>
<style>
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif;
    background: #f6f7f9;
    color: #1f2937;
  }
  @media (prefers-color-scheme: dark) {
    body { background: #0f1419; color: #e5e7eb; }
    .card { background: #1a2332; border-color: #2d3a4d; box-shadow: none; }
    .status { color: #f3f4f6; }
    .muted { color: #9ca3af; }
    .host { background: #243044; color: #d1d5db; }
  }
  .card {
    width: min(32rem, calc(100% - 2rem));
    padding: 2.5rem 2rem;
    border-radius: 12px;
    background: #fff;
    border: 1px solid #e5e7eb;
    box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
    text-align: center;
  }
  .status {
    margin: 0;
    font-size: clamp(3.5rem, 12vw, 5.5rem);
    font-weight: 700;
    letter-spacing: -0.04em;
    line-height: 1;
    color: #111827;
  }
  h1 {
    margin: 1rem 0 0.5rem;
    font-size: 1.25rem;
    font-weight: 600;
  }
  p { margin: 0.4rem 0; line-height: 1.6; }
  .muted { color: #6b7280; font-size: 0.95rem; }
  .host {
    display: inline-block;
    margin-top: 1.25rem;
    padding: 0.35rem 0.75rem;
    border-radius: 999px;
    background: #f3f4f6;
    color: #374151;
    font-size: 0.85rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    word-break: break-all;
  }
</style>
</head>
<body>
  <main class="card" role="main">
    <p class="status" aria-label="HTTP status">{{status}}</p>
    <h1>源站暂时无法提供服务</h1>
    <p class="muted">网关已拦截源站错误响应。请稍后重试；若问题持续，请联系站点管理员。</p>
    <p class="host">{{host}}</p>
  </main>
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
