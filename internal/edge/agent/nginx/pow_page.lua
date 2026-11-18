local cjson = require "cjson.safe"
local page = {html = cjson.decode([====[__POW_DEFAULT_HTML__]====]), widget = "progress"}

local file, open_error, code = io.open("__OPENFLARE_POW_PAGE_CONFIG__", "r")
if file then
    local raw = file:read("*a")
    file:close()
    local decoded, decode_error = cjson.decode(raw)
    if type(decoded) == "table" and type(decoded.html) == "string" then
        page = decoded
    else
        ngx.log(ngx.ERR, "openflare pow page decode failed: ", decode_error)
    end
elseif code ~= 2 then
    ngx.log(ngx.ERR, "openflare pow page read failed: ", open_error)
end

return {
    page = page,
    translations = {
        en = cjson.decode([====[__POW_EN__]====]),
        ["zh-CN"] = cjson.decode([====[__POW_ZH__]====]),
    },
}
