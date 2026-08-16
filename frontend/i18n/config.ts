export const locales = ['zh-CN', 'en'] as const;

export type AppLocale = (typeof locales)[number];

export const defaultLocale: AppLocale = 'zh-CN';

export const localeCookieName = 'NEXT_LOCALE';

export const localeLabels: Record<AppLocale, string> = {
  'zh-CN': '中文',
  en: 'English',
};

export function isAppLocale(
  value: string | undefined | null,
): value is AppLocale {
  return value === 'zh-CN' || value === 'en';
}

/** Normalize BCP 47 / browser tags into a supported app locale. */
export function normalizeLocale(input?: string | null): AppLocale {
  if (!input) return defaultLocale;
  const tag = input.trim().replaceAll('_', '-');
  if (!tag) return defaultLocale;

  const lower = tag.toLowerCase();
  if (lower === 'zh-cn' || lower.startsWith('zh-hans') || lower === 'zh') {
    return 'zh-CN';
  }
  if (lower.startsWith('zh')) {
    // zh-TW / zh-Hant etc. map to zh-CN until a dedicated locale exists
    return 'zh-CN';
  }
  if (lower === 'en' || lower.startsWith('en-')) {
    return 'en';
  }
  return defaultLocale;
}

/** Pick the best locale from an Accept-Language style list. */
export function pickLocaleFromAcceptLanguage(
  header?: string | null,
): AppLocale {
  if (!header) return defaultLocale;

  const candidates = header
    .split(',')
    .map((part) => {
      const [tag, ...params] = part.trim().split(';');
      const qParam = params.find((p) => p.trim().startsWith('q='));
      const q = qParam ? Number.parseFloat(qParam.split('=')[1] ?? '1') : 1;
      return { tag: tag?.trim() ?? '', q: Number.isFinite(q) ? q : 0 };
    })
    .filter((c) => c.tag)
    .sort((a, b) => b.q - a.q);

  for (const candidate of candidates) {
    const normalized = normalizeLocale(candidate.tag);
    // Only accept if the raw tag roughly matches a supported family
    const lower = candidate.tag.toLowerCase();
    if (lower.startsWith('zh') || lower.startsWith('en')) {
      return normalized;
    }
  }

  return defaultLocale;
}
