import { apiRequest } from '@/lib/api/client';

import type {
  ConfigDiffResult,
  ConfigPreviewResult,
  ConfigVersionDetail,
  ConfigVersionSummary,
} from '@/features/config-versions/types';

export function getConfigVersions() {
  return apiRequest<ConfigVersionSummary[]>('/config-versions/');
}

export function getConfigVersion(id: number) {
  return apiRequest<ConfigVersionDetail>(`/config-versions/${id}`);
}

export function getActiveConfigVersion() {
  return apiRequest<ConfigVersionDetail>('/config-versions/active');
}

export function getConfigVersionPreview() {
  return apiRequest<ConfigPreviewResult>('/config-versions/preview');
}

export function getConfigVersionDiff() {
  return apiRequest<ConfigDiffResult>('/config-versions/diff');
}

export function publishConfigVersion(force?: boolean) {
  const query = force ? '?force=true' : '';
  return apiRequest<ConfigVersionDetail>(`/config-versions/publish${query}`, {
    method: 'POST',
  });
}

export function activateConfigVersion(id: number) {
  return apiRequest<ConfigVersionDetail>(`/config-versions/${id}/activate`, {
    method: 'POST',
  });
}

export function cleanupConfigVersions(payload: { keep_count: number }) {
  return apiRequest<{ deleted_count: number }>('/config-versions/cleanup', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
