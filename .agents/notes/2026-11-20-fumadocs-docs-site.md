# Fumadocs 文档站

文档站改用 Next.js App Router 与 Fumadocs。现有中文页面继续使用根路径，英文页面保留 `/en/` 前缀；Markdown 仍位于 `docs/` 与 `docs/en/` 下，并由内容源白名单限制发布范围，避免把设计草稿和内部计划带入网站。

生产构建启用 Next.js 静态导出，包含本地静态搜索索引，发布目录为 `docs/out/`。Cloudflare Pages 项目根目录设为 `docs`、输出目录设为 `out`，无需 Node.js 服务运行时。
