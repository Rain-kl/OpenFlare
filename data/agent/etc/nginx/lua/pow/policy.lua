local M = {}

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
