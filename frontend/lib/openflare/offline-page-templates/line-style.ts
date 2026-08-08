/** 内置离线页模板：line-style */
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

        :root {
            --color-bg: #fcfcfc;
            --color-line: #e0e0e0;
            --color-line-hover: #1a1a1a;
            --color-text-main: #1a1a1a;
            --color-text-sub: #666666;
            --color-text-muted: #999999;
            --color-accent: #d4d4d4;
            --font-geo: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
            --transition-fast: 150ms cubic-bezier(0.4, 0, 0.2, 1);
        }

        body {
            font-family: var(--font-geo);
            background-color: var(--color-bg);
            background-image: radial-gradient(circle, var(--color-accent) 0.8px, transparent 0.8px);
            background-size: 24px 24px;
            color: var(--color-text-main);
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            padding: 48px 24px;
            -webkit-font-smoothing: antialiased;
            letter-spacing: 0.01em;
        }

        .main-container {
            width: 100%;
            max-width: 640px;
            display: flex;
            flex-direction: column;
            gap: 32px;
        }

        .line-box {
            border: 1px solid var(--color-line);
            background: rgba(255, 255, 255, 0.7);
            backdrop-filter: blur(4px);
            padding: 32px;
            transition: all var(--transition-fast);
            position: relative;
        }

        .line-box:hover {
            border-color: var(--color-line-hover);
            box-shadow: 0 0 0 1px var(--color-line-hover);
        }

        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            font-size: 11px;
            font-weight: 600;
            letter-spacing: 0.15em;
            text-transform: uppercase;
            color: var(--color-text-sub);
            margin-bottom: 12px;
        }

        .status-dot {
            width: 6px;
            height: 6px;
            border-radius: 50%;
            background-color: #f59e0b;
        }

        .title {
            font-size: 24px;
            font-weight: 700;
            letter-spacing: -0.02em;
            margin-bottom: 12px;
        }

        .description {
            font-size: 14px;
            line-height: 1.6;
            color: var(--color-text-sub);
            margin-bottom: 24px;
        }

        .contact-list {
            display: flex;
            flex-direction: column;
            gap: 12px;
            border-top: 1px dashed var(--color-line);
            padding-top: 20px;
        }

        .contact-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 13px;
        }

        .contact-label {
            color: var(--color-text-muted);
            font-size: 12px;
            letter-spacing: 0.05em;
            text-transform: uppercase;
        }

        .contact-value a {
            color: var(--color-text-main);
            font-weight: 600;
            text-decoration: none;
            border-bottom: 1px solid var(--color-line);
            transition: border-color var(--transition-fast);
        }

        .contact-value a:hover {
            border-color: var(--color-text-main);
        }

        .footer-bar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 11px;
            color: var(--color-text-muted);
            letter-spacing: 0.05em;
        }

        .brand {
            display: flex;
            align-items: center;
            gap: 6px;
            font-weight: 600;
            color: var(--color-text-main);
        }

        .brand-icon {
            width: 14px;
            height: 14px;
            fill: currentColor;
        }
    </style>
</head>
<body>
<div class="main-container">
    <div class="line-box">
        <div class="status-badge">
            <span class="status-dot"></span>
            SERVICE WORKER OFFLINE CACHE
        </div>
        <h1 class="title">网络中断或源站不可达</h1>
        <p class="description">
            系统已自动启用 Service Worker 兜底保护。如需紧急联系站长，请通过以下渠道：
        </p>
        <div class="contact-list">
            <div class="contact-row">
                <span class="contact-label">Email Support</span>
                <span class="contact-value"><a href="mailto:admin@example.com">admin@example.com</a></span>
            </div>
            <div class="contact-row">
                <span class="contact-label">Status Page</span>
                <span class="contact-value"><a href="https://status.example.com" target="_blank" rel="noopener">status.example.com</a></span>
            </div>
        </div>
    </div>
    <div class="footer-bar">
        <div class="brand">
            <svg class="brand-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                <path d="M13 2L3 14H12L11 22L21 10H12L13 2Z" />
            </svg>
            <span>OpenFlare Edge</span>
        </div>
        <span>FALLBACK ACTIVE</span>
    </div>
</div>
</body>
</html>
`;

export default html;
