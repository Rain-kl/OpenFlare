import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { DeploymentUploadDialog } from '@/app/(main)/pages/components/deployment-upload-dialog';
import { DeploymentHistory } from '@/app/(main)/pages/detail/components/deployment-history';
import { PagesSourceCard } from '@/app/(main)/pages/detail/components/pages-source-card';
import { AdminTaskService } from '@/lib/services/admin';
import {
  type PagesRemoteURLSource,
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
      syncSource: vi.fn(),
      listDeployments: vi.fn(),
      listDeploymentFiles: vi.fn(),
      activateDeployment: vi.fn(),
      deleteDeployment: vi.fn(),
      uploadDeployment: vi.fn(),
    },
  };
});

vi.mock('@/lib/services/admin', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services/admin')>();
  return {
    ...actual,
    AdminTaskService: {
      getTaskExecution: vi.fn(),
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
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

const remoteSource: PagesRemoteURLSource = {
  source_type: 'remote_url',
  has_remote_url: true,
  display_url: 'https://artifacts.example.com/site.zip?***',
  remote_network_policy: 'public',
  sync_status: 'idle',
  last_applied: {
    revision: 'a'.repeat(64),
    label: 'site.zip',
  },
  last_synced_at: '2026-07-19T10:00:00Z',
  last_error: '',
};

describe('Pages source UI', () => {
  beforeEach(() => {
    vi.mocked(PagesService.getSource).mockReset();
    vi.mocked(PagesService.updateSource).mockReset();
    vi.mocked(PagesService.deleteSource).mockReset();
    vi.mocked(PagesService.syncSource).mockReset();
    vi.mocked(PagesService.listDeployments).mockReset();
    vi.mocked(PagesService.listDeploymentFiles).mockReset();
    vi.mocked(PagesService.activateDeployment).mockReset();
    vi.mocked(PagesService.deleteDeployment).mockReset();
    vi.mocked(PagesService.uploadDeployment).mockReset();
    vi.mocked(AdminTaskService.getTaskExecution).mockReset();
  });

  it('keeps Phase 1 manual source focused on upload and Remote URL', async () => {
    vi.mocked(PagesService.getSource).mockResolvedValue({
      source_type: 'manual',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    expect(await screen.findByText('本地部署包')).toBeVisible();
    expect(
      screen.getByRole('button', { name: '配置 Remote URL' }),
    ).toBeVisible();
    expect(screen.queryByText('检查更新')).not.toBeInTheDocument();
    expect(screen.queryByText('自动更新')).not.toBeInTheDocument();
  });

  it('never reuses the masked URL as an editable value', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(remoteSource);
    vi.mocked(PagesService.updateSource).mockResolvedValue({
      source: remoteSource,
      check_task: null,
      warning: '',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    expect(
      await screen.findByText('https://artifacts.example.com/site.zip?***'),
    ).toBeVisible();
    expect(screen.queryByText(/token=secret/)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '编辑来源' }));
    await user.click(screen.getByRole('button', { name: '更换地址' }));

    const input = screen.getByPlaceholderText(
      'https://artifacts.example.com/site.zip?token=...',
    );
    expect(input).toHaveValue('');
    await user.type(input, 'https://new.example.com/site.zip?token=new');
    await user.click(screen.getByRole('button', { name: '保存 Remote 来源' }));

    await waitFor(() => {
      expect(PagesService.updateSource).toHaveBeenCalledWith(9, {
        source_type: 'remote_url',
        remote_url_set: true,
        remote_url: 'https://new.example.com/site.zip?token=new',
        remote_network_policy: 'public',
      });
    });
  });

  it('requires a second confirmation for trusted internal networking', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(remoteSource);
    vi.mocked(PagesService.updateSource).mockResolvedValue({
      source: { ...remoteSource, remote_network_policy: 'trusted_internal' },
      check_task: null,
      warning: '',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '编辑来源' }));
    await user.click(screen.getByRole('radio', { name: '受信内网模式' }));
    await user.click(screen.getByRole('button', { name: '保存 Remote 来源' }));

    expect(await screen.findByText('启用受信内网模式')).toBeVisible();
    expect(PagesService.updateSource).not.toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: '确认' }));
    await waitFor(() => {
      expect(PagesService.updateSource).toHaveBeenCalledWith(9, {
        source_type: 'remote_url',
        remote_url_set: false,
        remote_url: '',
        remote_network_policy: 'trusted_internal',
      });
    });
  });

  it('polls the existing task execution detail after sync dispatch', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(remoteSource);
    vi.mocked(PagesService.syncSource).mockResolvedValue({
      task_id: 'manual_of_pages_source_action_1',
      execution_id: '42',
      action: 'sync',
    });
    vi.mocked(AdminTaskService.getTaskExecution).mockResolvedValue({
      id: '42',
      task_id: 'manual_of_pages_source_action_1',
      task_type: 'of_pages_source_action',
      task_name: 'Pages 来源动作',
      status: 'succeeded',
      retryable: false,
      max_retry: 0,
      retry_count: 0,
      log: '',
      error_message: '',
      result: '',
      duration: 1,
      payload: '',
      triggered_by: 'admin:1',
      created_at: '2026-07-19T10:00:00Z',
      updated_at: '2026-07-19T10:00:01Z',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '同步并发布' }));

    await waitFor(() => {
      expect(PagesService.syncSource).toHaveBeenCalledWith(9, {});
      expect(AdminTaskService.getTaskExecution).toHaveBeenCalledWith('42');
    });
  });

  it('shows the actual project entry and no one-off URL upload tab', () => {
    renderWithQuery(
      <DeploymentUploadDialog
        open
        onOpenChange={vi.fn()}
        projectId={9}
        rootDir='dist/site'
        entryFile='home.html'
      />,
    );

    expect(screen.getByText('dist/site/home.html')).toBeVisible();
    expect(screen.queryByText('从 URL 下载')).not.toBeInTheDocument();
    expect(screen.queryByText('部署包下载链接')).not.toBeInTheDocument();
  });

  it('renders a deployment query failure instead of an empty history', async () => {
    vi.mocked(PagesService.listDeployments).mockRejectedValue(
      new Error('部署历史暂时不可用'),
    );

    renderWithQuery(<DeploymentHistory projectId={9} />);

    expect(await screen.findByText('部署历史暂时不可用')).toBeVisible();
    expect(screen.queryByText('暂无部署')).not.toBeInTheDocument();
  });
});
