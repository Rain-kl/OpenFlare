// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOpenRestyLuaIntegration exercises the shipped assets with LuaJIT and FFI.
func TestOpenRestyLuaIntegration(t *testing.T) {
	image := os.Getenv("OPENFLARE_TEST_OPENRESTY_IMAGE")
	if image == "" {
		t.Skip("set OPENFLARE_TEST_OPENRESTY_IMAGE to run the Docker integration test")
	}
	dir := t.TempDir()
	manager := &Manager{
		LuaDir:           filepath.Join(dir, "lua"),
		NginxCertDir:     filepath.Join(dir, "certs"),
		RuntimeConfigDir: filepath.Join(dir, "runtime"),
	}
	if err := manager.EnsureLuaAssets(); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(manager.RuntimeConfigDir, "waf_config.json"):             `{"rule_groups":[],"bindings":[]}`,
		filepath.Join(manager.RuntimeConfigDir, "waf_ip_groups.json"):          `{"groups":{}}`,
		filepath.Join(manager.RuntimeConfigDir, "waf_ip_groups.json.checksum"): "v1",
		filepath.Join(dir, "nginx.conf"): fmt.Sprintf(`
worker_processes 1;
pid /tmp/openflare-lua.pid;
error_log stderr notice;
events { worker_connections 32; }
http {
    access_log off;
    client_body_temp_path /tmp/client_temp;
    lua_package_path "%s/?.lua;%s/?/init.lua;;";
    lua_shared_dict openflare_waf_config 1m;
    lua_shared_dict openflare_waf_ip_groups 1m;
    lua_shared_dict openflare_pow_config 1m;
    lua_shared_dict openflare_pow_challenges 1m;
    lua_shared_dict openflare_pow_sessions 1m;
    init_worker_by_lua_block { require("waf.runtime").init() }
    server {
        listen 127.0.0.1:18080;
        location / { content_by_lua_file %s; }
    }
}
`, manager.LuaDir, manager.LuaDir, filepath.Join(dir, "check.lua")),
		filepath.Join(manager.RuntimeConfigDir, "pow_config.json"): `{"site":{"enabled":true,"config":{"whitelist":{"ip_cidrs":["192.0.2.0/24","2001:db8::/32"]}}}}`,
		filepath.Join(manager.NginxCertDir, "pow", "page.json"):    `{"html":"<html><body>published page</body></html>","widget":"progress"}`,
		filepath.Join(dir, "check.lua"): `
local matcher = require("waf.ip_matcher").compile({"192.0.2.0/24", "2001:db8::/32"})
assert(matcher:match("192.0.2.9", string.char(192, 0, 2, 9)))
assert(not matcher:match("198.51.100.1", string.char(198, 51, 100, 1)))
assert(matcher:match("2001:0db8:1::1"))
assert(not matcher:match("2001:db9::1"))
local policy = require("pow.policy")
assert(policy.match_any("192.0.2.9", "", "/", {ip_cidrs = {"192.0.2.0/24"}}))
assert(policy.match_any("2001:db8:1::1", "", "/", {ip_cidrs = {"2001:db8::/32"}}))
assert(policy.match_any("", "", "/ADMIN/users", {paths = {"/admin/*"}}))
assert(not policy.match_any("", "", "/fileXjson", {paths = {"/file.json"}}))
local config = {whitelist = {ip_cidrs = {"2001:db8::/32"}}, blacklist = {paths = {"/admin/*"}}}
local cjson = require("cjson.safe")
local before = cjson.encode(config)
policy.prepare(config)
assert(cjson.encode(config) == before, "compiled matchers must not modify serializable policies")
local runtime = require("waf.runtime")
runtime.init({config = {
    rule_groups = {{id = 1, enabled = true, is_global = true, graph = {entry = "allow", nodes = {allow = {type = "allow"}}}}},
    bindings = {{site_name = "site", rule_group_ids = {}}},
}, ip_groups = {groups = {}}, geo_lookup = function() end})
local rules = runtime.debug_active_rules("site")
assert(#rules == 1 and rules == runtime.debug_active_rules("site"))
assert(require("pow.config").get("site").enabled)
assert(require("pow.page").page.html:find("published page", 1, true))
io.open = function() error("request path must not read files") end
for _ = 1, 20 do
    assert(require("pow.config").get("site").enabled)
    assert(require("pow.page").page.html:find("published page", 1, true))
end
ngx.say("OpenResty LuaJIT integration passed")
`,
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "--read-only",
		"--tmpfs", "/tmp", "--volume", dir+":"+dir+":ro", image,
		"sh", "-c", `
openresty -p /tmp/ -c "$1" -g 'daemon off; master_process off;' &
task_nginx_pid=$!
trap 'kill "$task_nginx_pid" 2>/dev/null; wait "$task_nginx_pid" 2>/dev/null' EXIT
for attempt in $(seq 1 30); do
    if wget -T 1 -q -O - http://127.0.0.1:18080/; then exit 0; fi
    sleep 0.1
done
exit 1
`, "sh", filepath.Join(dir, "nginx.conf"))
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "OpenResty LuaJIT integration passed") {
		t.Fatalf("OpenResty integration (%s) failed: %v\n%s", image, err, output)
	}
}
