/** 内置错误页模板：baohaus（包豪斯风格） */
const html = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{status}} | OpenFlare</title>
    <style>
        :root {
            --color-red: #E53935;
            --color-yellow: #FDD835;
            --color-blue: #1E88E5;
            --color-black: #212121;
            --color-white: #FAFAFA;
            --color-grey: #9E9E9E;
            --font-primary: 'Futura', 'Century Gothic', 'Tw Cen MT', sans-serif;
        }

        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            background-color: var(--color-white);
            color: var(--color-black);
            font-family: var(--font-primary);
            min-height: 100vh;
            width: 100%;
            display: flex;
            flex-direction: column;
            position: relative;
            overflow-x: hidden;
        }

        body::before {
            content: "";
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background-image: radial-gradient(var(--color-grey) 1px, transparent 1px);
            background-size: 20px 20px;
            opacity: 0.15;
            z-index: -1;
            pointer-events: none;
        }

        .layout-container {
            display: grid;
            grid-template-columns: 1fr 1.618fr;
            grid-template-rows: 1fr 1fr;
            min-height: 100vh;
            width: 100%;
        }

        .zone-left {
            grid-column: 1 / 2;
            grid-row: 1 / 3;
            border-right: 4px solid var(--color-black);
            border-bottom: 4px solid var(--color-black);
            position: relative;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            background-color: var(--color-white);
            padding: 2rem;
            z-index: 2;
        }

        .decor-circle {
            position: absolute;
            width: 40vh;
            height: 40vh;
            background-color: var(--color-yellow);
            border-radius: 50%;
            top: 15%;
            left: 10%;
            z-index: 1;
        }

        .error-number {
            font-size: clamp(6rem, 12vw, 14rem);
            font-weight: 700;
            line-height: 0.9;
            letter-spacing: -0.04em;
            color: var(--color-black);
            position: relative;
            z-index: 2;
        }

        .brand-footer {
            position: absolute;
            bottom: 2rem;
            left: 2rem;
            display: flex;
            align-items: center;
            gap: 10px;
            z-index: 3;
        }

        .brand-icon {
            width: 24px;
            height: 24px;
            fill: var(--color-black);
        }

        .brand-text {
            font-size: 0.85rem;
            font-weight: 700;
            text-transform: uppercase;
            letter-spacing: 0.15em;
        }

        .zone-top-right {
            grid-column: 2 / 3;
            grid-row: 1 / 2;
            border-bottom: 4px solid var(--color-black);
            background-color: var(--color-white);
            padding: 4rem;
            display: flex;
            flex-direction: column;
            justify-content: center;
            z-index: 2;
        }

        .error-title {
            font-size: clamp(1.5rem, 3vw, 2.5rem);
            text-transform: uppercase;
            font-weight: 700;
            margin-bottom: 1.5rem;
            letter-spacing: 0.05em;
        }

        .error-desc {
            font-size: 1.1rem;
            line-height: 1.6;
            max-width: 450px;
            color: var(--color-black);
        }

        .meta-data {
            margin-top: 2rem;
            font-family: monospace;
            font-size: 0.8rem;
            color: var(--color-grey);
            border-top: 1px solid var(--color-grey);
            padding-top: 1rem;
            display: flex;
            flex-direction: column;
            gap: 0.35rem;
            word-break: break-all;
        }

        .zone-bottom-right {
            grid-column: 2 / 3;
            grid-row: 2 / 3;
            background-color: var(--color-white);
            position: relative;
            overflow: hidden;
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 2;
        }

        .geo-line {
            position: absolute;
            width: 100%;
            height: 3px;
            background-color: var(--color-black);
            top: 50%;
            left: 0;
        }

        .geo-triangle {
            position: absolute;
            right: 15%;
            bottom: 15%;
            width: 0;
            height: 0;
            border-left: 80px solid transparent;
            border-right: 80px solid transparent;
            border-bottom: 140px solid var(--color-blue);
        }

        .geo-rect {
            position: absolute;
            top: 40%;
            left: 20%;
            width: 150px;
            height: 30px;
            background-color: var(--color-red);
            transform: rotate(-45deg);
        }

        @media (max-width: 900px) {
            .layout-container {
                display: flex;
                flex-direction: column;
            }

            .zone-left {
                border-right: none;
                border-bottom: 4px solid var(--color-black);
                min-height: 50vh;
                padding-top: 4rem;
            }

            .decor-circle {
                width: 30vh;
                height: 30vh;
                top: 10%;
            }

            .zone-top-right {
                border-bottom: 4px solid var(--color-black);
                padding: 2rem;
            }

            .zone-bottom-right {
                min-height: 30vh;
            }

            .geo-triangle {
                border-left-width: 50px;
                border-right-width: 50px;
                border-bottom-width: 90px;
                right: 10%;
            }

            .geo-rect {
                width: 100px;
                height: 20px;
                left: 15%;
            }
        }
    </style>
</head>
<body>
    <div class="layout-container">
        <div class="zone-left">
            <div class="decor-circle"></div>
            <div class="error-number" aria-label="HTTP status">{{status}}</div>

            <div class="brand-footer">
                <svg class="brand-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                    <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
                </svg>
                <span class="brand-text">OpenFlare</span>
            </div>
        </div>

        <div class="zone-top-right">
            <h1 class="error-title">Origin Unavailable</h1>
            <p class="error-desc">
                The server received an invalid response from the upstream server.
                <br><br>
                <strong>Form follows function:</strong> The connection path is broken. Please verify the origin server status.
            </p>
            <div class="meta-data">
                <div>ERR_CODE: {{status}}</div>
                <div>HOST: {{host}}</div>
                <div>PROXY: OPENFLARE_EDGE</div>
                <div id="timestamp">--:--:-- UTC</div>
            </div>
        </div>

        <div class="zone-bottom-right">
            <div class="geo-line"></div>
            <div class="geo-rect"></div>
            <div class="geo-triangle"></div>
        </div>
    </div>

    <script>
        document.addEventListener('DOMContentLoaded', () => {
            const tsElement = document.getElementById('timestamp');
            if (tsElement) {
                const now = new Date();
                tsElement.textContent = now.toISOString().split('T')[1].split('.')[0] + ' UTC';
            }
        });
    </script>
</body>
</html>
`;

export default html;
