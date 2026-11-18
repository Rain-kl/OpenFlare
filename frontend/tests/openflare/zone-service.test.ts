import type { AxiosResponse } from 'axios';
import { beforeEach, expect, it, vi } from 'vitest';
import apiClient from '@/lib/services/core/api-client';
import { ZoneDomainService } from '@/lib/services/openflare/zone.service';

vi.mock('@/lib/services/core/api-client', () => ({
  default: { post: vi.fn() },
}));

beforeEach(() => {
  vi.mocked(apiClient.post).mockReset();
});

it('calls the cleanup-unlinked zone domains endpoint', async () => {
  vi.mocked(apiClient.post).mockResolvedValue({
    data: { error_msg: '', data: { deleted_count: 3 } },
  } as AxiosResponse);

  const result = await ZoneDomainService.cleanupUnlinked(7);
  expect(apiClient.post).toHaveBeenCalledWith(
    '/api/v1/d/zones/7/domains/cleanup-unlinked',
    undefined,
    undefined,
  );
  expect(result).toEqual({ deleted_count: 3 });
});
