import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import axe from 'axe-core';
import { NextIntlClientProvider } from 'next-intl';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PolicyEditorPageClient } from '@/app/(main)/performance/editor/page-client';
import PerformancePage from '@/app/(main)/performance/page';
import {
  PerformanceService,
  type PerformancePolicies,
  type PerformancePolicy,
} from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

const { pushMock, searchParamsMock } = vi.hoisted(() => ({
  pushMock: vi.fn(),
  searchParamsMock: { current: new URLSearchParams('') },
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  useSearchParams: () => searchParamsMock.current,
}));

vi.mock('next/link', () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    PerformanceService: {
      list: vi.fn(),
      save: vi.fn(),
      assign: vi.fn(),
      remove: vi.fn(),
    },
  };
});

const policyFixture: PerformancePolicy = {
  id: 2,
  name: '2C2G',
  is_default: false,
  settings: {
    threads: 2,
    threads_auto: false,
    work_stealing: true,
    upstream_keepalive_pool_size: 128,
    cache_enabled: true,
    cache_capacity_mib: 64,
  },
};

const listFixture: PerformancePolicies = {
  policies: [policyFixture],
  nodes: [
    { node_id: 'edge-1', name: 'Edge 1', status: 'online', policy_id: 2 },
  ],
  parameters: [
    { key: 'threads', group: 'runtime', default: 1, min: 1, max: 256 },
    { key: 'threads_auto', group: 'runtime', default: false },
    { key: 'work_stealing', group: 'runtime', default: true },
    {
      key: 'upstream_keepalive_pool_size',
      group: 'runtime',
      default: 128,
      min: 1,
      max: 65536,
    },
    { key: 'cache_enabled', group: 'cache', default: true },
    {
      key: 'cache_capacity_mib',
      group: 'cache',
      default: 128,
      min: 16,
      max: 4096,
    },
  ],
};

