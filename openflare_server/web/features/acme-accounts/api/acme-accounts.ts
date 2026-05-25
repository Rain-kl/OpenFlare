import { apiRequest } from '@/lib/api/client';
import type { AcmeAccountItem } from '@/features/acme-accounts/types';

export function getDefaultAcmeAccount() {
  return apiRequest<AcmeAccountItem>('/acme-accounts/default');
}
