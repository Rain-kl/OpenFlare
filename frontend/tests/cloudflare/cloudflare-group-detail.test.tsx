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

let mockGroupId = '7';
let mockParamId = '7';

vi.mock('next/link', () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock('next/navigation', () => ({
  useParams: () => ({ id: mockParamId }),
  usePathname: () => `/cloudflare/groups/${mockGroupId}`,
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    CloudflareService: {
      ...actual.CloudflareService,
      getGroup: vi.fn(),
      listGroups: vi.fn(),
      listAvailableDomains: vi.fn(),
      updateGroup: vi.fn(),
      createMember: vi.fn(),
      updateMember: vi.fn(),
      syncMember: vi.fn(),
      removeMember: vi.fn(),
      moveMember: vi.fn(),
      batchMoveMembers: vi.fn(),
      batchRemoveMembers: vi.fn(),
    },
    NodeService: { ...actual.NodeService, listNodes: vi.fn() },
  };
});

const mockGroup = {
  id: 7,
  name: '生产节点',
  target_mode: 'node' as const,
  record_type: 'A' as const,
  record_content: '',
  primary_node: { id: 1, name: '主节点', ip: '192.0.2.1' },
  backup_node: null,
  active_node: { id: 1, name: '主节点', ip: '192.0.2.1' },
  default_proxied: true,
  enabled: true,
  member_count: 2,
  created_at: '',
  updated_at: '',
};

const mockTargetGroup = {
  id: 8,
  name: '备用分组',
  target_mode: 'node' as const,
  record_type: 'A' as const,
  record_content: '',
  primary_node: { id: 2, name: '备用节点', ip: '192.0.2.2' },
  backup_node: null,
  active_node: { id: 2, name: '备用节点', ip: '192.0.2.2' },
  default_proxied: false,
  enabled: true,
  member_count: 0,
  created_at: '',
  updated_at: '',
};

const mockMembers = [
  {
    id: 101,
    group_id: 7,
    zone_domain_id: 1,
    domain: 'a.example.com',
    zone_id: 10,
    proxied: true,
    desired_ip: '192.0.2.1',
    sync_status: 'ok' as const,
    last_error: '',
    synced_at: null,
  },
  {
    id: 102,
    group_id: 7,
    zone_domain_id: 2,
    domain: 'b.example.com',
    zone_id: 10,
    proxied: false,
    desired_ip: '192.0.2.1',
    sync_status: 'ok' as const,
    last_error: '',
    synced_at: null,
  },
];

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
    mockGroupId = '7';
    mockParamId = '7';
    vi.mocked(CloudflareService.getGroup).mockReset();
    vi.mocked(CloudflareService.listGroups).mockReset();
    vi.mocked(CloudflareService.listAvailableDomains).mockReset();
    vi.mocked(CloudflareService.moveMember).mockReset();
    vi.mocked(CloudflareService.batchMoveMembers).mockReset();
    vi.mocked(CloudflareService.batchRemoveMembers).mockReset();
    vi.mocked(NodeService.listNodes).mockReset();

    window.HTMLElement.prototype.hasPointerCapture = vi.fn(() => false);
    window.HTMLElement.prototype.setPointerCapture = vi.fn();
    window.HTMLElement.prototype.releasePointerCapture = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = vi.fn();

    vi.mocked(CloudflareService.getGroup).mockResolvedValue({
      group: mockGroup,
      members: [],
    });
    vi.mocked(CloudflareService.listGroups).mockResolvedValue([
      mockGroup,
      mockTargetGroup,
    ]);
    vi.mocked(CloudflareService.listAvailableDomains).mockResolvedValue([]);
    vi.mocked(NodeService.listNodes).mockResolvedValue([]);
    vi.mocked(CloudflareService.moveMember).mockResolvedValue({
      ...mockMembers[0],
      group_id: 8,
    });
    vi.mocked(CloudflareService.batchMoveMembers).mockResolvedValue(undefined);
    vi.mocked(CloudflareService.batchRemoveMembers).mockResolvedValue(
      undefined,
    );
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

  it('uses the browser pathname ID when serving a static-export fallback shell', async () => {
    mockParamId = '1';
    renderPage();

    await waitFor(() => {
      expect(CloudflareService.getGroup).toHaveBeenCalledWith(7);
    });
    expect(CloudflareService.getGroup).not.toHaveBeenCalledWith(1);
    expect(
      await screen.findByRole('heading', { name: '生产节点' }),
    ).toBeVisible();
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

describe('Cloudflare group detail move and batch operations', () => {
  beforeEach(() => {
    mockGroupId = '7';
    mockParamId = '7';
    vi.mocked(CloudflareService.getGroup).mockReset();
    vi.mocked(CloudflareService.listGroups).mockReset();
    vi.mocked(CloudflareService.listAvailableDomains).mockReset();
    vi.mocked(CloudflareService.moveMember).mockReset();
    vi.mocked(CloudflareService.batchMoveMembers).mockReset();
    vi.mocked(CloudflareService.batchRemoveMembers).mockReset();
    vi.mocked(NodeService.listNodes).mockReset();

    window.HTMLElement.prototype.hasPointerCapture = vi.fn(() => false);
    window.HTMLElement.prototype.setPointerCapture = vi.fn();
    window.HTMLElement.prototype.releasePointerCapture = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = vi.fn();

    vi.mocked(CloudflareService.getGroup).mockResolvedValue({
      group: mockGroup,
      members: mockMembers,
    });
    vi.mocked(CloudflareService.listGroups).mockResolvedValue([
      mockGroup,
      mockTargetGroup,
    ]);
    vi.mocked(CloudflareService.listAvailableDomains).mockResolvedValue([]);
    vi.mocked(NodeService.listNodes).mockResolvedValue([]);
    vi.mocked(CloudflareService.moveMember).mockResolvedValue({
      ...mockMembers[0],
      group_id: 8,
    });
    vi.mocked(CloudflareService.batchMoveMembers).mockResolvedValue(undefined);
    vi.mocked(CloudflareService.batchRemoveMembers).mockResolvedValue(
      undefined,
    );
  });

  it('supports selecting all members and toggling selection', async () => {
    renderPage();

    expect(await screen.findByText('a.example.com')).toBeVisible();
    expect(screen.getByText('b.example.com')).toBeVisible();

    // Initially batch toolbar is not visible
    expect(screen.queryByText(/已选择/)).not.toBeInTheDocument();

    // Select all via header checkbox
    const selectAllCheckbox = screen.getByRole('checkbox', {
      name: 'Select all',
    });
    fireEvent.click(selectAllCheckbox);

    // Batch toolbar should appear with count 2
    expect(await screen.findByText('已选择 2 项')).toBeVisible();
    expect(screen.getByRole('button', { name: '批量移动' })).toBeVisible();
    expect(screen.getByRole('button', { name: '批量删除' })).toBeVisible();

    // Clear selection
    fireEvent.click(screen.getByRole('button', { name: '取消选择' }));
    expect(screen.queryByText(/已选择/)).not.toBeInTheDocument();
  });

  it('opens single move dialog and triggers moveMember', async () => {
    renderPage();

    expect(await screen.findByText('a.example.com')).toBeVisible();

    // Click single move button on first row
    const moveButtons = screen.getAllByRole('button', { name: '移动' });
    fireEvent.click(moveButtons[0]);

    // Move dialog should show single member title
    expect(
      await screen.findByRole('heading', {
        name: '移动域名「a.example.com」',
      }),
    ).toBeVisible();

    // Open target group select
    const selectTrigger = screen.getByRole('combobox');
    fireEvent.keyDown(selectTrigger, { key: 'ArrowDown' });

    // Select target group
    const targetOption = await screen.findByRole('option', {
      name: /备用分组/,
    });
    fireEvent.click(targetOption);

    // Submit move
    fireEvent.click(screen.getByRole('button', { name: '确认移动' }));

    await waitFor(() => {
      expect(CloudflareService.moveMember).toHaveBeenCalledWith(7, 101, 8);
    });
  });

  it('opens batch move dialog and triggers batchMoveMembers', async () => {
    renderPage();

    expect(await screen.findByText('a.example.com')).toBeVisible();

    // Select all members
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect(await screen.findByText('已选择 2 项')).toBeVisible();

    // Click batch move button
    fireEvent.click(screen.getByRole('button', { name: '批量移动' }));

    // Dialog title should show batch title
    expect(
      await screen.findByRole('heading', {
        name: '批量移动域名（共 2 项）',
      }),
    ).toBeVisible();

    // Open target group select
    const selectTrigger = screen.getByRole('combobox');
    fireEvent.keyDown(selectTrigger, { key: 'ArrowDown' });

    // Select target group
    const targetOption = await screen.findByRole('option', {
      name: /备用分组/,
    });
    fireEvent.click(targetOption);

    // Submit move
    fireEvent.click(screen.getByRole('button', { name: '确认移动' }));

    await waitFor(() => {
      expect(CloudflareService.batchMoveMembers).toHaveBeenCalledWith(
        7,
        [101, 102],
        8,
      );
    });
  });

  it('opens batch remove dialog and triggers batchRemoveMembers', async () => {
    renderPage();

    expect(await screen.findByText('a.example.com')).toBeVisible();

    // Select first member only
    fireEvent.click(
      screen.getByRole('checkbox', { name: 'Select a.example.com' }),
    );
    expect(await screen.findByText('已选择 1 项')).toBeVisible();

    // Click batch remove
    fireEvent.click(screen.getByRole('button', { name: '批量删除' }));

    // Confirmation dialog appears
    expect(
      await screen.findByRole('heading', { name: '确认批量删除域名' }),
    ).toBeVisible();
    expect(
      screen.getByText(/确认从当前分组移出已选中的 1 个域名吗？/),
    ).toBeVisible();

    // Confirm deletion
    fireEvent.click(screen.getByRole('button', { name: '确认删除' }));

    await waitFor(() => {
      expect(CloudflareService.batchRemoveMembers).toHaveBeenCalledWith(
        7,
        [101],
      );
    });
  });
});
