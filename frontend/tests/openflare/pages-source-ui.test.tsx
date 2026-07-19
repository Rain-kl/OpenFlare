import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { DeploymentUploadDialog } from '@/app/(main)/pages/components/deployment-upload-dialog';
import { DeploymentHistory } from '@/app/(main)/pages/detail/components/deployment-history';
import { PagesSourceCard } from '@/app/(main)/pages/detail/components/pages-source-card';
import {
  AdminTaskService,
  type TaskExecution,
  type TaskExecutionStatus,
} from '@/lib/services/admin';
import {
  type PagesDeployment,
  type PagesGitHubReleaseSource,
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
      checkSource: vi.fn(),
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
  remote_url: 'https://artifacts.example.com/site.zip?token=secret',
  allow_insecure: false,
  sync_status: 'idle',
  last_applied: {
    revision: 'a'.repeat(64),
    label: 'site.zip',
  },
  last_synced_at: '2026-07-19T10:00:00Z',
  last_error: '',
};

const githubLatestSource: PagesGitHubReleaseSource = {
  source_type: 'github_release',
  github_repository: 'openflare/site',
  release_selector: 'latest',
  asset_name: 'dist.zip',
  auto_update_enabled: false,
  check_interval_minutes: 1440,
  sync_status: 'update_available',
  update_available: true,
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
  last_synced_at: '2026-07-18T10:00:00Z',
  next_check_at: '2026-07-19T11:00:00Z',
  last_error: '',
};

const githubAttentionRevision = 'c'.repeat(64);
const githubAttentionSource: PagesGitHubReleaseSource = {
  ...githubLatestSource,
  sync_status: 'attention',
  update_available: true,
  last_seen: {
    revision: githubAttentionRevision,
    label: 'v1.2.3',
    asset_name: 'dist.zip',
  },
};

function taskExecution(
  status: TaskExecutionStatus,
  errorMessage = '',
): TaskExecution {
  return {
    id: '42',
    task_id: 'manual_of_pages_source_action_1',
    task_type: 'of_pages_source_action',
    task_name: 'Pages 来源动作',
    status,
    retryable: false,
    max_retry: 0,
    retry_count: 0,
    log: '',
    error_message: errorMessage,
    result: '',
    duration: 1,
    payload: '',
    triggered_by: 'admin:1',
    created_at: '2026-07-19T10:00:00Z',
    updated_at: '2026-07-19T10:00:01Z',
  };
}

