import { apiRequest } from '@/lib/api/client';
import type {
  DnsAccountItem,
  DnsAccountMutationPayload,
} from '@/features/dns-accounts/types';

export function getDnsAccounts() {
  return apiRequest<DnsAccountItem[]>('/dns-accounts/');
}

export function createDnsAccount(payload: DnsAccountMutationPayload) {
  return apiRequest<DnsAccountItem>('/dns-accounts/', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateDnsAccount(id: number, payload: DnsAccountMutationPayload) {
  return apiRequest<DnsAccountItem>(`/dns-accounts/${id}/update`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function deleteDnsAccount(id: number) {
  return apiRequest<void>(`/dns-accounts/${id}/delete`, {
    method: 'POST',
  });
}
