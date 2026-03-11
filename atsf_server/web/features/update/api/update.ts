import { apiRequest } from '@/lib/api/client';

import type { LatestReleaseInfo } from '@/features/update/types';

export function getLatestRelease() {
  return apiRequest<LatestReleaseInfo>('/update/latest-release');
}

export function upgradeServer() {
  return apiRequest<LatestReleaseInfo>('/update/upgrade', {
    method: 'POST',
  });
}
