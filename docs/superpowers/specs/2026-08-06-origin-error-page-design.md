# 源站错误页设计（Spec）

> 权威正文与产品文档索引见：[docs/design/origin-error-page.md](../../design/origin-error-page.md)  
> 本文为 brainstorming 流程落库副本，内容与上者保持一致。

---

## 摘要

源站/网关在用户配置的状态码（默认标签 `500-599`，支持 `522` 与 `500-599` 区间）上，返回全局可配置 HTML 错误页，替代当前透传行为。默认 Cloudflare 风格页；可在线自定义 HTML。HTTP **status 保持原错误码**，正文通过 `{{status}}` / `{{host}}` 展示。配置挂在 OpenFlare Option，随配置版本发布；侧栏「网站管理 → 错误页」。可关闭以恢复透传。

## 方案

**方案 A（已采纳）**：全局 Option → 配置快照 `ConfigSnapshot` → OpenResty 渲染 `proxy_intercept_errors` + `error_page` + internal location 模板替换。

非目标：按路由覆盖、上传文件、Pages 路由、改 WAF 自有页。

## 详细章节

完整章节（目标、状态码语法、配置模型、边缘渲染、前端、测试、实现清单、决策记录）见：

**[docs/design/origin-error-page.md](../../design/origin-error-page.md)**
