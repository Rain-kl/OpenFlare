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
                    active_alerts: 1,
                    lagging_nodes: 1,
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
                  config: {
                    active_version: '20260314-001',
                    lagging_nodes: 1,
                    pending_nodes: 0,
                  },
                  risk: {
                    critical_alerts: 1,
                    warning_alerts: 2,
                    info_alerts: 0,
                    offline_nodes: 0,
                    unhealthy_nodes: 1,
                    lagging_nodes: 1,
                    high_cpu_nodes: 1,
                    high_memory_nodes: 1,
                    high_storage_nodes: 1,
                  },
                  peaks: {
                    peak_request_hour: {
                      bucket_started_at: '2026-03-14T08:00:00Z',
                      request_count: 900,
                      error_count: 36,
                    },
                    peak_error_hour: {
                      bucket_started_at: '2026-03-14T09:00:00Z',
                      request_count: 400,
                      error_count: 60,
                    },
                    busiest_node: {
                      node_id: 'node-a',
                      node_name: 'edge-a',
                      request_count: 600,
                      error_count: 6,
                      cpu_usage_percent: 45,
                      active_event_count: 0,
                      openresty_status: 'healthy',
                      storage_usage_percent: 60,
                    },
                    riskiest_node: {
                      node_id: 'node-b',
                      node_name: 'edge-b',
                      request_count: 300,
                      error_count: 30,
                      cpu_usage_percent: 92,
                      active_event_count: 2,
                      openresty_status: 'unhealthy',
                      storage_usage_percent: 95,
                    },
                  },
                  distributions: {
                    source_countries: [
                      { key: 'CN', value: 440 },
                      { key: 'US', value: 320 },
                      { key: 'SG', value: 140 },
                    ],
                    status_codes: [
                      { key: '200', value: 820 },
                      { key: '502', value: 24 },
                      { key: '500', value: 12 },
                    ],
                    top_domains: [
                      { key: 'app.example.com', value: 580 },
                      { key: 'api.example.com', value: 220 },
                    ],
                  },
                  trends: {
                    traffic_24h: Array.from({ length: 24 }, (_, index) => ({
                      bucket_started_at: `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      request_count: index * 10,
                      error_count: index,
                      unique_visitor_count: index * 3,
                    })),
                    capacity_24h: Array.from({ length: 24 }, (_, index) => ({
                      bucket_started_at: `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      average_cpu_usage_percent: index,
                      average_memory_usage_percent: index + 10,
                      reported_nodes: 2,
                    })),
                    network_24h: Array.from({ length: 24 }, (_, index) => ({
                      bucket_started_at: `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      network_rx_bytes: index * 100,
                      network_tx_bytes: index * 120,
                      openresty_rx_bytes: index * 140,
                      openresty_tx_bytes: index * 160,
                      reported_nodes: 2,
                    })),
                    disk_io_24h: Array.from({ length: 24 }, (_, index) => ({
                      bucket_started_at: `2026-03-13T${String(index).padStart(2, '0')}:00:00Z`,
                      disk_read_bytes: index * 50,
                      disk_write_bytes: index * 70,
                      reported_nodes: 2,
                    })),
                  },
                  nodes: [
                    {
                      id: 1,
                      node_id: 'node-a',
                      name: 'edge-a',
                      geo_name: 'Shanghai',
                      geo_latitude: 31.2304,
                      geo_longitude: 121.4737,
                      status: 'online',
                      openresty_status: 'healthy',
                      current_version: '20260314-001',
                      last_seen_at: '2026-03-14T08:00:00Z',
                      active_event_count: 0,
                      cpu_usage_percent: 45,
                      memory_usage_percent: 50,
                      storage_usage_percent: 60,
                      request_count: 600,
                      error_count: 6,
                      unique_visitor_count: 120,
                    },
                    {
                      id: 2,
                      node_id: 'node-b',
                      name: 'edge-b',
                      geo_name: 'San Francisco',
                      geo_latitude: 37.7749,
                      geo_longitude: -122.4194,
                      status: 'online',
                      openresty_status: 'unhealthy',
                      current_version: '20260313-001',
                      last_seen_at: '2026-03-14T08:00:00Z',
                      active_event_count: 2,
                      cpu_usage_percent: 92,
                      memory_usage_percent: 88,
                      storage_usage_percent: 95,
                      request_count: 300,
                      error_count: 30,
                      unique_visitor_count: 80,
                    },
                  ],
                  active_alerts: [
                    {
                      node_id: 'node-b',
                      node_name: 'edge-b',
                      event_type: 'openresty_unhealthy',
                      severity: 'critical',
                      message: 'reload failed',
                      last_triggered_at: '2026-03-14T07:59:00Z',
                      status: 'active',
                    },
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
    expect(await screen.findByText('系统健康摘要')).toBeInTheDocument();
    expect(await screen.findByText('24 小时请求趋势')).toBeInTheDocument();
    expect(await screen.findByText('来源分布')).toBeInTheDocument();
    expect(await screen.findByText('Top Domain')).toBeInTheDocument();
    expect(await screen.findByText('Top 节点榜单')).toBeInTheDocument();
    expect(await screen.findByText('处置建议')).toBeInTheDocument();
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
                    active_alerts: 0,
                    lagging_nodes: 0,
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
                  config: {
                    active_version: '',
                    lagging_nodes: 0,
                    pending_nodes: 0,
                  },
                  risk: {
                    critical_alerts: 0,
                    warning_alerts: 0,
                    info_alerts: 0,
                    offline_nodes: 0,
                    unhealthy_nodes: 0,
                    lagging_nodes: 0,
                    high_cpu_nodes: 0,
                    high_memory_nodes: 0,
                    high_storage_nodes: 0,
                  },
                  peaks: {
                    peak_request_hour: {
                      bucket_started_at: '',
                      request_count: 0,
                      error_count: 0,
                    },
                    peak_error_hour: {
                      bucket_started_at: '',
                      request_count: 0,
                      error_count: 0,
                    },
                    busiest_node: null,
                    riskiest_node: null,
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
                  active_alerts: null,
                },
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderDashboardOverview();

    expect(await screen.findByText('暂无活动异常')).toBeInTheDocument();
    expect(await screen.findByText('暂无节点')).toBeInTheDocument();
    expect(await screen.findByText('暂无来源分布数据')).toBeInTheDocument();
  });
});
