import { readFileSync, readdirSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';

const root = process.argv[2]
  ? resolve(process.argv[2])
  : resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const translatedScript =
  /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}\p{Script=Cyrillic}]/u;
const failures = [];
// These six identifiers are persisted upload counter keys. Renaming them is a
// data migration, not a translation. The UI translates their labels separately.
const legacyCategoryKeys = new Set([
  '图片',
  '视频',
  '音频',
  '文档',
  '压缩包',
  '其他',
]);
const compatibilityFiles = new Set([
  'internal/apps/upload/stats/category.go',
  'frontend/app/(main)/admin/files/components/file-stats.tsx',
]);

function report(file, source, offset, text) {
  if (!translatedScript.test(text)) return;
  const name = relative(root, file).replaceAll('\\', '/');
  if (compatibilityFiles.has(name) && legacyCategoryKeys.has(text)) return;
  // Historical persisted notification recipient alias, not display text.
  if (name === 'internal/apps/admin/push/logics.go' && text === '系统') return;
  const line = source.slice(0, offset).split('\n').length;
  failures.push(`${name}:${line}`);
}

function checkGo(file, source) {
  for (let index = 0; index < source.length; ) {
    if (source.startsWith('//', index)) {
      const end = source.indexOf('\n', index);
      index = end < 0 ? source.length : end;
      continue;
    }
    if (source.startsWith('/*', index)) {
      const end = source.indexOf('*/', index + 2);
      index = end < 0 ? source.length : end + 2;
      continue;
    }
    const quote = source[index];
    if (!['"', '`', "'"].includes(quote)) {
      index++;
      continue;
    }
    const start = index++;
    while (index < source.length) {
      if (quote !== '`' && source[index] === '\\') {
        index += 2;
        continue;
      }
      if (source[index++] === quote) break;
    }
    let text = source.slice(start + 1, index - 1);
    // Go struct tags describe schemas/docs, rather than runtime messages.
    if (
      quote === '`' &&
      /^(?:json|gorm|form|binding|validate|mapstructure|example):/.test(text)
    )
      continue;
    if (quote !== '`') {
      text = text.replace(
        /(?<!\\)\\u([\da-f]{4})|(?<!\\)\\U([\da-f]{8})/gi,
        (_, short, long) =>
          String.fromCodePoint(Number.parseInt(short ?? long, 16)),
      );
    }
    report(file, source, start, text);
  }
}

function inspect(file) {
  const source = readFileSync(file, 'utf8');
  if (file.endsWith('.go')) {
    checkGo(file, source);
    return;
  }
  if (file.endsWith('.sh')) {
    let offset = 0;
    for (const line of source.split('\n')) {
      if (!line.trimStart().startsWith('#')) report(file, source, offset, line);
      offset += line.length + 1;
    }
    return;
  }
  const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
  function visit(node) {
    if (ts.isCallExpression(node) || ts.isNewExpression(node)) {
      const callee = node.expression.getText(ast);
      const isFormatter =
        /(?:Intl\.(?:NumberFormat|DateTimeFormat|DisplayNames)|\.toLocale(?:String|DateString|TimeString)|\.localeCompare)$/.test(
          callee,
        );
      const isFixedChinese = (argument) =>
        ts.isStringLiteral(argument) && argument.text === 'zh-CN';
      if (
        isFormatter &&
        node.arguments?.some(
          (argument) =>
            isFixedChinese(argument) ||
            (ts.isArrayLiteralExpression(argument) &&
              argument.elements.some(isFixedChinese)),
        )
      ) {
        failures.push(
          `${relative(root, file).replaceAll('\\', '/')}:${source.slice(0, node.getStart(ast)).split('\n').length} (fixed formatting locale)`,
        );
      }
    }
    if (
      ts.isStringLiteral(node) ||
      ts.isNoSubstitutionTemplateLiteral(node) ||
      ts.isJsxText(node) ||
      ts.isTemplateHead(node) ||
      ts.isTemplateMiddle(node) ||
      ts.isTemplateTail(node) ||
      ts.isIdentifier(node)
    ) {
      report(file, source, node.getStart(ast), node.text);
    }
    ts.forEachChild(node, visit);
  }
  visit(ast);
}

function walk(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (
      entry.name.startsWith('.') ||
      ['node_modules', 'messages', 'dist', 'out', 'testhelper'].includes(
        entry.name,
      )
    )
      continue;
    const file = resolve(directory, entry.name);
    if (entry.isDirectory()) walk(file);
    else if (
      /\.(?:go|tsx?|sh)$/.test(file) &&
      !/(?:_test\.go|\.(?:test|spec)\.tsx?)$/.test(file)
    )
      inspect(file);
  }
}

for (const directory of [
  'internal',
  'pkg',
  'cmd',
  'scripts',
  'frontend/app',
  'frontend/components',
  'frontend/contexts',
  'frontend/lib',
  'frontend/hooks',
  'frontend/i18n',
])
  walk(resolve(root, directory));

for (const file of ['frontend/proxy.ts', 'frontend/next.config.ts'])
  inspect(resolve(root, file));
if (failures.length) {
  console.error(
    'Non-English runtime text must be moved into a locale catalog:\n' +
      failures.join('\n'),
  );
  process.exitCode = 1;
} else {
  console.log(
    'Runtime language check passed (legacy persistence keys preserved)',
  );
}
