import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import axe from 'axe-core';

import { LoginPage } from '@/components/auth/login-page';
import { RegisterPage } from '@/components/auth/register-page';
import { OTPForm } from '@/components/auth/otp-form';
import { CapWidget } from '@/components/auth/cap-widget';
import { OtherTab } from '@/components/common/settings/other-tab';
import { SecurityTab } from '@/components/common/settings/security-tab';
import type { UseQueryResult } from '@tanstack/react-query';
import { UserProvider } from '@/contexts/user-context';
import type { SystemConfig } from '@/lib/services/admin';
import zhCN from '@/messages/zh-CN.json';

vi.mock('next/navigation', () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(''),
}));

const {
  getUserInfoMock,
  getPublicConfigMock,
  getAuthSourcesMock,
  listAuthSourcesMock,
} = vi.hoisted(() => ({
  getUserInfoMock: vi.fn(),
  getPublicConfigMock: vi.fn(),
  getAuthSourcesMock: vi.fn(),
  listAuthSourcesMock: vi.fn(),
}));

vi.mock('@/lib/services/auth', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services/auth')>();
  return {
    ...actual,
    AuthService: {
      ...actual.AuthService,
      getUserInfo: getUserInfoMock,
      getAuthSources: getAuthSourcesMock,
    },
  };
});
vi.mock('@/lib/services', () => ({
  default: {
    adminAuthSource: { listAuthSources: listAuthSourcesMock },
    adminSystemConfig: {
      updateSystemConfig: vi.fn().mockResolvedValue(undefined),
    },
  },
}));

vi.mock('@/lib/cap-solver', () => ({
  getCapToken: vi.fn().mockResolvedValue('test-cap-token'),
}));

vi.mock('@/lib/services/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services/config')>();
  return {
    ...actual,
    ConfigService: {
      ...actual.ConfigService,
      getPublicConfig: getPublicConfigMock,
    },
  };
});

async function runAxe(container: HTMLElement) {
  const results = await axe.run(container, {
    rules: {
      // jsdom 无布局引擎，颜色对比度规则无法评估
      'color-contrast': { enabled: false },
    },
  });
  return results.violations;
}

function renderWithProviders(page: React.ReactNode) {
  return render(
    <NextIntlClientProvider
      locale='zh-CN'
      messages={zhCN}
      timeZone='Asia/Shanghai'
    >
      <QueryClientProvider
        client={
          new QueryClient({
            defaultOptions: { queries: { retry: false } },
          })
        }
      >
        <UserProvider>{page}</UserProvider>
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

describe('a11y（axe-core 结构性规则）', () => {
  beforeEach(() => {
    getUserInfoMock.mockReset();
    getUserInfoMock.mockResolvedValue(null);
    getPublicConfigMock.mockReset();
    getPublicConfigMock.mockResolvedValue({
      registration_enabled: 'true',
      password_register_enabled: 'true',
    });
    getAuthSourcesMock.mockReset();
    getAuthSourcesMock.mockResolvedValue([]);
    listAuthSourcesMock.mockReset();
    listAuthSourcesMock.mockResolvedValue([]);
  });

  it('登录页表单无 axe 违规', async () => {
    renderWithProviders(<LoginPage />);

    await screen.findByRole('button', { name: /登录|submit|登录/ });
    expect(await runAxe(document.body)).toEqual([]);
  });

  it('注册页表单无 axe 违规', async () => {
    renderWithProviders(<RegisterPage />);

    await screen.findByRole('button', { name: /创建账号|submit/ });
    expect(await runAxe(document.body)).toEqual([]);
  });

  it('登录 OTP 验证表单无 axe 违规', async () => {
    renderWithProviders(
      <main>
        <OTPForm
          code=''
          setCode={() => {}}
          loginCodeTip='验证码已发送至邮箱'
          loginCooldown={0}
          isPending={false}
          onResend={() => {}}
          onSubmit={() => {}}
        />
      </main>,
    );

    await screen.findByRole('button', { name: /重新发送/ });
    expect(await runAxe(document.body)).toEqual([]);
  });

  it('人机验证小部件（手动模式）无 axe 违规', async () => {
    renderWithProviders(
      <main>
        <CapWidget autoStart={false} onToken={() => {}} />
      </main>,
    );

    await screen.findByRole('button');
    expect(await runAxe(document.body)).toEqual([]);
  });

  it('注册页（开启人机验证）无 axe 违规', async () => {
    getPublicConfigMock.mockResolvedValue({
      registration_enabled: 'true',
      password_register_enabled: 'true',
      cap_login_enabled: 'true',
      cap_auto_solve: 'true',
    });
    renderWithProviders(<RegisterPage />);

    await screen.findByRole('button', { name: /创建账号|submit/ });
    // CAPTCHA 自动求解完成后进入已通过状态
    await screen.findByText('人机验证通过');
    expect(await runAxe(document.body)).toEqual([]);
  });
});

it('设置-安全 Tab（人机验证配置表单）无 axe 违规', async () => {
  const mk = (key: string, value: string): SystemConfig => ({
    key,
    value,
    type: 'system',
    visibility: 0,
    description: '',
    created_at: '',
    updated_at: '',
  });
  const configs = {
    cap_login_enabled: mk('cap_login_enabled', 'true'),
    cap_challenge_count: mk('cap_challenge_count', '1'),
    cap_challenge_difficulty: mk('cap_challenge_difficulty', '4'),
    cap_challenge_size: mk('cap_challenge_size', '32'),
    cap_challenge_ttl_seconds: mk('cap_challenge_ttl_seconds', '600'),
    cap_token_ttl_seconds: mk('cap_token_ttl_seconds', '1200'),
    cap_auto_solve: mk('cap_auto_solve', 'true'),
    login_session_ttl_hours: mk('login_session_ttl_hours', '168'),
  };

  renderWithProviders(
    <main>
      <SecurityTab
        configs={configs}
        systemConfigsQuery={
          { data: Object.values(configs) } as UseQueryResult<
            SystemConfig[],
            Error
          >
        }
      />
    </main>,
  );

  await screen.findByLabelText(/难题数量/);
  expect(await runAxe(document.body)).toEqual([]);
});

it('设置-其他 Tab（菜单显示配置开关列表）无 axe 违规', async () => {
  const mk = (key: string, value: string): SystemConfig => ({
    key,
    value,
    type: 'system',
    visibility: 0,
    description: '',
    created_at: '',
    updated_at: '',
  });
  const configs = {
    menu_display_config: mk(
      'menu_display_config',
      JSON.stringify({ '/nodes': true, '/websites': false }),
    ),
  };

  renderWithProviders(
    <main>
      <OtherTab configs={configs} />
    </main>,
  );

  await screen.findByRole('switch', { name: /节点/ });
  expect(await runAxe(document.body)).toEqual([]);
});
