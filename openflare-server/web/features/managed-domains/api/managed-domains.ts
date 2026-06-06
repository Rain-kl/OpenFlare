import {apiRequest} from '@/lib/api/client';

import type {
    ManagedDomainCertificateOption,
    ManagedDomainItem,
    ManagedDomainMutationPayload,
} from '@/features/managed-domains/types';

export function getManagedDomains() {
  return apiRequest<ManagedDomainItem[]>('/managed-domains/');
}

export function createManagedDomain(payload: ManagedDomainMutationPayload) {
  return apiRequest<ManagedDomainItem>('/managed-domains/', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateManagedDomain(
  id: number,
  payload: ManagedDomainMutationPayload,
) {
  return apiRequest<ManagedDomainItem>(`/managed-domains/${id}/update`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function deleteManagedDomain(id: number) {
  return apiRequest<void>(`/managed-domains/${id}/delete`, {
    method: 'POST',
  });
}

export function getManagedDomainCertificates() {
  return apiRequest<ManagedDomainCertificateOption[]>('/tls-certificates/');
}
