# 站点级访问频率限制设计

日期：2026-07-20  
状态：已评审待实现  
方案：站点详情 Limits 暴露 `limit_req_per_ip`；渲染时按 effective rate 生成多 `limit_req_zone`，并用站点键隔离 IP 计数

## 背景

全局默认已有：

- `openresty_default_limit_conn_per_server`
- `openresty_default_limit_conn_per_ip`
- `openresty_default_limit_rate`
- `openresty_default_limit_req_per_ip`

站点级并发/带宽已在「反代站点详情 → 流量限制」中配置，语义为：空/`0` 继承、`-1` 关闭、自定义覆盖。

请求频率（`limit_req`）后端字段与 merge 已存在，但：

1. 前端站点详情未暴露 `limit_req_per_ip`
2. 渲染侧仅在全局默认非空时输出**单一** `limit_req_zone ... rate=全局值`，站点自定义 rate 无法真正独立生效（nginx 的 rate 写在 zone 上，不能仅靠 location 覆盖）

## 目标

1. 在**仅站点详情「流量限制」区块**配置单 IP 请求频率。
2. 语义与现有三项一致：空/`0` 继承全局；`-1` 关闭；合法 `Nr/s` / `Nr/m` 为站点自定义。
3. 站点自定义 rate **真正按该 rate 生效**（A 站 5r/s、B 站 10r/s 互不影响）。
4. 同 IP 在不同站点的频率配额**按站点隔离**。
5. 修改后仍需发布配置版本；Agent 使用与 Server 同源的 render 路径。

## 非目标

- 在「安全性 → 限流」页增加按站点列表编辑
- 新建站点表单中的频率字段
- 按路径 / URI 差异化频率限制
- 改变 `limit_conn_*` / `limit_rate` 的现有 zone 与合并模型
- 业务 Zone（顶级域 + 二级域名资源）模型变更

## 语义

### 站点字段 `limit_req_per_ip`（字符串）

| 值 | 含义 |
|----|------|
| 空 / `"0"` | 继承全局 `openresty_default_limit_req_per_ip` |
| `"-1"` | 本站显式关闭频率限制 |
| `^\d+r/[sm]$`（大小写不敏感，存小写） | 本站自定义 rate |

### 全局默认

| 值 | 含义 |
|----|------|
| 空 / `"0"` | 默认关闭；继承方亦不输出 `limit_req` |
| 合法 rate | 未配置站点的 effective rate |

### 合并（与现有 `mergeLimitRate` 一致）

```
if route == -1:          effective = off
else if route is set:    effective = route   // 合法 rate
else:                    effective = global  // route 空/0
// global 空/0 → off
```

## 渲染

### 问题

nginx `limit_req_zone` 的 `rate=` 在 zone 声明时固定；多个站点若 effective rate 不同，必须使用不同 zone。

### 步骤

1. 在 `RenderRouteConfig` / main 配置生成前，对全部 route 计算 effective `LimitReqPerIP`。
2. 收集非空 effective rate 的**去重集合**，在 `http {}`（`renderOpenRestyLimitZoneBlock` 扩展，需能访问 routes 或 precomputed rates）输出：

```nginx
# 变量键：站点名 + IP，保证跨站点计数隔离
# 实现可用 map 或在 server 内 set 后引用；zone key 采用组合键
limit_req_zone $openflare_req_key zone=openflare_req_<rate_token>:10m rate=<rate>;
```

`rate_token` 由 rate 规范化生成（如 `10r/s` → `10rs`，`100r/m` → `100rm`），仅作 zone 名片段，合法 nginx zone 名。

3. 每个业务 server 在 access 相关位置之前设置：

```nginx
set $openflare_req_key "$openflare_waf_site$binary_remote_addr";
```

（与现有 `set $openflare_waf_site "..."` 同源 site_name；若某 server 无 waf site 变量则用同一 displayName/site_name。）

4. `renderRouteLimitBlock` 在 effective rate 非空时输出：

```nginx
limit_req zone=openflare_req_<rate_token> burst=<calculateBurst> nodelay;
limit_req_status 429;
```

5. **无任何** effective rate 时：不输出任何 `limit_req_zone` / `limit_req`（避免引用不存在的 zone）。

6. 应用范围与现有 limit 块一致：HTTP/HTTPS 反代 `location /`、Pages 相关 location；不含 HTTP→HTTPS 重定向-only server。

### 与旧行为差异

| 项 | 旧 | 新 |
|----|----|----|
| zone 数量 | 全局最多 1 个 | 按不同 effective rate 多个 |
| zone key | `$binary_remote_addr` | `$openflare_req_key`（站点+IP） |
| 站点自定义 rate | 无法真正独立 | 引用对应 rate 的 zone |

快照 JSON **仍保留站点原始值**（含空/`-1`），不把 merge 结果写回 route。

## 数据与 API

- 列 `of_proxy_routes.limit_req_per_ip` 已存在；无新迁移（若环境已跑过既有迁移）。
- API `Input` / `View` 已有字段；normalize / 校验已存在。
- 前端类型与详情表单补齐即可。

## 前端

仅改站点详情 `limits-section.tsx`：

- 增加「单 IP 请求频率」输入
- 校验：空、`0`、`-1`、或 `^\d+r/[sm]$i`
- 规范化：trim + lower；`0` → `""`
- `ProxyRouteItem` / `ProxyRouteMutationPayload` 增加 `limit_req_per_ip`
- `buildPayloadFromRoute` 带上该字段，避免其它区块保存时丢失

文案：与并发/带宽一致（空或 0 继承；-1 关闭；例如 10r/s、100r/m 自定义）。

## Agent / 发布

- 配置保存后须**发布配置版本**
- Agent **本地** `RenderJSON`；必须部署含本设计 render 的 Agent，否则 source 有字段但 conf 无指令
- 若 Agent 已记录同 version/checksum，升级二进制后需触发重新 apply（重启或强制重同步）

## 测试

- `mergeRouteLimitConfig`：继承 / 覆盖 / `-1`（已有则补 rate 断言）
- 多站点不同 effective rate：main conf 含多个 `limit_req_zone`，各 location 引用正确 zone 名
- 全关闭：无 `limit_req` 相关指令
- 仅全局有值：一个 zone + 未自定义站点引用该 zone
- 前端类型与表单校验（手工或既有模式）

## 验收

1. 全局 `10r/s`，站点空 → 该站 location 有 limit_req，zone rate=10r/s  
2. 站点改 `5r/s` 并发布 → 该站引用 5r/s zone  
3. 站点 `-1` → 该站无 limit_req  
4. 两站不同 rate，同 IP 压测互不抢同一配额  

## 实现边界

| 层 | 工作量 |
|----|--------|
| 渲染 `pkg/render/openresty` | 多 zone + 站点键 + location 引用 |
| 前端详情 Limits + types + payload | 补字段 |
| 后端 API/DB | 已具备，仅回归 |
| 文档/changelog | 用户可见变更记中文 changelog |
