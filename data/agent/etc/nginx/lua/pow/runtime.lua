local _M = {}

function _M.check()
local source = debug.getinfo(1, "S").source or ""
if string.sub(source, 1, 1) == "@" then
    local script_path = string.sub(source, 2)
    local base_dir = string.match(script_path, "^(.*)/pow/[^/]+%.lua$")
    if base_dir and base_dir ~= "" and not string.find(package.path, base_dir, 1, true) then
        package.path = base_dir .. "/?.lua;" .. base_dir .. "/?/init.lua;" .. package.path
    end
end

local cjson = require "cjson.safe"
local policy = require "pow.policy"

local pow_config_dict = ngx.shared.openflare_pow_config
local pow_sessions = ngx.shared.openflare_pow_sessions

local function session_cookie(value, ttl)
    local cookie = "__openflare_pow=" .. value .. "; Path=/; HttpOnly; SameSite=Lax; Max-Age=" .. tostring(ttl)
    if ngx.var.scheme == "https" then
        cookie = cookie .. "; Secure"
    end
    return cookie
end

-- Lazy-load pow_config from file; reload when content changes
local function load_pow_config()
    local config_paths = {
        "/data/etc/openflare/waf_config.json",
        "/etc/nginx/openflare-lua/waf_config.json",
        "/usr/local/openresty/nginx/conf/waf_config.json"
    }
    for _, config_path in ipairs(config_paths) do
        local f = io.open(config_path, "r")
        if f then
            local content = f:read("*a")
            f:close()
            local current_hash = ngx.md5(content or "")

            if current_hash == pow_config_dict:get("_config_hash") then
                return
            end

            -- Clear old domain/site entries
            local old_keys = pow_config_dict:get("_domain_keys")
            if old_keys then
                for domain in string.gmatch(old_keys, "[^\n]+") do
                    pow_config_dict:delete(domain)
                end
            end

            local domain_keys = {}
            if content and content ~= "" and content ~= "{}" then
                local ok, decoded = pcall(cjson.decode, content)
                if ok and decoded and decoded.rule_groups and decoded.site_rule_groups then
                    -- Build rule groups map (group ID -> PoWConfig)
                    local groups = {}
                    for _, group in ipairs(decoded.rule_groups) do
                        if group.pow_enabled then
                            groups[tostring(group.id)] = group.pow_config
                        end
                    end
                    -- Build site name to pow_config map
                    for site, group_ids in pairs(decoded.site_rule_groups) do
                        local pow_config = nil
                        -- Check custom group IDs first
                        for _, id in ipairs(group_ids) do
                            pow_config = groups[tostring(id)]
                            if pow_config then
                                break
                            end
                        end
                        -- If not found, check global group IDs
                        if not pow_config then
                            for _, group in ipairs(decoded.rule_groups) do
                                if group.is_global and group.pow_enabled then
                                    pow_config = group.pow_config
                                    break
                                end
                            end
                        end
                        if pow_config then
                            pow_config_dict:set(site, cjson.encode({enabled = true, config = pow_config}), 0)
                            domain_keys[#domain_keys+1] = site
                        end
                    end
                end
            end

            pow_config_dict:set("_domain_keys", table.concat(domain_keys, "\n"), 0)
            pow_config_dict:set("_config_hash", current_hash, 0)
            return
        end
    end
end

load_pow_config()

local host = ngx.var.host
if not host or host == "" then
    return
end

local site = ngx.var.openflare_waf_site or ""
if site == "" then
    site = host
end

local config_raw = pow_config_dict:get(site)
if not config_raw then
    return
end

local ok, route_config = pcall(cjson.decode, config_raw)
if not ok or not route_config then
    return
end

if not route_config.enabled then
    return
end

local config = route_config.config or {}
local session_ttl = config.session_ttl or 600
local uri = ngx.var.uri or ""
local ua = ngx.var.http_user_agent or ""
local remote_ip = ngx.var.remote_addr or ""

-- Check whitelist: if matched, skip PoW
local whitelist = config.whitelist or {}
if policy.match_any(remote_ip, ua, uri, whitelist) then
    return
end

-- Check blacklist: if matched, require PoW
local blacklist = config.blacklist or {}
local has_blacklist = policy.has_entries(blacklist)
local need_pow = false
if has_blacklist then
    need_pow = policy.match_any(remote_ip, ua, uri, blacklist)
else
    -- No blacklist means all non-whitelisted need PoW
    need_pow = true
end

if not need_pow then
    return
end

-- Check valid session cookie
local cookie_val = ngx.var["cookie___openflare_pow"]
if cookie_val and cookie_val ~= "" then
    local session_key = host .. ":" .. cookie_val
    local session_data = pow_sessions:get(session_key)
    if session_data then
        pow_sessions:set(session_key, "1", session_ttl)
        ngx.header["Set-Cookie"] = session_cookie(cookie_val, session_ttl)
        return
    end
end

-- If requesting the challenge API endpoints, let them through (handled by content_by_lua)
local anubis_api_prefix = "/.within.website/x/cmd/anubis/api/"
local anubis_static_prefix = "/.within.website/x/cmd/anubis/static/"
if string.sub(uri, 1, #anubis_api_prefix) == anubis_api_prefix then
    return
end
if string.sub(uri, 1, #anubis_static_prefix) == anubis_static_prefix then
    return
end

-- Render the challenge page through an internal redirect so the browser stays
-- on the originally requested URL instead of seeing a 302 hop.
ngx.req.set_uri_args({
    redir = ngx.var.scheme .. "://" .. host .. uri .. (ngx.var.args and ("?" .. ngx.var.args) or ""),
    host = host
})
return ngx.exec("/.within.website/x/cmd/anubis/api/make-challenge")
end

return _M
