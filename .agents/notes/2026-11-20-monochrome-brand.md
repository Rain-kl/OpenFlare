# OpenFlare 黑白标识

控制台使用用户提供的矢量图形，浅色主题显示黑标，深色主题显示白标。登录与注册仍读取站点名称。默认名称使用横版标识，自定义名称与独立图标并排展示。文档站与 README 使用相同图形的黑白版本。

内置响应页将 `internal/openflare/edgeconfig/pow_static/img/openflare-mark.svg` 嵌入 HTML。Server 与 Edge 可独立生成页面，前端预览资源由 `frontend/scripts/generate-pow-assets.mjs` 同步。内置页面可在自定义域名、沙盒预览和离线缓存中显示标识。升级保留用户保存的自定义 HTML。
