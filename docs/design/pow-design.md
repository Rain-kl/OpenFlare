# PoW 防护设计

PoW 防护模块由站点策略、挑战页模板、Web Component 运行时以及与 WAF 共享的 IP/路径匹配原语构成。控制面逻辑位于 `internal/apps/openflare/pow`，策略以 `route_id` 为主键持久化在 `of_pow_policies`（删除站点时同事务清理），并在编译 `sites.v1` 时注入对应站点的 `pow_enabled` 与 `pow_config` 字段，由 `openflare-edge` 在内存中原子热更新生效。

---

## 1. 配置与产物构建

* **接口边界**：`GET /api/v1/d/pow/policies` 列出全部站点 PoW 策略；`PUT /api/v1/d/pow/policies/{id}` 在校验参数范围与站点存在性后保存 `enabled` 与 `config`；全局页面外观（`template`、`widget` 与自定义 `html`）由内部配置键 `pow_page` 管理。
* **策略结构（`internal/openflare/edgeconfig/pow_policy.go`）**：
  * 算法 `algorithm`：`fast` 或 `slow`；难度 `difficulty`：`1–16`；
  * 会话通行有效期 `session_ttl`：`60–604800` 秒；单次挑战有效期 `challenge_ttl`：`30–3600` 秒；
  * 路径规则 `path_rules`：`mode` 支持 `all` 或 `blacklist`（`blacklist` 模式下黑名单路径不可为空），黑白名单条目支持 `exact`、`prefix`、`wildcard`（均须以 `/` 开头）与 `regex`，每类列表上限 `100` 条；省略 `path_rules` 时按全站生效。
* **模板单一真相**：内置 HTML 挑战页模板与静态资源唯一位于 `shared/edgeassets/pow_templates/` 与 `pow_static/`；修改前端预览素材需执行 `node frontend/scripts/generate-pow-assets.mjs`，由 `make code-check` 校验一致。

---

## 2. 请求处理与验证流程

1. **接管判断**：若站点启用了独立 PoW 策略（`route.pow.is_some()`），`openflare-edge` 在 WAF 阶段自动跳过旧版 WAF 图中的 PoW 节点，改由独立 PoW 模块执行；若未启用独立策略，则保留旧 WAF PoW 节点的兼容执行路径。
2. **规则与通行凭证检查**：独立 PoW 运行时依次检查白名单（IP/CIDR/路径/UA）、黑名单模式以及有效的已签发通行 Cookie；精确/前缀/通配符路径在 `path_rules` 中保持大小写敏感，正则表达式与 IP 匹配器在配置加载时预编译至内存。
3. **挑战下发与浏览器求解**：未持有有效通行凭证时，边缘网关直接响应内置或自定义挑战 HTML（在 JSON 启动数据中转义 `<` 防止脚本注入，前端通过 `textContent` 渲染状态文案并挂载 Web Component / `openflare:pow` / `window.OpenFlarePoW` 接口）。
4. **SHA-256 核验与防重放**：浏览器 Worker 完成求解后向内置端点（`POW_API_PREFIX`）提交证明，边缘节点核验难度、当前 Host 与站点绑定，消费单次挑战并签发短期通行 Cookie，随后仅重定向回服务端挑战上下文内保存的原本地路径，拒绝客户端篡改跳转目标。

若独立 PoW 策略或模板文件损坏导致解析失败，节点拒绝应用坏快照并维持最后一份有效配置继续防护，确保防护状态不因配置错误意外降级为全放行。
