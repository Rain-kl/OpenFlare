# 独立 PoW 模块设计

PoW 由独立的站点策略、页面模板、浏览器运行时和组件组成。业务接口位于 `internal/apps/openflare/pow`，策略持久化在 `of_pow_policies`，通过 `internal/repository` 读写。配置入口位于规则详情的独立「PoW」标签页，直接使用当前规则的策略。每个站点一条策略，以 `route_id` 为主键；删除站点时在同一事务中清理策略。

## 配置与发布

- `GET /api/v1/d/pow/policies`：列出已配置的站点策略。
- `PUT /api/v1/d/pow/policies/{id}`：保存站点的 `enabled` 与 `config`，管理员权限与站点存在性由服务端验证。
- `pow_page`：内部业务配置，保存全局 `template`、`widget` 和自定义 `html`。

策略包含 `algorithm`、`difficulty`、`session_ttl`、`challenge_ttl` 与可选的 `path_rules`。`path_rules.mode` 为 `all` 或 `blacklist`，`whitelist` / `blacklist` 为 `{ "type": "exact|prefix|wildcard|regex", "value": "..." }` 数组。省略 `path_rules` 时按全站处理。

发布快照将独立策略附加到对应站点的 `pow_enabled` / `pow_config`；渲染器生成按站点标识索引的 `pow_config.json`。挑战页单独写入 `pow/page.json`，纳入配置校验和、备份与回滚。独立 PoW 的端点和静态资源可在没有 WAF PoW 节点的站点上渲染。

内置 HTML 的唯一来源为 `pkg/render/openresty/pow_templates/`。运行 `node frontend/scripts/generate-pow-assets.mjs` 可同步前端预览素材，`make code-check` 会检查素材是否同步。生成文件随前端源码交付，Docker 前端构建可直接使用；预览复用实际 Widget 源码。

canary 分支推送会同时触发 Server 与 Agent 镜像构建。Agent 的 canary 构建仅更新 `canary` 标签，不更新稳定版 `latest`；部署新的 PoW 策略时应同步更新 Server 与 Agent，再发布配置版本。

## 请求流程

1. `pow.runtime.prepare()` 从独立配置快照解析当前站点，供旧 WAF PoW 节点识别接管状态。
2. 执行 WAF；已有 PoW 节点在站点配置了独立策略时直接继续。
3. `pow.runtime.check()` 执行独立 PoW 策略，先匹配白名单，再匹配黑名单模式，最后检查有效通行 Cookie。
4. 需要挑战时，短期保存当前策略，通过显式参数内部跳转至挑战处理器，保留浏览器的原始地址。
5. 挑战处理器加载 HTML，转义文字占位符，插入 Widget 与 JSON 启动数据，再加载浏览器运行时。
6. Worker 求解 SHA-256；浏览器提交证明，服务端检查难度、域名与站点绑定，消费挑战并签发通行 Cookie。前端收到成功响应后显示成功状态并刷新原始地址。

Worker 的独立策略快照最多每秒检查一次文件；删除配置文件时清空旧快照，读取或解析失败时保留上一份可读快照并记录错误。页面按挑战请求读取发布的文件。

验证运行时以 Web Component 和 `openflare:pow` / `window.OpenFlarePoW` 暴露显示状态，页面布局无需依赖特定 DOM ID。求解完成后先进入 `verifying`，收到服务端成功响应才进入 `success`；失败时组件显示重试入口。

## 兼容与边界

保留旧 WAF 图内的 PoW 执行路径，避免把旧条件分支自动转换成全站策略。只有站点保存了独立策略，旧 PoW 节点才跳过。旧图配置和独立站点策略分别保存，回滚到旧发布版本时恢复该版本的配置文件。

挑战 JSON 中的 `<` 会转义，避免脚本元素提前结束；文字状态使用 `textContent`。挑战验证绑定原始域名与站点，成功后挑战立即失效。服务端跳转只使用挑战中保存的本地路径，拒绝客户端覆盖跳转目标。
