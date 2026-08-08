# Service Worker 离线兜底设计（issue #23）

- 日期：2026-08-08
- 状态：设计已确认
- 范围：Proxy Route（反代）+ Pages 静态托管 全覆盖

## 1. 背景与目标

当 CDN 域名被墙、浏览器对所有网络请求失败时，用户会直接流失。本功能通过给网站下发 Service Worker，缓存一个"联系站长"离线页；域名被墙后，SW 从缓存吐出该页，保留用户并引导联系站长。

核心约束：

- 平台一键批量下发，避免逐个 Agent 配置。
- 不改源页代码，全部在 OpenResty 边缘层完成。
- 覆盖反代（Proxy Route）与 Pages 静态托管两种网站类型。

## 2. 机制总览

采用「首次挑战页 + Cookie 放行 + UA 白名单」模式，替代 `sub_filter` 响应体重写。

| 环节 | 行为 |
|---|---|
| 真实浏览器 UA（含特征版本，如 `Chrome/120`）首次访问首页 | 返回 SW 挑战页（内嵌 `register('/sw.js')` 与离线页预缓存），设置长过期 Cookie |
| 带 Cookie 的请求 | 直接放行到上游，正常返回真实页面 |
| 未知 UA（爬虫、curl，无真实浏览器特征） | 直接放过，交给 WAF 处理，拿到真实内容 |

### 为什么不用 sub_filter

`sub_filter` 需处理上游 gzip / Content-Type / 大响应扫描 / 流式缓冲等多处坑。本方案不改上游 body，整体替换首次响应，以上问题全部规避；且爬虫（不匹配真实浏览器 UA）天然绕过挑战页，不伤 SEO。

## 3. 分层职责

```
apps/proxy_route ─┐
apps/pages        ─┼─ model → repository → 渲染(pkg/render/openresty) → Agent(OpenResty)
前端设置卡        ─┘                                              ↑ SW 挑战页 + sw.js/offline 落盘
```

### 后端数据（全局 Option，与 origin error page 同模式）

`sw_offline` 相关配置作为**全局 SystemConfig / OpenRestyConfig snapshot 字段**，对所有启用 HTTPS 的路由生效，实现"一键批量下发"。新增字段：

- `sw_offline_enabled`：是否启用 SW 离线兜底
- `sw_offline_html`：联系站长离线页 HTML 内容（默认提供内置模板）

### 渲染层（`pkg/render/openresty`）

新增 `renderServiceWorkerChallenger(cfg ConfigSnapshot)` 工具，为真实提供内容的 HTTPS server 块（`sw_offline_enabled` 且 `EnableHTTPS` 时）输出：

```nginx
# SW 脚本 + 离线页（作为 support file 落盘）
location = /sw.js        { alias .../sw.js;        add_header Service-Worker-Allowed /; }
location = /offline.html { alias .../offline.html; }

# 仅首页拦截：真实浏览器 UA 且无 cookie → 返回 SW 挑战页
# 否则（带 cookie / 未知 UA）→ 放行到上游
location = / {
    if (真实浏览器UA && 无cookie) { content_by_lua 返回 SW 挑战页; }
    放行到上游;
}
```

- SW 逻辑：`install` 阶段缓存 `/offline.html`；`fetch` 事件在网络失败时返回 `caches.match('/offline.html')`。
- 仅在 `EnableHTTPS` 时注入（SW 要求 HTTPS 安全上下文）。
- 多域名 server 块：`/sw.js`、`/offline.html`、挑战页在各 `server_name` 下同源可达。
- 仅对首页 `location = /` 触发；js/css/图片/API/子页面请求不拦，零额外开销。

## 4. 数据流

```
用户首次访问首页(真实UA, 无cookie)
  → OpenResty 判断：真实UA && 无cookie
      → 返回 SW 挑战页 (内嵌 register + 预缓存 offline.html)
      → 浏览器执行 → 注册 SW → 设置长过期 cookie
  → 用户再次请求(带cookie)
      → 放行到上游，正常返回真实页面
域名被墙后
  → 所有请求失败 → SW fetch 兜底 → 从缓存返回 /offline.html(联系页)
```

## 5. 边界与风险

| 项 | 处理 |
|---|---|
| 首次即被墙的用户 | SW 未注册，兜底无效（所有 SW 方案共性，接受） |
| HTTP-only 站点 | 跳过注入（SW 需 HTTPS） |
| 反代多域名 | 各域名同源提供 sw.js / offline.html / 挑战页 |
| Cookie 过期 | 设长过期（约 1 年），过期后重新走一次挑战页 |
| 未知 UA | 放过并交给 WAF 处理，不重复拦截 |
| 资源/API 请求 | 不拦，仅首页触发 |

## 6. 测试

- 渲染层单元测试：
  - `sw_offline_enabled` 时输出 sw.js / offline.html / 挑战页 location
  - 非 HTTPS 或未启用时不输出
  - 仅首页触发，子路径/资源不触发
- UA 判定：真实浏览器 / 爬虫 / curl 三种 UA 的放行分支。
- Cookie 有无的放行分支。
- 现有 config snapshot checksum / rebind 测试不回归。

## 7. 前端命名与入口

离线联系页设置与现有 origin error page 设置合并为同一个功能模块，命名为**「响应页面」**（路由 `responses`），内含两个 tab：

- **错误页设置**：源站错误兜底页（现有 origin error page）
- **联系页设置**：SW 离线兜底联系页（本功能）

两者同属「边缘层兜底展示页」语义，统一管理与入口。

## 8. 待实现确认项（写 plan 时细化）

- SW 挑战页与 sw.js 的具体 Lua 实现与落盘路径（对齐现有 support file 机制）。
- `sw_offline_html` 默认内置模板样式（参考 origin error page 内置模板）。
- 「响应页面」前端模块下错误页/联系页两个 tab 的具体位置与交互。
- UA 白名单默认真实浏览器特征集合（Chrome / Firefox / Safari / Edge + 版本号正则）。
- SW 落盘路径：sw.js / offline.html 通过 SupportFile 下发，Agent 替换占位符（类似 ErrorPageTmplPlaceholder 机制）。
