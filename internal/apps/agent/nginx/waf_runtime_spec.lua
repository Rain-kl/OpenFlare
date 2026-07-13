local runtime_path = assert(WAF_RUNTIME_PATH, "WAF_RUNTIME_PATH is required")

local function assert_equal(actual, expected, message)
    if actual ~= expected then
        error((message or "values differ") .. ": expected " .. tostring(expected) .. ", got " .. tostring(actual), 2)
    end
end

local output
local pow_calls
local pow_results
local shared_keys = {}
local logs = {}

ngx = {
    WARN = "WARN",
    ERR = "ERR",
    var = {},
    ctx = {},
    header = {},
    shared = {
        openflare_waf_config = {
            add = function(_, key)
                if shared_keys[key] then return false end
                shared_keys[key] = true
                return true
            end,
        },
    },
    req = { is_internal = function() return ngx.var.openflare_internal == true end },
    say = function(body) output.body = body end,
    exit = function(status) output.exit = status return status end,
    log = function(_, ...)
        local parts = { ... }
        for index, value in ipairs(parts) do parts[index] = tostring(value) end
        output.log = table.concat(parts)
        logs[#logs + 1] = output.log
    end,
}

local pow_stub = {}
function pow_stub.evaluate(config)
    pow_calls[#pow_calls + 1] = config.difficulty
    local result = pow_results[1]
    table.remove(pow_results, 1)
    return result
end

local function node(node_type, config, next_nodes)
    return { type = node_type, config = config or {}, next = next_nodes }
end

local function graph(nodes, entry)
    return { entry = entry or "start", nodes = nodes }
end

local function rule(id, is_global, rule_graph)
    return { id = id, enabled = true, is_global = is_global or false, graph = rule_graph }
end

local function start_to(target)
    return node("start", {}, { next = target })
end

local function load_runtime(config, options)
    local chunk = assert(loadfile(runtime_path))
    local runtime = chunk()
    options = options or {}
    runtime.init({
        config = config,
        ip_groups = options.ip_groups or { groups = {} },
        pow = pow_stub,
        geo_lookup = options.geo_lookup,
        runtime_dir = options.runtime_dir,
        geo_file_exists = options.geo_file_exists,
        country_mmdb_path = options.country_mmdb_path or (options.runtime_dir and (options.runtime_dir .. "/GeoLite2-Country.mmdb") or nil),
        city_mmdb_path = options.city_mmdb_path or (options.runtime_dir and (options.runtime_dir .. "/GeoLite2-City.mmdb") or nil),
    })
    return runtime
end

local function reset_request(site, ip, uri, is_internal)
    ngx.var = { openflare_waf_site = site, remote_addr = ip or "192.0.2.1", uri = uri or "/", request_id = "request-1", openflare_internal = is_internal == true }
    ngx.ctx = {}
    ngx.header = {}
    output = {}
    pow_calls = {}
    pow_results = {}
end

local function binding(site, ids)
    return { site_name = site, rule_group_ids = ids }
end

local function test_ip_true_and_false()
    local config = {
        rule_groups = { rule(1, false, graph({
            start = start_to("match"),
            match = node("ip_match", { ips = { "192.0.2.1" }, cidrs = { "198.51.100.0/24" }, ip_group_ids = { 7 } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 451, response_body = "ip blocked" }),
            allow = node("allow"),
        })) },
        bindings = { binding("ip-site", { 1 }) },
    }
    local runtime = load_runtime(config, { ip_groups = { groups = { ["7"] = { enabled = true, ip_list = { "203.0.113.7" } } } } })

    reset_request("ip-site", "192.0.2.1")
    runtime.check()
    assert_equal(output.exit, 451, "exact IP true branch")

    reset_request("ip-site", "198.51.100.8")
    runtime.check()
    assert_equal(output.exit, 451, "CIDR true branch")

    reset_request("ip-site", "203.0.113.7")
    runtime.check()
    assert_equal(output.exit, 451, "IP group true branch")

    reset_request("ip-site", "203.0.113.8")
    runtime.check()
    assert_equal(output.exit, nil, "IP false branch")
end

local function test_ipv6_exact_cidr_and_group()
    local config = {
        rule_groups = { rule(8, false, graph({
            start = start_to("match"),
            match = node("ip_match", { ips = { "2001:db8::1" }, cidrs = { "2001:db8:abcd::/48" }, ip_group_ids = { 9 } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 451, response_body = "ipv6 blocked" }),
            allow = node("allow"),
        })) },
        bindings = { binding("ipv6-site", { 8 }) },
    }
    local runtime = load_runtime(config, { ip_groups = { groups = { ["9"] = { enabled = true, ip_list = { "2001:db8:ffff::/48" } } } } })

    reset_request("ipv6-site", "2001:0db8:0:0:0:0:0:1")
    runtime.check()
    assert_equal(output.exit, 451, "canonical-equivalent IPv6 exact match")

    reset_request("ipv6-site", "2001:db8:abcd:12::9")
    runtime.check()
    assert_equal(output.exit, 451, "IPv6 CIDR true branch")

    reset_request("ipv6-site", "2001:db8:ffff:beef::9")
    runtime.check()
    assert_equal(output.exit, 451, "IP group IPv6 CIDR true branch")

    reset_request("ipv6-site", "2001:db9::1")
    runtime.check()
    assert_equal(output.exit, nil, "IPv6 false branch")
end

local function test_geo_true_and_false()
    local config = {
        rule_groups = { rule(2, false, graph({
            start = start_to("geo"),
            geo = node("geo_match", { countries = { "US" }, regions = { "DE-BE" } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 403, response_body = "geo blocked" }),
            allow = node("allow"),
        })) },
        bindings = { binding("geo-site", { 2 }) },
    }
    local country, region = "US", "NY"
    local runtime = load_runtime(config, { geo_lookup = function() return country, region end })

    reset_request("geo-site")
    runtime.check()
    assert_equal(output.exit, 403, "country true branch")

    country, region = "DE", "DE-BE"
    reset_request("geo-site")
    runtime.check()
    assert_equal(output.exit, 403, "region true branch")

    country, region = "DE", "BE"
    reset_request("geo-site")
    runtime.check()
    assert_equal(output.exit, nil, "geo false branch")
end

local function test_geo_module_is_initialized_once_and_composes_region()
    local init_calls, lookup_calls = 0, 0
    local initialized_profiles = {}
    package.loaded["resty.maxminddb"] = nil
    package.preload["resty.maxminddb"] = function()
        return {
            init = function(profiles)
                init_calls = init_calls + 1
                for profile, path in pairs(profiles) do initialized_profiles[profile] = path end
                return true
            end,
            has_profile = function(profile) return initialized_profiles[profile] ~= nil end,
            lookup = function(_, _, profile)
                lookup_calls = lookup_calls + 1
                assert_equal(profile, "city", "subdivision lookup uses City profile")
                return { country = { iso_code = "US" }, subdivisions = { { iso_code = "CA" } } }
            end,
        }
    end
    local config = {
        rule_groups = { rule(12, false, graph({
            start = start_to("geo"),
            geo = node("geo_match", { regions = { "US-CA" } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 403 }),
            allow = node("allow"),
        })) },
        bindings = { binding("geo-cache", { 12 }) },
    }
    local runtime = load_runtime(config, { runtime_dir = "/runtime", geo_file_exists = function() return true end })
    assert_equal(init_calls, 2, "each MaxMind profile initializes independently during worker init")
    assert_equal(initialized_profiles.city, "/runtime/GeoLite2-City.mmdb", "City profile path")
    assert_equal(initialized_profiles.country, "/runtime/GeoLite2-Country.mmdb", "Country profile path")
    for _ = 1, 3 do
        reset_request("geo-cache")
        runtime.check()
        assert_equal(output.exit, 403, "MaxMind subdivision composes validator-compatible region")
    end
    assert_equal(init_calls, 2, "MaxMind database is not initialized on requests")
    assert_equal(lookup_calls, 3, "requests only perform lookup")
end

local function test_geo_country_fallback_does_not_fake_region()
    local profiles
    package.loaded["resty.maxminddb"] = nil
    package.preload["resty.maxminddb"] = function()
        return {
            init = function(value) profiles = value return true end,
            has_profile = function(profile) return profiles[profile] ~= nil end,
            lookup = function(_, _, profile)
                assert_equal(profile, "country", "fallback lookup uses Country profile")
                return { country = { iso_code = "US" }, subdivisions = { { iso_code = "CA" } } }
            end,
        }
    end
    shared_keys = {}
    logs = {}
    local country_graph = graph({
        start = start_to("geo"),
        geo = node("geo_match", { countries = { "US" } }, { ["true"] = "blocked", ["false"] = "allow" }),
        blocked = node("block", { status_code = 403 }), allow = node("allow"),
    })
    local region_graph = graph({
        start = start_to("geo"),
        geo = node("geo_match", { regions = { "US-CA" } }, { ["true"] = "blocked", ["false"] = "allow" }),
        blocked = node("block", { status_code = 451 }), allow = node("allow"),
    })
    local runtime = load_runtime({
        rule_groups = { rule(15, false, country_graph), rule(16, false, region_graph) },
        bindings = { binding("country-only", { 15 }), binding("region-without-city", { 16 }) },
    }, {
        runtime_dir = "/runtime",
        geo_file_exists = function(path) return string.find(path, "Country", 1, true) ~= nil end,
    })

    reset_request("country-only")
    runtime.check()
    assert_equal(output.exit, 403, "Country fallback remains available")

    reset_request("region-without-city")
    runtime.check()
    assert_equal(output.exit, nil, "Country subdivisions must not satisfy region")
    runtime.check()
    assert_equal(#logs, 1, "missing City warning is rate limited")
end

local function test_geo_city_init_failure_retries_country_profile()
    local init_calls = {}
    local profiles = {}
    package.loaded["resty.maxminddb"] = nil
    package.preload["resty.maxminddb"] = function()
        return {
            init = function(value)
                init_calls[#init_calls + 1] = value
                if value.city then return false end
                profiles = value
                return true
            end,
            has_profile = function(profile) return profiles[profile] ~= nil end,
            lookup = function(_, _, profile)
                assert_equal(profile, "country", "corrupt City fallback uses Country")
                return { country = { iso_code = "DE" } }
            end,
        }
    end
    shared_keys = {}
    logs = {}
    local runtime = load_runtime({
        rule_groups = { rule(17, false, graph({
            start = start_to("geo"),
            geo = node("geo_match", { countries = { "DE" }, regions = { "DE-BE" } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 403 }), allow = node("allow"),
        })) },
        bindings = { binding("corrupt-city", { 17 }) },
    }, { runtime_dir = "/runtime", geo_file_exists = function() return true end })

    reset_request("corrupt-city")
    runtime.check()
    assert_equal(#init_calls, 2, "Country profile is retried after City profile init failure")
    assert_equal(output.exit, 403, "Country remains available after corrupt City init")
end

local function test_geo_partial_init_never_looks_up_corrupt_city()
    local opened = {}
    local lookups = {}
    package.loaded["resty.maxminddb"] = nil
    package.preload["resty.maxminddb"] = function()
        return {
            init = function(profiles)
                if profiles.country then opened.country = true end
                if profiles.city then return nil, "corrupt City" end
                return true
            end,
            initted = function() return next(opened) ~= nil end,
            lookup = function(_, _, profile)
                lookups[#lookups + 1] = profile
                assert_equal(opened[profile], true, "lookup must only use an opened profile")
                return { country = { iso_code = "DE" } }
            end,
        }
    end
    local runtime = load_runtime({
        rule_groups = { rule(18, false, graph({
            start = start_to("geo"),
            geo = node("geo_match", { countries = { "DE" } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 403 }), allow = node("allow"),
        })) },
        bindings = { binding("partial-corrupt-city", { 18 }) },
    }, { runtime_dir = "/runtime", geo_file_exists = function() return true end })

    reset_request("partial-corrupt-city")
    runtime.check()
    assert_equal(table.concat(lookups, ","), "country", "corrupt City is never looked up")
    assert_equal(output.exit, 403, "valid Country remains available")
end

local function test_geo_partial_init_never_looks_up_corrupt_country()
    local opened = {}
    local lookups = {}
    package.loaded["resty.maxminddb"] = nil
    package.preload["resty.maxminddb"] = function()
        return {
            init = function(profiles)
                if profiles.city then opened.city = true end
                if profiles.country then return nil, "corrupt Country" end
                return true
            end,
            initted = function() return next(opened) ~= nil end,
            lookup = function(_, _, profile)
                lookups[#lookups + 1] = profile
                assert_equal(opened[profile], true, "lookup must only use an opened profile")
                return nil, "address absent"
            end,
        }
    end
    local runtime = load_runtime({
        rule_groups = { rule(19, false, graph({
            start = start_to("geo"),
            geo = node("geo_match", { countries = { "DE" } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 403 }), allow = node("allow"),
        })) },
        bindings = { binding("partial-corrupt-country", { 19 }) },
    }, { runtime_dir = "/runtime", geo_file_exists = function() return true end })

    reset_request("partial-corrupt-country")
    runtime.check()
    assert_equal(table.concat(lookups, ","), "city", "corrupt Country is never used as fallback")
    assert_equal(output.exit, nil, "missing City result takes false branch without corrupt fallback")
end

local function test_geo_unavailable_warning_is_rate_limited()
    package.loaded["resty.maxminddb"] = nil
    package.preload["resty.maxminddb"] = function() error("module unavailable") end
    shared_keys = {}
    logs = {}
    local config = {
        rule_groups = { rule(13, false, graph({
            start = start_to("geo"),
            geo = node("geo_match", { countries = { "US" } }, { ["true"] = "blocked", ["false"] = "allow" }),
            blocked = node("block", { status_code = 403 }),
            allow = node("allow"),
        })) },
        bindings = { binding("geo-missing", { 13 }) },
    }
    local first = load_runtime(config)
    local second = load_runtime(config)
    reset_request("geo-missing")
    first.check()
    second.check()
    assert_equal(#logs, 1, "missing MaxMind warning is rate limited across workers")
end

local function test_pow_takeover_and_completion()
    local config = {
        rule_groups = { rule(3, false, graph({
            start = start_to("pow"),
            pow = node("pow", { algorithm = "fast", difficulty = 5, session_ttl = 600, challenge_ttl = 300 }, { next = "blocked" }),
            blocked = node("block", { status_code = 429, response_body = "after pow" }),
            allow = node("allow"),
        })) },
        bindings = { binding("pow-site", { 3 }) },
    }
    local runtime = load_runtime(config)

    reset_request("pow-site")
    pow_results = { false }
    runtime.check()
    assert_equal(output.exit, nil, "PoW takeover must stop graph execution")
    assert_equal(#pow_calls, 1, "PoW evaluated once")

    reset_request("pow-site")
    pow_results = { true }
    runtime.check()
    assert_equal(output.exit, 429, "completed PoW follows next edge")
end

local function test_pow_internal_redirect_bypasses_graph_as_takeover()
    local config = {
        rule_groups = { rule(14, false, graph({
            start = start_to("pow"),
            pow = node("pow", { difficulty = 4 }, { next = "blocked" }),
            blocked = node("block", { status_code = 429 }),
            allow = node("allow"),
        })) },
        bindings = { binding("pow-internal", { 14 }) },
    }
    local runtime = load_runtime(config)
    reset_request("pow-internal", "192.0.2.1", "/.within.website/x/cmd/anubis/api/make-challenge", true)
    pow_results = { true }
    runtime.check()
    assert_equal(#pow_calls, 0, "internal challenge continuation must not re-enter DAG")
    assert_equal(output.exit, nil, "internal challenge continuation must not follow pow next")
end

local function test_block_config_and_rule_order()
    local function pow_allow(difficulty)
        return graph({
            start = start_to("pow"),
            pow = node("pow", { algorithm = "fast", difficulty = difficulty, session_ttl = 600, challenge_ttl = 300 }, { next = "allow" }),
            allow = node("allow"),
        })
    end
    local config = {
        rule_groups = {
            rule(10, true, pow_allow(10)),
            rule(20, false, pow_allow(20)),
            rule(30, false, pow_allow(30)),
            rule(40, false, graph({
                start = start_to("blocked"),
                blocked = node("block", { status_code = 418, response_body = "custom block" }),
                allow = node("allow"),
            })),
        },
        bindings = { binding("ordered-site", { 30, 20, 40 }) },
    }
    local runtime = load_runtime(config)

    reset_request("ordered-site")
    pow_results = { true, true, true }
    runtime.check()
    assert_equal(table.concat(pow_calls, ","), "10,30,20", "global rule precedes binding order")
    assert_equal(output.exit, 418, "block status comes from reached block node")
    assert_equal(output.body, "custom block", "block body comes from reached block node")
    assert_equal(ngx.header["Content-Type"], "text/html; charset=utf-8", "block content type")
end

local function test_damaged_graphs_fail_closed()
    local configs = {
        graph({ start = start_to("unknown"), unknown = node("future_node"), allow = node("allow") }),
        graph({ start = start_to("missing"), allow = node("allow") }),
        graph({ start = start_to("loop"), loop = node("start", {}, { next = "loop" }), allow = node("allow") }),
    }
    for index, damaged in ipairs(configs) do
        local runtime = load_runtime({ rule_groups = { rule(index, false, damaged) }, bindings = { binding("damaged", { index }) } })
        reset_request("damaged")
        runtime.check()
        assert_equal(output.exit, 500, "damaged graph " .. index .. " must fail closed")
    end
end

local function test_null_binding_ids_are_treated_as_empty()
    local runtime = load_runtime({
        rule_groups = {},
        -- cjson decodes JSON null to userdata (ngx.null). io.stdout provides the
        -- same Lua value type in this standalone regression test.
        bindings = { binding("null-binding", io.stdout) },
    })

    reset_request("null-binding")
    local result = runtime.check()
    assert_equal(result, "ok", "null binding IDs allow the request")
    assert_equal(output.exit, nil, "null binding IDs never abort the request")
end

local function test_request_path_has_no_file_io()
    local opens = 0
    local original_open = io.open
    io.open = function(path, mode)
        opens = opens + 1
        local value = path:match("waf_ip_groups%.json$") and "IP_GROUPS" or "CONFIG"
        return {
            read = function() return value end,
            close = function() end,
        }
    end
    package.loaded["cjson.safe"] = nil
    package.preload["cjson.safe"] = function()
        return { decode = function(value)
            if value == "IP_GROUPS" then return { groups = {} } end
            return {
                rule_groups = { rule(1, false, graph({ start = start_to("allow"), allow = node("allow") })) },
                bindings = { binding("io-site", { 1 }) },
            }
        end }
    end
    local chunk = assert(loadfile(runtime_path))
    local runtime = chunk()
    runtime.init({
        runtime_dir = "/runtime",
        pow = pow_stub,
        ip_groups_runtime = {
            init = function() return true end,
            current = function() return { groups = {} } end,
        },
    })
    local init_opens = opens
    assert_equal(init_opens, 1, "WAF graph initializes once; IP groups are owned by refresh module")

    reset_request("io-site")
    for _ = 1, 3 do runtime.check() end
    assert_equal(opens, init_opens, "request execution performs no file I/O")
    io.open = original_open
end

test_ip_true_and_false()
test_ipv6_exact_cidr_and_group()
test_geo_true_and_false()
test_geo_module_is_initialized_once_and_composes_region()
test_geo_country_fallback_does_not_fake_region()
test_geo_city_init_failure_retries_country_profile()
test_geo_partial_init_never_looks_up_corrupt_city()
test_geo_partial_init_never_looks_up_corrupt_country()
test_geo_unavailable_warning_is_rate_limited()
test_pow_takeover_and_completion()
test_pow_internal_redirect_bypasses_graph_as_takeover()
test_block_config_and_rule_order()
test_damaged_graphs_fail_closed()
test_null_binding_ids_are_treated_as_empty()
test_request_path_has_no_file_io()

return true
