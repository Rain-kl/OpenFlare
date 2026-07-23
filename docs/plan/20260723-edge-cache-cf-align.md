# 边缘缓存对齐 Cloudflare 默认模型 — 实现计划

对应设计：[edge-cache-design.md](../design/edge-cache-design.md)

## 1. 目标与背景

* **需求背景**：现网对会话 Cookie / Authorization / 请求 Cache-Control 一律旁路，登录用户静态资源几乎全是「未缓存」，命中率远低于 Cloudflare 默认。需对齐 CF 两段闭环：请求 eligible × 响应可共享缓存。
* **Scope（必做）**
  1. 删除请求侧 Cookie、Authorization、请求 Cache-Control 旁路
  2. 响应侧：`proxy_no_cache` 绑定 `$upstream_http_set_cookie`
  3. 默认 `proxy_cache_valid`（200/206/301→120m，302/303→20m，404/410→3m）
  4. 默认静态扩展名移除 `json`；保留 `map`/`mjs`/`wasm`
  5. 渲染单测 + UI 文案 + 设计/changelog
* **Out of Scope**：Purge、Cache Rules、强制忽略源站 CC、Auth RFC 条件缓存、HEAD→GET

## 2. 设计决策摘要

| 决策 | 选择 |
| --- | --- |
| 请求 Cookie | 不旁路（对齐 CF） |
| Set-Cookie | 不入库 |
| 无源站 CC | 状态码默认 Edge TTL |
| json | 默认表移除 |
| 兼容 | `url`→`all` 不变；行为变更需重新发布配置 |

## 3. 修改清单

### 边缘渲染

* #### [MODIFY] `pkg/render/openresty/render.go`
  * `renderRouteCacheBlock`：仅保留非 GET 旁路；`proxy_no_cache $openflare_skip_cache $upstream_http_set_cookie`；追加三行 `proxy_cache_valid`
* #### [MODIFY] `pkg/render/openresty/types.go`
  * `DefaultStaticCacheExtensions`：去掉 `json`
* #### [MODIFY] `pkg/render/openresty/render_test.go`
  * 断言：无 cookie/auth/cache_control 旁路；含 set_cookie 与 proxy_cache_valid；表不含 json、含 map

### 前端

* #### [MODIFY] `frontend/app/(main)/proxy-routes/detail/components/cache-section.tsx`
  * 去掉「绕过登录 Cookie / Authorization」类文案
  * 改为 CF 对齐说明：源站 private/no-store、Set-Cookie 不入库、默认静态不含 HTML/JSON

### 文档

* #### [MODIFY] `docs/design/edge-cache-design.md`（已更新）
* #### [MODIFY] `docs/changelog/index.md` `[Unreleased]`
* #### [MODIFY] `docs/plan/index.md` 登记本计划

### 不改

* 无 DB 迁移
* 无 API 字段变更（策略枚举不变）

## 4. 验证计划

### 自动化

```bash
go test ./pkg/render/openresty/
make format
make code-check
```

### 数据面（配置发布后）

1. 站点 `cache_enabled` + `static`，全局缓存开
2. 带 session Cookie：`GET /static/app.js` 第二次应 HIT
3. `GET /index.html` 应为未缓存
4. 源站返回 `Set-Cookie` 的静态 URL 不应出现稳定 HIT
5. 访问日志 `cache_status` 与三态一致

## 5. 发布注意

* 节点需 **重新发布/拉取配置版本** 后旁路变更才生效
* 若站点依赖边缘缓存 `*.json`，改为 `suffix` 含 json 或 `all`

## 6. 状态

- [x] 设计定稿（用户确认：全量对齐 CF）
- [x] 渲染与单测
- [x] UI 文案
- [x] changelog / plan index
- [x] `make format` + `make code-check`
- [x] 复查补强：`all` 策略 UI 警告；proxy-config / troubleshooting 运维说明
- [ ] 用户确认后提交
