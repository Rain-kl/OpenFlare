// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"embed"
	"encoding/json"
	"path/filepath"
	"strings"

	openresty "github.com/Rain-kl/Wavelet/internal/openflare/openresty"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
)

//go:embed pow_static
var powStaticFS embed.FS

//go:embed pow_static/locales/challenge.en.json
var powChallengeEnglish string

//go:embed pow_static/locales/challenge.zh-CN.json
var powChallengeChinese string

const openRestyPowRuntimeLua = `local _M = {}

local source = debug.getinfo(1, "S").source or ""
if string.sub(source, 1, 1) == "@" then
    local script_path = string.sub(source, 2)
    local base_dir = string.match(script_path, "^(.*)/pow/[^/]+%.lua$")
    if base_dir and base_dir ~= "" and not string.find(package.path, base_dir, 1, true) then
        package.path = base_dir .. "/?.lua;" .. base_dir .. "/?/init.lua;" .. package.path
    end
end

local policy = require "pow.policy"
local pow_sessions = ngx.shared.openflare_pow_sessions
local pow_config_dict = ngx.shared.openflare_pow_config
local cjson = require "cjson.safe"

function _M.init()
    require("pow.config")
    require("pow.page")
end

_M.prepare_config = policy.prepare

local function session_cookie(value, ttl)
    local cookie = "__openflare_pow=" .. value .. "; Path=/; HttpOnly; SameSite=Lax; Max-Age=" .. tostring(ttl)
    if ngx.var.scheme == "https" then cookie = cookie .. "; Secure" end
    return cookie
end

-- evaluate is called by a DAG pow node. true continues along its next edge;
-- false means the challenge flow has taken ownership of the request.
function _M.evaluate(config, standalone)
    if not standalone and ngx.ctx.openflare_pow_standalone then return true end
    config = config or {}
    ngx.ctx.openflare_pow_config = config

    local host = ngx.var.host
    if not host or host == "" then return true end
    local session_ttl = config.session_ttl or 600
    local uri = ngx.var.uri or ""
    local ua = ngx.var.http_user_agent or ""
    local remote_ip = ngx.var.remote_addr or ""

    if config.path_rules and not policy.should_challenge(uri, config.path_rules) then return true end

    if policy.match_any(remote_ip, ua, uri, config.whitelist or {}) then return true end
    local blacklist = config.blacklist or {}
    if policy.has_entries(blacklist) and not policy.match_any(remote_ip, ua, uri, blacklist) then return true end

    local cookie_val = ngx.var["cookie___openflare_pow"]
    if cookie_val and cookie_val ~= "" then
        local session_key = host .. ":" .. cookie_val
        if pow_sessions:get(session_key) then
            pow_sessions:set(session_key, "1", session_ttl)
            ngx.header["Set-Cookie"] = session_cookie(cookie_val, session_ttl)
            return true
        end
    end

    local api_prefix = "/.within.website/x/cmd/anubis/api/"
    local static_prefix = "/.within.website/x/cmd/anubis/static/"
    if string.sub(uri, 1, #api_prefix) == api_prefix or string.sub(uri, 1, #static_prefix) == static_prefix then
        return false
    end

    local config_key = "_request_config:" .. (ngx.var.request_id or ngx.md5(host .. uri .. tostring(ngx.now())))
    local stored, err = pow_config_dict:set(config_key, cjson.encode(config), config.challenge_ttl or 300)
    if not stored then
        ngx.log(ngx.ERR, "openflare pow request configuration storage failed: ", err)
        ngx.status = 503
        ngx.say("Verification is temporarily unavailable")
        return false
    end
    local challenge_args = {
        redir = ngx.var.request_uri or (uri .. (ngx.var.args and ("?" .. ngx.var.args) or "")),
        host = host,
        openflare_pow_config_key = config_key,
    }
    ngx.req.set_uri_args(challenge_args)
    ngx.exec("/.within.website/x/cmd/anubis/api/make-challenge", challenge_args)
    return false
end

-- Resolve standalone policies before WAF executes legacy PoW nodes.
function _M.prepare()
    local policy = require("pow.config").get(ngx.var.openflare_waf_site or "")
    ngx.ctx.openflare_pow_standalone = policy
    return policy
end

function _M.check()
    local policy = ngx.ctx.openflare_pow_standalone or _M.prepare()
    if type(policy) ~= "table" or not policy.enabled then return true end
    return _M.evaluate(policy.config, true)
end

return _M
`

