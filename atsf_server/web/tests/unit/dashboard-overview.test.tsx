import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardOverview } from '@/features/dashboard/components/dashboard-overview';

describe('DashboardOverview', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders dashboard summary cards', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);

        if (url.includes('/nodes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, status: 'online', auto_update_enabled: true }],
              }),
            ),
          );
        }

        if (url.includes('/config-versions/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, version: '20260311-001', is_active: true, created_at: '2026-03-11T10:00:00Z' }],
              }),
            ),
          );
        }

        if (url.includes('/managed-domains/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, domain: '*.example.com', enabled: true, cert_id: 1 }],
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
                data: [
                  {
                    id: 1,
                    name: 'example',
                    not_after: '2026-04-01T00:00:00Z',
                    updated_at: '2026-03-10T10:00:00Z',
                  },
                ],
              }),
            ),
          );
        }

        if (url.includes('/proxy-routes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, enabled: true, enable_https: true, custom_headers: '[{"key":"X-Test","value":"1"}]' }],
              }),
            ),
          );
        }

        if (url.includes('/user/search')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, role: 100, status: 1, username: 'root' }],
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <DashboardOverview />
      </QueryClientProvider>,
    );

    expect(screen.getByText('控制台仪表盘')).toBeInTheDocument();
    expect(await screen.findByText('节点摘要')).toBeInTheDocument();
    expect(await screen.findByText('用户概览')).toBeInTheDocument();
  });
});
