export type AccessLogTab = 'overview' | 'ips' | 'list';

export type SearchDraft = {
  nodeId: string;
  remoteAddr: string;
  host: string;
  path: string;
  statusCode: string;
  since: string;
  until: string;
};

export type OverviewRangeHours = 24 | 168 | 360 | 720;

/** 限流分析等短窗口场景：仅 24 小时 / 3 天 */
export type RateLimitRangeHours = 24 | 72;

export const PAGE_SIZE_OPTIONS = [20, 50, 100, 200];

export const OVERVIEW_RANGE_OPTIONS: {
  value: OverviewRangeHours;
  labelKey: `range.h${OverviewRangeHours}`;
}[] = [
  { value: 24, labelKey: 'range.h24' },
  { value: 168, labelKey: 'range.h168' },
  { value: 360, labelKey: 'range.h360' },
  { value: 720, labelKey: 'range.h720' },
];

export const RATE_LIMIT_RANGE_OPTIONS: {
  value: RateLimitRangeHours;
  labelKey: 'range.h24' | 'range.h72';
}[] = [
  { value: 24, labelKey: 'range.h24' },
  { value: 72, labelKey: 'range.h72' },
];

export const DETAIL_SORT_OPTIONS = [
  { value: 'logged_at:desc', labelKey: 'sort.logged_at:desc' },
  { value: 'logged_at:asc', labelKey: 'sort.logged_at:asc' },
  { value: 'status_code:desc', labelKey: 'sort.status_code:desc' },
  { value: 'status_code:asc', labelKey: 'sort.status_code:asc' },
  { value: 'remote_addr:asc', labelKey: 'sort.remote_addr:asc' },
  { value: 'remote_addr:desc', labelKey: 'sort.remote_addr:desc' },
] as const;

export const IP_SORT_OPTIONS = [
  { value: 'total_requests:desc', labelKey: 'sort.total_requests:desc' },
  { value: 'total_requests:asc', labelKey: 'sort.total_requests:asc' },
  { value: 'request_length:desc', labelKey: 'sort.request_length:desc' },
  { value: 'request_length:asc', labelKey: 'sort.request_length:asc' },
  { value: 'bytes_sent:desc', labelKey: 'sort.bytes_sent:desc' },
  { value: 'bytes_sent:asc', labelKey: 'sort.bytes_sent:asc' },
  { value: 'success_ratio:desc', labelKey: 'sort.success_ratio:desc' },
  { value: 'success_ratio:asc', labelKey: 'sort.success_ratio:asc' },
  { value: 'last_seen_at:desc', labelKey: 'sort.last_seen_at:desc' },
  { value: 'last_seen_at:asc', labelKey: 'sort.last_seen_at:asc' },
] as const;

export function parseSortValue(value: string) {
  const [sortBy = 'logged_at', sortOrder = 'desc'] = value.split(':');
  return {
    sortBy,
    sortOrder: sortOrder === 'asc' ? ('asc' as const) : ('desc' as const),
  };
}

export function formatCompactNumber(value: number) {
  return new Intl.NumberFormat('zh-CN', {
    notation: value >= 10000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(value);
}

export function formatOverviewRangeHint(
  hours: number,
  t: (
    key: 'rangeHint.hours24' | 'rangeHint.days' | 'rangeHint.hours',
    values?: { count: number },
  ) => string,
) {
  if (hours <= 24) return t('rangeHint.hours24');
  if (hours % 24 === 0) return t('rangeHint.days', { count: hours / 24 });
  return t('rangeHint.hours', { count: hours });
}

export function formatOverviewTrendLabel(value: string, hours: number) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  const hour = `${date.getHours()}`.padStart(2, '0');
  if (hours <= 24) {
    return `${hour}:00`;
  }
  return `${month}/${day} ${hour}:00`;
}

export type CacheOutcome = 'hit' | 'origin' | 'uncached';

export function resolveCacheOutcome(
  cacheStatus: string | undefined | null,
): CacheOutcome {
  const status = (cacheStatus ?? '').trim().toUpperCase();
  if (
    status === 'HIT' ||
    status === 'STALE' ||
    status === 'REVALIDATED' ||
    status === 'UPDATING'
  ) {
    return 'hit';
  }
  if (status === 'MISS' || status === 'EXPIRED') {
    return 'origin';
  }
  return 'uncached';
}

export function cacheOutcomeLabel(
  outcome: CacheOutcome,
  t: (key: `cache.${CacheOutcome}`) => string,
) {
  return t(`cache.${outcome}`);
}
