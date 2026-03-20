import { apiRequest } from '@/lib/api/client';

import type {
  OriginDetail,
  OriginItem,
  OriginMutationPayload,
} from '@/features/origins/types';

export function getOrigins() {
  return apiRequest<OriginItem[]>('/origins/');
}

export function getOrigin(id: number) {
  return apiRequest<OriginDetail>(`/origins/${id}`);
}

export function createOrigin(payload: OriginMutationPayload) {
  return apiRequest<OriginItem>('/origins/', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateOrigin(id: number, payload: OriginMutationPayload) {
  return apiRequest<OriginItem>(`/origins/${id}/update`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function deleteOrigin(id: number) {
  return apiRequest<void>(`/origins/${id}/delete`, {
    method: 'POST',
  });
}
