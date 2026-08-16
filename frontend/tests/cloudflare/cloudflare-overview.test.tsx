import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import CloudflarePage from '@/app/(main)/cloudflare/page';
import { NextIntlClientProvider } from 'next-intl';
import zhCN from '@/messages/zh-CN.json';
import { AdminTaskService } from '@/lib/services/admin';
import { CloudflareService, NodeService } from '@/lib/services/openflare';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    CloudflareService: {
      ...actual.CloudflareService,
      getOverview: vi.fn(),
      listGroups: vi.fn(),
      createGroup: vi.fn(),
      syncGroup: vi.fn(),
      deleteGroup: vi.fn(),
    },
    NodeService: { ...actual.NodeService, listNodes: vi.fn() },
  };
});

vi.mock('@/lib/services/admin', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services/admin')>();
  return {
    ...actual,
    AdminTaskService: {
      ...actual.AdminTaskService,
      listTaskExecutions: vi.fn(),
      getTaskExecution: vi.fn(),
      retryTaskExecution: vi.fn(),
    },
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
        <CloudflarePage />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

describe('Cloudflare overview', () => {
  beforeEach(() => {
    vi.mocked(CloudflareService.listGroups).mockResolvedValue([]);
    vi.mocked(NodeService.listNodes).mockResolvedValue([]);
    vi.mocked(AdminTaskService.listTaskExecutions).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 10,
    });
    vi.mocked(CloudflareService.getOverview).mockResolvedValue({
      connection: {
        configured: false,
        ready: false,
        source: '',
        dns_account_id: null,
        status: '',
        verified_at: null,
      },
      group_count: 0,
      member_count: 0,
      ok_count: 0,
      pending_count: 0,
      error_count: 0,
    });
  });

  it('shows the token readiness gate without the synchronization boundary card', async () => {
    renderPage();

    expect(await screen.findByText('Cloudflare 连接尚未就绪')).toBeVisible();
    expect(screen.queryByText('同步边界')).not.toBeInTheDocument();
  });

  it('renders pointing groups directly on the overview page', async () => {
    vi.mocked(CloudflareService.getOverview).mockResolvedValue({
      connection: {
        configured: true,
        ready: true,
        source: 'standalone',
        dns_account_id: null,
        status: 'ready',
        verified_at: null,
      },
      group_count: 1,
      member_count: 2,
      ok_count: 1,
      pending_count: 1,
      error_count: 0,
    });
    vi.mocked(CloudflareService.listGroups).mockResolvedValue([
      {
        id: 7,
        name: '生产节点',
        primary_node: { id: 1, name: '主节点', ip: '192.0.2.1' },
        backup_node: null,
        active_node: { id: 1, name: '主节点', ip: '192.0.2.1' },
        default_proxied: true,
        enabled: true,
        member_count: 2,
        created_at: '',
        updated_at: '',
      },
    ]);

    renderPage();

    expect(await screen.findByText('生产节点')).toBeVisible();
    expect(screen.getByRole('link', { name: '管理' })).toHaveAttribute(
      'href',
      '/cloudflare/groups/7',
    );
    expect(await screen.findByText('同步任务')).toBeVisible();
  });
});
