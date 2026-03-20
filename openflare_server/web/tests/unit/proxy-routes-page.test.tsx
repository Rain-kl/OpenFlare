import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ThemeProvider } from '@/components/providers/theme-provider';
import { ProxyRoutesPage } from '@/features/proxy-routes/components/proxy-routes-page';

describe('ProxyRoutesPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubMatchMedia() {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockImplementation(() => ({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    );
  }

  function renderProxyRoutesPage() {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <ProxyRoutesPage />
        </ThemeProvider>
      </QueryClientProvider>,
    );
  }

  it('shows wildcard subdomain input after selecting a wildcard website', async () => {
    stubMatchMedia();

    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);

        if (url.includes('/proxy-routes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({ success: true, message: '', data: [] }),
            ),
          );
        }

        if (url.includes('/managed-domains/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [
                  {
                    id: 1,
                    domain: '*.example.com',
                    cert_id: 1,
                    enabled: true,
                    remark: '',
                    created_at: '2026-03-20T08:00:00Z',
                    updated_at: '2026-03-20T08:00:00Z',
                  },
                  {
                    id: 2,
                    domain: 'a.example.com',
                    cert_id: 1,
                    enabled: true,
                    remark: '',
                    created_at: '2026-03-20T08:00:00Z',
                    updated_at: '2026-03-20T08:00:00Z',
                  },
                ],
              }),
            ),
          );
        }

        if (url.includes('/origins/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [],
              }),
            ),
          );
        }

        if (url.includes('/tls-certificates/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, name: 'example-cert', not_after: null }],
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderProxyRoutesPage();

    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: '新增规则' }));
    const dialog = await screen.findByRole('dialog', { name: '新增规则' });

    await user.selectOptions(
      within(dialog).getAllByRole('combobox')[0],
      '*.example.com',
    );

    expect(await screen.findByPlaceholderText('ai')).toBeInTheDocument();
    expect(
      screen.getByText(
        '当前网站为通配符 *.example.com，这里只需填写前缀，例如 ai。',
      ),
    ).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('ai'), 'ai');

    expect(
      screen.getByText('当前将生成规则域名 ai.example.com'),
    ).toBeInTheDocument();
  });

  it('uses exact website directly without showing subdomain input', async () => {
    stubMatchMedia();

    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);

        if (url.includes('/proxy-routes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({ success: true, message: '', data: [] }),
            ),
          );
        }

        if (url.includes('/managed-domains/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [
                  {
                    id: 2,
                    domain: 'a.example.com',
                    cert_id: 1,
                    enabled: true,
                    remark: '',
                    created_at: '2026-03-20T08:00:00Z',
                    updated_at: '2026-03-20T08:00:00Z',
                  },
                ],
              }),
            ),
          );
        }

        if (url.includes('/origins/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [],
              }),
            ),
          );
        }

        if (url.includes('/tls-certificates/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, name: 'example-cert', not_after: null }],
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderProxyRoutesPage();

    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: '新增规则' }));
    const dialog = await screen.findByRole('dialog', { name: '新增规则' });

    await user.selectOptions(
      within(dialog).getAllByRole('combobox')[0],
      'a.example.com',
    );

    expect(screen.queryByPlaceholderText('ai')).not.toBeInTheDocument();
    expect(
      await screen.findByText(
        '当前将直接使用 a.example.com 作为规则域名，无需再填写网站名。',
      ),
    ).toBeInTheDocument();
  });
});
