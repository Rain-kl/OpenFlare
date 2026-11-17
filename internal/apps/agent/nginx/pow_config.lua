local cjson = require "cjson.safe"
local M = {}
local sites = {}
local next_read = 0

-- Published configuration is immutable between updates. Check for updates at
-- most once per second per worker, preserving the last readable snapshot.
function M.get(site)
    local now = ngx.now()
    if now >= next_read then
        next_read = now + 1
        local file, open_err, code = io.open("__OPENFLARE_RUNTIME_CONFIG_DIR__/pow_config.json", "r")
        if file then
            local raw = file:read("*a")
            file:close()
            local decoded, err = cjson.decode(raw)
            if type(decoded) == "table" then
                sites = decoded
            elseif ngx.log then
                ngx.log(ngx.ERR, "openflare pow configuration decode failed: ", err)
            end
        elseif code == 2 then
            sites = {}
        elseif ngx.log then
            ngx.log(ngx.ERR, "openflare pow configuration read failed: ", open_err)
        end
    end
    local policy = sites[site]
    return type(policy) == "table" and policy or nil
end

return M
