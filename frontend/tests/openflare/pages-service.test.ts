import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AxiosResponse } from 'axios';

import apiClient from '@/lib/services/core/api-client';
import { PagesService } from '@/lib/services/openflare/pages.service';

vi.mock('@/lib/services/core/api-client', () => ({
  default: {
    get: vi.fn(),
  },
}));

describe('PagesService', () => {
  beforeEach(() => {
    vi.mocked(apiClient.get).mockReset();
  });

  it('requests deployment files using the backend deployment route', async () => {
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { error_msg: '', data: [] },
      status: 200,
      statusText: 'OK',
      headers: {},
      config: { headers: {} },
    } as AxiosResponse);

    await PagesService.listDeploymentFiles(7);

    expect(apiClient.get).toHaveBeenCalledWith(
      '/api/v1/d/pages/deployments/7/files',
      expect.objectContaining({ params: undefined }),
    );
  });
});
