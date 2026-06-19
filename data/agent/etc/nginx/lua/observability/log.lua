local dict = ngx.shared.openflare_observability
if not dict then
    return
end

local request_uri = tostring(ngx.var.uri or "")
if request_uri == "/openflare/observability" or request_uri == "/openflare/stub_status" then
    return
end

local ttl = 7200
local now = ngx.time()
local window_size = 60
local window_start = now - (now % window_size)

local function ensure_counter(key)
    dict:add(key, 0, ttl)
end

local function incr(key, delta)
    ensure_counter(key)
    local value, err = dict:incr(key, delta)
    if not value and err == "not found" then
        dict:set(key, delta, ttl)
    end
end

local function remember_value(list_key, marker_key, value)
    if value == "" then
        return
    end
    if not dict:add(marker_key, 1, ttl) then
        return
    end
    local existing = dict:get(list_key)
    if not existing or existing == "" then
        dict:set(list_key, value, ttl)
        return
    end
    dict:set(list_key, existing .. "\n" .. value, ttl)
end

local window_prefix = tostring(window_start)
incr("request_count:" .. window_prefix, 1)

local status = tostring(ngx.status or 0)
if status ~= "0" then
    incr("status:" .. window_prefix .. ":" .. status, 1)
    remember_value(
        "status_keys:" .. window_prefix,
        "status_marker:" .. window_prefix .. ":" .. status,
        status
    )
    if tonumber(status) and tonumber(status) >= 500 then
        incr("error_count:" .. window_prefix, 1)
    end
end

local host = tostring(ngx.var.host or "")
if host ~= "" then
    incr("domain:" .. window_prefix .. ":" .. host, 1)
    remember_value(
        "domain_keys:" .. window_prefix,
        "domain_marker:" .. window_prefix .. ":" .. host,
        host
    )
end

local remote_addr = tostring(ngx.var.binary_remote_addr or ngx.var.remote_addr or "")
if remote_addr ~= "" and dict:add("visitor:" .. window_prefix .. ":" .. remote_addr, 1, ttl) then
    incr("unique_visitor_count:" .. window_prefix, 1)
end

local request_length = tonumber(ngx.var.request_length) or 0
if request_length > 0 then
    incr("openresty_rx_bytes:" .. window_prefix, request_length)
end

local bytes_sent = tonumber(ngx.var.bytes_sent) or tonumber(ngx.var.body_bytes_sent) or 0
if bytes_sent > 0 then
    incr("openresty_tx_bytes:" .. window_prefix, bytes_sent)
end
