import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ThemeProvider } from '@/components/providers/theme-provider';
import { DashboardOverview } from '@/features/dashboard/components/dashboard-overview';

vi.mock('echarts-for-react', () => ({
  default: () => <div data-testid="echarts-mock" />,
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: vi.fn(),
  }),
}));

describe('DashboardOverview', () => {
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

  function renderDashboardOverview() {
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
          <DashboardOverview />
        </ThemeProvider>
      </QueryClientProvider>,
    );
  }

  it('renders dashboard summary cards', async () => {
    stubMatchMedia();

    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);

        if (url.includes('/dashboard/overview')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: {
                  generated_at: '2026-03-14T08:00:00Z',
                  summary: {
                    total_nodes: 2,
                    online_nodes: 2,
                    offline_nodes: 0,
                    pending_nodes: 0,
                    unhealthy_nodes: 1,
                  },
                  traffic: {
                    request_count: 900,
                    unique_visitors: 200,
                    error_count: 36,
                    estimated_qps: 15,
                    reported_nodes: 2,
                  },
                  capacity: {
                    average_cpu_usage_percent: 68.5,
                    average_memory_usage_percent: 71.8,
                    high_cpu_nodes: 1,
                    high_memory_nodes: 1,
                    high_storage_nodes: 1,
                  },
                  distributions: {
                    source_countries: [
                      ['CN', 440],
                      ['US', 320],
                      ['SG', 140],
                    ],
                    status_codes: [
                      ['200', 820],
                      ['502', 24],
                      ['500', 12],
                    ],
                    top_domains: [
                      ['app.example.com', 580],
                      ['api.example.com', 220],
                    ],
                  },
                  trends: {
                    traffic_24h: Array.from({ length: 24 }, (_, index) => [
                      `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      index * 10,
                      index,
                      index * 3,
                    ]),
                    capacity_24h: Array.from({ length: 24 }, (_, index) => [
                      `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      index,
                      index + 10,
                      2,
                    ]),
                    network_24h: Array.from({ length: 24 }, (_, index) => [
                      `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      index * 100,
                      index * 120,
                      index * 140,
                      index * 160,
                      2,
                    ]),
                    disk_io_24h: Array.from({ length: 24 }, (_, index) => [
                      `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      index * 50,
                      index * 70,
                      2,
                    ]),
                  },
                  nodes: [
                    [
                      1,
                      'node-a',
                      'edge-a',
                      'Shanghai',
                      31.2304,
                      121.4737,
                      'online',
                      'healthy',
                      '20260314-001',
                      '2026-03-14T08:00:00Z',
                      0,
                      45,
                      50,
                      60,
                      600,
                      6,
                      120,
                    ],
                    [
                      2,
                      'node-b',
                      'edge-b',
                      'San Francisco',
                      37.7749,
                      -122.4194,
                      'online',
                      'unhealthy',
                      '20260313-001',
                      '2026-03-14T08:00:00Z',
                      2,
                      92,
                      88,
                      95,
                      300,
                      30,
                      80,
                    ],
                  ],
                },
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderDashboardOverview();

    expect(await screen.findByText('全球态势板')).toBeInTheDocument();
    expect(await screen.findByText('24 小时请求趋势')).toBeInTheDocument();
    expect(await screen.findByText('来源分布')).toBeInTheDocument();
    expect(await screen.findByText('Top Domain')).toBeInTheDocument();
    expect(await screen.findByText('Top 节点榜单')).toBeInTheDocument();
    expect(await screen.findByText('节点健康列表')).toBeInTheDocument();
  });

  it('renders empty state when dashboard arrays are returned as null', async () => {
    stubMatchMedia();

    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);

        if (url.includes('/dashboard/overview')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: {
                  generated_at: '2026-03-14T08:00:00Z',
                  summary: {
                    total_nodes: 0,
                    online_nodes: 0,
                    offline_nodes: 0,
                    pending_nodes: 0,
                    unhealthy_nodes: 0,
                  },
                  traffic: {
                    request_count: 0,
                    unique_visitors: 0,
                    error_count: 0,
                    estimated_qps: 0,
                    reported_nodes: 0,
                  },
                  capacity: {
                    average_cpu_usage_percent: 0,
                    average_memory_usage_percent: 0,
                    high_cpu_nodes: 0,
                    high_memory_nodes: 0,
                    high_storage_nodes: 0,
                  },
                  distributions: {
                    source_countries: null,
                    status_codes: null,
                    top_domains: null,
                  },
                  trends: {
                    traffic_24h: null,
                    capacity_24h: null,
                    network_24h: null,
                    disk_io_24h: null,
                  },
                  nodes: null,
                },
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderDashboardOverview();

    expect(await screen.findByText('暂无节点')).toBeInTheDocument();
    expect(await screen.findByText('暂无来源分布数据')).toBeInTheDocument();
  });
});