/* Removed legacy request-time configuration scanner. Graph execution now calls
evaluate(config) with the reached node.
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
        "__OPENFLARE_RUNTIME_CONFIG_DIR__/waf_config.json",
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
                            groups[tostring(group.id)] = group.pow_config or {}
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
                                    pow_config = group.pow_config or {}
                                    break
                                end
                            end
                        end
                        if pow_config ~= nil then
                            pow_config_dict:set(site, cjson.encode({enabled = true, config = pow_config}), 0)
                            domain_keys[#domain_keys+1] = site
                        end
                    end
                end
            end

            pow_config_dict:set("_domain_keys", table.concat(domain_keys, "\n"), 0)
            pow_config_dict:set("_config_hash", current_hash, 0)
            return true
        end
    end

    if pow_config_dict:add("_pow_unreadable_config_logged", true, 60) then
        ngx.log(ngx.WARN, "openflare pow config is not readable by worker; check directory permissions under ", "__OPENFLARE_RUNTIME_CONFIG_DIR__")
    end
    return false
end

load_pow_config()

local host = ngx.var.host
if not host or host == "" then
    return
end

local site = ngx.var.openflare_waf_site or ""
if site == "" then
    return
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
*/

const openRestyPowCheckLua = `local source = debug.getinfo(1, "S").source or ""
if string.sub(source, 1, 1) == "@" then
    local script_path = string.sub(source, 2)
    local base_dir = string.match(script_path, "^(.*)/pow/[^/]+%.lua$")
    if base_dir and base_dir ~= "" and not string.find(package.path, base_dir, 1, true) then
        package.path = base_dir .. "/?.lua;" .. base_dir .. "/?/init.lua;" .. package.path
    end
end

return require("pow.runtime").check()
`

//go:embed pow_challenge.lua
var powChallengeLuaTemplate string

//go:embed pow_page.lua
var powPageLuaTemplate string

var openRestyPowPageLua = strings.NewReplacer(
	"__POW_EN__", powChallengeEnglish,
	"__POW_ZH__", powChallengeChinese,
	"__POW_DEFAULT_HTML__", defaultPoWPageJSON(),
).Replace(powPageLuaTemplate)

var openRestyPowChallengeLua = powChallengeLuaTemplate

func defaultPoWPageJSON() string {
	raw, _ := json.Marshal(openresty.DefaultPoWPageHTML("shield"))
	return string(raw)
}

