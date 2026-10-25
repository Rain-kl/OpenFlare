import { getFormattingLocale } from '@/i18n/format';
const compactNumberFormatters = new Map<string, Intl.NumberFormat>();

function compactNumberFormatter() {
  const locale = getFormattingLocale();
  let formatter = compactNumberFormatters.get(locale);
  if (!formatter) {
    formatter = new Intl.NumberFormat(locale, { maximumFractionDigits: 1 });
    compactNumberFormatters.set(locale, formatter);
  }
  return formatter;
}

type FormatBytesOptions = {
  emptyText?: string;
  zeroText?: string;
};

export function formatCompactNumber(value?: number | null) {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '—';
  }
  return compactNumberFormatter().format(value);
}

export function formatPercent(value?: number | null) {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '—';
  }
  return `${formatCompactNumber(value)}%`;
}

export function formatBytes(
  value?: number | null,
  options: FormatBytesOptions = {},
) {
  const { emptyText = '—', zeroText = '0 B' } = options;

  if (value === undefined || value === null || Number.isNaN(value)) {
    return emptyText;
  }
  if (value <= 0) {
    return zeroText;
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let current = value;
  let index = 0;
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024;
    index += 1;
  }

  const digits = current >= 100 || index === 0 ? 0 : 1;
  return `${new Intl.NumberFormat(getFormattingLocale(), {
    maximumFractionDigits: digits,
    minimumFractionDigits: 0,
  }).format(current)} ${units[index]}`;
}

export function formatBytesPerSecond(
  value?: number | null,
  windowSeconds = 1,
  options?: FormatBytesOptions,
) {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return options?.emptyText ?? '—';
  }
  if (windowSeconds <= 0) {
    return options?.emptyText ?? '—';
  }
  return `${formatBytes(value / windowSeconds, options)}/s`;
}

export function calculateNiceAxisMax(values: number[]) {
  const rawMax = Math.max(
    0,
    ...values.map((value) => (Number.isFinite(value) && value > 0 ? value : 0)),
  );

  if (rawMax <= 0) {
    return 1;
  }

  const paddedMax = rawMax * 1.1;
  const magnitude = 10 ** Math.floor(Math.log10(paddedMax));
  const normalized = paddedMax / magnitude;
  const steps = [1, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10];
  const niceStep = steps.find((step) => normalized <= step) ?? 10;

  return niceStep * magnitude;
}