describe('Pages source UI', () => {
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
    vi.mocked(PagesService.uploadDeployment).mockReset();
    vi.mocked(AdminTaskService.getTaskExecution).mockReset();
  });

  it('offers the three source types without future repository build controls', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue({
      source_type: 'manual',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    expect(await screen.findByText('本地部署包')).toBeVisible();
    expect(screen.getByRole('button', { name: '配置' })).toBeVisible();
    expect(screen.queryByText('检查更新')).not.toBeInTheDocument();
    expect(screen.queryByText('自动更新')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '配置' }));
    expect(screen.getByRole('radio', { name: '手动部署' })).toBeVisible();
    expect(screen.getByRole('radio', { name: 'Remote URL' })).toBeVisible();
    expect(screen.getByRole('radio', { name: 'GitHub Release' })).toBeVisible();
    expect(screen.getByRole('radio', { name: '手动部署' })).toBeChecked();
    expect(screen.queryByText('构建命令')).not.toBeInTheDocument();
    expect(screen.queryByText('输出目录')).not.toBeInTheDocument();

    await user.click(screen.getByRole('radio', { name: 'GitHub Release' }));
    expect(screen.getByRole('switch', { name: '自动更新' })).not.toBeChecked();
    expect(screen.getByLabelText('检查间隔（分钟）')).toHaveValue(1440);
  });

  it('submits the GitHub latest automatic update settings', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue({
      source_type: 'manual',
    });
    vi.mocked(PagesService.updateSource).mockResolvedValue({
      source: githubLatestSource,
      check_task: null,
      warning: '',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '配置' }));
    await user.click(screen.getByRole('radio', { name: 'GitHub Release' }));
    await user.type(
      screen.getByLabelText('GitHub 仓库 URL'),
      'https://github.com/openflare/site',
    );
    expect(screen.getByLabelText('Release Asset 文件名')).toHaveValue(
      'dist.zip',
    );
    await user.click(screen.getByRole('switch', { name: '自动更新' }));
    const intervalInput = screen.getByLabelText('检查间隔（分钟）');
    await user.clear(intervalInput);
    await user.type(intervalInput, '15');
    await user.click(screen.getByRole('button', { name: '保存 GitHub 来源' }));

    await waitFor(() => {
      expect(PagesService.updateSource).toHaveBeenCalledWith(9, {
        source_type: 'github_release',
        repository_url: 'https://github.com/openflare/site',
        release_selector: 'latest',
        release_tag: '',
        asset_name: 'dist.zip',
        auto_update_enabled: true,
        check_interval_minutes: 15,
      });
    });
  });

  it('rejects non-canonical GitHub repository URL paths', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue({
      source_type: 'manual',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '配置' }));
    await user.click(screen.getByRole('radio', { name: 'GitHub Release' }));
    const repositoryInput = screen.getByLabelText('GitHub 仓库 URL');
    await user.type(repositoryInput, 'https://github.com//openflare/site');
    await user.click(screen.getByRole('button', { name: '保存 GitHub 来源' }));
    expect(
      screen.getByText(
        '请输入 https://github.com/{owner}/{repo} 格式的公开仓库地址',
      ),
    ).toBeVisible();

    await user.clear(repositoryInput);
    await user.type(repositoryInput, 'https://github.com/openflare/site/');
    await user.click(screen.getByRole('button', { name: '保存 GitHub 来源' }));
    expect(PagesService.updateSource).not.toHaveBeenCalled();
  });

  it('submits the GitHub tag discriminator with safe disabled defaults', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue({
      source_type: 'manual',
    });
    vi.mocked(PagesService.updateSource).mockResolvedValue({
      source: {
        ...githubLatestSource,
        release_selector: 'tag',
        release_tag: 'v1.2.3',
        auto_update_enabled: false,
        check_interval_minutes: 0,
        next_check_at: null,
      },
      check_task: null,
      warning: '',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '配置' }));
    await user.click(screen.getByRole('radio', { name: 'GitHub Release' }));
    await user.type(
      screen.getByLabelText('GitHub 仓库 URL'),
      'https://github.com/openflare/site',
    );
    await user.click(screen.getByRole('radio', { name: '固定 Tag' }));
    expect(
      screen.queryByRole('switch', { name: '自动更新' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText('检查间隔（分钟）')).not.toBeInTheDocument();
    await user.type(
      screen.getByLabelText('Release tag'),
      'release/candidate#1&channel=stable',
    );
    await user.clear(screen.getByLabelText('Release Asset 文件名'));
    await user.type(
      screen.getByLabelText('Release Asset 文件名'),
      ' site?arch=amd64#stable.zip ',
    );
    await user.click(screen.getByRole('button', { name: '保存 GitHub 来源' }));

    await waitFor(() => {
      expect(PagesService.updateSource).toHaveBeenCalledWith(9, {
        source_type: 'github_release',
        repository_url: 'https://github.com/openflare/site',
        release_selector: 'tag',
        release_tag: 'release/candidate#1&channel=stable',
        asset_name: ' site?arch=amd64#stable.zip ',
        auto_update_enabled: false,
        check_interval_minutes: 0,
      });
    });
  });

  it('rejects unsafe GitHub tag and asset values before saving', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue({
      source_type: 'manual',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '配置' }));
    await user.click(screen.getByRole('radio', { name: 'GitHub Release' }));
    await user.type(
      screen.getByLabelText('GitHub 仓库 URL'),
      'https://github.com/openflare/site',
    );
    await user.click(screen.getByRole('radio', { name: '固定 Tag' }));
    await user.type(screen.getByLabelText('Release tag'), 'release//candidate');
    await user.clear(screen.getByLabelText('Release Asset 文件名'));
    await user.type(
      screen.getByLabelText('Release Asset 文件名'),
      '../dist.zip',
    );
    await user.click(screen.getByRole('button', { name: '保存 GitHub 来源' }));

    expect(
      screen.getByText(
        'Release tag 须为有效 Git ref（1–255 字节，可使用 /、#、&、=）',
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        'Asset 文件名须为 1–255 字节，且不能是路径或包含控制、换行、双向文本字符',
      ),
    ).toBeVisible();
    expect(PagesService.updateSource).not.toHaveBeenCalled();
  });

  it('polls the initial check receipt and represents the queued window locally', async () => {
    const user = userEvent.setup();
    const idleGitHubSource: PagesGitHubReleaseSource = {
      ...githubLatestSource,
      sync_status: 'idle',
      update_available: false,
    };
    vi.mocked(PagesService.getSource)
      .mockResolvedValueOnce({ source_type: 'manual' })
      .mockResolvedValue(idleGitHubSource);
    vi.mocked(PagesService.updateSource).mockResolvedValue({
      source: idleGitHubSource,
      check_task: {
        task_id: 'manual_of_pages_source_action_1',
        execution_id: '42',
        action: 'check',
      },
      warning: '',
    });
    vi.mocked(AdminTaskService.getTaskExecution).mockResolvedValue(
      taskExecution('pending'),
    );

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '配置' }));
    await user.click(screen.getByRole('radio', { name: 'GitHub Release' }));
    await user.type(
      screen.getByLabelText('GitHub 仓库 URL'),
      'https://github.com/openflare/site',
    );
    await user.click(screen.getByRole('button', { name: '保存 GitHub 来源' }));

    await waitFor(() => {
      expect(AdminTaskService.getTaskExecution).toHaveBeenCalledWith('42');
      expect(screen.getByText('检查中')).toBeVisible();
    });
    const checkButton = await screen.findByRole('button', {
      name: /检查更新/,
    });
    expect(checkButton).toBeDisabled();
    expect(within(checkButton).getByRole('status')).toBeVisible();
    expect(PagesService.checkSource).not.toHaveBeenCalled();
    expect(PagesService.syncSource).not.toHaveBeenCalled();
  });

  it('shows GitHub latest automatic update state and schedule', async () => {
    vi.mocked(PagesService.getSource).mockResolvedValue(githubLatestSource);

    renderWithQuery(<PagesSourceCard projectId={9} />);

    expect(await screen.findByText('openflare/site')).toBeVisible();
    expect(screen.getByText('v1.2.3 · bbbbbbbbbbbb')).toBeVisible();
    expect(screen.getByText('v1.2.2 · aaaaaaaaaaaa')).toBeVisible();
    expect(screen.getByText('有可用更新')).toBeVisible();
    expect(screen.getByText('下次检查')).toBeVisible();
    expect(screen.getByText('自动更新')).toBeVisible();
    expect(screen.getByText('已关闭')).toBeVisible();
    expect(screen.getByText('检查间隔')).toBeVisible();
    expect(screen.getByText('1440 分钟')).toBeVisible();
  });

  it('dispatches a GitHub check and starts TaskExecution polling', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(githubLatestSource);
    vi.mocked(PagesService.checkSource).mockResolvedValue({
      task_id: 'manual_of_pages_source_action_1',
      execution_id: '42',
      action: 'check',
    });
    vi.mocked(AdminTaskService.getTaskExecution).mockResolvedValue(
      taskExecution('succeeded'),
    );

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '检查更新' }));

    await waitFor(() => {
      expect(PagesService.checkSource).toHaveBeenCalledWith(9);
      expect(AdminTaskService.getTaskExecution).toHaveBeenCalledWith('42');
      expect(PagesService.syncSource).not.toHaveBeenCalled();
    });
  });

  it('renders a GitHub check dispatch error', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(githubLatestSource);
    vi.mocked(PagesService.checkSource).mockRejectedValue(
      new Error('GitHub API 暂不可用'),
    );

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '检查更新' }));

    expect(await screen.findByText('GitHub API 暂不可用')).toBeVisible();
  });

  it('renders a TaskExecution polling error with an explicit retry', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(githubLatestSource);
    vi.mocked(PagesService.checkSource).mockResolvedValue({
      task_id: 'manual_of_pages_source_action_1',
      execution_id: '42',
      action: 'check',
    });
    vi.mocked(AdminTaskService.getTaskExecution).mockRejectedValue(
      new Error('任务状态暂时不可读取'),
    );

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '检查更新' }));

    expect(await screen.findByText('任务状态暂时不可读取')).toBeVisible();
    expect(screen.getByRole('button', { name: '重试' })).toBeVisible();
  });

  it('requires the exact currently displayed revision for attention sync', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(githubAttentionSource);
    vi.mocked(PagesService.syncSource).mockResolvedValue({
      task_id: 'manual_of_pages_source_action_1',
      execution_id: '42',
      action: 'sync',
    });
    vi.mocked(AdminTaskService.getTaskExecution).mockResolvedValue(
      taskExecution('succeeded'),
    );

    renderWithQuery(<PagesSourceCard projectId={9} />);

    const alert = await screen.findByRole('alert');
    expect(
      within(alert).getByText('Release Asset 发生变化，需要显式确认'),
    ).toBeVisible();
    expect(within(alert).getByText(githubAttentionRevision)).toBeVisible();

    await user.click(screen.getByRole('button', { name: '同步并发布' }));
    const dialog = screen.getByRole('alertdialog');
    expect(within(dialog).getByText(githubAttentionRevision)).toBeVisible();
    await user.click(
      within(dialog).getByRole('button', { name: '确认并发布' }),
    );

    await waitFor(() => {
      expect(PagesService.syncSource).toHaveBeenCalledWith(9, {
        confirmed_revision: githubAttentionRevision,
      });
    });
  });

  it('edits the remote URL as plain text', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(remoteSource);
    vi.mocked(PagesService.updateSource).mockResolvedValue({
      source: {
        ...remoteSource,
        remote_url: 'https://new.example.com/site.zip?token=new',
      },
      check_task: null,
      warning: '',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    expect(
      await screen.findByText(
        'https://artifacts.example.com/site.zip?token=secret',
      ),
    ).toBeVisible();

    await user.click(screen.getByRole('button', { name: '配置' }));
    const input = screen.getByLabelText('Remote URL');
    expect(input).toHaveAttribute('type', 'url');
    expect(input).toHaveValue(
      'https://artifacts.example.com/site.zip?token=secret',
    );
    await user.clear(input);
    await user.type(input, 'https://new.example.com/site.zip?token=new');
    await user.click(screen.getByRole('button', { name: '保存 Remote 来源' }));

    await waitFor(() => {
      expect(PagesService.updateSource).toHaveBeenCalledWith(9, {
        source_type: 'remote_url',
        remote_url: 'https://new.example.com/site.zip?token=new',
        allow_insecure: false,
      });
    });
  });

  it('saves the allow-insecure connection switch for remote sources', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.getSource).mockResolvedValue(remoteSource);
    vi.mocked(PagesService.updateSource).mockResolvedValue({
      source: { ...remoteSource, allow_insecure: true },
      check_task: null,
      warning: '',
    });

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '配置' }));
    expect(
      screen.getByRole('switch', { name: '允许不安全的连接' }),
    ).not.toBeChecked();
    await user.click(screen.getByRole('switch', { name: '允许不安全的连接' }));
    await user.click(screen.getByRole('button', { name: '保存 Remote 来源' }));

    await waitFor(() => {
      expect(PagesService.updateSource).toHaveBeenCalledWith(9, {
        source_type: 'remote_url',
        remote_url: 'https://artifacts.example.com/site.zip?token=secret',
        allow_insecure: true,
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
    vi.mocked(AdminTaskService.getTaskExecution)
      .mockResolvedValueOnce(taskExecution('pending'))
      .mockResolvedValue(taskExecution('succeeded'));

    renderWithQuery(<PagesSourceCard projectId={9} />);

    await user.click(await screen.findByRole('button', { name: '同步并发布' }));

    await waitFor(() => {
      expect(PagesService.syncSource).toHaveBeenCalledWith(9, {});
      expect(AdminTaskService.getTaskExecution).toHaveBeenCalledWith('42');
    });
    expect(screen.getByRole('button', { name: /同步并发布/ })).toBeDisabled();

    await waitFor(
      () => {
        expect(AdminTaskService.getTaskExecution).toHaveBeenCalledTimes(2);
      },
      { timeout: 3_500 },
    );
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

  it('renders the immutable GitHub deployment provenance in Chinese', async () => {
    const deployment: PagesDeployment = {
      id: 51,
      project_id: 9,
      deployment_number: 7,
      checksum: 'd'.repeat(64),
      status: 'active',
      file_count: 12,
      total_size: 4_096,
      created_by: 'user:1',
      source_type: 'github_release',
      source_label: 'v1.2.3',
      trigger_type: 'manual_sync',
      created_at: '2026-07-19T10:00:00Z',
      activated_at: '2026-07-19T10:00:01Z',
    };
    vi.mocked(PagesService.listDeployments).mockResolvedValue([deployment]);

    renderWithQuery(
      <DeploymentHistory projectId={9} activeDeploymentId={51} />,
    );

    expect(
      await screen.findAllByText('GitHub · v1.2.3 · 手动同步'),
    ).toHaveLength(2);
    expect(screen.getByText('Production')).toBeVisible();
    expect(screen.getByText('All deployments')).toBeVisible();
  });
});
