import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const fragmentsDir = resolve(root, 'messages/fragments');

function deepMerge(target, source) {
  for (const [key, value] of Object.entries(source)) {
    if (
      value &&
      typeof value === 'object' &&
      !Array.isArray(value) &&
      target[key] &&
      typeof target[key] === 'object' &&
      !Array.isArray(target[key])
    ) {
      deepMerge(target[key], value);
    } else {
      target[key] = value;
    }
  }
  return target;
}

function mergeLocale(locale) {
  const mainPath = resolve(root, `messages/${locale}.json`);
  // 主包是纯生成物：从空对象开始合并全部 fragment（fragments 为唯一源）
  const data = {};
  const suffix = `.${locale}.json`;
  for (const name of readdirSync(fragmentsDir).sort()) {
    if (!name.endsWith(suffix)) continue;
    const frag = JSON.parse(readFileSync(resolve(fragmentsDir, name), 'utf8'));
    deepMerge(data, frag);
    console.log(`merged ${name}`);
  }
  writeFileSync(mainPath, `${JSON.stringify(data, null, 2)}\n`);
}

mergeLocale('zh-CN');
mergeLocale('en');
console.log('fragments merged');
