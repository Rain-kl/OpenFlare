local cjson = require "cjson.safe"
local args = ngx.req.get_uri_args()
local pow_config_dict = ngx.shared.openflare_pow_config
local config = ngx.ctx.openflare_pow_config
local config_key = args.openflare_pow_config_key
if type(config_key) == "string" and config_key ~= "" then
    config = cjson.decode(pow_config_dict:get(config_key) or "")
    pow_config_dict:delete(config_key)
end
if type(config) ~= "table" then
    ngx.status = 403
    ngx.say("PoW policy was not evaluated for this request")
    return
end

local host = ngx.var.host or ""
local redir = type(args.redir) == "string" and args.redir or "/"
local entropy = table.concat({tostring(ngx.now()), tostring(ngx.worker.pid()), tostring(math.random()),
    ngx.var.remote_addr or "", ngx.var.http_user_agent or "", ngx.var.request_id or ""}, ":")
local id = ngx.md5(entropy .. ":id")
local data = ngx.md5(entropy .. ":data-a") .. ngx.md5(entropy .. ":data-b")
local session_ttl = config.session_ttl or 600
local stored, err = ngx.shared.openflare_pow_challenges:set(id, cjson.encode({
    data = data, difficulty = config.difficulty or 4, host = host,
    site = ngx.var.openflare_waf_site or "", redir = redir, session_ttl = session_ttl,
}), config.challenge_ttl or 300)
if not stored then
    ngx.log(ngx.ERR, "openflare pow challenge storage failed: ", err)
    ngx.status = 503
    ngx.say("Verification is temporarily unavailable")
    return
end

local page = {html = cjson.decode([====[__POW_DEFAULT_HTML__]====]), widget = "progress"}
local file = io.open("__OPENFLARE_POW_PAGE_CONFIG__", "r")
if file then
    local raw = file:read("*a")
    file:close()
    local decoded = cjson.decode(raw)
    if type(decoded) == "table" and type(decoded.html) == "string" then page = decoded end
end
local lang = string.find(string.lower(ngx.var.http_accept_language or ""), "zh", 1, true) and "zh-CN" or "en"
local translations = {
    en = cjson.decode([====[__POW_EN__]====]),
    ["zh-CN"] = cjson.decode([====[__POW_ZH__]====]),
}
local text = translations[lang]
local function escape(value)
    return (string.gsub(tostring(value or ""), "[&<>\"']", {
        ["&"] = "&amp;", ["<"] = "&lt;", [">"] = "&gt;", ['"'] = "&quot;", ["'"] = "&#39;",
    }))
end
local widget = page.widget == "turnstile" and "turnstile" or "progress"
local values = {lang = lang, host = host}
for key, value in pairs(text) do values[key] = value end
local html = string.gsub(page.html, "{{([%w_]+)}}", function(key)
    if key == "widget" then return '<openflare-pow appearance="' .. widget .. '"></openflare-pow>' end
    return values[key] and escape(values[key]) or "{{" .. key .. "}}"
end)
local boot = cjson.encode({id = id, randomData = data, difficulty = config.difficulty or 4,
    algorithm = config.algorithm or "fast", challengeTTL = config.challenge_ttl or 300, lang = lang, widget = widget})
-- JSON is embedded in a script element; request values must never close it.
boot = string.gsub(boot, "<", "\\u003c")
local runtime = '<script id="openflare-pow-config" type="application/json">' .. boot .. '</script>' ..
    '<script type="module" src="/.within.website/x/cmd/anubis/static/js/main.mjs"></script>'
local count
html, count = string.gsub(html, "</[bB][oO][dD][yY]%s*>", function() return runtime .. "</body>" end, 1)
if count == 0 then html = html .. runtime end
ngx.header.content_type = "text/html; charset=utf-8"
ngx.header["Cache-Control"] = "no-store"
ngx.say(html)
