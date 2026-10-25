#!/usr/bin/env node
// Read-only inventory of all non-ASCII text in tracked and unignored files.
import { execFileSync } from 'node:child_process';
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { dirname, extname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const output = resolve(root, process.argv[2] ?? '.codex-artifacts/non-english-audit');
const paths = [...new Set(execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], { cwd: root, maxBuffer: 16 * 1024 * 1024 }).toString('utf8').split('\0').filter(Boolean))].sort();
const scripts = ['Han', 'Hiragana', 'Katakana', 'Hangul', 'Cyrillic', 'Arabic', 'Hebrew', 'Greek', 'Thai', 'Devanagari', 'Latin'];
const scriptPatterns = scripts.map((name) => [name, new RegExp(`\\p{Script=${name}}`, 'u')]);
const nonASCII = /[^\x00-\x7f]/gu;
const escapes = /\\u\{([0-9a-f]{1,6})\}|\\u([0-9a-f]{4})|\\U([0-9a-f]{8})/gi;
const records = [];
const skipped = [];
let scanned = 0;

function component(path) {
  const parts = path.split('/');
  if (path.startsWith('frontend/app/')) return parts.slice(0, 5).join('/');
  if (path.startsWith('internal/apps/')) return parts.slice(0, 3).join('/');
  return parts.slice(0, Math.min(3, parts.length - 1)).join('/') || '.';
}

// Comments are classified lexically; ambiguous source remains a review candidate.
function commentMask(text, path) {
  const extension = extname(path);
  const mask = new Uint8Array(text.length);
  const slashComments = ['.go', '.js', '.mjs', '.cjs', '.ts', '.tsx', '.jsx', '.css', '.scss', '.proto'].includes(extension);
  const hashComments = ['.sh', '.yaml', '.yml', '.toml', '.py', '.conf'].includes(extension) || /(^|\/)(\.env[^/]*|\.gitignore|Dockerfile|Makefile)$/.test(path);
  let state = '', quote = '';
  for (let i = 0; i < text.length; i++) {
    const c = text[i], next = text[i + 1];
    if (state === 'line') {
      mask[i] = 1;
      if (c === '\n') state = '';
    } else if (state === 'block') {
      mask[i] = 1;
      if (c === '*' && next === '/') { mask[++i] = 1; state = ''; }
    } else if (quote) {
      if (c === '\\' && !(extension === '.go' && quote === '`')) i++;
      else if (c === quote) quote = '';
    } else if (slashComments && c === '/' && next === '/') {
      mask[i] = mask[++i] = 1; state = 'line';
    } else if (slashComments && c === '/' && next === '*') {
      mask[i] = mask[++i] = 1; state = 'block';
    } else if (hashComments && c === '#') {
      mask[i] = 1; state = 'line';
    } else if (c === '"' || c === "'" || (slashComments && c === '`')) quote = c;
  }
  return mask;
}

