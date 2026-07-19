import { act } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { DeploymentHistory } from '@/app/(main)/pages/detail/components/deployment-history';
import {
  getLatestSourceIdlePollingDecision,
  PagesSourceCard,
} from '@/app/(main)/pages/detail/components/pages-source-card';
import {
  deploymentsQueryKey,
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '@/app/(main)/pages/components/pages-utils';
import {
  type PagesDeployment,
  type PagesGitHubReleaseSource,
  PagesService,
} from '@/lib/services/openflare';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    PagesService: {
      getSource: vi.fn(),
      updateSource: vi.fn(),
      deleteSource: vi.fn(),
      checkSource: vi.fn(),
      syncSource: vi.fn(),
      listDeployments: vi.fn(),
      listDeploymentFiles: vi.fn(),
      activateDeployment: vi.fn(),
      deleteDeployment: vi.fn(),
    },
  };
});

function renderWithQuery(ui: React.ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  const rendered = render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
  return {
    ...rendered,
    queryClient,
    rerenderWithQuery: (nextUI: React.ReactNode) =>
      rendered.rerender(
        <QueryClientProvider client={queryClient}>
          {nextUI}
        </QueryClientProvider>,
      ),
  };
}

const latestSource: PagesGitHubReleaseSource = {
  source_type: 'github_release',
  github_repository: 'openflare/site',
  release_selector: 'latest',
  asset_name: 'dist.zip',
  auto_update_enabled: true,
  check_interval_minutes: 120,
  sync_status: 'idle',
  update_available: false,
  last_seen: {
    revision: 'b'.repeat(64),
    label: 'v1.2.3',
    asset_name: 'dist.zip',
  },
  last_applied: {
    revision: 'a'.repeat(64),
    label: 'v1.2.2',
    asset_name: 'dist.zip',
  },
  last_checked_at: '2026-07-19T10:00:00Z',
  last_synced_at: '2026-07-19T09:00:00Z',
  next_check_at: '2026-07-19T12:00:00Z',
  last_error: '',
};

