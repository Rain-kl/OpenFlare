package nginx

import (
	"embed"
	"openflare-agent/internal/protocol"
	"path/filepath"
	"strings"
)

//go:embed pow_static
var powStaticFS embed.FS

const openRestyPowCheckLua = `local source = debug.getinfo(1, "S").source or ""
if string.sub(source, 1, 1) == "@" then
    local script_path = string.sub(source, 2)
    local base_dir = string.match(script_path, "^(.*)/pow/[^/]+%.lua$")
    if base_dir and base_dir ~= "" then
        package.path = base_dir .. "/?.lua;" .. base_dir .. "/?/init.lua;" .. package.path
    end
end

local cjson = require "cjson.safe"
local policy = require "pow.policy"

local pow_config_dict = ngx.shared.openflare_pow_config
local pow_sessions = ngx.shared.openflare_pow_sessions

-- Lazy-load pow_config from file; reload when content changes
local function load_pow_config()
    local config_paths = {
        ngx.config.prefix() .. "openflare-lua/pow_config.json",
        "/etc/nginx/openflare-lua/pow_config.json",
        "/usr/local/openresty/nginx/conf/pow_config.json"
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

            -- Clear old domain entries
            local old_keys = pow_config_dict:get("_domain_keys")
            if old_keys then
                for domain in string.gmatch(old_keys, "[^\n]+") do
                    pow_config_dict:delete(domain)
                end
            end

            local domain_keys = {}
            if content and content ~= "" and content ~= "{}" then
                local ok, entries = pcall(cjson.decode, content)
                if ok and entries and type(entries) == "table" then
                    for _, entry in ipairs(entries) do
                        if entry.domains then
                            for _, domain in ipairs(entry.domains) do
                                pow_config_dict:set(domain, cjson.encode(entry), 0)
                                domain_keys[#domain_keys+1] = domain
                            end
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

local config_raw = pow_config_dict:get(host)
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
    local session_data = pow_sessions:get(host .. ":" .. cookie_val)
    if session_data then
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
`

const openRestyPowChallengeLua = `local cjson = require "cjson.safe"

local pow_config_dict = ngx.shared.openflare_pow_config
local pow_challenges = ngx.shared.openflare_pow_challenges

local function generate_entropy()
    local pieces = {
        tostring(ngx.now()),
        tostring(ngx.worker.pid()),
        tostring(math.random()),
        ngx.var.remote_addr or "",
        ngx.var.http_user_agent or "",
        ngx.var.request_id or "",
    }
    return table.concat(pieces, ":")
end

local args = ngx.req.get_uri_args()
local host = args["host"] or ngx.var.host or ""
local redir = args["redir"] or ""

local config_raw = pow_config_dict:get(host)
if not config_raw then
    ngx.status = 403
    ngx.say("PoW not configured for this host")
    return
end

local ok, route_config = pcall(cjson.decode, config_raw)
if not ok or not route_config or not route_config.enabled then
    ngx.status = 403
    ngx.say("PoW not enabled for this host")
    return
end

local config = route_config.config or {}
local difficulty = config.difficulty or 4
local algorithm = config.algorithm or "fast"
local challenge_ttl = config.challenge_ttl or 300
local session_ttl = config.session_ttl or 86400

-- Generate challenge data without depending on ngx.random_bytes, which is not
-- available in every OpenResty runtime build.
local entropy = generate_entropy()
local challenge_id = ngx.md5(entropy .. ":id")
local challenge_data = ngx.md5(entropy .. ":data-a") .. ngx.md5(entropy .. ":data-b")

-- Store challenge
local challenge_info = cjson.encode({
    data = challenge_data,
    difficulty = difficulty,
    host = host,
    redir = redir,
    session_ttl = session_ttl
})
pow_challenges:set(challenge_id, challenge_info, challenge_ttl)

local static_prefix = "/.within.website/x/cmd/anubis/static/"
local title = "Making sure you're not a bot!"
local lang = "en"

ngx.header.content_type = "text/html; charset=utf-8"
ngx.say([[<!DOCTYPE html>
<html lang="]] .. lang .. [[">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>]] .. title .. [[</title>
<link rel="stylesheet" href="]] .. static_prefix .. [[css/xess.css">
<style>
body,html{height:100%;display:flex;justify-content:center;align-items:center;margin-left:auto;margin-right:auto}
.centered-div{text-align:center}
#status{font-variant-numeric:tabular-nums}
#progress{display:none;width:min(20rem,90%);height:2rem;border-radius:1rem;overflow:hidden;margin:1rem 0 2rem;outline-offset:2px;outline:#b16286 solid 4px}
.bar-inner{background-color:#b16286;height:100%;width:0;transition:width .25s ease-in}
</style>
<script id="anubis_version" type="application/json">"openflare-pow"</script>
<script id="anubis_challenge" type="application/json">]] .. cjson.encode({
    challenge = {
        id = challenge_id,
        randomData = challenge_data,
        method = algorithm
    },
    rules = {
        difficulty = difficulty,
        algorithm = algorithm
    }
}) .. [[</script>
<script id="anubis_base_prefix" type="application/json">""</script>
<script id="anubis_public_url" type="application/json">"__openflare_internal__"</script>
</head>
<body id="top">
<main>
<h1 id="title" class="centered-div">]] .. title .. [[</h1>
<div class="centered-div">
<img id="image" style="width:100%;max-width:256px;" src="]] .. static_prefix .. [[img/pensive.webp?cacheBuster=openflare-pow">
<p id="status">Loading...</p>
<p>This site is protected by a Proof-of-Work challenge. Your browser will solve a small puzzle before the upstream response is shown.</p>
<div id="progress" role="progressbar" aria-labelledby="status"><div class="bar-inner"></div></div>
<details>
<summary>Why am I seeing this?</summary>
<p>OpenFlare is asking your browser to complete a lightweight computation to distinguish normal browser traffic from automated abuse. This should finish automatically.</p>
</details>
<noscript><p>JavaScript is required to pass this verification. Please enable JavaScript and reload.</p></noscript>
</div>
</main>
<script type="module" src="]] .. static_prefix .. [[js/main.mjs"></script>
</body>
</html>]])
`

