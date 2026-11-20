import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
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
    expect(threadsInput).not.toBeDisabled();
    await user.click(screen.getByRole('switch', { name: '自动配置线程数' }));
    expect(threadsInput).toBeDisabled();

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

  it('switches to the invalid group tab and focuses invalid fields on submit', async () => {
    const user = userEvent.setup();
    searchParamsMock.current = new URLSearchParams('id=2');
    renderWithProviders(<PolicyEditorPageClient />);

    await screen.findByDisplayValue('2C2G');
    await user.click(screen.getByRole('tab', { name: '缓存' }));
    const capacityInput = screen.getByLabelText('缓存容量');
    await user.clear(capacityInput);
    await user.type(capacityInput, '8');

    await user.click(screen.getByRole('tab', { name: '运行时' }));
    await user.click(screen.getByRole('button', { name: '保存策略' }));

    expect(PerformanceService.save).not.toHaveBeenCalled();
    const invalidCapacity = await screen.findByLabelText('缓存容量');
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
