local _M = {}

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
    if rules_config then
        return true
    end
    options = options or {}
    local runtime_dir = options.runtime_dir or "__OPENFLARE_RUNTIME_CONFIG_DIR__"
    if options.config then
        rules_config = options.config
    else
        local err
        rules_config, err = load_json(runtime_dir .. "/waf_config.json")
        assert(rules_config, "load waf_config.json failed: " .. tostring(err))
    end
    if options.ip_groups then
        ip_groups_config = options.ip_groups
    else
        ip_groups_runtime = options.ip_groups_runtime or require("waf.ip_groups")
        local initialized, init_error = ip_groups_runtime.init({ runtime_dir = runtime_dir })
        assert(initialized, "initialize WAF IP groups failed: " .. tostring(init_error))
    end
    pow_runtime = options.pow or require("pow.runtime")
    if options.geo_lookup then
        geo_lookup = options.geo_lookup
    else
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
    ip_groups_config = snapshot or { groups = {} }
end

local function list_contains(items, value)
    if type(items) ~= "table" or not value then return false end
    value = string.upper(value)
    for _, item in ipairs(items) do
        if string.upper(tostring(item)) == value then return true end
    end
    return false
end

local function parse_ipv4(value)
    local a, b, c, d = string.match(value or "", "^(%d+)%.(%d+)%.(%d+)%.(%d+)$")
    if not a then return nil end
    a, b, c, d = tonumber(a), tonumber(b), tonumber(c), tonumber(d)
    if a > 255 or b > 255 or c > 255 or d > 255 then return nil end
    return ((a * 256 + b) * 256 + c) * 256 + d
end

