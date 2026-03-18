import { apiRequest } from '@/lib/api/client';

import type {
  AccessLogCleanupPayload,
  AccessLogCleanupResult,
  AccessLogFilters,
  AccessLogIPSummaryFilters,
  AccessLogIPSummaryList,
  AccessLogIPTrend,
  AccessLogIPTrendFilters,
  AccessLogList,
  FoldedAccessLogFilters,
  FoldedAccessLogList,
} from '@/features/access-logs/types';

function buildSearchParams(filters: object) {
  const searchParams = new URLSearchParams();
  Object.entries(filters as Record<string, string | number | undefined>).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') {
      return;
    }
    searchParams.set(key, String(value));
  });
  return searchParams.toString();
}

export function getAccessLogs(filters: AccessLogFilters) {
  const query = buildSearchParams(filters);
  return apiRequest<AccessLogList>(`/access-logs/${query ? `?${query}` : ''}`);
}

export function getFoldedAccessLogs(filters: FoldedAccessLogFilters) {
  const query = buildSearchParams(filters);
  return apiRequest<FoldedAccessLogList>(`/access-logs/folds${query ? `?${query}` : ''}`);
}

export function getAccessLogIPSummaries(filters: AccessLogIPSummaryFilters) {
  const query = buildSearchParams(filters);
  return apiRequest<AccessLogIPSummaryList>(
    `/access-logs/ip-summary${query ? `?${query}` : ''}`,
  );
}

export function getAccessLogIPTrend(filters: AccessLogIPTrendFilters) {
  const query = buildSearchParams(filters);
  return apiRequest<AccessLogIPTrend>(
    `/access-logs/ip-summary/trend${query ? `?${query}` : ''}`,
  );
}

export function cleanupAccessLogs(payload: AccessLogCleanupPayload) {
  return apiRequest<AccessLogCleanupResult>('/access-logs/cleanup', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
