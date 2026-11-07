import type { AppLocale } from './config';
import { defaultLocale, normalizeLocale } from './config';

// Match the language currently rendered by the provider, including the initial
// static-export render. This module also remains safe in server contexts.
export function getFormattingLocale(): AppLocale {
  return typeof document === 'undefined'
    ? defaultLocale
    : normalizeLocale(document.documentElement.lang);
}

const localeToIntl: Record<AppLocale, string> = {
  'zh-CN': 'zh-CN',
  en: 'en-US',
  ru: 'ru-RU',
};

export function formatDateTime(
  dateStr: string | Date,
  locale: AppLocale = getFormattingLocale(),
  timeZone = 'Asia/Shanghai',
): string {
  try {
    const date = typeof dateStr === 'string' ? new Date(dateStr) : dateStr;
    return new Intl.DateTimeFormat(localeToIntl[locale] ?? locale, {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(date);
  } catch {
    return String(dateStr);
  }
}

export function formatNumber(
  value: number,
  locale: AppLocale = getFormattingLocale(),
): string {
  try {
    return new Intl.NumberFormat(localeToIntl[locale] ?? locale).format(value);
  } catch {
    return String(value);
  }
}
