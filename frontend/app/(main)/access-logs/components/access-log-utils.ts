export type AccessLogTab = 'overview' | 'ips' | 'list';

export type SearchDraft = {
  nodeId: string;
  remoteAddr: string;
  host: string;
  path: string;
};

export type OverviewRangeHours = 24 | 168 | 360 | 720;

/** 限流分析等短窗口场景：仅 24 小时 / 7 天 */
export type RateLimitRangeHours = 24 | 168;

export const PAGE_SIZE_OPTIONS = [20, 50, 100, 200];

export const OVERVIEW_RANGE_OPTIONS: {
  value: OverviewRangeHours;
  label: string;
}[] = [
  { value: 24, label: '24 小时' },
  { value: 168, label: '7 天' },
  { value: 360, label: '15 天' },
  { value: 720, label: '30 天' },
];

export const RATE_LIMIT_RANGE_OPTIONS: {
  value: RateLimitRangeHours;
  label: string;
}[] = [
  { value: 24, label: '24 小时' },
  { value: 168, label: '7 天' },
];

export const DETAIL_SORT_OPTIONS = [
  { value: 'logged_at:desc', label: '时间从新到旧' },
  { value: 'logged_at:asc', label: '时间从旧到新' },
  { value: 'status_code:desc', label: '状态码从高到低' },
  { value: 'status_code:asc', label: '状态码从低到高' },
  { value: 'remote_addr:asc', label: 'IP 正序' },
  { value: 'remote_addr:desc', label: 'IP 倒序' },
];

export const IP_SORT_OPTIONS = [
  { value: 'total_requests:desc', label: '请求数从高到低' },
  { value: 'total_requests:asc', label: '请求数从低到高' },
  { value: 'request_length:desc', label: '入站从高到低' },
  { value: 'request_length:asc', label: '入站从低到高' },
  { value: 'bytes_sent:desc', label: '出站从高到低' },
  { value: 'bytes_sent:asc', label: '出站从低到高' },
  { value: 'success_ratio:desc', label: '2xx 比例从高到低' },
  { value: 'success_ratio:asc', label: '2xx 比例从低到高' },
  { value: 'last_seen_at:desc', label: '最后访问从新到旧' },
  { value: 'last_seen_at:asc', label: '最后访问从旧到新' },
];

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

export function formatOverviewRangeHint(hours: number) {
  if (hours <= 24) return '近 24 小时';
  if (hours % 24 === 0) return `近 ${hours / 24} 天`;
  return `近 ${hours} 小时`;
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

export function cacheOutcomeLabel(outcome: CacheOutcome) {
  switch (outcome) {
    case 'hit':
      return '命中';
    case 'origin':
      return '回源';
    default:
      return '未缓存';
  }
}