local function split_ipv6_side(value)
    local result = {}
    if value == "" then return result end
    for part in string.gmatch(value, "[^:]+") do
        if string.find(part, ".", 1, true) then
            local ipv4 = parse_ipv4(part)
            if not ipv4 then return nil end
            result[#result + 1] = math.floor(ipv4 / 65536)
            result[#result + 1] = ipv4 % 65536
        else
            if #part > 4 or not string.match(part, "^[%x]+$") then return nil end
            local number = tonumber(part, 16)
            if not number or number > 65535 then return nil end
            result[#result + 1] = number
        end
    end
    return result
end

local function parse_ipv6(value)
    value = string.lower(value or "")
    local compressed_at = string.find(value, "::", 1, true)
    if compressed_at and string.find(value, "::", compressed_at + 2, true) then return nil end
    local left, right
    if compressed_at then
        left = split_ipv6_side(string.sub(value, 1, compressed_at - 1))
        right = split_ipv6_side(string.sub(value, compressed_at + 2))
    else
        if string.sub(value, 1, 1) == ":" or string.sub(value, -1) == ":" then return nil end
        left, right = split_ipv6_side(value), {}
    end
    if not left or not right then return nil end
    local missing = 8 - #left - #right
    if (compressed_at and missing < 1) or (not compressed_at and missing ~= 0) then return nil end
    local result = {}
    for _, number in ipairs(left) do result[#result + 1] = number end
    for _ = 1, missing do result[#result + 1] = 0 end
    for _, number in ipairs(right) do result[#result + 1] = number end
    if #result ~= 8 then return nil end
    return result
end

local function ipv6_equal(left, right)
    left, right = parse_ipv6(left), parse_ipv6(right)
    if not left or not right then return false end
    for index = 1, 8 do
        if left[index] ~= right[index] then return false end
    end
    return true
end

local function ip_in_cidr(ip, cidr)
    local base, bits = string.match(cidr or "", "^([^/]+)/(%d+)$")
    bits = tonumber(bits)
    if not base or not bits then return false end
    local ip_number, base_number = parse_ipv4(ip), parse_ipv4(base)
    if ip_number and base_number then
        if bits < 0 or bits > 32 then return false end
        if bits == 0 then return true end
        local size = 2 ^ (32 - bits)
        return ip_number - (ip_number % size) == base_number - (base_number % size)
    end
    local ip_groups, base_groups = parse_ipv6(ip), parse_ipv6(base)
    if not ip_groups or not base_groups or bits < 0 or bits > 128 then return false end
    local full_groups, remaining_bits = math.floor(bits / 16), bits % 16
    for index = 1, full_groups do
        if ip_groups[index] ~= base_groups[index] then return false end
    end
    if remaining_bits > 0 then
        local size = 2 ^ (16 - remaining_bits)
        local index = full_groups + 1
        if math.floor(ip_groups[index] / size) ~= math.floor(base_groups[index] / size) then return false end
    end
    return true
end

local function matches_ip_values(config, ip)
    for _, item in ipairs(array_or_empty(config.ips)) do
        if item == ip or ipv6_equal(item, ip) then return true end
    end
    for _, cidr in ipairs(array_or_empty(config.cidrs)) do
        if ip_in_cidr(ip, cidr) then return true end
    end
    local snapshot = ip_groups_config or ip_groups_runtime.current()
    local groups = (snapshot or {}).groups or {}
    for _, id in ipairs(array_or_empty(config.ip_group_ids)) do
        local group = groups[tostring(id)]
        if group and group.enabled then
            for _, item in ipairs(array_or_empty(group.ip_list)) do
                if item == ip or ipv6_equal(item, ip) or ip_in_cidr(ip, item) then return true end
            end
        end
    end
    return false
end

local function ua_trim(value)
    return (string.gsub(value or "", "^%s*(.-)%s*$", "%1"))
end

local function ua_label_in(items, value)
    if type(items) ~= "table" or not value then return false end
    for _, item in ipairs(items) do
        if tostring(item) == value then return true end
    end
    return false
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

local function parse_browser_name(ua)
    return match_ua_rules(string.lower(ua or ""), browser_rules, "Other")
end

local function parse_os_name(ua)
    return match_ua_rules(string.lower(ua or ""), os_rules, "Other")
end

local function matches_ua_check(config)
    config = config or {}
    local ua = ua_trim(ngx.var.http_user_agent or "")
    if config.require_ua and ua == "" then return false end
    local browser = parse_browser_name(ua)
    local os_name = parse_os_name(ua)
    if config.block_common_bots and (browser == "Bot" or os_name == "Bot") then return false end
    if config.block_abnormal_ua and (browser == "Bot" or browser == "Other" or browser == "Unknown") then
        return false
    end
    local browsers = array_or_empty(config.browsers)
    local operating_systems = array_or_empty(config.operating_systems)
    local has_browsers = #browsers > 0
    local has_os = #operating_systems > 0
    if not has_browsers and not has_os then return true end
    local browser_ok = ua_label_in(browsers, browser)
    local os_ok = ua_label_in(operating_systems, os_name)
    if has_browsers and not has_os then return browser_ok end
    if has_os and not has_browsers then return os_ok end
    local mode = config.match_mode
    if mode ~= "and" and mode ~= "or" then mode = "or" end
    if mode == "and" then return browser_ok and os_ok end
    return browser_ok or os_ok
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

local function execute_graph(graph)
    if type(graph) ~= "table" or type(graph.nodes) ~= "table" or type(graph.entry) ~= "string" then
        return nil, "invalid graph"
    end
    local node_count = 0
    for _ in pairs(graph.nodes) do node_count = node_count + 1 end
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

local function active_rules(site)
    local by_id, result = {}, {}
    for _, rule in ipairs(array_or_empty(rules_config.rule_groups)) do
        by_id[tostring(rule.id)] = rule
        if rule.enabled and rule.is_global then result[#result + 1] = rule end
    end
    for _, binding in ipairs(array_or_empty(rules_config.bindings)) do
        if binding.site_name == site then
            for _, id in ipairs(array_or_empty(binding.rule_group_ids)) do
                local rule = by_id[tostring(id)]
                if rule and rule.enabled and not rule.is_global then result[#result + 1] = rule end
            end
            break
        end
    end
    return result
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

return _M
