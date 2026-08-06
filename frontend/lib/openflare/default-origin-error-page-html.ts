/**
 * Built-in Cloudflare-style origin error page.
 * Keep in sync with Go DefaultOriginErrorPageHTML in
 * pkg/render/openresty/origin_error_page.go
 */
export const DEFAULT_ORIGIN_ERROR_PAGE_HTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{status}} | 源站暂时无法提供服务</title>
<style>
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif;
    background: #f6f7f9;
    color: #1f2937;
  }
  @media (prefers-color-scheme: dark) {
    body { background: #0f1419; color: #e5e7eb; }
    .card { background: #1a2332; border-color: #2d3a4d; box-shadow: none; }
    .status { color: #f3f4f6; }
    .muted { color: #9ca3af; }
    .host { background: #243044; color: #d1d5db; }
  }
  .card {
    width: min(32rem, calc(100% - 2rem));
    padding: 2.5rem 2rem;
    border-radius: 12px;
    background: #fff;
    border: 1px solid #e5e7eb;
    box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
    text-align: center;
  }
  .status {
    margin: 0;
    font-size: clamp(3.5rem, 12vw, 5.5rem);
    font-weight: 700;
    letter-spacing: -0.04em;
    line-height: 1;
    color: #111827;
  }
  h1 {
    margin: 1rem 0 0.5rem;
    font-size: 1.25rem;
    font-weight: 600;
  }
  p { margin: 0.4rem 0; line-height: 1.6; }
  .muted { color: #6b7280; font-size: 0.95rem; }
  .host {
    display: inline-block;
    margin-top: 1.25rem;
    padding: 0.35rem 0.75rem;
    border-radius: 999px;
    background: #f3f4f6;
    color: #374151;
    font-size: 0.85rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    word-break: break-all;
  }
</style>
</head>
<body>
  <main class="card" role="main">
    <p class="status" aria-label="HTTP status">{{status}}</p>
    <h1>源站暂时无法提供服务</h1>
    <p class="muted">网关已拦截源站错误响应。请稍后重试；若问题持续，请联系站点管理员。</p>
    <p class="host">{{host}}</p>
  </main>
</body>
</html>
`;

/** Max HTML size in bytes (aligned with backend 256 KiB). */
export const ORIGIN_ERROR_PAGE_HTML_MAX_BYTES = 256 * 1024;

export function effectiveOriginErrorPageHTML(html: string): string {
  return html.trim() === '' ? DEFAULT_ORIGIN_ERROR_PAGE_HTML : html;
}

export function previewOriginErrorPageHTML(
  html: string,
  status = '502',
  host = 'example.com',
): string {
  return effectiveOriginErrorPageHTML(html)
    .replaceAll('{{status}}', status)
    .replaceAll('{{host}}', host);
}
