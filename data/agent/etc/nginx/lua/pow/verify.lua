local cjson = require "cjson.safe"

local pow_challenges = ngx.shared.openflare_pow_challenges
local pow_sessions = ngx.shared.openflare_pow_sessions

local args = ngx.req.get_uri_args()
local challenge_id = args["id"] or ""
local response = args["response"] or ""
local nonce_str = args["nonce"] or ""
local redir = args["redir"] or ""
local elapsed = args["elapsedTime"] or ""

if challenge_id == "" or response == "" or nonce_str == "" then
    ngx.status = 400
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "missing parameters"}))
    return
end

local nonce = tonumber(nonce_str)
if not nonce then
    ngx.status = 400
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "invalid nonce"}))
    return
end

-- Get stored challenge
local challenge_raw = pow_challenges:get(challenge_id)
if not challenge_raw then
    ngx.status = 410
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "challenge expired or not found"}))
    return
end

local ok, challenge_info = pcall(cjson.decode, challenge_raw)
if not ok or not challenge_info then
    ngx.status = 500
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "invalid challenge data"}))
    return
end

local challenge_data = challenge_info.data or ""
local difficulty = challenge_info.difficulty or 4
local host = challenge_info.host or ngx.var.host or ""
local session_ttl = challenge_info.session_ttl or 600

-- Compute SHA-256(challenge_data + nonce)
local calc_string = challenge_data .. tostring(math.floor(nonce))
local calculated = ngx.sha1_bin ~= nil and "" or ""

-- Use resty.sha256 for proper SHA-256
local sha256 = require "resty.sha256"
local str = require "resty.string"
local hasher = sha256:new()
hasher:update(calc_string)
local hash_bytes = hasher:final()
local hash_hex = str.to_hex(hash_bytes)

-- Verify hash matches response
if hash_hex ~= string.lower(response) then
    ngx.status = 403
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "hash mismatch"}))
    return
end

-- Verify difficulty (leading zeros in hex)
local prefix = string.rep("0", difficulty)
if string.sub(hash_hex, 1, difficulty) ~= prefix then
    ngx.status = 403
    ngx.header.content_type = "application/json"
    ngx.say(cjson.encode({error = "insufficient difficulty"}))
    return
end

-- Invalidate challenge (prevent replay)
pow_challenges:delete(challenge_id)

-- Generate session token
local session_token = str.to_hex(ngx.sha1_bin(challenge_id .. ngx.now() .. tostring(ngx.worker.pid())))

-- Store session
pow_sessions:set(host .. ":" .. session_token, "1", session_ttl)

-- Set cookie. Secure cookies are not sent over HTTP, so only add Secure when
-- the current request itself is HTTPS.
local cookie = "__openflare_pow=" .. session_token .. "; Path=/; HttpOnly; SameSite=Lax; Max-Age=" .. tostring(session_ttl)
if ngx.var.scheme == "https" then
    cookie = cookie .. "; Secure"
end
ngx.header["Set-Cookie"] = cookie

if redir ~= "" then
    return ngx.redirect(redir)
end

ngx.header.content_type = "application/json"
ngx.say(cjson.encode({ok = true}))
