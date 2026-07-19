# 边缘限流全局默认设计

日期：2026-07-19  
状态：已评审待实现  
方案：渲染时按站点合并全局默认（方案 A）

## 背景

当前边缘限流仅挂在站点（Proxy Route）上，字段为：

- `limit_conn_per_server`：站点并发连接上限
- `limit_conn_per_ip`：单 IP 并发连接上限
- `limit_rate`：单请求带宽

OpenResty 渲染行为：

- `http {}` 始终声明共享 `limit_conn_zone`
- 各站点 `location` 在字段 `>0` / 非空时输出 `limit_conn` / `limit_rate`
- 站点值为 `0` 或空表示**关闭**，无全局默认

期望：在全局增加默认限流策略；站点未设置时继承默认，可覆盖或显式关闭。

## 目标

1. 提供三项全局默认限流配置，覆盖全部现有维度。
2. 站点 `0`/空 = 继承全局；`-1` = 显式关闭；`>0`/合法带宽串 = 站点自定义。
3. 合并发生在配置渲染路径，仍在各站点 `location` 输出生效指令（不在 `http {}` 写默认 `limit_conn`/`limit_rate`）。
4. 管理入口：侧栏「安全性」下新增子页「限流」。
5. 全局默认初始为 `0`/空，存量发布行为与现网一致。

## 非目标

- 引入 `limit_req`（按 RPS 限流）
- 在 `http {}` 上下文直接写默认 `limit_conn` / `limit_rate`
- 按路径 / URI 差异化限流
- 改变 `limit_conn` zone 键模型（仍为 `$server_name` 与 `$binary_remote_addr`）

## 语义

### 站点字段

| 值 | `limit_conn_*` | `limit_rate` |
|----|----------------|--------------|
| `0` / 空 | 继承全局默认 | 空或 `"0"` 规范化为空串后继承 |
| `-1` | 显式关闭该维度 | 字面 `"-1"` 表示显式关闭 |
| `>0` / 合法带宽 | 使用站点值 | 合法 `^\d+[kKmM]?$` 使用站点值 |

### 全局默认

| 值 | 含义 |
|----|------|
| `0` / 空 | 默认关闭；继承方亦不输出指令 |
| `>0` / 合法带宽串 | 作为未配置站点的生效值 |

全局默认**不允许** `-1`（无意义）；仅 `>=0` 或合法 rate / 空。

### 合并规则（逐字段）

```
if route == -1:          effective = off
else if route is set:    effective = route   // conn > 0 或 rate 合法非空
else:                    effective = global  // route 为 0/空
// global 为 0/空 → off（不输出）
```

`limit_rate` 的「set」判定：规范化后非空且不等于 `"-1"`。

## 配置存储

沿用 `system_configs` + Option API + 发布快照，与其它 OpenResty 选项一致。

| Key | 类型语义 | 默认 |
|-----|----------|------|
| `openresty_default_limit_conn_per_server` | 非负整数 | `0` |
| `openresty_default_limit_conn_per_ip` | 非负整数 | `0` |
| `openresty_default_limit_rate` | 空或 `^\d+[kKmM]?$` | `""` |

实现要点：

- `internal/model/system_configs.go` 增加 `ConfigKeyOpenRestyDefaultLimit*` 常量
- goose seed/升级迁移写入默认值
- `internal/apps/openflare/option` 注册校验器（conn ≥ 0；rate 与站点同一套 pattern，允许空）
- `openRestyConfigSnapshot` / `buildOpenRestyConfigSnapshot` 增加三字段
- 变更进入 OpenResty option diff；**需重新发布配置版本后下发节点**

## 站点模型与 API

- DB 列类型不变（`INTEGER` / `VARCHAR(32)`），无 schema 变更
- `normalizeProxyRouteLimitConnValue`：允许 `>= -1`（原 `>= 0`）
- `normalizeProxyRouteLimitRate`：允许 `"-1"` 存为关闭标记；空/`0` → `""`（继承）
- View / Input / 前端类型同步暴露 `-1` 语义
- 错误文案更新（非法负数除 `-1` 外拒绝）

## 渲染路径

合并**唯一**发生在 `pkg/render/openresty.RenderRouteConfig`：该函数已接收完整 `Document`，可从 `doc.OpenRestyConfig` 读取全局默认，与各 `doc.Routes[i]` 的站点字段合并。Server 预览渲染与 Agent 落地渲染共用同一路径，禁止在 snapshot 构建或其它层再合一次。

步骤：

1. 对每个 route：用站点限流字段 + `doc.OpenRestyConfig` 中的默认三项 → `routeLimitConfig`
2. `renderRouteLimitBlock` 保持「有值才输出」
3. 应用范围不变：
   - HTTP/HTTPS 反代 `location /`
   - Pages 相关 location
   - **不含** HTTP→HTTPS 重定向-only server
