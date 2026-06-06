import { apiRequest } from '@/lib/api/client';

import type {
  ApplyLogCleanupPayload,
  ApplyLogCleanupResult,
  ApplyLogList,
  ApplyLogListQuery,
} from '@/features/apply-logs/types';

export function getApplyLogs(query: ApplyLogListQuery = {}) {
  const params = new URLSearchParams();
  const normalizedNodeId = query.node_id?.trim();
  if (normalizedNodeId) {
    params.set('node_id', normalizedNodeId);
  }
  if (query.pageNo) {
    params.set('pageNo', String(query.pageNo));
  }
  if (query.pageSize) {
    params.set('pageSize', String(query.pageSize));
  }
  const suffix = params.size > 0 ? `?${params.toString()}` : '';
  return apiRequest<ApplyLogList>(`/apply-logs/${suffix}`);
}

export function cleanupApplyLogs(payload: ApplyLogCleanupPayload) {
  return apiRequest<ApplyLogCleanupResult>('/apply-logs/cleanup', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
