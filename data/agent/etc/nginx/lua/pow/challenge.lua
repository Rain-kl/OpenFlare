local cjson = require "cjson.safe"

local pow_config_dict = ngx.shared.openflare_pow_config
local pow_challenges = ngx.shared.openflare_pow_challenges

local function generate_entropy()
    local pieces = {
        tostring(ngx.now()),
        tostring(ngx.worker.pid()),
        tostring(math.random()),
        ngx.var.remote_addr or "",
        ngx.var.http_user_agent or "",
        ngx.var.request_id or "",
    }
    return table.concat(pieces, ":")
end

local args = ngx.req.get_uri_args()
local host = args["host"] or ngx.var.host or ""
local redir = args["redir"] or ""

local site = ngx.var.openflare_waf_site or ""
if site == "" then
    site = host
end

local config_raw = pow_config_dict:get(site)
if not config_raw then
    ngx.status = 403
    ngx.say("PoW not configured for this site")
    return
end

local ok, route_config = pcall(cjson.decode, config_raw)
if not ok or not route_config or not route_config.enabled then
    ngx.status = 403
    ngx.say("PoW not enabled for this site")
    return
end

local config = route_config.config or {}
local difficulty = config.difficulty or 4
local algorithm = config.algorithm or "fast"
local challenge_ttl = config.challenge_ttl or 300
local session_ttl = config.session_ttl or 600

-- Generate challenge data without depending on ngx.random_bytes, which is not
-- available in every OpenResty runtime build.
local entropy = generate_entropy()
local challenge_id = ngx.md5(entropy .. ":id")
local challenge_data = ngx.md5(entropy .. ":data-a") .. ngx.md5(entropy .. ":data-b")

-- Store challenge
local challenge_info = cjson.encode({
    data = challenge_data,
    difficulty = difficulty,
    host = host,
    redir = redir,
    session_ttl = session_ttl
})
pow_challenges:set(challenge_id, challenge_info, challenge_ttl)

local static_prefix = "/.within.website/x/cmd/anubis/static/"
local accept_lang = ngx.var.http_accept_language or ""
local lang = "en"
if string.find(accept_lang, "zh") then
    lang = "zh-CN"
end

local t_title = "Making sure you're not a bot!"
local t_status = "Loading..."
local t_protected = "This site is protected by a Proof-of-Work challenge. Your browser will solve a small puzzle before the upstream response is shown."
local t_why = "Why am I seeing this?"
local t_why_desc = "OpenFlare is asking your browser to complete a lightweight computation to distinguish normal browser traffic from automated abuse. This should finish automatically."
local t_noscript = "JavaScript is required to pass this verification. Please enable JavaScript and reload."

if lang == "zh-CN" then
    t_title = "正在确认你是不是机器人！"
    t_status = "加载中..."
    t_protected = "本网站受工作量证明（Proof-of-Work）挑战保护。在显示源站响应之前，您的浏览器将解决一个微型谜题。"
    t_why = "为什么我会看到这个？"
    t_why_desc = "OpenFlare 正在要求您的浏览器完成一项轻量级计算，以区分正常的浏览器流量和自动化的恶意请求。这应该会自动完成。"
    t_noscript = "很遗憾，您必须启用 JavaScript 才能通过这项验证。请开启 JavaScript 并刷新页面。"
end

ngx.header.content_type = "text/html; charset=utf-8"
ngx.say([[<!DOCTYPE html>
<html lang="]] .. lang .. [[">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>]] .. t_title .. [[</title>
<link rel="stylesheet" href="]] .. static_prefix .. [[css/xess.css">
<style>
body,html{height:100%;display:flex;justify-content:center;align-items:center;margin-left:auto;margin-right:auto}
.centered-div{text-align:center}
#status{font-variant-numeric:tabular-nums}
#progress{display:none;width:min(20rem,90%);height:2rem;border-radius:1rem;overflow:hidden;margin:1rem 0 2rem;outline-offset:2px;outline:#b16286 solid 4px}
.bar-inner{background-color:#b16286;height:100%;width:0;transition:width .25s ease-in}
</style>
<script id="anubis_version" type="application/json">"openflare-pow"</script>
<script id="anubis_challenge" type="application/json">]] .. cjson.encode({
    challenge = {
        id = challenge_id,
        randomData = challenge_data,
        method = algorithm
    },
    rules = {
        difficulty = difficulty,
        algorithm = algorithm
    }
}) .. [[</script>
<script id="anubis_base_prefix" type="application/json">""</script>
<script id="anubis_public_url" type="application/json">"__openflare_internal__"</script>
</head>
<body id="top">
<main>
<h1 id="title" class="centered-div">]] .. t_title .. [[</h1>
<div class="centered-div">
<img id="image" style="width:100%;max-width:256px;" src="]] .. static_prefix .. [[img/pensive.webp?cacheBuster=openflare-pow">
<p id="status">]] .. t_status .. [[</p>
<p>]] .. t_protected .. [[</p>
<div id="progress" role="progressbar" aria-labelledby="status"><div class="bar-inner"></div></div>
<details>
<summary>]] .. t_why .. [[</summary>
<p>]] .. t_why_desc .. [[</p>
</details>
<noscript><p>]] .. t_noscript .. [[</p></noscript>
</div>
</main>
<script type="module" src="]] .. static_prefix .. [[js/main.mjs"></script>
</body>
</html>]])
