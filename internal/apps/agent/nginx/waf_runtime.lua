local _M = {}
local ip_matcher = require("waf.ip_matcher")
local compile_ip_matcher = ip_matcher.compile
local rules_by_site, global_rules = {}, {}
local prepare_rules
local rules_prepared = false

local rules_config
local ip_groups_config
local ip_groups_runtime
local pow_runtime
local geo_lookup
local geo_module
local geo_profiles = { city = false, country = false }

local function read_file(path)
    local file, err = io.open(path, "r")
    if not file then
        return nil, err
    end
    local content = file:read("*a")
    file:close()
    return content
end

local function load_json(path)
    local content, err = read_file(path)
    if not content or content == "" then
        return nil, err or "empty file"
    end
    local decoded, decode_err = require("cjson.safe").decode(content)
    if not decoded then
        return nil, decode_err or "invalid JSON"
    end
    return decoded
end

local function warn_rate_limited(key, ...)
    local dict = ngx.shared and ngx.shared.openflare_waf_config
    if not dict or not dict.add or dict:add(key, true, 60) then
        ngx.log(ngx.WARN, ...)
    end
end

local function array_or_empty(value)
    if type(value) == "table" then return value end
    return {}
end

local function file_exists(path)
    local file = io.open(path, "rb")
    if not file then return false end
    file:close()
    return true
end

local function init_geo_databases(country_path, city_path, path_exists, region_required)
    local ok, module_or_error = pcall(require, "resty.maxminddb")
    if not ok or not module_or_error then
        warn_rate_limited("_geo_module_unavailable", "openflare waf GeoIP module unavailable: ", module_or_error)
        return
    end
    geo_module = module_or_error
    local profiles = {}
    if path_exists(city_path) then profiles.city = city_path end
    if path_exists(country_path) then profiles.country = country_path end
    if not profiles.city and region_required then
        warn_rate_limited("_geo_city_unavailable", "openflare waf GeoLite2 City database unavailable; region match takes false branch")
    end
    if not profiles.country and not profiles.city then
        warn_rate_limited("_geo_database_unavailable", "openflare waf GeoIP databases unavailable")
        return
    end
    local function initialize_profile(profile, path)
        local called, init_result, init_error = pcall(geo_module.init, { [profile] = path })
        if not called or init_result ~= true then
            return false, init_error or init_result
        end
        geo_profiles[profile] = true
        return true
    end
    local city_initialized, city_error = false, nil
    if profiles.city then
        city_initialized, city_error = initialize_profile("city", profiles.city)
        if not city_initialized and region_required then
            warn_rate_limited("_geo_city_unavailable", "openflare waf GeoLite2 City database initialization failed; region match takes false branch: ", city_error)
        end
    end
    local country_initialized, country_error = false, nil
    if profiles.country then
        country_initialized, country_error = initialize_profile("country", profiles.country)
    end
    if not city_initialized and not country_initialized then
        warn_rate_limited("_geo_database_unavailable", "openflare waf GeoIP database initialization failed: ", country_error or city_error)
    end
end

local function lookup_geo_profile(ip, profile)
    if not geo_module or not geo_profiles[profile] then return nil end
    local ok, result, lookup_error = pcall(geo_module.lookup, ip, nil, profile)
    if not ok or not result then
        warn_rate_limited("_geo_lookup_failed_" .. profile, "openflare waf GeoIP ", profile, " lookup failed: ", lookup_error or result)
        return nil
    end
    return result
end

local function default_geo_lookup(ip, region_required)
    local result = lookup_geo_profile(ip, "city")
    local from_city = result ~= nil
    if not result then result = lookup_geo_profile(ip, "country") end
    if not result then return nil, nil end
    local country = result.country and result.country.iso_code or nil
    local subdivision
    if from_city then
        subdivision = result.most_specific_subdivision and result.most_specific_subdivision.iso_code or nil
        if not subdivision and result.subdivisions and result.subdivisions[1] then
            subdivision = result.subdivisions[1].iso_code
        end
    elseif region_required then
        warn_rate_limited("_geo_city_unavailable", "openflare waf GeoLite2 City database unavailable; region match takes false branch")
    end
    country = country and string.upper(country) or nil
    subdivision = subdivision and string.upper(subdivision) or nil
    local region = subdivision
    if country and subdivision and not string.match(subdivision, "^[A-Z][A-Z]%-") then
        region = country .. "-" .. subdivision
    end
    return country, region
