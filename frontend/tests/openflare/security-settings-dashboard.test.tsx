import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import axe from 'axe-core';
import { NextIntlClientProvider } from 'next-intl';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import OpenFlareDashboardPage from '@/app/(main)/page';
import SecuritySettingsPage from '@/app/(main)/security-settings/page';
import {
  isNavGroupActive,
  isOpenFlareConsoleRoute,
  openflareSecurityNavGroup,
  openflareSidebarNav,
} from '@/lib/navigation/openflare-nav';
import {
  AccessLogService,
  DashboardService,
  OptionService,
  ZoneService,
  type AccessLogOverview,
  type DashboardOverview,
} from '@/lib/services/openflare';
import { searchData } from '@/lib/utils/search-data';
import zhCN from '@/messages/zh-CN.json';

const { authStateMock } = vi.hoisted(() => ({
  authStateMock: {
    user: {
      id: 1,
      username: 'root',
      is_admin: true,
    } as { id: number; username: string; is_admin: boolean } | null,
    loading: false,
  },
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => '/security-settings',
}));

vi.mock('next/link', () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock('next/dynamic', () => ({
  default: () => () => <div data-testid='world-stage-map-stub' />,
}));

vi.mock('echarts-for-react', () => ({
  default: () => <div data-testid='echarts-stub' />,
}));

vi.mock('@/components/providers/auth-provider', () => ({
  useAuth: () => authStateMock,
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    DashboardService: {
      getOverview: vi.fn(),
    },
    AccessLogService: {
      getOverview: vi.fn(),
    },
    OptionService: {
      list: vi.fn(),
      updateBatch: vi.fn(),
    },
    ZoneService: {
      list: vi.fn(),
      getOverview: vi.fn(),
    },
  };
});

const dashboardFixture: DashboardOverview = {
  generated_at: '2026-11-20T04:00:00Z',
  summary: {
    total_nodes: 1,
    online_nodes: 1,
    offline_nodes: 0,
    pending_nodes: 0,
    unhealthy_nodes: 0,
  },
  traffic: {
    request_count: 1000,
    error_count: 5,
    status_4xx_count: 4,
    unique_visitors: 120,
    estimated_qps: 1.5,
    reported_nodes: 1,
  },
  capacity: {
    average_cpu_usage_percent: 12,
    average_memory_usage_percent: 34,
    high_cpu_nodes: 0,
    high_memory_nodes: 0,
    high_storage_nodes: 0,
  },
  nodes: [],
  distributions: {
    source_countries: [{ key: 'CN', value: 600 }],
    status_codes: [{ key: '2xx', value: 995 }],
    top_domains: [{ key: 'example.com', value: 900 }],
  },
  country_latencies: [
    {
      region: 'CN',
      request_count: 600,
      avg_request_time_ms: 42.5,
      avg_tcp_rtt_ms: 18.2,
    },
  ],
  trends: {
    traffic_24h: [
      {
        bucket_started_at: '2026-11-20T03:00:00Z',
        request_count: 1000,
        error_count: 5,
        unique_visitor_count: 120,
        status_2xx_count: 995,
        status_4xx_count: 4,
        status_5xx_count: 1,
      },
    ],
    capacity_24h: [
      {
        bucket_started_at: '2026-11-20T03:00:00Z',
        average_cpu_usage_percent: 12,
        average_memory_usage_percent: 34,
        reported_nodes: 1,
      },
    ],
    network_24h: [
      {
        bucket_started_at: '2026-11-20T03:00:00Z',
        bytes_received: 1024,
        bytes_provided: 4096,
        reported_nodes: 1,
      },
    ],
    disk_io_24h: [],
  },
};

const accessOverviewFixture: AccessLogOverview = {
  generated_at: '2026-11-20T04:00:00Z',
  hours: 24,
  bucket_minutes: 3,
  summary: {
    total_requests: 86400,
    total_visits: 1200,
    bandwidth_served: 1048576,
  },
  trends: {
    requests: [{ bucket_started_at: '2026-11-20T03:57:00Z', value: 360 }],
    visits: [{ bucket_started_at: '2026-11-20T03:57:00Z', value: 18 }],
    bandwidth: [{ bucket_started_at: '2026-11-20T03:57:00Z', value: 4096 }],
  },
  device_types: [],
  status_codes: [],
  top_paths: [],
  top_hosts: [{ key: 'cdn.example.com', value: 86400 }],
  top_ips: [{ key: '203.0.113.10', value: 43200 }],
  top_browsers: [],
  top_operating_systems: [],
  top_user_agents: [],
};

function renderWithProviders(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <NextIntlClientProvider
      locale='zh-CN'
      messages={zhCN}
      timeZone='Asia/Shanghai'
    >
      <QueryClientProvider client={client}>
        <main>{ui}</main>
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

describe('Security menu split, dashboard layout, and global settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    authStateMock.user = { id: 1, username: 'root', is_admin: true };
    authStateMock.loading = false;
    vi.mocked(DashboardService.getOverview).mockResolvedValue(dashboardFixture);
    vi.mocked(AccessLogService.getOverview).mockResolvedValue(
      accessOverviewFixture,
    );
    vi.mocked(ZoneService.list).mockResolvedValue([]);
    vi.mocked(OptionService.list).mockResolvedValue([
      {
        key: 'edge_default_limit_conn_per_server',
        value: '200',
      },
      {
        key: 'edge_default_limit_conn_per_ip',
        value: '20',
      },
      {
        key: 'edge_default_limit_rate',
        value: '512k',
      },
      {
        key: 'edge_default_limit_req_per_ip',
        value: '10r/s',
      },
    ]);
    vi.mocked(OptionService.updateBatch).mockResolvedValue(undefined);
  });

  it('replaces /rate-limits with /security-settings in navigation and search metadata', () => {
    expect(openflareSecurityNavGroup.items).toEqual([
      { titleKey: 'waf', url: '/waf' },
      { titleKey: 'attackProtection', url: '/attack-protection' },
      { titleKey: 'ipGroups', url: '/ip-groups' },
      { titleKey: 'securitySettings', url: '/security-settings' },
    ]);
    expect(
      isNavGroupActive('/security-settings', openflareSecurityNavGroup),
    ).toBe(true);
    expect(isOpenFlareConsoleRoute('/security-settings')).toBe(true);
    expect(isOpenFlareConsoleRoute('/rate-limits')).toBe(false);
    expect(searchData.some((item) => item.url === '/rate-limits')).toBe(false);
    expect(
      searchData.some(
        (item) =>
          item.url === '/security-settings' &&
          item.titleKey === 'nav.securitySettings',
      ),
    ).toBe(true);
  });

  it('places attack logs above access logs in sidebar navigation', () => {
    const logItemIndexes = openflareSidebarNav.flatMap((entry, index) => {
      if (
        entry.kind === 'item' &&
        ['attackLogs', 'accessLogs'].includes(entry.titleKey)
      ) {
        return [{ titleKey: entry.titleKey, index }];
      }
      return [];
    });

    expect(logItemIndexes).toEqual([
      { titleKey: 'attackLogs', index: logItemIndexes[0].index },
      { titleKey: 'accessLogs', index: logItemIndexes[0].index + 1 },
    ]);
  });

  it('places request pressure below 24h request trend and RPS rankings above traffic & capacity trend on dashboard', async () => {
    const user = userEvent.setup();
    renderWithProviders(<OpenFlareDashboardPage />);

    const latencyView = await screen.findByText('全球延迟视图');
    const reqTrend = await screen.findByText('24 小时请求趋势');
    const pressure = await screen.findByText('请求压力');
    const topDomain = await screen.findByText('Top Domain');
    const topHostRps = await screen.findByText('平均 RPS 最高域名');
    const topIpRps = await screen.findByText('平均 RPS 最高 IP');
    const capacityTrend = await screen.findByText('24 小时业务流量与容量趋势');

    expect(
      latencyView.compareDocumentPosition(reqTrend) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      reqTrend.compareDocumentPosition(pressure) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      pressure.compareDocumentPosition(topDomain) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      topDomain.compareDocumentPosition(topHostRps) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      topHostRps.compareDocumentPosition(capacityTrend) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      topIpRps.compareDocumentPosition(capacityTrend) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    await user.click(screen.getByRole('radio', { name: '3 天' }));
    await waitFor(() =>
      expect(AccessLogService.getOverview).toHaveBeenCalledWith({
        hours: 72,
        hosts: undefined,
        bucket_minutes: 3,
      }),
    );
  });

  it('renders and saves global default rate limits in /security-settings with no axe violations', async () => {
    const user = userEvent.setup();
    renderWithProviders(<SecuritySettingsPage />);

    expect(
      await screen.findByRole('heading', { name: '全局设置' }),
    ).toBeInTheDocument();
    const rateInput = await screen.findByLabelText('默认限速');
    expect(rateInput).toHaveValue('512k');

    await user.clear(rateInput);
    await user.type(rateInput, '1M');
    await user.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() =>
      expect(OptionService.updateBatch).toHaveBeenCalledWith([
        { key: 'edge_default_limit_conn_per_server', value: '200' },
        { key: 'edge_default_limit_conn_per_ip', value: '20' },
        { key: 'edge_default_limit_rate', value: '1m' },
        { key: 'edge_default_limit_req_per_ip', value: '10r/s' },
      ]),
    );

    const results = await axe.run(document.body, {
      rules: { 'color-contrast': { enabled: false } },
    });
    expect(results.violations).toEqual([]);
  });
});
