export type AccessLogTab = 'overview' | 'list';

export type SearchDraft = {
  nodeId: string;
  remoteAddr: string;
  host: string;
  path: string;
};

export type OverviewRangeHours = 24 | 168 | 360 | 720;

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

export const DETAIL_SORT_OPTIONS = [
  { value: 'logged_at:desc', label: '时间从新到旧' },
  { value: 'logged_at:asc', label: '时间从旧到新' },
  { value: 'status_code:desc', label: '状态码从高到低' },
  { value: 'status_code:asc', label: '状态码从低到高' },
  { value: 'remote_addr:asc', label: 'IP 正序' },
  { value: 'remote_addr:desc', label: 'IP 倒序' },
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
