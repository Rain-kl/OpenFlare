package nginx

import "github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"

// Local OpenResty observability endpoint (target model):
// GET /openflare/observability returns instantaneous health/connections only.
// Business traffic is collected exclusively from access.log.

const openRestyObservabilityInitLua = `return
`

// log.lua no longer accumulates business counters (access.log is the authority).
const openRestyObservabilityLogLua = `return
`

// read.lua exposes stub_status-style connection gauges as JSON.
const openRestyObservabilityReadLua = `local cjson = require "cjson.safe"

local function read_stub_status()
    local res = ngx.location.capture("/openflare/stub_status")
    if not res or res.status ~= 200 or not res.body then
        return nil
    end
    local body = res.body
    local active = tonumber(string.match(body, "Active connections:%s*(%d+)")) or 0
    local reading = tonumber(string.match(body, "Reading:%s*(%d+)")) or 0
    local writing = tonumber(string.match(body, "Writing:%s*(%d+)")) or 0
    local waiting = tonumber(string.match(body, "Waiting:%s*(%d+)")) or 0
    return {
        active = active,
        reading = reading,
        writing = writing,
        waiting = waiting
    }
end

local connections = read_stub_status()
local payload = {
    ok = connections ~= nil,
    captured_at_unix = ngx.time(),
    connections = connections or {
        active = 0,
        reading = 0,
        writing = 0,
        waiting = 0
    }
}

ngx.header.content_type = "application/json"
ngx.status = ngx.HTTP_OK
ngx.say(cjson.encode(payload))
`

// ManagedObservabilityLuaFiles returns embedded Lua assets for OpenResty observability.
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
