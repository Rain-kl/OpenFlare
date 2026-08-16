import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import axe from 'axe-core';

import { LoginPage } from '@/components/auth/login-page';
import { RegisterPage } from '@/components/auth/register-page';
import { UserProvider } from '@/contexts/user-context';
import zhCN from '@/messages/zh-CN.json';

vi.mock('next/navigation', () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(''),
}));

const { getUserInfoMock, getPublicConfigMock, getAuthSourcesMock } =
  vi.hoisted(() => ({
    getUserInfoMock: vi.fn(),
    getPublicConfigMock: vi.fn(),
    getAuthSourcesMock: vi.fn(),
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
});