const openRestyPowVerifyLua = `local cjson = require "cjson.safe"

local pow_challenges = ngx.shared.openflare_pow_challenges
local pow_sessions = ngx.shared.openflare_pow_sessions

local site = ngx.var.openflare_waf_site or ""
if site == "" then
    ngx.status = 403
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "PoW site not resolved; openflare_waf_site is required"}))
    return
end

local args = ngx.req.get_uri_args()
local challenge_id = args["id"] or ""
local response = args["response"] or ""
local nonce_str = args["nonce"] or ""
local redir = args["redir"] or ""
local elapsed = args["elapsedTime"] or ""

if challenge_id == "" or response == "" or nonce_str == "" then
    ngx.status = 400
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "missing parameters"}))
    return
end

local nonce = tonumber(nonce_str)
if not nonce or nonce < 0 or nonce > 9007199254740991 or nonce ~= math.floor(nonce) then
    ngx.status = 400
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "invalid nonce"}))
    return
end

-- Get stored challenge
local challenge_raw = pow_challenges:get(challenge_id)
if not challenge_raw then
    ngx.status = 410
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "challenge expired or not found"}))
    return
end

local ok, challenge_info = pcall(cjson.decode, challenge_raw)
if not ok or not challenge_info then
    ngx.status = 500
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "invalid challenge data"}))
    return
end

local challenge_data = challenge_info.data or ""
local difficulty = challenge_info.difficulty or 4
local host = challenge_info.host or ngx.var.host or ""
local session_ttl = challenge_info.session_ttl or 600

if host ~= (ngx.var.host or "") or (challenge_info.site and challenge_info.site ~= site) then
    ngx.status = 403
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "challenge site mismatch"}))
    return
end

-- Compute SHA-256(challenge_data + nonce)
local calc_string = challenge_data .. tostring(math.floor(nonce))
local calculated = ngx.sha1_bin ~= nil and "" or ""

-- Use resty.sha256 for proper SHA-256
local sha256 = require "resty.sha256"
local str = require "resty.string"
local hasher = sha256:new()
hasher:update(calc_string)
local hash_bytes = hasher:final()
local hash_hex = str.to_hex(hash_bytes)

-- Verify hash matches response
if hash_hex ~= string.lower(response) then
    ngx.status = 403
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "hash mismatch"}))
    return
end

-- Verify difficulty (leading zeros in hex)
local prefix = string.rep("0", difficulty)
if string.sub(hash_hex, 1, difficulty) ~= prefix then
    ngx.status = 403
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "insufficient difficulty"}))
    return
end

-- Invalidate challenge (prevent replay)
pow_challenges:delete(challenge_id)

-- Generate session token
local session_token = str.to_hex(ngx.sha1_bin(challenge_id .. ngx.now() .. tostring(ngx.worker.pid())))

-- Store session
pow_sessions:set(host .. ":" .. session_token, "1", session_ttl)

-- Set cookie. Secure cookies are not sent over HTTP, so only add Secure when
-- the current request itself is HTTPS.
local cookie = "__openflare_pow=" .. session_token .. "; Path=/; HttpOnly; SameSite=Lax; Max-Age=" .. tostring(session_ttl)
if ngx.var.scheme == "https" then
    cookie = cookie .. "; Secure"
end
ngx.header["Set-Cookie"] = cookie

if redir ~= "" then
    local target = challenge_info.redir or "/"
    if string.sub(target, 1, 1) ~= "/" or string.sub(target, 1, 2) == "//" or string.find(target, "[\\\r\n]") then target = "/" end
    return ngx.redirect(target)
end

ngx.header.content_type = "application/json"
ngx.say(cjson.encode({ok = true}))
`

const openRestyPowPolicyLua = `local M = {}

local ip_matcher = require "waf.ip_matcher"
local ip_cache = setmetatable({}, { __mode = "k" })
local wildcard_cache = setmetatable({}, { __mode = "k" })
local path_rule_cache = setmetatable({}, { __mode = "k" })

-- JSON null is truthy userdata in cjson, so value or {} is not sufficient.
local function table_or_empty(value)
    return type(value) == "table" and value or {}
end

local function matcher_for_list(list)
    local matcher = ip_cache[list]
    if matcher then return matcher end
    local entries = {}
    for _, ip in ipairs(table_or_empty(list.ips)) do entries[#entries + 1] = ip end
    for _, cidr in ipairs(table_or_empty(list.ip_cidrs)) do entries[#entries + 1] = cidr end
    matcher = ip_matcher.compile(entries)
    ip_cache[list] = matcher
    return matcher
end

local function wildcard_pattern(value)
    return "^" .. string.gsub(value, "([%^%$%(%)%%%.%[%]%+%-%?%*])", function(char)
        if char == "*" then return ".*" end
        return "%" .. char
    end) .. "$"
end

local function wildcards_for_list(patterns)
    local compiled = wildcard_cache[patterns]
    if compiled then return compiled end
    compiled = {}
    for _, pattern in ipairs(patterns) do
        compiled[#compiled + 1] = wildcard_pattern(string.lower(pattern))
    end
    wildcard_cache[patterns] = compiled
    return compiled
end

local function match_path(uri, patterns)
    if type(patterns) ~= "table" or #patterns == 0 then return false end
    local lower_uri = string.lower(uri)
    for _, pattern in ipairs(wildcards_for_list(patterns)) do
        if string.match(lower_uri, pattern) then return true end
    end
    return false
end

local function match_path_regex(uri, patterns)
    if type(patterns) ~= "table" or #patterns == 0 then return false end
    for _, pattern in ipairs(patterns) do
        local ok, match = pcall(ngx.re.find, uri, pattern, "jo")
        if ok and match then
            return true
        end
    end
    return false
end

local function match_ua(ua, patterns)
    if type(patterns) ~= "table" or #patterns == 0 then return false end
    for _, pattern in ipairs(patterns) do
        if ua and string.find(ua, pattern, 1, true) then
            return true
        end
    end
    return false
end

function M.match_any(remote_ip, ua, uri, list)
    if type(list) ~= "table" then return false end
    local matcher = matcher_for_list(list)
    local binary_ip = ngx.var and ngx.var.remote_addr == remote_ip and ngx.var.binary_remote_addr or nil
    if matcher:match(remote_ip, binary_ip) then return true end
    if match_path(uri, list.paths) then return true end
    if match_path_regex(uri, list.path_regexes) then return true end
    if match_ua(ua, list.user_agents) then return true end
    return false
end

function M.has_entries(list)
    if type(list) ~= "table" then return false end
    for _, value in pairs(list) do
        if type(value) == "table" and #value > 0 then return true end
    end
    return false
end

local function path_matches(uri, rules)
    if type(rules) ~= "table" then return false end
    for _, rule in ipairs(rules) do
        local value = rule.value
        if rule.type == "exact" and uri == value then return true end
        if rule.type == "prefix" and string.sub(uri, 1, #value) == value then return true end
        if rule.type == "wildcard" then
            local pattern = path_rule_cache[rule]
            if not pattern then
                pattern = wildcard_pattern(value)
                path_rule_cache[rule] = pattern
            end
            if string.match(uri, pattern) then return true end
        end
        if rule.type == "regex" then
            local from = ngx.re.find(uri, value, "jo")
            if from then return true end
        end
    end
    return false
end

function M.prepare(config)
    for _, list in ipairs({table_or_empty(config.whitelist), table_or_empty(config.blacklist)}) do
        matcher_for_list(list)
        if type(list.paths) == "table" then wildcards_for_list(list.paths) end
    end
    local rules = table_or_empty(config.path_rules)
    for _, list in ipairs({table_or_empty(rules.whitelist), table_or_empty(rules.blacklist)}) do
        for _, rule in ipairs(list) do
            if rule.type == "wildcard" then path_rule_cache[rule] = wildcard_pattern(rule.value) end
        end
    end
end

function M.should_challenge(uri, rules)
    rules = table_or_empty(rules)
    if path_matches(uri, rules.whitelist) then return false end
    if rules.mode == "blacklist" then return path_matches(uri, rules.blacklist) end
    return true
end

return M
`

