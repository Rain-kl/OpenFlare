package nginx

import "github.com/Rain-kl/Wavelet/internal/agent/protocol"

const (
	openRestyObservabilityWindowTTL  = "7200"
	openRestyObservabilityWindowSize = "60"
)

const openRestyObservabilityInitLua = `local dict = ngx.shared.openflare_observability
if not dict then
    return
end

return
`

const openRestyObservabilityLogLua = `local dict = ngx.shared.openflare_observability
if not dict then
    return
end

local request_uri = tostring(ngx.var.uri or "")
if request_uri == "/openflare/observability" or request_uri == "/openflare/stub_status" then
    return
end

local ttl = ` + openRestyObservabilityWindowTTL + `
local now = ngx.time()
local window_size = ` + openRestyObservabilityWindowSize + `
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
`

const openRestyObservabilityReadLua = `local cjson = require "cjson.safe"

local dict = ngx.shared.openflare_observability
if not dict then
    ngx.status = ngx.HTTP_SERVICE_UNAVAILABLE
    ngx.say(cjson.encode({ message = "shared dict unavailable" }))
    return
end

local now = ngx.time()
local window_size = ` + openRestyObservabilityWindowSize + `
local window_start = now - (now % window_size)
local current_window = tostring(window_start)

local function read_counter(key)
    return tonumber(dict:get(key) or 0) or 0
end

local function read_map(window_id, prefix, list_key)
    local result = {}
    local raw = dict:get(list_key .. ":" .. window_id)
    if not raw or raw == "" then
        return result
    end
    for value in string.gmatch(raw, "[^\n]+") do
        result[value] = read_counter(prefix .. ":" .. window_id .. ":" .. value)
    end
    return result
end

local payload = {
    window_started_at_unix = window_start,
    window_ended_at_unix = now,
    request_count = read_counter("request_count:" .. current_window),
    error_count = read_counter("error_count:" .. current_window),
    unique_visitor_count = read_counter("unique_visitor_count:" .. current_window),
    status_codes = read_map(current_window, "status", "status_keys"),
    top_domains = read_map(current_window, "domain", "domain_keys"),
    source_countries = {},
    openresty_rx_bytes = read_counter("openresty_rx_bytes:" .. current_window),
    openresty_tx_bytes = read_counter("openresty_tx_bytes:" .. current_window)
}

ngx.header.content_type = "application/json"
ngx.say(cjson.encode(payload))
`

func ManagedObservabilityLuaFiles() []protocol.SupportFile {
	return []protocol.SupportFile{
		{Path: "init.lua", Content: openRestyObservabilityInitLua},
		{Path: "log.lua", Content: openRestyObservabilityLogLua},
		{Path: "read.lua", Content: openRestyObservabilityReadLua},
		{Path: "observability/init.lua", Content: openRestyObservabilityInitLua},
		{Path: "observability/log.lua", Content: openRestyObservabilityLogLua},
		{Path: "observability/read.lua", Content: openRestyObservabilityReadLua},
	}
}