4. `http {}` 仍只输出现有 `limit_conn_zone` 两行

快照 JSON **保留站点原始值**（含 `0`/`-1`），不把合并结果写回 route；节点 conf 中只看到最终指令。

伪代码：

```go
func mergeRouteLimit(route routeLimits, def defaultLimits) routeLimitConfig {
    return routeLimitConfig{
        LimitConnPerServer: mergeConn(route.LimitConnPerServer, def.LimitConnPerServer),
        LimitConnPerIP:     mergeConn(route.LimitConnPerIP, def.LimitConnPerIP),
        LimitRate:          mergeRate(route.LimitRate, def.LimitRate),
    }
}

func mergeConn(route, def int) int {
    if route == -1 {
        return 0 // off
    }
    if route > 0 {
        return route
    }
    if def > 0 {
        return def
    }
    return 0
}

func mergeRate(route, def string) string {
    r := strings.TrimSpace(strings.ToLower(route))
    if r == "-1" {
        return ""
    }
    if r != "" && r != "0" {
        return r
    }
    d := strings.TrimSpace(strings.ToLower(def))
    if d != "" && d != "0" {
        return d
    }
    return ""
}
```
## 前端

### 安全性 → 限流

- 导航：`openflareSecurityNavGroup` 增加 `{ title: '限流', url: '/rate-limits' }`
- 页面：`frontend/app/(main)/rate-limits/page.tsx`
- 通过 `OptionService.list` / `updateBatch` 读写上述 3 个 key
- UI 模式对齐性能页：标题规范、卡片分区、保存反馈
- 文案说明：`0`/空 = 默认关闭；`>0` = 未单独配置站点的默认生效值；修改后需发布配置版本

### 站点流量限制

- 更新 `limits-section.tsx` 与校验 helpers：
  - `0`/空 = 继承全局默认
  - `-1` = 关闭
  - `>0` / 合法 rate = 自定义
- 可选：展示当前全局默认值作提示（只读）
- 创建站点默认仍为 `0`/空（即继承）

## 兼容性

| 场景 | 结果 |
|------|------|
| 升级后全局默认 0，站点全 0 | 与升级前一致：不限流 |
| 管理员设置全局默认后发布 | 所有 `0`/空站点自动生效默认 |
| 站点需保持关闭 | 将该项改为 `-1` 后保存并发布 |
| 旧 API 客户端只写 `0` | 合法；语义变为继承 |
| 旧快照无默认字段 | 按 0/空处理 |

## 边界说明

- `limit_conn_per_ip` zone 仍按 `$binary_remote_addr` 全局共享；各 location 的 N 可不同，计数空间共享（现网行为，本设计不改）。
- 多域名共享一条路由 → 共享合并后策略（产品边界不变）。
- 仅改全局默认不自动 reload 节点；走标准「选项变更 → 配置版本 diff → 发布」。

## 测试计划

1. **render 表驱动**：继承 / 显式关 / 覆盖 / 全局关 × 三字段
2. **normalize**：`-1`、`0`、`>0`、非法负值、rate `"-1"` / 空 / 合法 / 非法
3. **snapshot**：默认字段进入 `openresty_config`；option diff 可检测变更
4. **option 校验**：非法全局 rate / 负 conn 拒绝
5. 前端：限流页读写与站点文案（可选手测）

## 文档与变更记录

- 本设计文档：`docs/superpowers/specs/2026-07-19-http-default-rate-limit-design.md`
- 实现时更新中文 changelog `[Unreleased]`（用户可见语义与新设置页）
- 如有配置参考页，补充三个 key 的中文说明
- 不要求同步英文文档

## 实现落点（文件索引）

| 区域 | 路径 |
|------|------|
| 配置键 / seed | `internal/model/system_configs.go`，goose 迁移 |
| 校验 | `internal/apps/openflare/option/openresty_validators.go` |
| 快照 | `internal/apps/openflare/config_version/snapshot.go` |
| 站点规范化 | `internal/apps/openflare/proxy_route/helpers.go` |
| 渲染合并 | `pkg/render/openresty/render.go`（及调用处传参） |
| 导航 | `frontend/lib/navigation/openflare-nav.ts` |
| 限流设置页 | `frontend/app/(main)/rate-limits/` |
| 站点 UI | `frontend/app/(main)/proxy-routes/detail/components/limits-section.tsx` |

## 验收标准

1. 全局默认可在「安全性 → 限流」读写，初始 0/空。
2. 全局设为有效值并发布后，站点限流为 0/空的 location 出现对应指令。
3. 站点 `-1` 在全局有默认时仍不输出该维度。
4. 站点 `>0` 覆盖全局。
5. 全局与站点均为 0/空时 conf 无 `limit_conn`/`limit_rate` 指令。
6. `make code-check` 通过；相关单测覆盖合并与规范化。