function category(path, line, spans, mask, offset) {
  if (path.startsWith('frontend/messages/') || /\/locales\//.test(path)) return 'translation-resource';
  if (/\.(md|mdx)$/i.test(path)) return 'documentation';
  if (['docs/docs.go', 'docs/swagger.json', 'docs/swagger.yaml'].includes(path)) return 'generated-api-documentation';
  if (path.startsWith('docs/')) return 'documentation-site';
  if (path.startsWith('.agents/') || path === 'scripts/audit-non-english.mjs') return 'developer-tooling';
  if (path.startsWith('.auto/')) return 'development-history';
  if (path.startsWith('.github/ISSUE_TEMPLATE/') || /close.ticket/i.test(path)) return 'community-template';
  if (/(_test\.go|\.(test|spec)\.[^.]+)$/.test(path) || /(^|\/)(tests?|testdata|testhelper)\//.test(path)) return 'test-fixture';
  if (path.startsWith('internal/infra/persistence/migrator/goose/')) return 'historical-migration';
  if (!spans.some((span) => /\p{L}/u.test(span.value))) return 'symbol-or-typography';
  if (spans.every((span) => mask[offset + span.column - 1])) return 'source-comment';
  if ((path === 'internal/apps/upload/stats/category.go' || path === 'frontend/app/(main)/admin/files/components/file-stats.tsx' || path === 'frontend/scripts/check-runtime-language.mjs') && spans.every((span) => /^[图片视频音频文档压缩包其他]+$/u.test(span.value))) return 'legacy-identifier';
  if (extname(path) === '.go' && /`[^`]*(?:json|gorm|description|example):/.test(line)) return 'schema-metadata';
  return 'review-candidate';
}

for (const path of paths) {
  // Never include private environment values, even if accidentally tracked.
  if (/(^|\/)\.env(?:\.|$)/.test(path) && !/(example|sample|template)/.test(path)) { skipped.push({ path, reason: 'private-environment' }); continue; }
  let bytes;
  try { bytes = readFileSync(resolve(root, path)); } catch (error) { skipped.push({ path, reason: error.code }); continue; }
  let text;
  try {
    if (bytes[0] === 0xff && bytes[1] === 0xfe) text = new TextDecoder('utf-16le', { fatal: true }).decode(bytes);
    else if (bytes[0] === 0xfe && bytes[1] === 0xff) text = new TextDecoder('utf-16be', { fatal: true }).decode(bytes);
    else {
      if (bytes.includes(0)) { skipped.push({ path, reason: 'binary' }); continue; }
      text = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
    }
  } catch { skipped.push({ path, reason: 'non-utf8-or-binary' }); continue; }
  scanned++;
  const mask = commentMask(text, path);
  let offset = 0, lineNumber = 0;
  for (const line of text.split('\n')) {
    lineNumber++;
    const spans = [...line.matchAll(nonASCII)].map((match) => ({ column: match.index + 1, value: match[0], escaped: false }));
    for (const match of line.matchAll(escapes)) {
      const code = parseInt(match[1] ?? match[2] ?? match[3], 16);
      if (code > 127 && code <= 0x10ffff) spans.push({ column: match.index + 1, value: String.fromCodePoint(code), escaped: true });
    }
    if (spans.length) {
      spans.sort((a, b) => a.column - b.column);
      const characters = [...new Set(spans.map((span) => span.value))];
      const names = scriptPatterns.filter(([, pattern]) => characters.some((c) => pattern.test(c))).map(([name]) => name);
      if (characters.some((c) => /\p{L}/u.test(c) && !scriptPatterns.some(([, pattern]) => pattern.test(c)))) names.push('Other-letter');
      records.push({ path, line: lineNumber, component: component(path), category: category(path, line, spans, mask, offset), scripts: names, codepoints: characters.map((c) => `U+${c.codePointAt(0).toString(16).toUpperCase().padStart(4, '0')}`), spans, text: line.replace(/\r$/, '') });
    }
    offset += line.length + 1;
  }
  if (nonASCII.test(path)) records.push({ path, line: 0, component: component(path), category: 'non-ascii-path', scripts: [], codepoints: [], spans: [], text: path });
  nonASCII.lastIndex = 0;
}

mkdirSync(output, { recursive: true });
const totals = {};
for (const record of records) totals[record.category] = (totals[record.category] ?? 0) + 1;
const candidates = records.filter((record) => record.category === 'review-candidate');
const decisions = {
  'translation-resource': 'Keep in locale resources; expected localized content.',
  documentation: 'Keep: project policy requires Chinese documentation.',
  'generated-api-documentation': 'Keep: generated Chinese API documentation, not runtime log or UI literals.',
  'documentation-site': 'Keep: Chinese documentation navigation, locale names and documentation assets.',
  'developer-tooling': 'Keep: agent instructions, examples and audit policy; outside application runtime.',
  'development-history': 'Keep: development history, not imported by application code.',
  'community-template': 'Keep: upstream Chinese community issue/message templates.',
  'test-fixture': 'Keep: test samples and expected Unicode behavior.',
  'historical-migration': 'Keep: migration history; do not rewrite applied migrations.',
  'source-comment': 'Keep: source comments do not produce runtime text.',
  'legacy-identifier': 'Keep: persisted upload category identifiers; display labels use translations.',
  'schema-metadata': 'Review separately: schema descriptions/examples, not runtime log messages.',
  'symbol-or-typography': 'Keep: punctuation, emoji, math and explicit Unicode/control-character handling.',
  'review-candidate': 'Review source: not covered by the classifications above.',
};
const csv = (rows) => '\ufeff' + [['path', 'line', 'component', 'category', 'scripts', 'text'], ...rows.map((r) => [r.path, r.line, r.component, r.category, r.scripts.join(', '), r.text])].map((row) => row.map((cell) => `"${String(cell).replaceAll('"', '""')}"`).join(',')).join('\r\n');
writeFileSync(resolve(output, 'occurrences.json'), JSON.stringify({ coverage: { discovered: paths.length, scanned, skipped, columns: '1-based UTF-16 offsets', scope: 'git ls-files --cached --others --exclude-standard; ignored dependencies, build output and private environment files excluded; binary files are not OCRed or unpacked' }, totals, decisions, records }, null, 2) + '\n');
writeFileSync(resolve(output, 'occurrences.csv'), csv(records));
writeFileSync(resolve(output, 'review-candidates.csv'), csv(candidates));
const groups = new Map();
for (const r of candidates) groups.set(r.component, (groups.get(r.component) ?? 0) + 1);
const components = new Map();
for (const r of records) {
  if (!components.has(r.component)) components.set(r.component, { files: new Set(), lines: 0, categories: new Set() });
  const group = components.get(r.component);
  group.files.add(r.path);
  group.lines++;
  group.categories.add(r.category);
}
const componentRows = [...components].sort((a, b) => b[1].lines - a[1].lines);
writeFileSync(resolve(output, 'components.json'), JSON.stringify(componentRows.map(([name, group]) => ({ component: name, files: [...group.files].sort(), lines: group.lines, categories: [...group.categories].sort() })), null, 2) + '\n');
writeFileSync(resolve(output, 'summary.md'), [
  '# Non-English source inventory', '',
  `Text files scanned: **${scanned}** of **${paths.length}** tracked and unignored files. Skipped: **${skipped.length}** (see occurrences.json).`,
  `Non-ASCII or escaped Unicode lines: **${records.length}**, in **${new Set(records.map((r) => r.path)).size}** files. Review candidates: **${candidates.length}**.`, '',
  'All characters outside ASCII are included, including punctuation, symbols and emoji. Language cannot be inferred reliably from a script (Han may be Chinese or Japanese). Script names describe characters, not detected language.', '',
  'Classification is a review aid, not proof that a string executes or reaches a user. Comments, test data, migration history, schemas, community templates and legacy identifiers are retained. No source files are changed by this command.', '',
  'Ignored dependencies, generated build output and private environment files are excluded. Binary files (including screenshots, fonts, GeoIP data and archives) are listed as skipped; image text is not OCRed and archives are not unpacked by this command.', '',
  '## Categories', '', '| Category | Lines | Disposition |', '| --- | ---: | --- |', ...Object.entries(totals).sort().map(([name, count]) => `| ${name} | ${count} | ${decisions[name] ?? 'Review'} |`), '',
  '## Scripts', '', '| Script | Lines |', '| --- | ---: |', ...scripts.concat('Other-letter').map((script) => `| ${script} | ${records.filter((r) => r.scripts.includes(script)).length} |`), '',
  '## Components requiring review', '', '| Component | Lines |', '| --- | ---: |', ...[...groups].sort((a, b) => b[1] - a[1]).map(([name, count]) => `| ${name} | ${count} |`), '',
  '## Largest components (all categories)', '', '| Component | Lines | Files |', '| --- | ---: | ---: |', ...componentRows.slice(0, 30).map(([name, group]) => `| ${name} | ${group.lines} | ${group.files.size} |`), '',
  '## Files', '', '- occurrences.csv: every matching line with its source path and component.', '- occurrences.json: exact character positions, Unicode code points, script classifications and skipped files.', '- components.json: every affected component, its source files, counts and categories.', '- review-candidates.csv: remaining source/configuration candidates, including embedded templates.', '',
  'Regenerate from the repository root: `node scripts/audit-non-english.mjs`.', '',
].join('\n'));
console.log(JSON.stringify({ scanned, skipped: skipped.length, lines: records.length, files: new Set(records.map((r) => r.path)).size, totals, output }, null, 2));
