package nginx

import (
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

const openRestySWRuntimeLua = `local _M = {}

local source = debug.getinfo(1, "S").source or ""
if string.sub(source, 1, 1) == "@" then
    local script_path = string.sub(source, 2)
    local base_dir = string.match(script_path, "^(.*)/sw/[^/]+%.lua$")
    if base_dir and base_dir ~= "" and not string.find(package.path, base_dir, 1, true) then
        package.path = base_dir .. "/?.lua;" .. base_dir .. "/?/init.lua;" .. package.path
    end
end

local function is_real_browser(ua)
    if not ua or ua == "" then return false end
    -- Chrome/Edge/CentOS-style: "Chrome/120" (pattern mode: %d = digit)
    if string.find(ua, "Chrome/%d", 1) then return true end
    -- Firefox: "Firefox/120"
    if string.find(ua, "Firefox/%d", 1) then return true end
    -- Safari (non-Chrome, e.g. "Version/17.0 Safari")
    if not string.find(ua, "Chrome", 1, true) and string.find(ua, "Safari", 1, true) then return true end
    return false
end

local function pass_through()
    return true
end

function _M.check()
    local ua = ngx.var.http_user_agent or ""
    if not is_real_browser(ua) then return pass_through() end

    local uri = ngx.var.uri or ""
    if uri ~= "/" then return pass_through() end

    if ngx.req.get_method and ngx.req.get_method() ~= "GET" then return pass_through() end

    local cookie = ngx.var["cookie___openflare_sw"]
    if cookie and cookie ~= "" then return pass_through() end

    -- intercept: internal redirect to challenge page, which registers SW + sets cookie
    local redir = ngx.var.scheme .. "://" .. ngx.var.host .. uri .. (ngx.var.args and ("?" .. ngx.var.args) or "")
    ngx.req.set_uri_args({ redir = redir })
    return ngx.exec("/__openflare_sw_challenge")
end

return _M
`

const openRestySWChallengeLua = `local args = ngx.req.get_uri_args()
local redir = args["redir"] or "/"

-- Escape redir for embedding inside a JS string literal within an HTML
-- <script> element. Backslashes first so later escapes stay escaped, then
-- double quotes (string-literal break-out), then "<" (prevents a raw
-- "</script" sequence ending the element, which the HTML parser matches
-- case-insensitively), then CR/LF (a raw newline would end the literal).
local function escape_redir(value)
    local escaped = string.gsub(value, "\\", "\\\\")
    escaped = string.gsub(escaped, '"', '\\"')
    escaped = string.gsub(escaped, "<", "\\x3C")
    escaped = string.gsub(escaped, string.char(0xE2, 0x80, 0xA8), "\\u2028")
    escaped = string.gsub(escaped, string.char(0xE2, 0x80, 0xA9), "\\u2029")
    escaped = string.gsub(escaped, "\r", "\\r")
    escaped = string.gsub(escaped, "\n", "\\n")
    return escaped
end
redir = escape_redir(redir)

ngx.header.content_type = "text/html; charset=utf-8"
ngx.say([[<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>加载中...</title>
<script>
if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").then(function () {
    document.cookie = "__openflare_sw=1; Path=/; Max-Age=31536000; Secure; SameSite=Lax";
    location.replace("]] .. redir .. [[");
  }).catch(function () {
    location.replace("]] .. redir .. [[");
  });
} else {
  document.cookie = "__openflare_sw=1; Path=/; Max-Age=31536000; Secure; SameSite=Lax";
  location.replace("]] .. redir .. [[");
}
</script>
</head>
<body>正在加载...</body>
</html>]])
`

// ManagedSWLuaFiles returns embedded Lua assets for the SW offline challenge.
func ManagedSWLuaFiles() []protocol.SupportFile {
	return []protocol.SupportFile{
		{Path: "sw/runtime.lua", Content: openRestySWRuntimeLua},
		{Path: "sw/challenge.lua", Content: openRestySWChallengeLua},
	}
}
