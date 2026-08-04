import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { createElement } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import CloudflarePage from '@/app/(main)/cloudflare/page';
import { CloudflareService } from '@/lib/services/openflare';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    CloudflareService: { ...actual.CloudflareService, getOverview: vi.fn() },
  };
});

describe('Cloudflare overview', () => {
  beforeEach(() => {
    vi.mocked(CloudflareService.getOverview).mockResolvedValue({
      connection: {
        configured: false,
        ready: false,
        source: '',
        dns_account_id: null,
        status: '',
        verified_at: null,
      },
      group_count: 0,
      member_count: 0,
      ok_count: 0,
      pending_count: 0,
      error_count: 0,
    });
  });

  it('shows the token readiness gate and phase-one limitation', async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });
    render(
      createElement(
        QueryClientProvider,
        { client },
        createElement(CloudflarePage),
      ),
    );

    expect(await screen.findByText('Cloudflare 连接尚未就绪')).toBeVisible();
    expect(screen.getByText(/一期不提供自动故障切换/)).toBeVisible();
  });
});
