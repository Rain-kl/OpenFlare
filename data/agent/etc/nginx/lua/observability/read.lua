local cjson = require "cjson.safe"

local dict = ngx.shared.openflare_observability
if not dict then
    ngx.status = ngx.HTTP_SERVICE_UNAVAILABLE
    ngx.say(cjson.encode({ message = "shared dict unavailable" }))
    return
end

local now = ngx.time()
local window_size = 60
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
