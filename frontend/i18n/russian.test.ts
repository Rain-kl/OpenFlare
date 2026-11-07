import { describe, expect, it } from 'vitest';
import { createTranslator, type AbstractIntlMessages } from 'next-intl';
import english from '@/messages/en.json';
import russian from '@/messages/ru.json';
import { normalizeLocale, pickLocaleFromAcceptLanguage } from './config';
import { effectiveOfflinePageHTML } from '@/lib/openflare/offline-page-templates';
import { getOriginErrorPageTemplate } from '@/lib/openflare/origin-error-page-templates';

function flatten(
  messages: AbstractIntlMessages,
  prefix = '',
): Record<string, string> {
  return Object.fromEntries(
    Object.entries(messages).flatMap(([key, value]) => {
      const path = prefix ? `${prefix}.${key}` : key;
      return typeof value === 'string'
        ? [[path, value]]
        : Object.entries(flatten(value, path));
    }),
  );
}

describe('Russian UI', () => {
  it('negotiates Russian variants, quality values and English fallback', () => {
    expect(normalizeLocale('ru_RU')).toBe('ru');
    expect(normalizeLocale('ru-BY')).toBe('ru');
    expect(pickLocaleFromAcceptLanguage('de,ru-RU;q=0.9,en;q=0.7')).toBe('ru');
    expect(pickLocaleFromAcceptLanguage('ru;q=0,en;q=0.5')).toBe('en');
    expect(pickLocaleFromAcceptLanguage('de,fr')).toBe('en');
  });

  it('keeps every message and named argument, and formats Russian ICU messages', () => {
    const source = flatten(english);
    const translated = flatten(russian);
    expect(Object.keys(translated).sort()).toEqual(Object.keys(source).sort());
    const errors: string[] = [];
    const t = createTranslator({
      locale: 'ru',
      messages: russian,
      onError: (error) => errors.push(error.message),
    });
    for (const [key, text] of Object.entries(source)) {
      const args = (value: string) =>
        [
          ...new Set(
            [...value.matchAll(/\{(\w+)(?:\s*[,}])/g)].map((match) => match[1]),
          ),
        ].sort();
      expect(args(translated[key]), key).toEqual(args(text));
      // HTML, code examples and Go templates are intentionally read with t.raw.
      if (
        !text ||
        /\{\{|\{\w+:|className=|function |<\/?[a-zA-Z]|^\s*[{[]/.test(text)
      )
        continue;
      const numericArguments = new Set(
        [
          ...translated[key].matchAll(
            /\{(\w+),\s*(?:plural|selectordinal|number)/g,
          ),
        ].map((match) => match[1]),
      );
      const values = Object.fromEntries(
        args(text).map((name) => [
          name,
          /^(count|total|current|page|pages|totalPages|value\d+)$/.test(name) ||
          numericArguments.has(name)
            ? 2
            : 'CUSTOM',
        ]),
      );
      t(key as Parameters<typeof t>[0], values);
      expect(errors, key).toEqual([]);
    }
    expect(errors).toEqual([]);
  });

  it('uses Russian plural forms for 1, 2, 5, 21 and fractional counts', () => {
    const t = createTranslator({ locale: 'ru', messages: russian });
    for (const [count, expected] of [
      [1, '1 минута'],
      [2, '2 минуты'],
      [5, '5 минут'],
      [21, '21 минута'],
      [1.5, '1,5 минуты'],
    ] as const) {
      expect(t('pages.source.intervalMinutes', { count })).toBe(expected);
    }
    expect(
      t('admin.files.stats.maxCountHint', { name: 'PDF', count: 2 }),
    ).toContain('2 файла');
  });

  it('formats certificate expiry and retry seconds across Russian plural boundaries', () => {
    const t = createTranslator({ locale: 'ru', messages: russian });
    for (const count of [
      0, 1, 2, 4, 5, 11, 12, 13, 14, 21, 22, 23, 24, 25, 101, 111,
    ]) {
      const last = count % 10;
      const teen = count % 100 >= 11 && count % 100 <= 14;
      const day =
        !teen && last === 1
          ? 'день'
          : !teen && last >= 2 && last <= 4
            ? 'дня'
            : 'дней';
      const second =
        !teen && last === 1
          ? 'секунду'
          : !teen && last >= 2 && last <= 4
            ? 'секунды'
            : 'секунд';
      expect(
        t('certificates.certStatus.expiresInDays', { days: count }),
      ).toContain(`${count} ${day}`);
      expect(
        t('runtime.too_many_requests_retry_in_value0_seconds_ab5e78f1', {
          value0: count,
        }),
      ).toContain(`${count} ${second}`);
    }
  });

  it('renders the empty-domain hint as rich text with its website link', () => {
    const errors: string[] = [];
    const t = createTranslator({
      locale: 'ru',
      messages: russian,
      onError: (error) => errors.push(error.message),
    });
    const result = t.rich('proxyRoutes.noZoneDomains', {
      websites: (chunks) => `[LINK]${chunks}[/LINK]`,
    });
    expect(errors).toEqual([]);
    expect(result).toContain('[LINK]');
    expect(result).not.toBe('proxyRoutes.noZoneDomains');
  });

  it('localizes built-in page templates while retaining custom HTML and placeholders', () => {
    document.cookie = 'NEXT_LOCALE=ru; Path=/';
    try {
      expect(effectiveOfflinePageHTML('')).toContain('lang="ru"');
      for (const id of ['minimalist', 'line-style', 'baohaus']) {
        const html = getOriginErrorPageTemplate(id)?.html;
        expect(html).toContain('lang="ru"');
        const originals: Record<string, string> = {
          minimalist: english.runtime.origin_error_page_minimalist,
          'line-style': english.runtime.origin_error_page_line_style,
          baohaus: english.runtime.origin_error_page_baohaus,
        };
        expect(html?.match(/\{\{\w+\}\}/g)).toEqual(
          originals[id].match(/\{\{\w+\}\}/g),
        );
      }
      expect(effectiveOfflinePageHTML('<h1>Custom site</h1>')).toBe(
        '<h1>Custom site</h1>',
      );
    } finally {
      document.cookie = 'NEXT_LOCALE=; Max-Age=0; Path=/';
    }
  });
});
