local module_path = assert(WAF_IP_GROUPS_PATH, "WAF_IP_GROUPS_PATH is required")

local function assert_equal(actual, expected, message)
    if actual ~= expected then
        error((message or "values differ") .. ": expected " .. tostring(expected) .. ", got " .. tostring(actual), 2)
    end
end

local shared_data = {}
local locks = {}
local shared = {}
function shared:get(key) return shared_data[key] end
function shared:set(key, value) shared_data[key] = value return true end
function shared:delete(key) shared_data[key] = nil return true end
function shared:safe_set(key, value) return shared:set(key, value) end
function shared:add(key, value, ttl)
    assert_equal(ttl, 4, "coordination lock TTL")
    if locks[key] then return false end
    locks[key] = value
    return true
end
function shared:safe_add(key, value, ttl) return shared:add(key, value, ttl) end
local function advance_time() locks = {} end

local disk_checksum = "v1"
local disk_json = "valid-v1"
local checksum_reads = 0
local json_reads = 0
local timer_callbacks = {}

local function decode(raw)
    if raw == "valid-v1" then
        return { groups = { ["1"] = { enabled = true, ip_list = { "192.0.2.1" } } } }
    end
    if raw == "valid-v2" then
        return { groups = { ["2"] = { enabled = true, ip_list = { "198.51.100.2" } } } }
    end
    if raw == "valid-v3" then
        return { groups = { ["3"] = { enabled = true, ip_list = { "203.0.113.3" } } } }
    end
    return nil, "invalid json"
end

