# 边缘缓存默认 static 策略 — 实现计划

对应设计：[edge-cache-design.md](../design/edge-cache-design.md)

## 目标

路由开启缓存后默认仅缓存标准静态扩展名（`static`）；存量 `url` 映射为 `all`。

## 修改清单

1. **渲染** `pkg/render/openresty/render.go`：`static` 内置扩展名；`url`/`all` 无路径限制
2. **校验** `internal/apps/openflare/proxy_route/helpers.go`：策略枚举与规范化
3. **前端** `cache-section.tsx` + helpers：默认 `static`，选项文案
4. **测试** render + helpers
5. **changelog**

## 验证

```bash
go test ./pkg/render/openresty/ ./internal/apps/openflare/proxy_route/
make code-check  # 或至少 go test + frontend tsc
```
