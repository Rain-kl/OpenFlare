/** 内置错误页模板：line-style */
const html = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{status}} | OpenFlare</title>
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
            -moz-osx-font-smoothing: grayscale;
            letter-spacing: 0.01em;
        }

        .main-container {
            width: 100%;
            max-width: 640px;
            display: flex;
            flex-direction: column;
            gap: 48px;
        }

        .line-box {
            border: 1px solid var(--color-line);
            background: rgba(255, 255, 255, 0.6);
            backdrop-filter: blur(4px);
            padding: 32px;
            transition: all var(--transition-fast);
            position: relative;
        }

        .line-box:hover {
            border-color: var(--color-line-hover);
            box-shadow: 0 0 0 1px var(--color-line-hover);
        }

        .line-box:active {
            transform: scale(0.995);
            transition-duration: 100ms;
        }

        .error-header {
            text-align: center;
        }

        .error-code {
            font-size: 64px;
            font-weight: 700;
            line-height: 1;
            letter-spacing: -0.03em;
            color: var(--color-text-main);
            margin-bottom: 12px;
        }

        .error-label {
            font-size: 12px;
            font-weight: 500;
            letter-spacing: 0.2em;
            text-transform: uppercase;
            color: var(--color-text-sub);
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
        }

        .error-label::before,
        .error-label::after {
            content: '';
            width: 24px;
            height: 1px;
            background-color: var(--color-line);
        }

        .topology-wrapper {
            width: 100%;
            overflow: hidden;
        }

        .topology-svg {
            width: 100%;
            height: 140px;
            display: block;
        }

        .node-circle {
            fill: var(--color-bg);
            stroke: var(--color-text-main);
            stroke-width: 1.5;
            transition: all var(--transition-fast);
        }

        .node-circle.error {
            stroke: var(--color-text-muted);
            stroke-dasharray: 3 3;
        }

        .flow-line {
            stroke: var(--color-accent);
            stroke-width: 1.5;
            fill: none;
            transition: stroke var(--transition-fast);
        }

        .flow-line.error {
            stroke: var(--color-text-muted);
            stroke-dasharray: 4 4;
        }

        .node-text {
            font-family: var(--font-geo);
            font-size: 10px;
            font-weight: 600;
            letter-spacing: 0.1em;
            fill: var(--color-text-sub);
            text-anchor: middle;
        }

        .status-text {
            font-family: var(--font-geo);
            font-size: 9px;
            font-weight: 500;
            letter-spacing: 0.05em;
            fill: var(--color-text-muted);
            text-anchor: middle;
        }

        .status-text.error {
            fill: #b0b0b0;
        }

        .description {
            text-align: center;
        }

        .desc-text {
            font-size: 18px;
            line-height: 1.6;
            color: var(--color-text-sub);
            font-weight: 400;
            max-width: 480px;
            margin: 0 auto 24px;
        }

        .meta-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 16px;
            border-top: 1px solid var(--color-line);
            padding-top: 24px;
            text-align: left;
        }

        .meta-item {
            display: flex;
            flex-direction: column;
            gap: 4px;
        }

        .meta-key {
            font-size: 10px;
            font-weight: 600;
            letter-spacing: 0.1em;
            text-transform: uppercase;
            color: var(--color-text-muted);
        }

        .meta-val {
            font-size: 13px;
            font-family: 'SF Mono', 'Fira Code', monospace, var(--font-geo);
            color: var(--color-text-main);
            font-weight: 500;
            word-break: break-all;
        }

        .footer-brand {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 8px;
            color: var(--color-text-muted);
            font-size: 13px;
            font-weight: 500;
            letter-spacing: 0.05em;
            transition: color var(--transition-fast);
            cursor: default;
        }

        .footer-brand:hover {
            color: var(--color-text-main);
        }

        .brand-icon {
            width: 20px;
            height: 20px;
            stroke: currentColor;
            stroke-width: 1.5;
            stroke-linecap: round;
            stroke-linejoin: round;
            fill: none;
        }

        @media (max-width: 480px) {
            body {
                padding: 32px 16px;
            }
            .error-code {
                font-size: 48px;
            }
            .line-box {
                padding: 24px 16px;
            }
            .meta-grid {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="main-container">
        <div class="line-box error-header">
            <div class="error-code" aria-label="HTTP status">{{status}}</div>
            <div class="error-label">Origin Unavailable</div>
        </div>

        <div class="line-box topology-wrapper">
            <svg class="topology-svg" viewBox="0 0 600 140" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <defs>
                    <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                        <path d="M 0 1 L 8 5 L 0 9 z" fill="#d4d4d4" />
                    </marker>
                    <marker id="arrow-error" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                        <path d="M 0 1 L 8 5 L 0 9 z" fill="#999999" />
                    </marker>
                </defs>

                <line x1="135" y1="70" x2="265" y2="70" class="flow-line" marker-end="url(#arrow)" />
                <line x1="335" y1="70" x2="465" y2="70" class="flow-line error" marker-end="url(#arrow-error)" />

                <circle cx="110" cy="70" r="20" class="node-circle" />
                <text x="110" y="74" class="node-text" style="font-size: 8px; fill: #1a1a1a;">CLI</text>
                <text x="110" y="115" class="node-text">CLIENT</text>
                <text x="110" y="130" class="status-text">REQ_SENT</text>

                <circle cx="300" cy="70" r="20" class="node-circle" />
                <path d="M296 70 L304 64 L301 70 L305 70 L296 76 L299 70 Z" fill="none" stroke="#1a1a1a" stroke-width="1" stroke-linejoin="round"/>
                <text x="300" y="115" class="node-text">OPENFLARE</text>
                <text x="300" y="130" class="status-text">EDGE_PROXY</text>

                <circle cx="490" cy="70" r="20" class="node-circle error" />
                <line x1="484" y1="64" x2="496" y2="76" stroke="#999" stroke-width="1.5" stroke-linecap="round"/>
                <line x1="496" y1="64" x2="484" y2="76" stroke="#999" stroke-width="1.5" stroke-linecap="round"/>
                <text x="490" y="115" class="node-text" style="fill: #999;">ORIGIN</text>
                <text x="490" y="130" class="status-text error">UNAVAILABLE</text>
            </svg>
        </div>

        <div class="footer-brand">
            <svg class="brand-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
            </svg>
            <span>OpenFlare</span>
        </div>
    </div>

    <script>
        document.addEventListener('DOMContentLoaded', () => {
            const tsEl = document.getElementById('timestamp');
            if (tsEl) {
                const now = new Date();
                tsEl.textContent = now.toISOString().replace('T', ' ').substring(0, 19) + ' UTC';
            }
        });
    </script>
</body>
</html>
`;

export default html;
