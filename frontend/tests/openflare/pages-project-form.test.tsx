import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { memo } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ProjectFormFields,
  usePagesProjectForm,
} from '@/app/(main)/pages/components/project-form';
import { ProjectEditorDialog } from '@/app/(main)/pages/components/project-editor-dialog';
import { ProjectSettingsCard } from '@/app/(main)/pages/detail/components/project-settings-card';
import { type PagesProject, PagesService } from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    PagesService: { createProject: vi.fn(), updateProject: vi.fn() },
  };
});

const project: PagesProject = {
  id: 1,
  name: 'speedtest',
  slug: 'speedtest',
  description: '',
  enabled: true,
  spa_fallback_enabled: true,
  spa_fallback_path: '/index.html',
  api_proxy_enabled: false,
  api_proxy_path: '',
  api_proxy_pass: '',
  api_proxy_rewrite: '',
  entry_file: 'index.html',
  deployment_count: 1,
  created_at: '',
  updated_at: '',
};

function renderPage(ui: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  });
  return render(
    <NextIntlClientProvider
      locale='zh-CN'
      messages={zhCN}
      timeZone='Asia/Shanghai'
    >
      <QueryClientProvider client={client}>{ui}</QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

const MemoizedFields = memo(ProjectFormFields);

function MemoizedForm() {
  const form = usePagesProjectForm();
  return <MemoizedFields form={form} />;
}

describe('Pages project switches', () => {
  it('updates switches when the form fields are memoized', async () => {
    const user = userEvent.setup();
    renderPage(<MemoizedForm />);
    const [spa, api] = screen.getAllByRole('switch');
    await user.click(spa);
    expect(spa).toBeChecked();
    await user.click(api);
    expect(api).toBeChecked();
  });

  beforeEach(() => vi.clearAllMocks());

  it('toggles both options and submits them when creating a project', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.createProject).mockResolvedValue(project);
    renderPage(<ProjectEditorDialog open onOpenChange={vi.fn()} />);

    await user.type(screen.getByLabelText('项目名称'), 'test');
    const [spa, api] = screen.getAllByRole('switch');
    await user.click(spa);
    expect(spa).toBeChecked();
    await user.click(api);
    expect(api).toBeChecked();
    await user.type(screen.getByLabelText('匹配路径'), '/api');
    await user.type(screen.getByLabelText('后端地址'), 'https://example.com');
    await user.click(screen.getByRole('button', { name: '创建项目' }));
    await waitFor(() =>
      expect(PagesService.createProject).toHaveBeenCalledWith(
        expect.objectContaining({
          spa_fallback_enabled: true,
          api_proxy_enabled: true,
        }),
      ),
    );
  });

  it('toggles, resets, and saves options in project settings', async () => {
    const user = userEvent.setup();
    vi.mocked(PagesService.updateProject).mockResolvedValue(project);
    renderPage(<ProjectSettingsCard project={project} />);

    const [spa, api] = screen.getAllByRole('switch');
    await user.click(spa);
    expect(spa).not.toBeChecked();
    await user.click(api);
    expect(api).toBeChecked();
    await user.click(screen.getByRole('button', { name: '重置' }));
    expect(spa).toBeChecked();
    expect(api).not.toBeChecked();
    await user.click(spa);
    await user.click(screen.getByRole('button', { name: '保存修改' }));
    await waitFor(() =>
      expect(PagesService.updateProject).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          spa_fallback_enabled: false,
          api_proxy_enabled: false,
        }),
      ),
    );
  });
});
