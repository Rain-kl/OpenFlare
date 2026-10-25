import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { messageContractTokens } from './message-contract.mjs';

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

const en = JSON.parse(readFileSync(resolve(root, 'messages/en.json'), 'utf8'));
const enKeys = flatten(en);
for (const locale of ['zh-CN', 'ru']) {
  const messages = JSON.parse(
    readFileSync(resolve(root, `messages/${locale}.json`), 'utf8'),
  );
  const keys = flatten(messages);
  const extra = [...keys].filter((key) => !enKeys.has(key)).sort();
  const missing = [...enKeys].filter((key) => !keys.has(key)).sort();
  if (extra.length || missing.length) {
    console.error(`i18n key mismatch: ${locale}`, { extra, missing });
    process.exitCode = 1;
  }
  for (const key of enKeys) {
    const get = (source) =>
      key.split('.').reduce((value, part) => value?.[part], source);
    const source = get(en);
    const translated = get(messages);
    if (
      typeof source === 'string' &&
      typeof translated === 'string' &&
      JSON.stringify(messageContractTokens(source)) !==
        JSON.stringify(messageContractTokens(translated))
    ) {
      console.error(`${locale} changes template variables or tags in ${key}`);
      process.exitCode = 1;
    }
  }
}
if (!process.exitCode)
  console.log(`i18n keys OK (${enKeys.size} keys, 3 locales)`);
