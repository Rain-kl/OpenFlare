# 边缘缓存默认 static 策略 — 实现计划

对应设计：[edge-cache-design.md](../design/edge-cache-design.md)

## 目标

路由开启缓存后，**新建推荐**仅缓存标准静态扩展名（`static`）；存量 `url`/空策略映射为 `all`，不收窄缓存范围。

## 兼容规则（评审后定稿）

| 场景 | 行为 |
| --- | --- |
| 已启用 + `''` / `url` | 读 API / 快照 / 渲染 → **`all`** |
| 写入时 enabled 且 policy 为空 | 规范为 **`all`**（旧客户端兼容） |
| UI 新建/推荐默认 | **显式提交** `static` |
| 关闭缓存 | policy 存 `''`，rules 清空 |

## 修改清单

1. **渲染** `pkg/render/openresty/render.go`：`static` 内置扩展名；空/`url`/`all` 无路径限制
2. **校验/展示** `proxy_route/helpers.go`：`normalizeCachePolicy` + `displayCachePolicy`
3. **快照** `config_version/logics.go`：`normalizeSnapshotCachePolicy`
4. **前端** `cache-section.tsx` + helpers：存量 empty/url→`all`；关闭时提交 `''`；新建默认 `static`
5. **测试** render + proxy_route
6. **设计/changelog** 同步兼容说明

## 验证

```bash
go test ./pkg/render/openresty/ ./internal/apps/openflare/proxy_route/ ./internal/apps/openflare/config_version/
# 已通过（2026-07-18）
```

## 状态

- [x] 功能实现 + 评审修复（empty→all，禁止静默收窄）
- [ ] 提交 `fix(cache): ...`（待用户确认）
- [ ] 合并 / 发布后需重新发布节点配置