end

local function config_geo_requirements(config)
    local uses_geo, uses_region = false, false
    for _, rule in ipairs(array_or_empty(config.rule_groups)) do
        for _, node in pairs((rule.graph or {}).nodes or {}) do
            if node.type == "geo_match" then
                uses_geo = true
                local node_config = node.config or {}
                if type(node_config.regions) == "table" and #node_config.regions > 0 then uses_region = true end
            end
        end
    end
    return uses_geo, uses_region
end

function _M.init(options)
    options = options or {}
    local runtime_dir = options.runtime_dir or "__OPENFLARE_RUNTIME_CONFIG_DIR__"
    -- Always apply explicit test/runtime injections; only short-circuit cold disk load once.
    if options.config then
        rules_config = options.config
    elseif not rules_config then
        local err
        rules_config, err = load_json(runtime_dir .. "/waf_config.json")
        assert(rules_config, "load waf_config.json failed: " .. tostring(err))
    end
    if options.ip_groups then
        ip_groups_config = options.ip_groups
        -- Drop stale compiled matchers when tests inject a fresh snapshot table.
        local groups = (ip_groups_config.groups or {})
        for _, group in pairs(groups) do
            if type(group) == "table" then group._matcher = nil end
        end
        ip_matcher.prepare_groups(ip_groups_config)
    elseif not ip_groups_config and not ip_groups_runtime then
        ip_groups_runtime = options.ip_groups_runtime or require("waf.ip_groups")
        local initialized, init_error = ip_groups_runtime.init({ runtime_dir = runtime_dir })
        assert(initialized, "initialize WAF IP groups failed: " .. tostring(init_error))
    end
    if options.pow then
        pow_runtime = options.pow
    elseif not pow_runtime then
        pow_runtime = require("pow.runtime")
    end
    if pow_runtime.init then pow_runtime.init() end
    if options.config or not rules_prepared then
        prepare_rules(rules_config)
        rules_prepared = true
    end
    if options.geo_lookup then
        geo_lookup = options.geo_lookup
    elseif not geo_lookup then
        local uses_geo, uses_region = config_geo_requirements(rules_config)
        if uses_geo then
            init_geo_databases(
                options.country_mmdb_path or "__OPENFLARE_COUNTRY_MMDB_PATH__",
                options.city_mmdb_path or "__OPENFLARE_CITY_MMDB_PATH__",
                options.geo_file_exists or file_exists,
                uses_region
            )
        end
        geo_lookup = default_geo_lookup
    end
    return true
end

-- Task 7 can atomically replace the worker-local IP group snapshot through this seam.
function _M.replace_ip_groups(snapshot)
    local replacement = snapshot or { groups = {} }
    ip_matcher.prepare_groups(replacement)
    ip_groups_config = replacement
end

local function list_contains(items, value)
    if type(items) ~= "table" or not value then return false end
    value = string.upper(value)
    for _, item in ipairs(items) do
        if string.upper(tostring(item)) == value then return true end
    end
    return false
end

local node_ip_matcher_cache = setmetatable({}, { __mode = "k" })

