import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

function flatten(obj, prefix = '') {
  const keys = new Set();
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      for (const child of flatten(value, path)) keys.add(child);
    } else {
      keys.add(path);
    }
  }
  return keys;
}

const zh = JSON.parse(
  readFileSync(resolve(root, 'messages/zh-CN.json'), 'utf8'),
);
const en = JSON.parse(readFileSync(resolve(root, 'messages/en.json'), 'utf8'));
const zhKeys = flatten(zh);
const enKeys = flatten(en);
const onlyZh = [...zhKeys].filter((k) => !enKeys.has(k)).sort();
const onlyEnFixed = [...enKeys].filter((k) => !zhKeys.has(k)).sort();

if (onlyZh.length || onlyEnFixed.length) {
  console.error('i18n key mismatch');
  if (onlyZh.length) console.error('only in zh-CN:', onlyZh);
  if (onlyEnFixed.length) console.error('only in en:', onlyEnFixed);
  process.exit(1);
}

console.log(`i18n keys OK (${zhKeys.size} keys)`);
