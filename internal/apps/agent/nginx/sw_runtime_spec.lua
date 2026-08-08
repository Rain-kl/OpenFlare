local runtime_path = assert(SW_RUNTIME_PATH, "SW_RUNTIME_PATH is required")
local challenge_path = assert(SW_CHALLENGE_PATH, "SW_CHALLENGE_PATH is required")

local function assert_equal(actual, expected, message)
    if actual ~= expected then
        error((message or "values differ") .. ": expected " .. tostring(expected) .. ", got " .. tostring(actual), 2)
    end
end

-- Stable tables: never rebind `exec_calls` / `redir_args` (closures capture
-- the upvalue slot; rebinding can leave stale values visible under
-- gopher-lua across long test sequences). Clear them in place instead.
local output = {}
local exec_calls = {}
local redir_args = {}

local function clear_state()
    for i = 1, #exec_calls do exec_calls[i] = nil end
    redir_args.redir = nil
end

ngx = {
    var = {},
    header = {},
    exec = function(uri)
        exec_calls[#exec_calls + 1] = uri
        return true
    end,
    say = function(body) output.body = body end,
    req = {
        get_uri_args = function() return redir_args end,
        set_uri_args = function(args) redir_args.redir = args.redir end,
    },
}

local function load_runtime()
    local chunk = assert(loadfile(runtime_path))
    return chunk()
end

local function reset_request(user_agent, uri, cookie, args, method)
    clear_state()
    ngx.var = {
        http_user_agent = user_agent,
        uri = uri or "/",
        scheme = "https",
        host = "example.com",
        args = args,
        ["cookie___openflare_sw"] = cookie,
    }
    ngx.req.get_method = function() return method or "GET" end
end

local function test_module_contract()
    local runtime = load_runtime()
    assert_equal(type(runtime), "table", "sw.runtime must return a module table, not true/nil")
    assert_equal(type(runtime.check), "function", "sw.runtime must export check()")
end

local function test_non_browser_ua_passes_through()
    local runtime = load_runtime()
    reset_request("curl/8.0.1")
    assert_equal(runtime.check(), true, "non-browser UA passes through")
    assert_equal(#exec_calls, 0, "non-browser UA must not intercept")

    reset_request("")
    assert_equal(runtime.check(), true, "empty UA passes through")

    reset_request(nil)
    assert_equal(runtime.check(), true, "missing UA passes through")
end

local function test_browser_ua_non_get_passes_through()
    local runtime = load_runtime()
    reset_request(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
        "/",
        nil,
        nil,
        "POST"
    )
    assert_equal(runtime.check(), true, "non-GET request passes through")
    assert_equal(#exec_calls, 0, "non-GET request must not be intercepted")
end

local function test_browser_ua_with_cookie_passes_through()
    local runtime = load_runtime()
    reset_request(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
        "/",
        "1"
    )
    assert_equal(runtime.check(), true, "browser UA with cookie passes through")
    assert_equal(#exec_calls, 0, "cookie holder must not be intercepted")
end

local function test_browser_ua_root_without_cookie_intercepts()
    local runtime = load_runtime()
    local chrome = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    reset_request(chrome, "/")
    runtime.check()
    assert_equal(#exec_calls, 1, "browser without cookie on / must be intercepted once")
    assert_equal(exec_calls[1], "/__openflare_sw_challenge", "intercept targets the challenge page")
    assert_equal(redir_args.redir, "https://example.com/", "redir arg preserves scheme+host+uri")

    reset_request(chrome, "/", nil, "a=1&b=2")
    runtime.check()
    assert_equal(#exec_calls, 1, "second request also intercepted")
    assert_equal(redir_args.redir, "https://example.com/?a=1&b=2", "redir arg keeps the query string")

    reset_request("Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", "/")
    runtime.check()
    assert_equal(exec_calls[1], "/__openflare_sw_challenge", "Firefox intercepted")

    reset_request(
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
        "/"
    )
    runtime.check()
    assert_equal(exec_calls[1], "/__openflare_sw_challenge", "Safari intercepted")
end

local function test_browser_ua_non_root_passes_through()
    local runtime = load_runtime()
    reset_request(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
        "/about"
    )
    assert_equal(runtime.check(), true, "non-root uri passes through")
    assert_equal(#exec_calls, 0, "non-root uri must not be intercepted")
end

local function run_challenge(redir_value)
    output.body = nil
    ngx.header = {}
    redir_args.redir = redir_value
    local chunk = assert(loadfile(challenge_path))
    chunk()
    return output.body
end

local function test_challenge_embeds_plain_redir()
    local body = run_challenge("https://example.com/page?a=1&b=2")
    assert_equal(
        string.find(body, 'location.replace("https://example.com/page?a=1&b=2")', 1, true) ~= nil,
        true,
        "plain redir embedded verbatim"
    )
end

local function test_challenge_escapes_script_breakout()
    local payload = '"/><script>alert(1)</script>'
    local body = run_challenge(payload)
    assert_equal(string.find(body, '"><script>', 1, true), nil, "raw breakout sequence must not appear")
    assert_equal(string.find(body, '\\x3C/script>', 1, true) ~= nil, true, "less-than must be hex-escaped")
    assert_equal(string.find(body, '\\"', 1, true) ~= nil, true, "double quote must be backslash-escaped")
end

local function test_challenge_escapes_backslash_and_newline()
    local payload = 'a\\b";' .. string.char(13, 10)
    local body = run_challenge(payload)
    assert_equal(string.find(body, 'a\\\\b\\";\\r\\n', 1, true) ~= nil, true, "backslash, quote and CRLF escaped")
end

test_module_contract()
test_non_browser_ua_passes_through()
test_browser_ua_non_get_passes_through()
test_browser_ua_with_cookie_passes_through()
test_browser_ua_root_without_cookie_intercepts()
test_browser_ua_non_root_passes_through()
test_challenge_embeds_plain_redir()
test_challenge_escapes_script_breakout()
test_challenge_escapes_backslash_and_newline()

return true
