import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';

import {ZonePageClient} from '@/app/(main)/websites/[zoneId]/page-client';
import {TlsCertificateService, ZoneService} from '@/lib/services/openflare';

vi.mock('next/link', () => ({
  default: ({children, href}: {children: React.ReactNode; href: string}) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    ZoneService: {
      getOverview: vi.fn(),
      deleteById: vi.fn(),
      list: vi.fn(),
    },
    TlsCertificateService: {
      list: vi.fn(),
    },
  };
});

function renderPage(zoneId: number) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {retry: false, gcTime: 0},
    },
  });
  return render(
    <QueryClientProvider client={client}>
      <ZonePageClient zoneId={zoneId} />
    </QueryClientProvider>,
  );
}

describe('ZonePageClient', () => {
  beforeEach(() => {
    vi.mocked(ZoneService.getOverview).mockReset();
    vi.mocked(ZoneService.deleteById).mockReset();
    vi.mocked(TlsCertificateService.list).mockReset();
    vi.mocked(TlsCertificateService.list).mockResolvedValue([]);
  });

  it('loads the overview by stable ID and exposes all tabs including empty domains', async () => {
    vi.mocked(ZoneService.getOverview).mockImplementation(async () => ({
      zone: {id: 42, domain: 'arctel.de', remark: '', created_at: '', updated_at: ''},
      domains: [],
    }));

    renderPage(42);

    await waitFor(() => {
      expect(ZoneService.getOverview).toHaveBeenCalledWith(42);
    });
    expect(await screen.findByRole('heading', {name: 'arctel.de'})).toBeVisible();
    expect(screen.getByRole('tab', {name: '域名 (0)'})).toBeVisible();
    expect(screen.getByRole('tab', {name: '证书 (0)'})).toBeVisible();
    expect(screen.getByRole('tab', {name: '路由'})).toBeVisible();
    expect(screen.getByRole('tab', {name: '设置'})).toBeVisible();
  });

  it('renders a not-found state for a missing Zone', async () => {
    vi.mocked(ZoneService.getOverview).mockRejectedValue(new Error('Zone 不存在'));

    renderPage(42);

    expect(
      await screen.findByText('网站不存在，可能已被删除或 ID 无效。'),
    ).toBeVisible();
  });

  it('renders invalid ID empty state without calling the API', async () => {
    renderPage(0);
    expect(
      await screen.findByText('无效的网站 ID，请从网站列表进入详情页。'),
    ).toBeVisible();
    expect(ZoneService.getOverview).not.toHaveBeenCalled();
  });
});
