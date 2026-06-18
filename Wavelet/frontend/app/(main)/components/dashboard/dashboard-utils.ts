export {
  calculateNiceAxisMax,
  formatBytes,
  formatBytesPerSecond,
  formatCompactNumber,
  formatPercent,
} from '@/lib/utils/metrics';

export function formatTrendHour(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return `${date.getHours().toString().padStart(2, '0')}:00`;
}