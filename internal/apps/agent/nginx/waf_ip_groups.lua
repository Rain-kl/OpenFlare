local _M = {}

local current_groups = { groups = {} }
local current_version
local initialized = false
local shared
local read_checksum
local read_json
local decode
local log_warning
local max_snapshot_bytes

local refresh_lock_key = "ip_groups_refresh_lock"
local raw_snapshot_prefix = "ip_groups_raw:"
local version_key = "ip_groups_version"
local previous_version_key = "ip_groups_previous_version"

local function warn(message, err, forcible)
    local suffix = err and (": " .. tostring(err)) or ""
    if forcible then suffix = suffix .. " (forcible eviction refused)" end
    pcall(log_warning, "openflare WAF IP group refresh " .. message .. suffix)
end

local function safe_set(key, value, description)
    local ok, err, forcible = shared:safe_set(key, value)
    if ok ~= true or forcible == true then
        warn(description, err, forcible)
        return false
    end
    return true
end

local function read_file(path)
    local file, err = io.open(path, "rb")
    if not file then return nil, err end
    local content = file:read("*a")
    file:close()
    return content
end

local function valid_snapshot(snapshot)
    return type(snapshot) == "table" and type(snapshot.groups) == "table"
end

local function decode_snapshot(raw)
    if type(raw) ~= "string" or raw == "" then return nil end
    local called, snapshot = pcall(decode, raw)
    if not called or not valid_snapshot(snapshot) then return nil end
    return snapshot
end

local function refresh_from_checksum()
    local called, checksum = pcall(read_checksum)
    if not called or type(checksum) ~= "string" then return end
    checksum = string.match(checksum, "^%s*(.-)%s*$")
    local committed_version = shared:get(version_key)
    if checksum == "" or checksum == committed_version then return end

    local json_called, raw = pcall(read_json)
    if not json_called then
        warn("JSON read failed", raw)
        return
    end
    if type(raw) ~= "string" or #raw > max_snapshot_bytes then
        warn("snapshot exceeds maximum " .. tostring(max_snapshot_bytes) .. " bytes")
        return
    end
    if not decode_snapshot(raw) then return end
    local raw_key = raw_snapshot_prefix .. checksum
    local existing_raw = shared:get(raw_key)
    local published_new_raw = false
    if existing_raw == nil then
        if not safe_set(raw_key, raw, "raw publication failed") then return end
        published_new_raw = true
    elseif existing_raw ~= raw then
        return
    end
    if not safe_set(version_key, checksum, "commit pointer publication failed") then
        if published_new_raw then shared:delete(raw_key) end
        return
    end

    local previous_version = shared:get(previous_version_key)
    if type(committed_version) == "string" and committed_version ~= "" and committed_version ~= checksum then
        if not safe_set(previous_version_key, committed_version, "previous version metadata publication failed") then return end
        if type(previous_version) == "string" and previous_version ~= "" and
            previous_version ~= committed_version and previous_version ~= checksum then
            shared:delete(raw_snapshot_prefix .. previous_version)
        end
    end
end

local function adopt_shared_snapshot_if_changed()
    local version = shared:get(version_key)
    if type(version) ~= "string" or version == "" or version == current_version then return end
    local snapshot = decode_snapshot(shared:get(raw_snapshot_prefix .. version))
    if not snapshot then return end
    current_groups = snapshot
    current_version = version
end

local function tick(premature)
    if premature then return end
    local locked, lock_error, forcible = shared:safe_add(refresh_lock_key, true, 4)
    if forcible == true then
        warn("coordination lock refused forcible eviction", lock_error, true)
        locked = false
    elseif not locked and lock_error and lock_error ~= "exists" then
        warn("coordination lock failed", lock_error)
    end
    if locked then refresh_from_checksum() end
    adopt_shared_snapshot_if_changed()
end

function _M.init(options)
    if initialized then return true end
    options = options or {}
    local runtime_dir = options.runtime_dir or "__OPENFLARE_RUNTIME_CONFIG_DIR__"
    shared = options.shared or (ngx.shared and ngx.shared.openflare_waf_ip_groups)
    assert(shared, "openflare_waf_ip_groups shared dictionary is required")
    max_snapshot_bytes = options.max_snapshot_bytes or tonumber("__OPENFLARE_WAF_IP_GROUPS_MAX_SNAPSHOT_BYTES__")
    assert(max_snapshot_bytes and max_snapshot_bytes > 0, "WAF IP group maximum snapshot size is required")
    log_warning = options.log_warning or function(message)
        if ngx and ngx.log then ngx.log(ngx.WARN, message) end
    end
    read_checksum = options.read_checksum or function()
        return read_file(runtime_dir .. "/waf_ip_groups.json.checksum")
    end
    read_json = options.read_json or function()
        return read_file(runtime_dir .. "/waf_ip_groups.json")
    end
    if options.decode then
        decode = options.decode
    else
        local cjson = require("cjson.safe")
        decode = cjson.decode
    end
    local timer_every = options.timer_every or ngx.timer.every
    local ok, err = timer_every(5, tick)
    if not ok then return nil, err end
    initialized = true
    tick(false)
    return true
end

function _M.current()
    return current_groups
end

return _M
