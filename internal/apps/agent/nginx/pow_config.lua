local cjson = require "cjson.safe"
local policy = require "pow.policy"
local M = {}
local sites = {}

-- Policies belong to the published version; each new worker loads its snapshot.
local file, open_error, code = io.open("__OPENFLARE_RUNTIME_CONFIG_DIR__/pow_config.json", "r")
if file then
    local raw = file:read("*a")
    file:close()
    local decoded, decode_error = cjson.decode(raw)
    assert(type(decoded) == "table", "load pow_config.json failed: " .. tostring(decode_error))
    sites = decoded
elseif code ~= 2 then
    error("load pow_config.json failed: " .. tostring(open_error))
end
for _, site in pairs(sites) do
    if type(site) == "table" and type(site.config) == "table" then
        policy.prepare(site.config)
    end
end

function M.get(site)
    local policy = sites[site]
    return type(policy) == "table" and policy or nil
end

return M
