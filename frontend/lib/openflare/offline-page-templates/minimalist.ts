/** 内置离线页模板：minimalist（默认） */
const html = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>离线页 | OpenFlare</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: #ffffff;
      color: #333333;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      justify-content: center;
      align-items: center;
      text-align: center;
      padding: 48px 24px;
    }
    .container {
      max-width: 560px;
      width: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 24px;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      padding: 4px 12px;
      border-radius: 9999px;
      background-color: #f3f4f6;
      color: #4b5563;
      font-size: 12px;
      font-weight: 500;
      letter-spacing: 0.05em;
    }
    .title {
      font-size: 32px;
      font-weight: 700;
      color: #111827;
      line-height: 1.25;
    }
    .description {
      font-size: 15px;
      line-height: 1.6;
      color: #6b7280;
      max-width: 480px;
    }
    .contact-card {
      width: 100%;
      background: #f9fafb;
      border: 1px solid #e5e7eb;
      border-radius: 12px;
      padding: 24px;
      display: flex;
      flex-direction: column;
      gap: 16px;
      text-align: left;
    }
    .contact-item {
      display: flex;
      align-items: center;
      gap: 12px;
      font-size: 14px;
      color: #374151;
    }
    .contact-item svg {
      width: 18px;
      height: 18px;
      color: #6b7280;
      flex-shrink: 0;
    }
    .contact-item a {
      color: #2563eb;
      text-decoration: none;
    }
    .contact-item a:hover {
      text-decoration: underline;
    }
    .footer {
      margin-top: 32px;
      display: flex;
      align-items: center;
      gap: 8px;
      color: #9ca3af;
      font-size: 13px;
    }
    .brand-icon {
      width: 20px;
      height: 20px;
      fill: currentColor;
    }
  </style>
</head>
<body>
<div class="container">
  <div class="badge">OFFLINE CONTACT</div>
  <h1 class="title">站点暂不可用，联系站长</h1>
  <p class="description">
    当前无法连接至源站。页面已开启 Service Worker 离线保护，您可以通过以下方式与站长取得联系。
  </p>
  <div class="contact-card">
    <div class="contact-item">
      <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"></path></svg>
      <span>联系邮箱：<a href="mailto:admin@example.com">admin@example.com</a></span>
    </div>
    <div class="contact-item">
      <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
      <span>服务状态页：<a href="https://status.example.com" target="_blank" rel="noopener">status.example.com</a></span>
    </div>
  </div>
  <div class="footer">
    <svg class="brand-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
      <path d="M13 2L3 14H12L11 22L21 10H12L13 2Z" />
    </svg>
    <span>OpenFlare Protection</span>
  </div>
</div>
</body>
</html>
`;

export default html;
