local _M = {}

local function array_or_empty(value)
    return type(value) == "table" and value or {}
end

local function warn_rate_limited(key, ...)
    local dict = ngx.shared and ngx.shared.openflare_waf_config
    if not dict or not dict.add or dict:add(key, true, 60) then
        ngx.log(ngx.WARN, ...)
    end
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

local function ipv6_key(groups)
    return table.concat(groups, ":")
end

local function preparse_cidr(cidr)
    local base, bits = string.match(cidr or "", "^([^/]+)/(%d+)$")
    bits = tonumber(bits)
    if not base or not bits then return nil end
    local base_v4 = parse_ipv4(base)
    if base_v4 then
        if bits < 0 or bits > 32 then return nil end
        if bits == 0 then return { kind = "v4", bits = 0, network = 0, size = 0 } end
        local size = 2 ^ (32 - bits)
        return { kind = "v4", bits = bits, network = base_v4 - (base_v4 % size), size = size }
    end
    local base_v6 = parse_ipv6(base)
    if not base_v6 or bits < 0 or bits > 128 then return nil end
    return { kind = "v6", bits = bits, groups = base_v6 }
end

local function ipv4_in_preparsed(ip_number, cidr)
    if cidr.bits == 0 then return true end
    return ip_number - (ip_number % cidr.size) == cidr.network
end

local function ipv6_in_preparsed(ip_groups, cidr)
    local full_groups, remaining_bits = math.floor(cidr.bits / 16), cidr.bits % 16
    for index = 1, full_groups do
        if ip_groups[index] ~= cidr.groups[index] then return false end
    end
    if remaining_bits > 0 then
        local size = 2 ^ (16 - remaining_bits)
        local index = full_groups + 1
        if math.floor(ip_groups[index] / size) ~= math.floor(cidr.groups[index] / size) then
            return false
        end
    end
    return true
end

-- Prefer resty.ipmatcher. Keep the pre-parsed Lua fallback for unsupported runtimes.
local resty_ipmatcher
local resty_ipmatcher_loaded = false

local function load_resty_ipmatcher()
    if resty_ipmatcher_loaded then return resty_ipmatcher end
    resty_ipmatcher_loaded = true
    local ok, mod = pcall(require, "resty.ipmatcher")
    if ok and type(mod) == "table" and type(mod.new) == "function" then
        resty_ipmatcher = mod
    else
        resty_ipmatcher = nil
    end
    return resty_ipmatcher
end

local empty_ip_matcher = {
    empty = true,
    match = function() return false end,
}

local function compile_fallback_ip_matcher(entries)
    local exact, cidrs = {}, {}
    for _, item in ipairs(entries) do
        if string.find(item, "/", 1, true) then
            local parsed = preparse_cidr(item)
            if parsed then cidrs[#cidrs + 1] = parsed end
        else
            exact[item] = true
            local v6 = parse_ipv6(item)
            if v6 then exact["v6:" .. ipv6_key(v6)] = true end
        end
    end
    return {
        empty = false,
        match = function(_, ip, _bin, ip_v4, ip_v6)
            if exact[ip] then return true end
            if not ip_v4 and not ip_v6 then
                ip_v4 = parse_ipv4(ip)
                if not ip_v4 then ip_v6 = parse_ipv6(ip) end
            end
            if ip_v6 and exact["v6:" .. ipv6_key(ip_v6)] then return true end
            for _, cidr in ipairs(cidrs) do
                if cidr.kind == "v4" and ip_v4 and ipv4_in_preparsed(ip_v4, cidr) then
                    return true
                end
                if cidr.kind == "v6" and ip_v6 and ipv6_in_preparsed(ip_v6, cidr) then
                    return true
                end
            end
            return false
        end,
    }
end

local function compile_ip_matcher(entries)
    local list = {}
    for _, item in ipairs(array_or_empty(entries)) do
        if type(item) == "string" and item ~= "" then
            list[#list + 1] = item
        end
    end
    if #list == 0 then return empty_ip_matcher end

    local mod = load_resty_ipmatcher()
    if mod then
        local matcher, err = mod.new(list)
        if matcher then
            return {
                empty = false,
                match = function(_, ip, bin_ip)
                    if bin_ip and matcher.match_bin then
                        local matched, match_error = matcher:match_bin(bin_ip)
                        if not match_error then return matched == true end
                    end
                    return matcher:match(ip) == true
                end,
            }
        end
        warn_rate_limited("_ipmatcher_new_failed", "openflare waf ipmatcher.new failed: ", err)
    end
    return compile_fallback_ip_matcher(list)
end

function _M.parse(ip)
    if load_resty_ipmatcher() then return nil, nil end
    local ipv4 = parse_ipv4(ip)
    if ipv4 then return ipv4, nil end
    return nil, parse_ipv6(ip)
end

function _M.prepare_groups(snapshot)
    for _, group in pairs(snapshot.groups or {}) do
        if type(group) == "table" and group.enabled then
            group._matcher = compile_ip_matcher(group.ip_list)
        end
    end
end

_M.compile = compile_ip_matcher

return _M
