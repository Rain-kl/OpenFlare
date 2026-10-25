import { locales } from '@/i18n/config';
import { getRuntimeTranslator } from '@/i18n/runtime';

export function isLogStorageDisabledMessage(message: string): boolean {
  const normalized = message.toLowerCase();
  return (
    message.includes('ClickHouse') ||
    locales.some((locale) =>
      normalized.includes(
        getRuntimeTranslator(locale)('disabled_f95ea7f4').toLowerCase(),
      ),
    )
  );
}