//go:embed pow_config.lua
var openRestyPowConfigLua string

// ManagedPowLuaFiles returns embedded Lua assets for proof-of-work challenges.
func ManagedPowLuaFiles() []protocol.SupportFile {
	return []protocol.SupportFile{
		{Path: "pow/runtime.lua", Content: openRestyPowRuntimeLua},
		{Path: "pow/config.lua", Content: openRestyPowConfigLua},
		{Path: "pow/page.lua", Content: openRestyPowPageLua},
		{Path: "pow/check.lua", Content: openRestyPowCheckLua},
		{Path: "pow/challenge.lua", Content: openRestyPowChallengeLua},
		{Path: "pow/verify.lua", Content: openRestyPowVerifyLua},
		{Path: "pow/policy.lua", Content: openRestyPowPolicyLua},
	}
}

// ManagedPowStaticFiles returns embedded static assets served by the PoW module.
func ManagedPowStaticFiles() ([]protocol.SupportFile, error) {
	var files []protocol.SupportFile
	entries, err := powStaticFS.ReadDir("pow_static")
	if err != nil {
		return nil, err
	}
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := powStaticFS.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			fullPath := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				if err := walk(fullPath); err != nil {
					return err
				}
				continue
			}
			data, err := powStaticFS.ReadFile(fullPath)
			if err != nil {
				return err
			}
			// Convert pow_static/css/xess.css -> pow/static/css/xess.css
			relPath := strings.TrimPrefix(fullPath, "pow_static/")
			files = append(files, protocol.SupportFile{
				Path:    "pow/static/" + relPath,
				Content: string(data),
			})
		}
		return nil
	}
	for _, entry := range entries {
		fullPath := filepath.Join("pow_static", entry.Name())
		if entry.IsDir() {
			if err := walk(fullPath); err != nil {
				return nil, err
			}
		} else {
			data, err := powStaticFS.ReadFile(fullPath)
			if err != nil {
				return nil, err
			}
			relPath := strings.TrimPrefix(fullPath, "pow_static/")
			files = append(files, protocol.SupportFile{
				Path:    "pow/static/" + relPath,
				Content: string(data),
			})
		}
	}
	return files, nil
}