local function matcher_for_node_ip_config(config)
    config = config or {}
    local cached = node_ip_matcher_cache[config]
    if cached then return cached end
    local entries = {}
    for _, item in ipairs(array_or_empty(config.ips)) do entries[#entries + 1] = item end
    for _, item in ipairs(array_or_empty(config.cidrs)) do entries[#entries + 1] = item end
    local matcher = compile_ip_matcher(entries)
    node_ip_matcher_cache[config] = matcher
    return matcher
end

local function matcher_for_ip_group(group)
    if type(group) ~= "table" then return compile_ip_matcher({}) end
    if group._matcher then return group._matcher end
    group._matcher = compile_ip_matcher(group.ip_list)
    return group._matcher
end

local function matches_ip_values(config, ip)
    if type(ip) ~= "string" or ip == "" then return false end
    local bin_ip = ngx.var and ngx.var.binary_remote_addr or nil
    local ip_v4, ip_v6 = ip_matcher.parse(ip)

    local node_matcher = matcher_for_node_ip_config(config)
    if not node_matcher.empty and node_matcher:match(ip, bin_ip, ip_v4, ip_v6) then
        return true
    end

    local snapshot = ip_groups_config or (ip_groups_runtime and ip_groups_runtime.current())
    local groups = (snapshot or {}).groups or {}
    for _, id in ipairs(array_or_empty(config.ip_group_ids)) do
        local group = groups[tostring(id)]
        if group and group.enabled then
            local matcher = matcher_for_ip_group(group)
            if matcher:match(ip, bin_ip, ip_v4, ip_v6) then return true end
        end
    end
    return false
end

local function ua_trim(value)
    return (string.gsub(value or "", "^%s*(.-)%s*$", "%1"))
end

local function ua_label_set(items)
    if type(items) ~= "table" then return nil, false end
    local set, count = {}, 0
    for _, item in ipairs(items) do
        set[tostring(item)] = true
        count = count + 1
    end
    return set, count > 0
end

local function match_ua_rules(ua_lower, rules, fallback)
    if ua_lower == "" then return "Unknown" end
    for _, rule in ipairs(rules) do
        local matched = false
        for _, token in ipairs(rule.contains or {}) do
            if string.find(ua_lower, token, 1, true) then
                matched = true
                break
            end
        end
        if not matched and type(rule.all_of) == "table" and #rule.all_of > 0 then
            matched = true
            for _, token in ipairs(rule.all_of) do
                if not string.find(ua_lower, token, 1, true) then
                    matched = false
                    break
                end
            end
        end
        if matched then
            local excluded = false
            for _, token in ipairs(rule.none_of or {}) do
                if string.find(ua_lower, token, 1, true) then
                    excluded = true
                    break
                end
            end
            if not excluded then return rule.label end
        end
    end
    return fallback
end

-- Mirrors internal/repository/analytics/browser.go browserRules / osRules.
local browser_rules = {
    { label = "WeChat", contains = { "micromessenger" } },
    { label = "Postman", contains = { "postman" } },
    { label = "CLI", contains = { "curl/", "wget/" } },
    { label = "Edge", contains = { "edg/", "edgios/", "edga/" } },
    { label = "Opera", contains = { "opr/", "opera" } },
    { label = "Firefox", contains = { "firefox", "fxios" } },
    { label = "Chrome", contains = { "crios", "chrome" }, none_of = { "chromium" } },
    { label = "Chromium", contains = { "chromium" } },
    { label = "Safari", contains = { "safari" } },
    { label = "Bot", contains = { "bot", "spider", "crawler", "slurp" } },
}

local os_rules = {
    { label = "Android", contains = { "android" } },
    { label = "iOS", contains = { "iphone", "ipad", "ipod", "ios" } },
    { label = "Windows", contains = { "windows" } },
    { label = "macOS", contains = { "mac os x", "macintosh", "macos" } },
    { label = "Chrome OS", contains = { "cros" } },
    { label = "Linux", contains = { "linux" } },
    { label = "Bot", contains = { "bot", "spider", "crawler" } },
}

local function parse_browser_name_lower(ua_lower)
    return match_ua_rules(ua_lower, browser_rules, "Other")
end

local function parse_os_name_lower(ua_lower)
    return match_ua_rules(ua_lower, os_rules, "Other")
end

local function ua_matches_custom_patterns(ua, patterns)
    for _, pattern in ipairs(array_or_empty(patterns)) do
        if type(pattern) == "string" and pattern ~= "" then
            local ok, matched = pcall(function()
                return string.find(ua, pattern) ~= nil
            end)
            if ok and matched then return true end
        end
    end
    return false
end

local ua_config_cache = setmetatable({}, { __mode = "k" })

local function compile_ua_config(config)
    local browsers, has_browsers = ua_label_set(config.browsers)
    local operating_systems, has_os = ua_label_set(config.operating_systems)
    local compiled = {
        browsers = browsers, has_browsers = has_browsers,
        operating_systems = operating_systems, has_os = has_os,
    }
    ua_config_cache[config] = compiled
    return compiled
end

local function matches_ua_check(config)
    config = config or {}
    local ua = ua_trim(ngx.var.http_user_agent or "")
    if config.require_ua and ua == "" then return false end
    local ua_lower = string.lower(ua)
    local browser = parse_browser_name_lower(ua_lower)
    local os_name = parse_os_name_lower(ua_lower)
    if config.block_common_bots and (browser == "Bot" or os_name == "Bot") then return false end
    -- Abnormal excludes search-engine / crawler Bot labels; use block_common_bots for those.
    if config.block_abnormal_ua and (browser == "Other" or browser == "Unknown") then
        return false
    end
    if config.block_custom_ua and ua_matches_custom_patterns(ua, config.custom_ua_patterns) then
        return false
    end
    local compiled = ua_config_cache[config]
    if not compiled then
        compiled = compile_ua_config(config)
    end
    local browser_set, has_browsers = compiled.browsers, compiled.has_browsers
    local os_set, has_os = compiled.operating_systems, compiled.has_os
    if not has_browsers and not has_os then return true end
    local browser_ok = has_browsers and browser_set[browser] == true
    local os_ok = has_os and os_set[os_name] == true
    if has_browsers and not has_os then return browser_ok end
    if has_os and not has_browsers then return os_ok end
    local mode = config.match_mode
    if mode ~= "and" and mode ~= "or" then mode = "or" end
    if mode == "and" then return browser_ok and os_ok end
    return browser_ok or os_ok
end

local security_body_max = 65536

local function url_decode(value)
    value = string.gsub(value or "", "+", " ")
    value = string.gsub(value, "%%(%x%x)", function(hex)
        return string.char(tonumber(hex, 16))
    end)
    return value
end

local function security_decode(value)
    local once = url_decode(value)
    local twice = url_decode(once)
    return string.lower(once), string.lower(twice)
end

local function security_match_any(haystacks, patterns)
    for _, hay in ipairs(haystacks) do
        if type(hay) == "string" and hay ~= "" then
            for _, pattern in ipairs(patterns) do
                if string.find(hay, pattern, 1, true) then return true end
            end
        end
    end
    return false
end

-- SQL sleep/benchmark: require digit arg to avoid product names like sleep(better).
local function security_match_sql_timed(haystacks)
    for _, hay in ipairs(haystacks) do
        if type(hay) == "string" and hay ~= "" then
            if string.find(hay, "sleep(%d", 1, true) or string.find(hay, "benchmark(%d", 1, true) then
                return true
            end
            -- Also accept sleep( 1 ) with optional spaces: sleep( + digit
            local i = 1
            while true do
                local s, e = string.find(hay, "sleep(", i, true)
                if not s then break end
                local rest = string.sub(hay, e + 1)
                if string.match(rest, "^%s*%d") then return true end
                i = e + 1
            end
            i = 1
            while true do
                local s, e = string.find(hay, "benchmark(", i, true)
                if not s then break end
                local rest = string.sub(hay, e + 1)
                if string.match(rest, "^%s*%d") then return true end
                i = e + 1
            end
        end
    end
    return false
end

-- XSS: tag/event handlers and URI schemes; skip prose like "javascript: the good parts".
local function security_match_xss(haystacks)
    local tag_like = { "<script", "<iframe", "onerror=", "onload=", "onmouseover=", "document.cookie" }
    for _, hay in ipairs(haystacks) do
        if type(hay) == "string" and hay ~= "" then
            for _, pattern in ipairs(tag_like) do
                if string.find(hay, pattern, 1, true) then return true end
            end
            -- javascript: as URI scheme with code-like body (alert/void/'/") not prose titles.
            local i = 1
            while true do
                local s, e = string.find(hay, "javascript:", i, true)
                if not s then break end
                local prev_ok = (s == 1) or string.match(string.sub(hay, s - 1, s - 1), "[=\"'(<;,]")
                if prev_ok then
                    local rest = string.sub(hay, e + 1)
                    if string.match(rest, "^%s*[\"'`(]")
                        or string.match(rest, "^%s*alert%s*%(")
                        or string.match(rest, "^%s*void%s*%(")
                        or string.match(rest, "^%s*eval%s*%(")
                        or string.match(rest, "^%s*window%.")
                        or string.match(rest, "^%s*document%.") then
                        return true
                    end
                end
                i = e + 1
            end
            if string.find(hay, "eval(", 1, true) then
                local _, e = string.find(hay, "eval(", 1, true)
                local rest = string.sub(hay, e + 1)
                if string.match(rest, "^%s*[\"'`(]") then return true end
            end
        end
    end
    return false
end

local function security_append_decoded(list, value)
    if type(value) ~= "string" or value == "" then return end
    local once, twice = security_decode(value)
    list[#list + 1] = once
    if twice ~= once then list[#list + 1] = twice end
end

local function security_collect_args(list)
    if not ngx.req or not ngx.req.get_uri_args then
        security_append_decoded(list, ngx.var.args or "")
        return
    end
    local args = ngx.req.get_uri_args(100)
    if type(args) ~= "table" then return end
    for key, value in pairs(args) do
        security_append_decoded(list, tostring(key))
        if type(value) == "table" then
            for _, item in ipairs(value) do security_append_decoded(list, tostring(item)) end
        else
            security_append_decoded(list, tostring(value))
        end
    end
end

-- Only Cookie / Referer for injection surfaces. Generic browser headers (UA, Accept, …)
-- are high-volume and high false-positive / CPU cost if scanned for SQL/cmd/XSS.
local function security_collect_sensitive_headers(list)
    local cookie = ngx.var.http_cookie
    if type(cookie) == "string" and cookie ~= "" then
        security_append_decoded(list, cookie)
    end
    local referer = ngx.var.http_referer
    if type(referer) == "string" and referer ~= "" then
        security_append_decoded(list, referer)
    end
end

local function security_read_body()
    local content_length = tonumber(ngx.var.content_length or "") or 0
    if content_length <= 0 or content_length > security_body_max then return nil end
    if not ngx.req or not ngx.req.read_body or not ngx.req.get_body_data then return nil end
    local ok = pcall(ngx.req.read_body)
    if not ok then return nil end
    local body = ngx.req.get_body_data()
    if type(body) ~= "string" or body == "" then return nil end
    return body
end

local path_traversal_patterns = {
    "../", "..\\", "..%2f", "..%5c", "%2e%2e/", "%2e%2e\\", "%252e%252e",
    "....//", "/etc/passwd",
}
local file_inclusion_patterns = {
    "php://", "file://", "zip://", "data://text", "expect://", "/etc/passwd",
    "/proc/self", "%00",
}
-- Prefer attack-shaped tokens; avoid bare "&&"/"||" and bare shell names.
local command_patterns = {
    ";wget", ";curl", ";bash", ";sh ", "|bash", "|sh ", "|sh\t", "`id`", "$(id)",
    "&&wget", "&&curl", "&&bash", "&&sh ", "||wget", "||curl", "||bash",
    "/bin/sh ", "/bin/bash ", "cmd.exe /c", "powershell -", "powershell.exe",
}
-- URL-shaped only: bare "localhost"/"0.0.0.0" match Chrome UA / normal text.
local ssrf_patterns = {
    "http://127.0.0.1", "https://127.0.0.1", "http://localhost", "https://localhost",
    "http://0.0.0.0", "https://0.0.0.0", "http://[::1]", "https://[::1]",
    "://169.254.", "169.254.169.254", "metadata.google",
    "file://", "gopher://", "dict://",
}
local upload_patterns = {
    ".php.", ".jsp.", ".asp.", ".aspx.", ".phtml", ".phar",
    "application/x-php", "application/x-httpd-php",
}
local xxe_patterns = {
    "<!entity", " system \"", " system '", "file://",
}
-- Keep encoded CRLF; bare %0a alone is too common in benign encoded text.
local crlf_patterns = {
    "%0d%0a", "\r\n",
}
local sql_static_patterns = {
    "union select", " or 1=1", "' or '", "\" or \"",
    "information_schema", "xp_cmdshell", "load_file(", " into outfile",
    "/**/", "/*!", "*/--", "@@version",
}

local function security_flag_enabled(value)
    return value == true or value == 1 or value == "true" or value == "1"
end

local function security_append_list(dst, src)
    for _, item in ipairs(src) do dst[#dst + 1] = item end
end

local function matches_security_check(config)
    config = config or {}
    local sql_injection = security_flag_enabled(config.sql_injection)
    local path_traversal = security_flag_enabled(config.path_traversal)
    local command_injection = security_flag_enabled(config.command_injection)
    local xss = security_flag_enabled(config.xss)
    local ssrf = security_flag_enabled(config.ssrf)
    local file_inclusion = security_flag_enabled(config.file_inclusion)
    local malicious_upload = security_flag_enabled(config.malicious_upload)
    local xxe = security_flag_enabled(config.xxe)
    local crlf_injection = security_flag_enabled(config.crlf_injection)
    if not (sql_injection or path_traversal or command_injection or xss or ssrf
        or file_inclusion or malicious_upload or xxe or crlf_injection) then
        return true
    end

    -- Collect only what enabled checks need (P1). Path uses uri only (not request_uri)
    -- to avoid re-scanning query; query is collected separately when needed (P0).
    local need_path = path_traversal or file_inclusion
    local need_query = path_traversal or file_inclusion or sql_injection or command_injection
        or xss or ssrf or crlf_injection
    local need_sensitive_headers = sql_injection or command_injection or xss or ssrf or crlf_injection
    local need_body = malicious_upload or xxe
        or ((sql_injection or path_traversal or command_injection or xss or ssrf
            or file_inclusion or crlf_injection)
            and (tonumber(ngx.var.content_length or "") or 0) > 0)

    local path_inputs, query_inputs, header_inputs, body_inputs = {}, {}, {}, {}
    if need_path then
        security_append_decoded(path_inputs, ngx.var.uri or "")
    end
    if need_query then
        security_collect_args(query_inputs)
    end
    if need_sensitive_headers then
        security_collect_sensitive_headers(header_inputs)
    end

    local body
    if need_body then body = security_read_body() end
    if body then security_append_decoded(body_inputs, body) end

    if path_traversal or file_inclusion then
        local pq = {}
        security_append_list(pq, path_inputs)
        security_append_list(pq, query_inputs)
        security_append_list(pq, body_inputs)
        if path_traversal and security_match_any(pq, path_traversal_patterns) then return false end
        if file_inclusion and security_match_any(pq, file_inclusion_patterns) then return false end
    end

    if sql_injection or command_injection or xss or ssrf or crlf_injection then
        local qhb = {}
        security_append_list(qhb, query_inputs)
        security_append_list(qhb, header_inputs)
        security_append_list(qhb, body_inputs)
        if sql_injection then
            if security_match_any(qhb, sql_static_patterns) or security_match_sql_timed(qhb) then
                return false
            end
        end
        if command_injection and security_match_any(qhb, command_patterns) then return false end
        if xss and security_match_xss(qhb) then return false end
        if ssrf and security_match_any(qhb, ssrf_patterns) then return false end
        if crlf_injection and security_match_any(qhb, crlf_patterns) then return false end
    end

    if malicious_upload and body then
        local content_type = string.lower(ngx.var.content_type or "")
        if string.find(content_type, "multipart/", 1, true) then
            if security_match_any(body_inputs, upload_patterns) then return false end
        end
    end
    if xxe and body then
        local content_type = string.lower(ngx.var.content_type or "")
        if string.find(content_type, "xml", 1, true) or string.find(string.lower(body), "<?xml", 1, true) then
            if security_match_any(body_inputs, xxe_patterns) then return false end
        end
    end
    return true
end

local function fail_closed(reason)
    local dict = ngx.shared and ngx.shared.openflare_waf_config
    if not dict or not dict.add or dict:add("_damaged_graph_logged", true, 60) then
        ngx.log(ngx.ERR, "openflare waf damaged runtime graph: ", reason)
    end
    ngx.ctx.openflare_waf_blocked = true
    ngx.status = 500
    ngx.header["Content-Type"] = "text/plain; charset=utf-8"
    ngx.say("OpenFlare WAF runtime error")
    return ngx.exit(500)
end

local function render_block(config)
    config = config or {}
    local status = tonumber(config.status_code) or 403
    ngx.ctx.openflare_waf_blocked = true
    ngx.status = status
    local body = config.response_body or ""
    if body ~= "" then
        ngx.header["Content-Type"] = "text/html; charset=utf-8"
        ngx.say(body)
    end
    return ngx.exit(status)
end

local graph_node_counts = setmetatable({}, { __mode = "k" })

local function graph_node_count(graph)
    local count = graph_node_counts[graph]
    if count then return count end
    count = 0
    for _ in pairs(graph.nodes) do count = count + 1 end
    graph_node_counts[graph] = count
    return count
end

local function execute_graph(graph)
    if type(graph) ~= "table" or type(graph.nodes) ~= "table" or type(graph.entry) ~= "string" then
        return nil, "invalid graph"
    end
    local node_count = graph_node_count(graph)
    local current = graph.entry
    for _ = 1, node_count do
        local node = graph.nodes[current]
        if type(node) ~= "table" or type(node.type) ~= "string" then
            return nil, "missing node " .. tostring(current)
        end
        if node.type == "allow" then
            return { kind = "allow" }
        end
        if node.type == "block" then
            return { kind = "block", config = node.config }
        end
        local handle
        if node.type == "start" then
            handle = "next"
        elseif node.type == "ip_match" then
            handle = matches_ip_values(node.config or {}, ngx.var.remote_addr or "") and "true" or "false"
        elseif node.type == "geo_match" then
            local config = node.config or {}
            local region_required = type(config.regions) == "table" and #config.regions > 0
            local country, region = geo_lookup(ngx.var.remote_addr or "", region_required)
            handle = (list_contains(config.countries, country) or list_contains(config.regions, region)) and "true" or "false"
        elseif node.type == "ua_check" then
            handle = matches_ua_check(node.config or {}) and "true" or "false"
        elseif node.type == "security_check" then
            handle = matches_security_check(node.config or {}) and "true" or "false"
        elseif node.type == "pow" then
            if pow_runtime.evaluate(node.config or {}) ~= true then
                return { kind = "takeover" }
            end
            handle = "next"
        else
            return nil, "unknown node type " .. node.type
        end
        if type(node.next) ~= "table" or type(node.next[handle]) ~= "string" then
            return nil, "missing " .. handle .. " edge from " .. current
        end
        current = node.next[handle]
    end
    return nil, "graph exceeded maximum steps"
end

prepare_rules = function(config)
    local by_id = {}
    global_rules, rules_by_site = {}, {}
    node_ip_matcher_cache = setmetatable({}, { __mode = "k" })
    ua_config_cache = setmetatable({}, { __mode = "k" })
    graph_node_counts = setmetatable({}, { __mode = "k" })
    for _, rule in ipairs(array_or_empty(config.rule_groups)) do
        by_id[tostring(rule.id)] = rule
        if rule.enabled and rule.is_global then global_rules[#global_rules + 1] = rule end
        local graph = rule.graph
        if type(graph) == "table" and type(graph.nodes) == "table" then
            graph_node_count(graph)
            for _, node in pairs(graph.nodes) do
                if node.type == "ip_match" then
                    matcher_for_node_ip_config(node.config)
                elseif node.type == "ua_check" then
                    compile_ua_config(node.config or {})
                elseif node.type == "pow" and pow_runtime.prepare_config then
                    pow_runtime.prepare_config(node.config or {})
                end
            end
        end
    end
    for _, binding in ipairs(array_or_empty(config.bindings)) do
        if not rules_by_site[binding.site_name] then
            local result = {}
            for _, rule in ipairs(global_rules) do result[#result + 1] = rule end
            for _, id in ipairs(array_or_empty(binding.rule_group_ids)) do
                local rule = by_id[tostring(id)]
                if rule and rule.enabled and not rule.is_global then result[#result + 1] = rule end
            end
            rules_by_site[binding.site_name] = result
        end
    end
end

local function active_rules(site)
    return rules_by_site[site] or global_rules
end

local function is_internal_pow_continuation()
    if not ngx.req or not ngx.req.is_internal or not ngx.req.is_internal() then return false end
    local uri = ngx.var.uri or ""
    local api_prefix = "/.within.website/x/cmd/anubis/api/"
    local static_prefix = "/.within.website/x/cmd/anubis/static/"
    return string.sub(uri, 1, #api_prefix) == api_prefix or string.sub(uri, 1, #static_prefix) == static_prefix
end

function _M.check()
    if not rules_config then
        return fail_closed("runtime not initialized")
    end
    if is_internal_pow_continuation() then
        ngx.ctx.openflare_pow_takeover = true
        return
    end
    for _, rule in ipairs(active_rules(ngx.var.openflare_waf_site or "")) do
        local decision, err = execute_graph(rule.graph)
        if not decision then return fail_closed(err) end
        if decision.kind == "block" then return render_block(decision.config) end
        if decision.kind == "takeover" then return end
    end
    return "ok"
end

-- Test helpers for unit specs.
function _M.debug_security_check(config)
    return matches_security_check(config or {})
end

function _M.debug_active_rules(site)
    return active_rules(site or "")
end

function _M.debug_execute_graph(graph)
    return execute_graph(graph)
end

function _M.debug_compile_ip_matcher(entries)
    return compile_ip_matcher(entries)
end

function _M.debug_matches_ip_values(config, ip)
    return matches_ip_values(config or {}, ip or "")
end

return _M
