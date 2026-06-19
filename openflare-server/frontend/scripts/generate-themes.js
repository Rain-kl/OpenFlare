/* eslint-disable @typescript-eslint/no-require-imports */
const fs = require('fs');
const path = require('path');

const STYLE_DIR = path.join(__dirname, '../public/style');
const OUTPUT_FILE = path.join(__dirname, '../lib/theme/themes.json');

function parseCSSVariables(cssSection) {
  const variables = {};
  const regex = /--([a-z0-9-]+):\s*([^;]+);/g;
  let match;
  while ((match = regex.exec(cssSection)) !== null) {
    variables[match[1]] = match[2].trim();
  }
  return variables;
}

try {
  if (!fs.existsSync(STYLE_DIR)) {
    console.error(`Style directory not found: ${STYLE_DIR}`);
    process.exit(1);
  }

  const files = fs.readdirSync(STYLE_DIR);
  const themes = files
    .filter((file) => file.endsWith('.css'))
    .map((file) => {
      /* 生成主题名称 */
      const name = file === 'default.css'
        ? 'Default'
        : file
            .replace('.css', '')
            .split('-')
            .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
            .join(' ');

      const content = fs.readFileSync(path.join(STYLE_DIR, file), 'utf-8');

      /* 从 :root 解析明亮模式颜色 */
      const rootSection = content.match(/:root\s*\{([^}]+)\}/s)?.[1] || "";
      const lightColors = parseCSSVariables(rootSection);

      /* 从 .dark 解析暗色模式颜色 */
      const darkSection = content.match(/\.dark\s*\{([^}]+)\}/s)?.[1] || "";
      const darkColors = parseCSSVariables(darkSection);

      return {
        id: file,
        name,
        colors: {
          light: lightColors,
          dark: darkColors,
        }
      };
    });

  /* 默认主题排在最前面，其余按名称排序 */
  themes.sort((a, b) => {
    if (a.id === 'default.css') return -1;
    if (b.id === 'default.css') return 1;
    return a.name.localeCompare(b.name);
  });

  // 确保输出文件的父目录存在
  const outputDir = path.dirname(OUTPUT_FILE);
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  fs.writeFileSync(OUTPUT_FILE, JSON.stringify(themes, null, 2), 'utf-8');
  console.log(`Successfully generated themes.json at ${OUTPUT_FILE}`);
} catch (error) {
  console.error('Failed to generate themes.json:', error);
  process.exit(1);
}
