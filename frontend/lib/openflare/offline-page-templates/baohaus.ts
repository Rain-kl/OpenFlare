/** 内置离线页模板：baohaus */
const html = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>离线页 | OpenFlare</title>
  <style>
    *, *::before, *::after {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }

    body {
      font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: #f4f3ef;
      color: #121212;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      justify-content: center;
      align-items: center;
      padding: 48px 24px;
      -webkit-font-smoothing: antialiased;
    }

    .bauhaus-card {
      width: 100%;
      max-width: 580px;
      background: #ffffff;
      border: 3px solid #121212;
      box-shadow: 8px 8px 0px #121212;
      display: flex;
      flex-direction: column;
      overflow: hidden;
    }

    .card-accent-bar {
      height: 12px;
      display: flex;
    }

    .accent-red { flex: 2; background-color: #e63946; }
    .accent-blue { flex: 3; background-color: #1d3557; }
    .accent-yellow { flex: 1; background-color: #f1faee; }

    .card-body {
      padding: 40px 32px;
      display: flex;
      flex-direction: column;
      gap: 24px;
    }

    .tag {
      align-self: flex-start;
      background-color: #121212;
      color: #ffffff;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.1em;
      text-transform: uppercase;
      padding: 4px 10px;
    }

    .title {
      font-size: 28px;
      font-weight: 800;
      line-height: 1.2;
      letter-spacing: -0.02em;
    }

    .message {
      font-size: 15px;
      line-height: 1.6;
      color: #4a4a4a;
    }

    .contact-box {
      border: 2px solid #121212;
      background-color: #f9f8f3;
      padding: 20px;
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    .contact-line {
      display: flex;
      align-items: center;
      justify-content: space-between;
      font-size: 14px;
      font-weight: 600;
    }

    .contact-line a {
      color: #e63946;
      text-decoration: none;
      font-weight: 700;
    }

    .contact-line a:hover {
      text-decoration: underline;
    }

    .card-footer {
      border-top: 2px solid #121212;
      padding: 16px 32px;
      background-color: #faf9f6;
      display: flex;
      align-items: center;
      justify-content: space-between;
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.05em;
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .brand-icon {
      width: 16px;
      height: 16px;
      fill: currentColor;
    }
  </style>
</head>
<body>
<div class="bauhaus-card">
  <div class="card-accent-bar">
    <div class="accent-red"></div>
    <div class="accent-blue"></div>
    <div class="accent-yellow"></div>
  </div>
  <div class="card-body">
    <div class="tag">OFFLINE FALLBACK</div>
    <h1 class="title">离线联系站长</h1>
    <p class="message">
      源站网络临时不可达。离线 Service Worker 已拦截请求，您可以使用下方联系方式取得支持。
    </p>
    <div class="contact-box">
      <div class="contact-line">
        <span>站长邮箱</span>
        <a href="mailto:admin@example.com">admin@example.com</a>
      </div>
      <div class="contact-line">
        <span>服务监控</span>
        <a href="https://status.example.com" target="_blank" rel="noopener">status.example.com</a>
      </div>
    </div>
  </div>
  <div class="card-footer">
    <div class="brand">
      <svg class="brand-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <path d="M13 2L3 14H12L11 22L21 10H12L13 2Z" />
      </svg>
      <span>OpenFlare Service Worker</span>
    </div>
    <span>OFFLINE MODE</span>
  </div>
</div>
</body>
</html>
`;

export default html;
