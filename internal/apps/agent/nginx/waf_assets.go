package nginx

import (
	_ "embed"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

//go:embed waf_runtime.lua
var openRestyWAFRuntimeLua string

//go:embed waf_ip_groups.lua
var openRestyWAFIPGroupsLua string

const openRestyWAFCheckLua = `local source = debug.getinfo(1, "S").source or ""
if string.sub(source, 1, 1) == "@" then
    local script_path = string.sub(source, 2)
    local base_dir = string.match(script_path, "^(.*)/waf/[^/]+%.lua$")
    if base_dir and base_dir ~= "" and not string.find(package.path, base_dir, 1, true) then
        package.path = base_dir .. "/?.lua;" .. base_dir .. "/?/init.lua;" .. package.path
    end
end

return require("waf.runtime").check()
`

// ManagedWAFLuaFiles returns the embedded Lua source files that must be deployed to the WAF runtime directory.
func ManagedWAFLuaFiles() []protocol.SupportFile {
	return []protocol.SupportFile{
		{Path: "waf/runtime.lua", Content: openRestyWAFRuntimeLua},
		{Path: "waf/ip_groups.lua", Content: openRestyWAFIPGroupsLua},
		{Path: "waf/check.lua", Content: openRestyWAFCheckLua},
	}
}
