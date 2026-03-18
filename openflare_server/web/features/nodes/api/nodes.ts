import { apiRequest } from '@/lib/api/client';

import type {
  NodeAgentReleaseInfo,
  NodeAgentUpdatePayload,
  NodeBootstrapToken,
  NodeItem,
  NodeMutationPayload,
  NodeObservability,
} from '@/features/nodes/types';
import type { ReleaseChannel } from '@/features/update/types';

export function getNodes() {
  return apiRequest<NodeItem[]>('/nodes/');
}

export function createNode(payload: NodeMutationPayload) {
  return apiRequest<NodeItem>('/nodes/', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateNode(id: number, payload: NodeMutationPayload) {
  return apiRequest<NodeItem>(`/nodes/${id}/update`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function deleteNode(id: number) {
  return apiRequest<void>(`/nodes/${id}/delete`, {
    method: 'POST',
  });
}

export function getNodeBootstrapToken() {
  return apiRequest<NodeBootstrapToken>('/nodes/bootstrap-token');
}

export function rotateNodeBootstrapToken() {
  return apiRequest<NodeBootstrapToken>('/nodes/bootstrap-token/rotate', {
    method: 'POST',
  });
}

export function getNodeAgentRelease(
  id: number,
  channel: ReleaseChannel = 'stable',
) {
  return apiRequest<NodeAgentReleaseInfo>(
    `/nodes/${id}/agent-release?channel=${channel}`,
  );
}

export function requestNodeAgentUpdate(
  id: number,
  payload?: NodeAgentUpdatePayload,
) {
  return apiRequest<NodeItem>(`/nodes/${id}/agent-update`, {
    method: 'POST',
    body: JSON.stringify(payload ?? {}),
  });
}

export function requestNodeOpenrestyRestart(id: number) {
  return apiRequest<NodeItem>(`/nodes/${id}/openresty-restart`, {
    method: 'POST',
  });
}

export function getNodeObservability(
  id: number,
  options?: { hours?: number; limit?: number },
) {
  const params = new URLSearchParams();
  if (options?.hours) {
    params.set('hours', String(options.hours));
  }
  if (options?.limit) {
    params.set('limit', String(options.limit));
  }
  const query = params.toString();
  return apiRequest<NodeObservability>(
    `/nodes/${id}/observability${query ? `?${query}` : ''}`,
  );
}

export function cleanupNodeHealthEvents(id: number) {
  return apiRequest<{ node_id: string; deleted_count: number }>(
    `/nodes/${id}/observability/cleanup`,
    {
      method: 'POST',
    },
  );
}
