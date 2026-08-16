import type { AppLocale } from './config';
import { defaultLocale } from './config';

const localeToIntl: Record<AppLocale, string> = {
  'zh-CN': 'zh-CN',
  en: 'en-US',
};

export function formatDateTime(
  dateStr: string | Date,
  locale: AppLocale = defaultLocale,
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
  locale: AppLocale = defaultLocale,
): string {
  try {
    return new Intl.NumberFormat(localeToIntl[locale] ?? locale).format(value);
  } catch {
    return String(value);
  }
}
