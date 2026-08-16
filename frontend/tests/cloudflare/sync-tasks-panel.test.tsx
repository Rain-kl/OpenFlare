import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { SyncTasksPanel } from '@/app/(main)/cloudflare/components/sync-tasks-panel';
import { AdminTaskService } from '@/lib/services/admin';
import type { TaskExecution } from '@/lib/services/admin';
import { NextIntlClientProvider } from 'next-intl';
import zhCN from '@/messages/zh-CN.json';

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

function renderPanel() {
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
        <SyncTasksPanel />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

function execution(
  partial: Partial<TaskExecution> &
    Pick<TaskExecution, 'id' | 'task_type' | 'task_name'>,
): TaskExecution {
  return {
    task_id: `task_${partial.id}`,
    status: 'succeeded',
    retryable: true,
    max_retry: 2,
    retry_count: 0,
    log: '',
    error_message: '',
    result: 'ok',
    started_at: null,
    finished_at: null,
    duration: 100,
    payload: '{}',
    triggered_by: 'manual',
    created_at: '2026-08-04T10:00:00Z',
    updated_at: '2026-08-04T10:00:01Z',
    ...partial,
  };
}

describe('Cloudflare sync tasks panel', () => {
  beforeEach(() => {
    vi.mocked(AdminTaskService.listTaskExecutions).mockReset();
  });

  it('queries only sync_member and sync_group and hides other tasks', async () => {
    vi.mocked(AdminTaskService.listTaskExecutions).mockImplementation(
      async (request = {}) => {
        if (request.task_type === 'cloudflare:sync_member') {
          return {
            items: [
              execution({
                id: '1',
                task_type: 'cloudflare:sync_member',
                task_name: 'Cloudflare 域名同步',
                created_at: '2026-08-04T12:00:00Z',
              }),
            ],
            total: 1,
            page: 1,
            page_size: 50,
          };
        }
        if (request.task_type === 'cloudflare:sync_group') {
          return {
            items: [
              execution({
                id: '2',
                task_type: 'cloudflare:sync_group',
                task_name: 'Cloudflare 分组同步',
                created_at: '2026-08-04T11:00:00Z',
              }),
              // Defensive: even if API returns unrelated rows, panel must drop them.
              execution({
                id: '99',
                task_type: 'system:cleanup',
                task_name: '系统垃圾清理',
                triggered_by: 'schedule',
                created_at: '2026-08-04T13:00:00Z',
              }),
            ],
            total: 2,
            page: 1,
            page_size: 50,
          };
        }
        return { items: [], total: 0, page: 1, page_size: 50 };
      },
    );

    renderPanel();

    expect(await screen.findByText('Cloudflare 域名同步')).toBeVisible();
    expect(screen.getByText('Cloudflare 分组同步')).toBeVisible();
    expect(screen.queryByText('系统垃圾清理')).not.toBeInTheDocument();
    expect(screen.queryByText('system:cleanup')).not.toBeInTheDocument();

    await waitFor(() => {
      expect(AdminTaskService.listTaskExecutions).toHaveBeenCalledWith(
        expect.objectContaining({
          task_type: 'cloudflare:sync_member',
          page: 1,
          page_size: 50,
        }),
      );
      expect(AdminTaskService.listTaskExecutions).toHaveBeenCalledWith(
        expect.objectContaining({
          task_type: 'cloudflare:sync_group',
          page: 1,
          page_size: 50,
        }),
      );
    });
    expect(AdminTaskService.listTaskExecutions).toHaveBeenCalledTimes(2);
  });

  it('shows empty state when no cloudflare tasks exist', async () => {
    vi.mocked(AdminTaskService.listTaskExecutions).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
    });

    renderPanel();

    expect(await screen.findByText('暂无同步任务')).toBeVisible();
  });
});
