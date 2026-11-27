import { getFormattingLocale } from '@/i18n/format';
import type {
  NetToolsNodeResultStatus,
  NetToolsTaskStatus,
} from '@/lib/services/openflare';

export function getNetToolsErrorMessage(
  err: unknown,
  fallback: string,
): string {
  if (err instanceof Error && err.message) {
    return err.message;
  }
  return fallback;
}

export function formatNetToolsMs(value?: number | null): string {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '—';
  }
  return value.toFixed(1);
}

export function formatNetToolsPercent(value?: number | null): string {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return '—';
  }
  return value.toFixed(1);
}

export function formatNetToolsBytes(bytes?: number | null): string {
  if (!bytes || bytes <= 0) {
    return '0 B';
  }
  const units = ['B', 'KiB', 'MiB', 'GiB'];
  const idx = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  return `${(bytes / 1024 ** idx).toFixed(1)} ${units[idx]}`;
}

export function formatNetToolsTime(value?: string | null): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(getFormattingLocale());
}

export function getTaskStatusVariant(
  status: NetToolsTaskStatus | NetToolsNodeResultStatus,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'completed':
      return 'default';
    case 'running':
    case 'pending':
      return 'secondary';
    case 'failed':
    case 'timeout':
      return 'destructive';
    default:
      return 'outline';
  }
}
