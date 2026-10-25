import type { AppLocale } from '@/i18n/config';
import { getRuntimeTranslator } from '@/i18n/runtime';

const durationKeys = {
  day: 'system_metric_day',
  hour: 'system_metric_hour',
  minute: 'system_metric_minute',
  second: 'system_metric_second',
} as const;

// The status API owns these English diagnostic values. Localize their display
// without changing the raw values used by memory calculations or other clients.
export function formatSystemMetric(
  value: string | number,
  locale: AppLocale,
): string {
  const number = (n: number, digits = 0) =>
    new Intl.NumberFormat(locale, {
      minimumFractionDigits: digits,
      maximumFractionDigits: Math.max(digits, 3),
    }).format(n);
  if (typeof value === 'number') return number(value);
  const t = getRuntimeTranslator(locale);
  if (value === 'Unknown') return t('system_metric_unknown');
  if (value === 'None') return t('system_metric_none');
  const size = /^(\d+(?:\.\d+)?) (B|KiB|MiB|GiB|TiB)$/.exec(value);
  if (size) return `${number(Number(size[1]))} ${size[2]}`;
  const seconds = /^(\d+(?:\.\d+)?)s$/.exec(value);
  if (seconds) {
    const digits = seconds[1].split('.')[1]?.length ?? 0;
    return t('system_metric_second', {
      count: number(Number(seconds[1]), digits),
    });
  }
  const parts = [
    ...value.matchAll(/(\d+)\s*(days?|hours?|minutes?|seconds?)/g),
  ];
  if (
    !parts.length ||
    parts
      .map((part) => part[0])
      .join('')
      .replaceAll(/\s/g, '') !== value.replaceAll(/\s/g, '')
  )
    return value;
  return parts
    .map((part) => {
      const unit = part[2].replace(/s$/, '') as keyof typeof durationKeys;
      return t(durationKeys[unit], { count: number(Number(part[1])) });
    })
    .join(' ');
}
