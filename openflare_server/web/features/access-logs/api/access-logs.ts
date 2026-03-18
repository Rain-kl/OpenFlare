import { apiRequest } from '@/lib/api/client';

import type { AccessLogList } from '@/features/access-logs/types';

export function getAccessLogs(page: number, nodeId?: string, pageSize = 20) {
  const normalizedNodeId = nodeId?.trim();
  const searchParams = new URLSearchParams({
    p: String(Math.max(page, 0)),
    page_size: String(pageSize),
  });
  if (normalizedNodeId) {
    searchParams.set('node_id', normalizedNodeId);
  }
  return apiRequest<AccessLogList>(`/access-logs/?${searchParams.toString()}`);
}
