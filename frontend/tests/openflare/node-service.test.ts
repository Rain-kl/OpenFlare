import type { AxiosResponse } from 'axios';
import { beforeEach, expect, it, vi } from 'vitest';
import apiClient from '@/lib/services/core/api-client';
import { NodeService } from '@/lib/services/openflare/node.service';

vi.mock('@/lib/services/core/api-client', () => ({
  default: { get: vi.fn() },
}));

beforeEach(() => {
  vi.mocked(apiClient.get).mockReset();
});

it('keeps legacy arrays and requests bounded pages and individual details separately', async () => {
  vi.mocked(apiClient.get).mockResolvedValue({
    data: { error_msg: '', data: [] },
  } as AxiosResponse);
  await NodeService.listNodes();
  expect(apiClient.get).toHaveBeenLastCalledWith(
    '/api/v1/d/nodes/',
    expect.objectContaining({ params: undefined }),
  );
  await NodeService.listNodePage({
    page: 2,
    page_size: 20,
    status: 'online',
    node_type: 'tunnel_relay',
    search: 'edge',
  });
  expect(apiClient.get).toHaveBeenLastCalledWith(
    '/api/v1/d/nodes/page',
    expect.objectContaining({
      params: {
        page: 2,
        page_size: 20,
        status: 'online',
        node_type: 'tunnel_relay',
        search: 'edge',
      },
    }),
  );
  await NodeService.getNode(42);
  expect(apiClient.get).toHaveBeenLastCalledWith(
    '/api/v1/d/nodes/42',
    expect.objectContaining({ params: undefined }),
  );
});