const openRestyPowVerifyLua = `local cjson = require "cjson.safe"

local pow_challenges = ngx.shared.openflare_pow_challenges
local pow_sessions = ngx.shared.openflare_pow_sessions

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
if not nonce then
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
local session_ttl = challenge_info.session_ttl or 86400

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
    return ngx.redirect(redir)
end

ngx.header.content_type = "application/json"
ngx.say(cjson.encode({ok = true}))
`

const openRestyPowPolicyLua = `local M = {}

local function match_ip(remote_ip, ips)
    if not ips or #ips == 0 then return false end
    for _, ip in ipairs(ips) do
        if ip == remote_ip then
            return true
        end
    end
    return false
end

local function match_cidr(remote_ip, cidrs)
    if not cidrs or #cidrs == 0 then return false end
    for _, cidr in ipairs(cidrs) do
        local m, err = ngx.re.match(cidr, "^(\\\\d{1,3}\\\\.\\\\d{1,3}\\\\.\\\\d{1,3}\\\\.\\\\d{1,3})/(\\\\d{1,2})$")
        if m then
            local mask_bits = tonumber(m[2])
            if mask_bits and mask_bits >= 0 and mask_bits <= 32 then
                local function ip_to_num(ip_str)
                    local parts = {}
                    for part in string.gmatch(ip_str, "%d+") do
                        parts[#parts+1] = tonumber(part) or 0
                    end
                    if #parts ~= 4 then return 0 end
                    return parts[1]*16777216 + parts[2]*65536 + parts[3]*256 + parts[4]
                end
                local remote_num = ip_to_num(remote_ip)
                local net_num = ip_to_num(m[1])
                if mask_bits == 0 then
                    return true
                end
                local mask = math.floor(2^(32 - mask_bits))
                mask = 4294967296 - mask
                if bit.band(remote_num, mask) == bit.band(net_num, mask) then
                    return true
                end
            end
        end
    end
    return false
end

local function match_path(uri, patterns)
    if not patterns or #patterns == 0 then return false end
    for _, pattern in ipairs(patterns) do
        local ok, match = pcall(ngx.re.match, uri, "^" .. ngx.re.gsub(pattern, "([%^%$%(%)%%%.%[%]%+%-%?])", function(c)
            if c == "*" then return ".*" end
            return "%" .. c
        end) .. "$", "i")
        if ok and match then
            return true
        end
    end
    return false
end

local function match_path_regex(uri, patterns)
    if not patterns or #patterns == 0 then return false end
    for _, pattern in ipairs(patterns) do
        local ok, match = pcall(ngx.re.match, uri, pattern)
        if ok and match then
            return true
        end
    end
    return false
end

local function match_ua(ua, patterns)
    if not patterns or #patterns == 0 then return false end
    for _, pattern in ipairs(patterns) do
        if ua and string.find(ua, pattern, 1, true) then
            return true
        end
    end
    return false
end

function M.match_any(remote_ip, ua, uri, list)
    if not list then return false end
    if match_ip(remote_ip, list.ips) then return true end
    if match_cidr(remote_ip, list.ip_cidrs) then return true end
    if match_path(uri, list.paths) then return true end
    if match_path_regex(uri, list.path_regexes) then return true end
    if match_ua(ua, list.user_agents) then return true end
    return false
end

function M.has_entries(list)
    if not list then return false end
    return (#(list.ips or {}) + #(list.ip_cidrs or {}) + #(list.paths or {}) + #(list.path_regexes or {}) + #(list.user_agents or {})) > 0
end

return M
`

func ManagedPowLuaFiles() []protocol.SupportFile {
	return []protocol.SupportFile{
		{Path: "pow/check.lua", Content: openRestyPowCheckLua},
		{Path: "pow/challenge.lua", Content: openRestyPowChallengeLua},
		{Path: "pow/verify.lua", Content: openRestyPowVerifyLua},
		{Path: "pow/policy.lua", Content: openRestyPowPolicyLua},
	}
}

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