describe('Pages latest source automatic updates', () => {
  beforeEach(() => {
    vi.mocked(PagesService.getSource).mockReset();
    vi.mocked(PagesService.updateSource).mockReset();
    vi.mocked(PagesService.deleteSource).mockReset();
    vi.mocked(PagesService.checkSource).mockReset();
    vi.mocked(PagesService.syncSource).mockReset();
    vi.mocked(PagesService.listDeployments).mockReset();
    vi.mocked(PagesService.listDeploymentFiles).mockReset();
    vi.mocked(PagesService.activateDeployment).mockReset();
    vi.mocked(PagesService.deleteDeployment).mockReset();
  });

  it('backfills latest settings and hides them after selecting a fixed tag', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(latestSource);

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '编辑来源' }));
    expect(screen.getByRole('switch', { name: '自动更新' })).toBeChecked();
    expect(screen.getByLabelText('检查间隔（分钟）')).toHaveValue(120);

    await user.click(screen.getByRole('radio', { name: '固定 Tag' }));
    expect(
      screen.queryByRole('switch', { name: '自动更新' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText('检查间隔（分钟）')).not.toBeInTheDocument();
  });

  it('does not show automatic schedule details for a fixed tag source', async () => {
    vi.mocked(PagesService.getSource).mockResolvedValue({
      ...latestSource,
      release_selector: 'tag',
      release_tag: 'v1.2.3',
      auto_update_enabled: false,
      check_interval_minutes: 0,
      next_check_at: null,
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    expect(await screen.findByText('固定 Tag · v1.2.3')).toBeVisible();
    expect(screen.queryByText('自动更新')).not.toBeInTheDocument();
    expect(screen.queryByText('检查间隔')).not.toBeInTheDocument();
    expect(screen.queryByText('下次检查时间')).not.toBeInTheDocument();
  });

  it('rejects a latest interval outside 5–1440 minutes', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(latestSource);

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '编辑来源' }));
    const intervalInput = screen.getByLabelText('检查间隔（分钟）');
    await user.clear(intervalInput);
    await user.type(intervalInput, '4');
    await user.click(screen.getByRole('button', { name: '保存 GitHub 来源' }));

    expect(screen.getByRole('alert')).toHaveTextContent(
      '检查间隔须为 5–1440 分钟的整数',
    );
    expect(intervalInput).toHaveAttribute(
      'aria-describedby',
      'pages-github-check-interval-description pages-github-check-interval-error',
    );
    expect(PagesService.updateSource).not.toHaveBeenCalled();
  });

  it('keeps an unsaved source draft when idle polling updates runtime fields', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(latestSource);
    const { queryClient } = renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '编辑来源' }));
    const assetInput = screen.getByLabelText('Release Asset 文件名');
    const intervalInput = screen.getByLabelText('检查间隔（分钟）');
    await user.clear(assetInput);
    await user.type(assetInput, 'draft.zip');
    await user.clear(intervalInput);
    await user.type(intervalInput, '30');
    await user.click(screen.getByRole('switch', { name: '自动更新' }));

    act(() => {
      queryClient.setQueryData(sourceQueryKey(9), {
        ...latestSource,
        last_checked_at: '2026-07-19T10:30:00Z',
        next_check_at: '2026-07-19T12:30:00Z',
      });
    });

    await waitFor(() => {
      expect(assetInput).toHaveValue('draft.zip');
      expect(intervalInput).toHaveValue(30);
      expect(
        screen.getByRole('switch', { name: '自动更新' }),
      ).not.toBeChecked();
    });
  });

  it('remounts project-scoped source state when the route project changes', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockImplementation(async (projectId) => ({
      ...latestSource,
      github_repository:
        projectId === 9 ? 'openflare/project-nine' : 'openflare/project-ten',
    }));
    const { rerenderWithQuery } = renderWithQuery(
      <PagesSourceCard key='source-9' projectId={9} />,
    );

    await user.click(await screen.findByRole('button', { name: '编辑来源' }));
    expect(screen.getByRole('dialog')).toBeVisible();

    rerenderWithQuery(<PagesSourceCard key='source-10' projectId={10} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(await screen.findByText('openflare/project-ten')).toBeVisible();
  });

  it('uses low-frequency, near-due and bounded overdue polling', () => {
    const now = Date.parse('2026-07-19T10:00:00Z');
    const farSource = {
      ...latestSource,
      next_check_at: new Date(now + 60 * 60 * 1_000).toISOString(),
    };
    expect(getLatestSourceIdlePollingDecision(farSource, now, null)).toEqual({
      interval: 5 * 60 * 1_000,
      overdueWindow: null,
    });

    const nearSource = {
      ...latestSource,
      next_check_at: new Date(now + 60_000).toISOString(),
    };
    expect(getLatestSourceIdlePollingDecision(nearSource, now, null)).toEqual({
      interval: 30_000,
      overdueWindow: null,
    });

    const overdueSource = {
      ...latestSource,
      next_check_at: new Date(now - 1).toISOString(),
    };
    const overdue = getLatestSourceIdlePollingDecision(
      overdueSource,
      now,
      null,
    );
    expect(overdue.interval).toBe(30_000);
    expect(
      getLatestSourceIdlePollingDecision(
        overdueSource,
        now + 10 * 60 * 1_000,
        overdue.overdueWindow,
      ).interval,
    ).toBe(false);

    expect(
      getLatestSourceIdlePollingDecision(
        {
          ...latestSource,
          release_selector: 'tag',
          release_tag: 'v1.2.3',
          auto_update_enabled: false,
          check_interval_minutes: 0,
        },
        now,
        null,
      ).interval,
    ).toBe(false);
  });

  it('refreshes source consumers after a background deployment is applied', async () => {
    vi.mocked(PagesService.getSource).mockResolvedValue(latestSource);
    const { queryClient } = renderWithQuery(<PagesSourceCard projectId={9} />);
    expect(await screen.findByText('openflare/site')).toBeVisible();

    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
    const updatedSource: PagesGitHubReleaseSource = {
      ...latestSource,
      last_applied: {
        revision: 'b'.repeat(64),
        label: 'v1.2.3',
        asset_name: 'dist.zip',
      },
      last_synced_at: '2026-07-19T10:30:00Z',
    };
    vi.mocked(PagesService.getSource).mockResolvedValue(updatedSource);

    act(() => {
      queryClient.setQueryData(sourceQueryKey(9), updatedSource);
    });

    await waitFor(() => {
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: sourceQueryKey(9),
      });
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: projectQueryKey(9),
      });
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: deploymentsQueryKey(9),
      });
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['openflare', 'pages', 'deployment-files', 9],
      });
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: projectsQueryKey,
      });
    });
  });

  it('warns that rollback closes auto update and refreshes four query groups', async () => {
    const user = userEvent.setup();
    const deployment: PagesDeployment = {
      id: 51,
      project_id: 9,
      deployment_number: 7,
      checksum: 'd'.repeat(64),
      status: 'uploaded',
      file_count: 12,
      total_size: 4_096,
      created_by: 'user:1',
      source_type: 'github_release',
      source_label: 'v1.2.2',
      trigger_type: 'scheduled_auto_update',
      created_at: '2026-07-19T10:00:00Z',
      activated_at: null,
    };
    vi.mocked(PagesService.listDeployments).mockResolvedValue([deployment]);
    vi.mocked(PagesService.activateDeployment).mockResolvedValue({} as never);

    const { queryClient } = renderWithQuery(
      <DeploymentHistory projectId={9} activeDeploymentId={52} />,
    );
    await screen.findByText('GitHub · v1.2.2 · 定时更新');
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    await user.click(screen.getByRole('button', { name: '激活' }));
    const dialog = screen.getByRole('alertdialog');
    expect(
      within(dialog).getByText(
        '激活其它历史部署会终止当前来源任务；若已开启自动更新，将同时关闭自动更新。',
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: '确认' }));

    await waitFor(() => {
      expect(PagesService.activateDeployment).toHaveBeenCalledWith(9, 51);
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: deploymentsQueryKey(9),
      });
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: projectQueryKey(9),
      });
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: sourceQueryKey(9),
      });
      expect(invalidateQueries).toHaveBeenCalledWith({
        queryKey: projectsQueryKey,
      });
    });
  });
});
