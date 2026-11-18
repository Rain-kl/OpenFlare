# PoW 空名单导致节点返回 HTTP 500

## 原因与现场证据

2026-11-18 排查启用 PoW 后的站点 HTTP 500：节点 `pow_config.json` 中黑白名单的空切片被 Go JSON 序列化为 `null`，OpenResty 的 `cjson.safe` 解码后得到 truthy userdata。`ipairs(list.ips or {})` 无法将该值转换为空表，在 worker 初始化阶段抛出 `table expected, got userdata`；后续请求加载 `pow.config` 时继续失败，导致受保护路径、已排除路径及挑战页资源返回 500。

## 修复

- 在 PoW 名单预编译与路径策略处理中显式按 Lua 类型提取列表，将缺省或 `null` 视为空表。
- 保持 worker 配置快照和既有路径匹配行为，不改变已发布配置的格式。
- 扩展现有 OpenResty 集成测试，使用 `RenderPoWConfig` 生成含 nil 切片的真实配置，经原生 LuaJIT/cjson 验证初始化、挑战页输出和 API 白名单放行。

## 验证

修复前，更新后的原生集成测试稳定复现相同的 `ipairs` 类型错误与 HTTP 500。修复后：

- `OPENFLARE_TEST_OPENRESTY_IMAGE=openresty/openresty:alpine-slim go test ./internal/apps/agent/nginx -run '^TestOpenRestyLuaIntegration$' -count=1 -v` 通过，验证真实发布配置加载、挑战页输出和 API 白名单放行。
- `go test ./internal/apps/agent/nginx ./pkg/render/openresty -count=1` 通过。
- `golangci-lint run ./internal/apps/agent/nginx/...` 通过，`make format` 通过。
- `make code-check` 在既有 `cloudflare/logics.go` 的 cyclop 和 `zone/legacy_import.go` 的 nestif 问题处停止，未执行其后续前端检查。

## 部署

修复位于 Agent 内嵌 Lua 资源，需要更新 Agent 并重新加载配置。仅重新发布相同策略不能修复旧代码对 `null` 的处理。
