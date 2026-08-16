import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CloudflareGroupDetailPageClient } from '@/app/(main)/cloudflare/groups/[id]/page-client';
import { CloudflareService, NodeService } from '@/lib/services/openflare';
import { NextIntlClientProvider } from 'next-intl';
import zhCN from '@/messages/zh-CN.json';

vi.mock('next/navigation', () => ({
  useParams: () => ({ id: '7' }),
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    CloudflareService: {
      ...actual.CloudflareService,
      getGroup: vi.fn(),
      listAvailableDomains: vi.fn(),
      updateGroup: vi.fn(),
      createMember: vi.fn(),
      updateMember: vi.fn(),
      syncMember: vi.fn(),
      removeMember: vi.fn(),
    },
    NodeService: { ...actual.NodeService, listNodes: vi.fn() },
  };
});

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  render(
    <NextIntlClientProvider
      locale='zh-CN'
      messages={zhCN}
      timeZone='Asia/Shanghai'
    >
      <QueryClientProvider client={client}>
        <CloudflareGroupDetailPageClient />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

describe('Cloudflare group detail refresh', () => {
  beforeEach(() => {
    vi.mocked(CloudflareService.getGroup).mockReset();
    vi.mocked(CloudflareService.listAvailableDomains).mockReset();
    vi.mocked(NodeService.listNodes).mockReset();
    vi.mocked(CloudflareService.getGroup).mockResolvedValue({
      group: {
        id: 7,
        name: '生产节点',
        primary_node: { id: 1, name: '主节点', ip: '192.0.2.1' },
        backup_node: null,
        active_node: { id: 1, name: '主节点', ip: '192.0.2.1' },
        default_proxied: true,
        enabled: true,
        member_count: 0,
        created_at: '',
        updated_at: '',
      },
      members: [],
    });
    vi.mocked(CloudflareService.listAvailableDomains).mockResolvedValue([]);
    vi.mocked(NodeService.listNodes).mockResolvedValue([]);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('refreshes detail data when the refresh button is clicked', async () => {
    renderPage();

    expect(
      await screen.findByRole('heading', { name: '生产节点' }),
    ).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));

    await waitFor(() => {
      expect(CloudflareService.getGroup).toHaveBeenCalledTimes(2);
    });
  });

  it('automatically refreshes detail data every five seconds', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderPage();

    expect(
      await screen.findByRole('heading', { name: '生产节点' }),
    ).toBeVisible();
    expect(CloudflareService.getGroup).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });

    await waitFor(() => {
      expect(CloudflareService.getGroup).toHaveBeenCalledTimes(2);
    });
  });
});
