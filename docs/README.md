# OpenFlare 文档站

文档站使用 Next.js App Router 与 Fumadocs，页面以静态方式导出，不需要 Node.js 运行时。中文与英文 Markdown 仍分别维护在 `docs/` 与 `docs/en/` 下的指南、部署、参考、设计和更新日志目录。

## 本地开发

```sh
pnpm install --frozen-lockfile
pnpm dev
```

生产构建使用 `pnpm build`，静态文件输出到 `docs/out/`。

## Cloudflare Pages

将 Cloudflare Pages 项目的根目录设为 `docs`，构建命令设为 `pnpm build`，输出目录设为 `out`，并使用 Node.js 22 或更新版本。Fumadocs 搜索索引也会静态生成并随站点一同发布。

同一个 `out/` 目录也可作为 Workers Static Assets 的静态资源目录。