local function load_worker()
    local worker = assert(loadfile(module_path))()
    worker.init({
        shared = shared,
        timer_every = function(interval, callback)
            assert_equal(interval, 5, "refresh interval")
            timer_callbacks[#timer_callbacks + 1] = callback
            return true
        end,
        read_checksum = function()
            checksum_reads = checksum_reads + 1
            return disk_checksum
        end,
        read_json = function()
            json_reads = json_reads + 1
            return disk_json
        end,
        decode = decode,
        max_snapshot_bytes = 20 * 1024 * 1024,
    })
    return worker
end

local first = load_worker()
local second = load_worker()
assert_equal(#timer_callbacks, 2, "each worker schedules a refresh timer")
assert_equal(checksum_reads, 1, "one worker coordinates initial checksum read")
assert_equal(json_reads, 1, "one worker reads initial JSON")
assert_equal(first.current().groups["1"].ip_list[1], "192.0.2.1", "first worker adopts initial snapshot")
assert_equal(second.current().groups["1"].ip_list[1], "192.0.2.1", "second worker adopts initial snapshot")

local function tick_all()
    advance_time()
    for _, callback in ipairs(timer_callbacks) do callback(false) end
end

checksum_reads = 0
json_reads = 0
for _ = 1, 3 do tick_all() end
assert_equal(checksum_reads, 3, "stable 15 seconds reads checksum once per interval")
assert_equal(json_reads, 0, "unchanged checksum never reads JSON")

disk_checksum = "v2"
disk_json = "valid-v2"
tick_all()
assert_equal(json_reads, 1, "changed snapshot JSON is read once across workers")
assert_equal(first.current().groups["2"].ip_list[1], "198.51.100.2", "first worker adopts v2")
assert_equal(second.current().groups["2"].ip_list[1], "198.51.100.2", "second worker adopts v2")

disk_checksum = "v3"
disk_json = "valid-v3"
tick_all()
assert_equal(shared_data.ip_groups_previous_version, "v2", "previous pointer follows committed version")
assert_equal(shared_data["ip_groups_raw:v1"], nil, "snapshot older than previous is cleaned")
assert_equal(shared_data["ip_groups_raw:v2"], "valid-v2", "previous committed raw is retained")
assert_equal(shared_data["ip_groups_raw:v3"], "valid-v3", "current committed raw is retained")

disk_checksum = "v2"
disk_json = "valid-v2"
tick_all()
assert_equal(shared_data.ip_groups_version, "v2", "rollback checksum becomes current commit")
assert_equal(shared_data.ip_groups_previous_version, "v3", "rollback retains former current as previous")
assert_equal(shared_data["ip_groups_raw:v2"], "valid-v2", "rollback must not clean its new current raw")
assert_equal(shared_data["ip_groups_raw:v3"], "valid-v3", "rollback retains previous raw")

disk_checksum = "v4"
disk_json = "invalid-v4"
tick_all()
assert_equal(shared_data.ip_groups_version, "v2", "invalid update preserves shared version")
assert_equal(first.current().groups["2"].ip_list[1], "198.51.100.2", "invalid update preserves first worker")
assert_equal(second.current().groups["2"].ip_list[1], "198.51.100.2", "invalid update preserves second worker")

local reads_before_requests = checksum_reads + json_reads
for _ = 1, 20 do
    assert_equal(first.current().groups["2"].enabled, true, "request reads worker-local object")
end
assert_equal(checksum_reads + json_reads, reads_before_requests, "current() performs zero file I/O")

timer_callbacks[1](true)
assert_equal(checksum_reads + json_reads, reads_before_requests, "premature timer performs zero file I/O")

local function test_failed_commit_never_exposes_unpublished_raw_to_new_worker()
    local data = {}
    local held_locks = {}
    local callbacks = {}
    local checksum = "v1"
    local raw = "valid-v1"
    local reads = 0
    local fail_commit = false
    local interleaved_worker
    local load_regression_worker
    local regression_shared = {}

    function regression_shared:get(key) return data[key] end
    function regression_shared:add(key, value)
        if held_locks[key] then return false end
        held_locks[key] = value
        return true
    end
    function regression_shared:delete(key) data[key] = nil return true end
    local function set_regression_value(key, value)
        if key == "ip_groups_version" and fail_commit then
            return false, "shared dictionary full"
        end
        data[key] = value
        if fail_commit and string.sub(key, 1, #"ip_groups_raw") == "ip_groups_raw" and not interleaved_worker then
            interleaved_worker = load_regression_worker()
        end
        return true
    end
    function regression_shared:set(key, value) return set_regression_value(key, value) end
    function regression_shared:safe_set(key, value) return set_regression_value(key, value) end
    function regression_shared:safe_add(key, value) return regression_shared:add(key, value) end

    load_regression_worker = function()
        local worker = assert(loadfile(module_path))()
        assert(worker.init({
            shared = regression_shared,
            timer_every = function(_, callback) callbacks[#callbacks + 1] = callback return true end,
            read_checksum = function() return checksum end,
            read_json = function() reads = reads + 1 return raw end,
            decode = decode,
            max_snapshot_bytes = 20 * 1024 * 1024,
        }))
        return worker
    end

    local established_worker = load_regression_worker()
    assert_equal(established_worker.current().groups["1"].ip_list[1], "192.0.2.1", "v1 is committed before failure")

    held_locks = {}
    reads = 0
    checksum = "v2"
    raw = "valid-v2"
    fail_commit = true
    callbacks[1](false)

    assert_equal(reads, 1, "failed commit still reads changed JSON only once")
    assert_equal(data.ip_groups_version, "v1", "failed pointer write preserves committed version")
    assert_equal(data["ip_groups_raw:v2"], nil, "failed commit cleans only unpublished v2 raw")
    assert_equal(established_worker.current().groups["1"].ip_list[1], "192.0.2.1", "existing worker preserves committed v1")
    assert(interleaved_worker, "raw publication must interleave a newly initialized worker")
    assert_equal(interleaved_worker.current().groups["2"], nil, "new worker must not expose unpublished v2")
    assert_equal(interleaved_worker.current().groups["1"].ip_list[1], "192.0.2.1", "new worker must never adopt unpublished v2 raw")
end

test_failed_commit_never_exposes_unpublished_raw_to_new_worker()

local function test_capacity_failure_never_evicts_committed_snapshot()
    local data = {
        ip_groups_version = "v1",
        ip_groups_previous_version = "v0",
        ["ip_groups_raw:v1"] = "valid-v1",
        ["ip_groups_raw:v0"] = "valid-v0",
    }
    local locks = {}
    local callbacks = {}
    local disk_checksum = "v1"
    local disk_raw = "valid-v1"
    local json_reads = 0
    local ordinary_writes = 0
    local warnings = {}
    local dict = {}
    function dict:get(key) return data[key] end
    function dict:delete(key) data[key] = nil return true end
    function dict:add(key, value)
        if locks[key] then return false end
        locks[key] = value
        return true
    end
    function dict:safe_add(key, value) return dict:add(key, value) end
    function dict:set(key, value)
        ordinary_writes = ordinary_writes + 1
        if key == "ip_groups_raw:v2" then
            data = { [key] = value }
            return true, nil, true
        end
        data[key] = value
        return true, nil, false
    end
    function dict:safe_set(key, value)
        if key == "ip_groups_raw:v2" then return nil, "no memory", false end
        data[key] = value
        return true, nil, false
    end

    local worker = assert(loadfile(module_path))()
    assert(worker.init({
        shared = dict,
        timer_every = function(_, callback) callbacks[1] = callback return true end,
        read_checksum = function() return disk_checksum end,
        read_json = function() json_reads = json_reads + 1 return disk_raw end,
        decode = decode,
        max_snapshot_bytes = 20 * 1024 * 1024,
        log_warning = function(message) warnings[#warnings + 1] = message end,
    }))
    assert_equal(worker.current().groups["1"].ip_list[1], "192.0.2.1", "worker starts from committed v1")

    locks = {}
    disk_checksum = "v2"
    disk_raw = "valid-v2"
    callbacks[1](false)

    assert_equal(ordinary_writes, 0, "snapshot publication must never use evicting set")
    assert_equal(json_reads, 1, "capacity failure reads changed JSON once")
    assert_equal(data.ip_groups_version, "v1", "capacity failure preserves commit pointer")
    assert_equal(data.ip_groups_previous_version, "v0", "capacity failure preserves previous metadata")
    assert_equal(data["ip_groups_raw:v1"], "valid-v1", "capacity failure preserves current raw")
    assert_equal(data["ip_groups_raw:v0"], "valid-v0", "capacity failure preserves previous raw")
    assert_equal(data["ip_groups_raw:v2"], nil, "capacity failure does not publish new raw")
    assert_equal(worker.current().groups["1"].ip_list[1], "192.0.2.1", "capacity failure preserves worker-local snapshot")
    assert_equal(#warnings, 1, "capacity failure is logged")
end

local function test_previous_metadata_failure_keeps_committed_snapshot_without_cleanup()
    local data = {
        ip_groups_version = "v1",
        ip_groups_previous_version = "v0",
        ["ip_groups_raw:v1"] = "valid-v1",
        ["ip_groups_raw:v0"] = "valid-v0",
    }
    local locks = {}
    local callback
    local checksum = "v1"
    local raw = "valid-v1"
    local deletes = 0
    local warnings = {}
    local dict = {}
    function dict:get(key) return data[key] end
    function dict:delete(key) deletes = deletes + 1 data[key] = nil return true end
    function dict:add(key, value)
        if locks[key] then return false end
        locks[key] = value
        return true
    end
    function dict:safe_add(key, value) return dict:add(key, value) end
    function dict:set(key, value) data[key] = value return true end
    function dict:safe_set(key, value)
        if key == "ip_groups_previous_version" then return nil, "no memory", false end
        data[key] = value
        return true, nil, false
    end

    local worker = assert(loadfile(module_path))()
    assert(worker.init({
        shared = dict,
        timer_every = function(_, value) callback = value return true end,
        read_checksum = function() return checksum end,
        read_json = function() return raw end,
        decode = decode,
        max_snapshot_bytes = 20 * 1024 * 1024,
        log_warning = function(message) warnings[#warnings + 1] = message end,
    }))

    locks = {}
    checksum = "v2"
    raw = "valid-v2"
    callback(false)

    assert_equal(data.ip_groups_version, "v2", "successful commit pointer remains authoritative")
    assert_equal(data.ip_groups_previous_version, "v0", "failed previous metadata write is not forced")
    assert_equal(data["ip_groups_raw:v2"], "valid-v2", "new committed raw remains")
    assert_equal(data["ip_groups_raw:v1"], "valid-v1", "old current raw remains when cleanup is skipped")
    assert_equal(data["ip_groups_raw:v0"], "valid-v0", "old previous raw remains when cleanup is skipped")
    assert_equal(deletes, 0, "previous metadata failure skips all cleanup")
    assert_equal(worker.current().groups["2"].ip_list[1], "198.51.100.2", "worker adopts valid committed v2")
    assert_equal(#warnings, 1, "previous metadata failure is logged")
end

local function test_oversized_raw_is_rejected_before_shared_publication()
    local data = { ip_groups_version = "v1", ["ip_groups_raw:v1"] = "valid-v1" }
    local locks = {}
    local callback
    local checksum = "v1"
    local raw = "valid-v1"
    local shared_writes = 0
    local warnings = {}
    local dict = {}
    function dict:get(key) return data[key] end
    function dict:delete(key) data[key] = nil return true end
    function dict:add(key, value) if locks[key] then return false end locks[key] = value return true end
    function dict:safe_add(key, value) return dict:add(key, value) end
    function dict:set(key, value) shared_writes = shared_writes + 1 data[key] = value return true end
    function dict:safe_set(key, value) shared_writes = shared_writes + 1 data[key] = value return true, nil, false end

    local worker = assert(loadfile(module_path))()
    assert(worker.init({
        shared = dict,
        timer_every = function(_, value) callback = value return true end,
        read_checksum = function() return checksum end,
        read_json = function() return raw end,
        decode = decode,
        max_snapshot_bytes = 4,
        log_warning = function(message) warnings[#warnings + 1] = message end,
    }))

    locks = {}
    checksum = "v2"
    raw = "valid-v2"
    callback(false)

    assert_equal(shared_writes, 0, "oversized raw is rejected before shared writes")
    assert_equal(data.ip_groups_version, "v1", "oversized raw preserves commit pointer")
    assert_equal(data["ip_groups_raw:v1"], "valid-v1", "oversized raw preserves committed data")
    assert_equal(worker.current().groups["1"].ip_list[1], "192.0.2.1", "oversized raw preserves worker-local snapshot")
    assert_equal(#warnings, 1, "oversized raw rejection is logged")
end

test_capacity_failure_never_evicts_committed_snapshot()
test_previous_metadata_failure_keeps_committed_snapshot_without_cleanup()
test_oversized_raw_is_rejected_before_shared_publication()

return true
