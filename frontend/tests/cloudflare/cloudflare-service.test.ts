import type { AxiosResponse } from 'axios';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import apiClient from '@/lib/services/core/api-client';
import { CloudflareService } from '@/lib/services/openflare/cloudflare.service';

vi.mock('@/lib/services/core/api-client', () => ({
  default: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
}));

function response<T>(data: T) {
  return {
    data: { error_msg: '', data },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: { headers: {} },
  } as AxiosResponse;
}

describe('CloudflareService', () => {
  beforeEach(() => {
    vi.mocked(apiClient.get).mockReset();
    vi.mocked(apiClient.post).mockReset();
    vi.mocked(apiClient.put).mockReset();
  });

  it('uses the Cloudflare management API paths', async () => {
    vi.mocked(apiClient.get).mockResolvedValue(response([]));
    vi.mocked(apiClient.post).mockResolvedValue(
      response({ task_id: 'task-1' }),
    );
    vi.mocked(apiClient.put).mockResolvedValue(response({ configured: true }));

    await CloudflareService.saveConnection({
      source: 'standalone',
      dns_account_id: 0,
      api_token: 'secret',
    });
    await CloudflareService.listGroups();
    await CloudflareService.syncMember(7, 9);

    expect(apiClient.put).toHaveBeenCalledWith(
      '/api/v1/d/cloudflare/connection',
      expect.objectContaining({ source: 'standalone' }),
      undefined,
    );
    expect(apiClient.get).toHaveBeenCalledWith(
      '/api/v1/d/cloudflare/groups',
      expect.objectContaining({ params: undefined }),
    );
    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/cloudflare/groups/7/members/9/sync',
      undefined,
      undefined,
    );
  });
});
