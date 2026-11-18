-- Copyright 2026 Arctel.net
-- SPDX-License-Identifier: Apache-2.0

local bit = require "bit"
local str = require "resty.string"
local sha256 = require "resty.sha256"
local _M = {}

local login_path = "/.openflare/site-auth"
local cookie_name = "__Host-openflare_site_auth"
local cookie_pattern = [[(?:^|;\s*)__Host-openflare_site_auth=([^;]*)]]
local html

local function equal_digest(left, right)
    if #left ~= #right then
        return false
    end
    local diff = 0
    for i = 1, #left do
        diff = bit.bor(diff, bit.bxor(left:byte(i), right:byte(i)))
    end
    return diff == 0
end

local function sign(key, expiry)
    return str.to_hex(ngx.hmac_sha1(key, ngx.var.host .. ":" .. expiry))
end

local function valid_session(key)
    local cookie = ngx.re.match(ngx.var.http_cookie or "", cookie_pattern, "jo")
    if not cookie then
        return false
    end
    local expiry, signature = cookie[1]:match("^(%d+)%.([0-9a-f]+)$")
    return expiry and #expiry <= 11 and tonumber(expiry) > ngx.time()
        and equal_digest(signature, sign(key, expiry))
end

function _M.check(key)
    if ngx.var.scheme ~= "https" then
        return ngx.exit(ngx.HTTP_FORBIDDEN)
    end
    if ngx.var.uri == login_path or valid_session(key) then
        return
    end
    ngx.header["Cache-Control"] = "private, no-store"
    local method = ngx.req.get_method()
    if method == "GET" or method == "HEAD" then
        return ngx.redirect(login_path .. "?next=" .. ngx.escape_uri(ngx.var.request_uri), ngx.HTTP_SEE_OTHER)
    end
    return ngx.exit(ngx.HTTP_FORBIDDEN)
end

function _M.forward_cookie()
    local cookie = ngx.var.http_cookie or ""
    local filtered = ngx.re.gsub(cookie, cookie_pattern, "", "jo")
    if not filtered then
        return ""
    end
    return (filtered:gsub("^;%s*", ""))
end

local function return_path()
    local args, err = ngx.req.get_uri_args(8)
    local candidate = args.next
    if err or type(candidate) ~= "string" or candidate:sub(1, 1) ~= "/"
        or candidate:sub(1, 2) == "//" or candidate:find("[%c%s\\]") then
        return "/"
    end
    return candidate
end

local function render_page(path, invalid)
    if not html then
        local file = io.open(path, "rb")
        if not file then
            ngx.log(ngx.ERR, "site authentication HTML is missing")
            return ngx.exit(ngx.HTTP_INTERNAL_SERVER_ERROR)
        end
        html = file:read("*a")
        file:close()
    end
    ngx.header["Content-Type"] = "text/html; charset=utf-8"
    ngx.header["Referrer-Policy"] = "no-referrer"
    ngx.header["Content-Security-Policy"] = "form-action 'self'; frame-ancestors 'none'; base-uri 'none'"
    if invalid then
        ngx.status = ngx.HTTP_FORBIDDEN
    end
    ngx.print((html:gsub("{{login_error}}", invalid and "账号或密码错误 / Invalid username or password" or "")))
end

function _M.login(key, ttl, credentials_digest, page_path)
    ngx.header["Cache-Control"] = "private, no-store"
    local method = ngx.req.get_method()
    if method == "GET" or method == "HEAD" then
        return render_page(page_path, false)
    end
    if method ~= "POST" then
        ngx.header["Allow"] = "GET, HEAD, POST"
        return ngx.exit(ngx.HTTP_NOT_ALLOWED)
    end
    local origin = ngx.var.http_origin
    if origin and origin ~= "https://" .. (ngx.var.http_host or ngx.var.host) then
        return ngx.exit(ngx.HTTP_FORBIDDEN)
    end
    local content_type = ngx.var.content_type or ""
    if not ngx.re.find(content_type, [[^application/x-www-form-urlencoded(?:\s*;|$)]], "ijo") then
        return ngx.exit(ngx.HTTP_UNSUPPORTED_MEDIA_TYPE)
    end
    ngx.req.read_body()
    local form, err = ngx.req.get_post_args(8)
    if err or not form or type(form.username) ~= "string" or type(form.password) ~= "string"
        or #form.username > 255 or #form.password > 255 or form.username:find("%z") or form.password:find("%z") then
        return render_page(page_path, true)
    end

    local hash = sha256:new()
    hash:update(key .. "\0" .. form.username .. "\0" .. form.password)
    if not equal_digest(str.to_hex(hash:final()), credentials_digest) then
        return render_page(page_path, true)
    end

    local expiry = tostring(ngx.time() + ttl)
    ngx.header["Set-Cookie"] = cookie_name .. "=" .. expiry .. "." .. sign(key, expiry)
        .. "; Max-Age=" .. ttl .. "; Path=/; Secure; HttpOnly; SameSite=Lax"
    return ngx.redirect(return_path(), ngx.HTTP_SEE_OTHER)
end

return _M