function renderWithProviders(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <NextIntlClientProvider
      locale='zh-CN'
      messages={zhCN}
      timeZone='Asia/Shanghai'
    >
      <QueryClientProvider client={client}>
        <main>{ui}</main>
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

describe('Performance policy editor secondary page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParamsMock.current = new URLSearchParams('');
    vi.mocked(PerformanceService.list).mockResolvedValue(listFixture);
    vi.mocked(PerformanceService.save).mockResolvedValue(policyFixture);
  });

  it('links create and edit actions on the list page to /performance/editor', async () => {
    renderWithProviders(<PerformancePage />);

    const createLink = await screen.findByRole('link', { name: '新建策略' });
    expect(createLink).toHaveAttribute('href', '/performance/editor');

    const editLink = await screen.findByRole('link', { name: '编辑策略' });
    expect(editLink).toHaveAttribute('href', '/performance/editor?id=2');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('lists the default first and counts nodes that inherit it', async () => {
    const defaultPolicy: PerformancePolicy = {
      id: 4,
      name: 'Pingora',
      is_default: true,
      settings: { ...policyFixture.settings, threads_auto: true },
    };
    vi.mocked(PerformanceService.list).mockResolvedValue({
      ...listFixture,
      policies: [policyFixture, defaultPolicy],
      nodes: [
        ...listFixture.nodes,
        {
          node_id: 'edge-2',
          name: 'Edge 2',
          status: 'online',
          policy_id: null,
        },
      ],
    });
    renderWithProviders(<PerformancePage />);

    const defaultBadge = await screen.findByText('默认策略');
    const card = defaultBadge.closest<HTMLElement>('[data-slot="card"]');
    expect(card).not.toBeNull();
    if (!card) throw new Error('Default strategy card was not rendered');
    expect(within(card).getByText('1 个节点使用此策略')).toBeInTheDocument();
    expect(
      within(card).getByText('按节点资源自动配置线程与回源连接池'),
    ).toBeInTheDocument();
    expect(
      within(card).queryByRole('button', { name: '分配节点' }),
    ).not.toBeInTheDocument();
    expect(
      within(card).queryByRole('button', { name: '删除策略' }),
    ).not.toBeInTheDocument();
    expect(
      screen.getAllByRole('link', { name: '编辑策略' })[0],
    ).toHaveAttribute('href', '/performance/editor?id=4');
  });

  it('renders switches inside horizontal fields without a dialog or inner scrollbar', async () => {
    const user = userEvent.setup();
    renderWithProviders(<PolicyEditorPageClient />);

    const nameInput = await screen.findByLabelText('策略名称');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: '返回性能策略' })).toHaveAttribute(
      'href',
      '/performance',
    );

    const switches = screen.getAllByRole('switch');
    expect(switches.length).toBe(2);
    for (const sw of switches) {
      const field = sw.closest('[data-slot="field"]');
      expect(field).toHaveAttribute('data-orientation', 'horizontal');
    }

    const panel = screen.getByRole('tabpanel');
    expect(panel).not.toHaveClass('overflow-y-auto');

    const threadsInput = screen.getByLabelText('工作线程数');
    const poolInput = screen.getByLabelText('空闲回源连接池容量');
    expect(threadsInput).not.toBeDisabled();
    expect(poolInput).not.toBeDisabled();
    await user.clear(poolInput);
    await user.type(poolInput, '0');
    await user.click(screen.getByRole('switch', { name: '自动配置' }));
    expect(threadsInput).toBeDisabled();
    expect(poolInput).toBeDisabled();

    await user.type(nameInput, '4C8G');
    await user.click(screen.getByRole('button', { name: '保存策略' }));

    await waitFor(() =>
      expect(PerformanceService.save).toHaveBeenCalledWith(undefined, {
        name: '4C8G',
        settings: {
          threads: 1,
          threads_auto: true,
          work_stealing: true,
          upstream_keepalive_pool_size: 128,
          cache_enabled: true,
          cache_capacity_mib: 128,
        },
      }),
    );
    expect(pushMock).toHaveBeenCalledWith('/performance');
  });

  it('lets operators edit disk cache parameters in the cache tab', async () => {
    const user = userEvent.setup();
    const diskFields = [
      {
        key: 'cache_disk_capacity_mib',
        group: 'cache',
        default: 0,
        min: 0,
        max: 1048576,
      },
      {
        key: 'cache_disk_inactive_seconds',
        group: 'cache',
        default: 172800,
        min: 60,
        max: 2592000,
      },
      {
        key: 'cache_disk_reading_max',
        group: 'cache',
        default: 32,
        min: 1,
        max: 1024,
      },
      {
        key: 'cache_disk_writing_max',
        group: 'cache',
        default: 16,
        min: 1,
        max: 256,
      },
    ];
    vi.mocked(PerformanceService.list).mockResolvedValue({
      ...listFixture,
      parameters: [...listFixture.parameters, ...diskFields],
    });
    searchParamsMock.current = new URLSearchParams('id=2');
    renderWithProviders(<PolicyEditorPageClient />);

    await user.click(await screen.findByRole('tab', { name: '缓存' }));
    expect(screen.getByLabelText('磁盘缓存清理时间')).toHaveValue(172800);
    expect(screen.getByLabelText('磁盘并发读取上限')).toHaveValue(32);
    expect(screen.getByLabelText('磁盘并发写入上限')).toHaveValue(16);
    const capacity = screen.getByLabelText('磁盘缓存容量');
    await user.clear(capacity);
    await user.type(capacity, '512');
    await user.click(screen.getByRole('button', { name: '保存策略' }));

    await waitFor(() =>
      expect(PerformanceService.save).toHaveBeenCalledWith(
        2,
        expect.objectContaining({
          settings: expect.objectContaining({
            cache_disk_capacity_mib: 512,
            cache_disk_inactive_seconds: 172800,
            cache_disk_reading_max: 32,
            cache_disk_writing_max: 16,
          }),
        }),
      ),
    );
  });

  it('switches to the invalid group tab and focuses invalid fields on submit', async () => {
    const user = userEvent.setup();
    searchParamsMock.current = new URLSearchParams('id=2');
    renderWithProviders(<PolicyEditorPageClient />);

    await screen.findByDisplayValue('2C2G');
    await user.click(screen.getByRole('tab', { name: '缓存' }));
    const capacityInput = screen.getByLabelText('内存缓存容量');
    await user.clear(capacityInput);
    await user.type(capacityInput, '8');

    await user.click(screen.getByRole('tab', { name: '运行时' }));
    await user.click(screen.getByRole('button', { name: '保存策略' }));

    expect(PerformanceService.save).not.toHaveBeenCalled();
    const invalidCapacity = await screen.findByLabelText('内存缓存容量');
    expect(invalidCapacity).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByText('允许范围为 16 至 4096。')).toBeInTheDocument();
  });

  it('updates an existing policy when id query parameter is provided', async () => {
    const user = userEvent.setup();
    searchParamsMock.current = new URLSearchParams('id=2');
    renderWithProviders(<PolicyEditorPageClient />);

    const nameInput = await screen.findByDisplayValue('2C2G');
    await user.clear(nameInput);
    await user.type(nameInput, '2C4G');
    await user.click(screen.getByRole('button', { name: '保存策略' }));

    await waitFor(() =>
      expect(PerformanceService.save).toHaveBeenCalledWith(
        2,
        expect.objectContaining({ name: '2C4G' }),
      ),
    );
    expect(pushMock).toHaveBeenCalledWith('/performance');
  });

  it('handles invalid and missing policy ids gracefully', async () => {
    searchParamsMock.current = new URLSearchParams('id=invalid');
    const { unmount } = renderWithProviders(<PolicyEditorPageClient />);
    expect(await screen.findByText('无效的性能策略 ID。')).toBeInTheDocument();
    expect(PerformanceService.list).not.toHaveBeenCalled();
    unmount();

    searchParamsMock.current = new URLSearchParams('id=999');
    renderWithProviders(<PolicyEditorPageClient />);
    expect(await screen.findByText('未找到该性能策略。')).toBeInTheDocument();
  });

  it('has no axe-core accessibility violations', async () => {
    renderWithProviders(<PolicyEditorPageClient />);
    await screen.findByLabelText('策略名称');
    const results = await axe.run(document.body, {
      rules: { 'color-contrast': { enabled: false } },
    });
    expect(results.violations).toEqual([]);
  });
});
