import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';

import {ZonePageClient} from '@/app/(main)/websites/[zoneId]/page-client';
import {ProxyRouteService, TlsCertificateService, ZoneService} from '@/lib/services/openflare';

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

vi.stubGlobal('ResizeObserver', ResizeObserverMock);

let mockZoneId = '42';
let mockParamZoneId = '42';

const replaceMock = vi.fn();

vi.mock('next/link', () => ({
  default: ({children, href}: {children: React.ReactNode; href: string}) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({replace: replaceMock, back: vi.fn()}),
  usePathname: () => `/websites/${mockZoneId}`,
  useSearchParams: () => new URLSearchParams(),
  useParams: () => ({zoneId: mockParamZoneId}),
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    ZoneService: {
      getOverview: vi.fn(),
      getStats: vi.fn(),
      deleteById: vi.fn(),
      list: vi.fn(),
    },
    TlsCertificateService: {
      list: vi.fn(),
    },
    ProxyRouteService: {
      list: vi.fn(),
    },
  };
});

function renderPage(zoneId: number, paramZoneId = zoneId) {
  mockZoneId = String(zoneId);
  mockParamZoneId = String(paramZoneId);
  const client = new QueryClient({
    defaultOptions: {
      queries: {retry: false, gcTime: 0},
    },
  });
  return render(
    <QueryClientProvider client={client}>
      <ZonePageClient />
    </QueryClientProvider>,
  );
}

describe('ZonePageClient', () => {
  beforeEach(() => {
    mockParamZoneId = mockZoneId;
    vi.mocked(ZoneService.getOverview).mockReset();
    vi.mocked(ZoneService.getStats).mockReset();
    vi.mocked(ZoneService.getStats).mockResolvedValue({
      range: '24h',
      range_hours: 24,
      window_started_at: new Date().toISOString(),
      window_ended_at: new Date().toISOString(),
      bucket_minutes: 60,
      unique_visitors: 0,
      request_count: 0,
      bytes_sent: 0,
      domain_count: 0,
      available: true,
      series: [],
    });
    vi.mocked(ZoneService.deleteById).mockReset();
    vi.mocked(TlsCertificateService.list).mockReset();
    vi.mocked(TlsCertificateService.list).mockResolvedValue([]);
    vi.mocked(ProxyRouteService.list).mockReset();
    vi.mocked(ProxyRouteService.list).mockResolvedValue([]);
  });

  it('loads the overview by stable ID and exposes all tabs including empty domains', async () => {
    vi.mocked(ZoneService.getOverview).mockImplementation(async () => ({
      zone: {id: 42, domain: 'example.com', created_at: '', updated_at: ''},
      domains: [],
    }));

    renderPage(42);

    await waitFor(() => {
      expect(ZoneService.getOverview).toHaveBeenCalledWith(42);
    });
    expect(await screen.findByRole('heading', {name: 'example.com'})).toBeVisible();
    expect(await screen.findByText('唯一访问者')).toBeVisible();
    expect(screen.getByText('请求总数')).toBeVisible();
    expect(screen.getByText('已提供的数据总计')).toBeVisible();
    expect(screen.getByRole('tab', {name: '域名 (0)'})).toBeVisible();
    expect(screen.getByRole('tab', {name: '证书 (0)'})).toBeVisible();
    expect(screen.queryByRole('tab', {name: '路由'})).not.toBeInTheDocument();
    expect(screen.getByRole('tab', {name: '设置'})).toBeVisible();
  });

  it('uses the browser pathname ID when serving a static-export fallback shell', async () => {
    vi.mocked(ZoneService.getOverview).mockImplementation(async () => ({
      zone: {id: 42, domain: 'example.com', created_at: '', updated_at: ''},
      domains: [],
    }));

    renderPage(42, 1);

    await waitFor(() => {
      expect(ZoneService.getOverview).toHaveBeenCalledWith(42);
    });
    expect(ZoneService.getOverview).not.toHaveBeenCalledWith(1);
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
