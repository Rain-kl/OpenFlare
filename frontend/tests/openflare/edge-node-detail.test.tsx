import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, within } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { EdgeNodeDetail } from '@/app/(main)/nodes/components/edge-node-detail';
import {
  NodeService,
  type NodeItem,
  type NodeObservability as NodeObservabilityData,
} from '@/lib/services/openflare';
import en from '@/messages/en.json';

let mockSearchParams = new URLSearchParams();
const mockReplace = vi.fn();
const mockPush = vi.fn();

vi.mock('next/navigation', () => ({
  useSearchParams: () => mockSearchParams,
  usePathname: () => '/nodes/detail',
  useRouter: () => ({ replace: mockReplace, push: mockPush }),
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/services/openflare')>()),
  NodeService: {
    getObservability: vi.fn(),
    updateNode: vi.fn(),
    requestForceSync: vi.fn(),
    requestEdgeRestart: vi.fn(),
    requestAgentUpdate: vi.fn(),
    unquarantine: vi.fn(),
    deleteNode: vi.fn(),
    cleanupHealthEvents: vi.fn(),
  },
}));

vi.mock('@/app/(main)/nodes/components/node-hardware-overview', () => ({
  NodeHardwareOverview: () => <div data-testid='mock-hardware' />,
}));
vi.mock('@/app/(main)/nodes/components/node-load-dashboard', () => ({
  NodeLoadDashboard: () => <div data-testid='mock-load-dashboard' />,
}));
vi.mock('@/app/(main)/nodes/components/node-ping-panel', () => ({
  NodePingPanel: () => <div data-testid='mock-ping-panel' />,
}));
vi.mock('@/app/(main)/components/dashboard/traffic-trend-chart', () => ({
  TrafficTrendChart: () => <div data-testid='mock-traffic-trend' />,
}));
vi.mock('@/app/(main)/nodes/components/network-trend-chart', () => ({
  NetworkTrendChart: () => <div data-testid='mock-network-trend' />,
}));
vi.mock('@/app/(main)/nodes/components/disk-io-trend-chart', () => ({
  DiskIOTrendChart: () => <div data-testid='mock-disk-trend' />,
}));
vi.mock('@/app/(main)/nodes/components/install-command', () => ({
  InstallCommand: () => <div data-testid='mock-install-command' />,
}));
vi.mock('@/app/(main)/nodes/components/node-security-section', () => ({
  NodeSecuritySection: () => <div data-testid='mock-security-section' />,
  NodeQuarantineDialog: () => null,
}));
vi.mock('@/app/(main)/nodes/components/agent-update-dialog', () => ({
  AgentUpdateDialog: () => null,
}));
vi.mock('@/app/(main)/nodes/components/node-editor-dialog', () => ({
  NodeEditorDialog: () => null,
}));

function makeNode(): NodeItem {
  return {
    id: 1,
    node_id: 'node-4a04009084c915cb51211f3908704867',
    name: 'US-DMIT',
    ip: '198.51.100.10',
    geo_name: 'Los Angeles',
    node_type: 'edge_node',
    status: 'online',
    openresty_status: 'healthy',
    version: '1.0.3-canary.88',
    ext_version: '2.0.0',
    current_version: 'edge-live-r34',
    latest_apply_result: 'success',
    latest_apply_at: '2026-11-25T00:30:00Z',
    last_seen_at: '0001-01-01T00:00:00Z',
    auto_update_enabled: false,
    update_requested: false,
    update_tag: '',
    restart_openresty_requested: false,
    openresty_message: '',
    last_error: '',
    ip_manual_override: false,
    created_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-25T00:30:00Z',
  } as unknown as NodeItem;
}

function makeObservability(): NodeObservabilityData {
  return {
    node_id: 'node-4a04009084c915cb51211f3908704867',
    profile: null,
    metric_snapshots: [
      {
        captured_at: '2026-11-25T00:35:00Z',
        cpu_usage_percent: 3.5,
        memory_used_bytes: 512,
        memory_total_bytes: 2048,
        storage_used_bytes: 26,
        storage_total_bytes: 100,
      },
    ],
    health_events: [],
    analytics: {
      traffic: {
        request_count: 435,
        unique_visitor_count: 72,
        error_rate_percent: 0,
      },
      health: {
        has_capacity_risk: false,
      },
      distributions: {
        status_codes: [],
        top_domains: [],
        source_countries: [{ key: 'Romania', value: 259 }],
      },
    },
    trends: {
      traffic_24h: [],
      network_24h: [],
      disk_io_24h: [],
    },
  } as unknown as NodeObservabilityData;
}

function renderDetail(tab: 'overview' | 'manage' = 'overview') {
  mockSearchParams = new URLSearchParams(
    tab === 'manage' ? 'id=1&tab=manage' : 'id=1',
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <NextIntlClientProvider locale='en' messages={en} timeZone='UTC'>
      <QueryClientProvider client={client}>
        <EdgeNodeDetail node={makeNode()} />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(NodeService.getObservability).mockResolvedValue(
    makeObservability(),
  );
});

afterEach(() => {
  cleanup();
});

describe('EdgeNodeDetail card layout swap', () => {
  it('shows the 4 diagnostic cards in the top hero and omits them from the overview tab body', async () => {
    const { container } = renderDetail('overview');

    const hero = container.querySelector('section');
    expect(hero).not.toBeNull();
    const heroScope = within(hero as HTMLElement);

    await heroScope.findByText('435');
    expect(heroScope.getByText('Runtime diagnosis')).toBeInTheDocument();
    expect(heroScope.getByText('Stable')).toBeInTheDocument();
    expect(heroScope.getByText('No active health events')).toBeInTheDocument();
    expect(heroScope.getByText('Window requests')).toBeInTheDocument();
    expect(heroScope.getByText('Capacity pressure')).toBeInTheDocument();
    expect(heroScope.getByText('Normal')).toBeInTheDocument();
    expect(heroScope.getByText('Source signal')).toBeInTheDocument();
    expect(heroScope.getByText('Romania')).toBeInTheDocument();

    expect(heroScope.queryByText('Node ID')).not.toBeInTheDocument();
    expect(heroScope.queryByText('Agent version')).not.toBeInTheDocument();
    expect(heroScope.queryByText('Current config')).not.toBeInTheDocument();
    expect(heroScope.queryByText('Last heartbeat')).not.toBeInTheDocument();

    expect(screen.getAllByText('Runtime diagnosis')).toHaveLength(1);
    expect(screen.getAllByText('Window requests')).toHaveLength(1);
    expect(screen.getAllByText('Capacity pressure')).toHaveLength(1);
    expect(screen.getAllByText('Source signal')).toHaveLength(1);
  });

  it('places the 4 identity and deployment KPI cards inside the Status & Deployment tab', async () => {
    const { container } = renderDetail('manage');

    const hero = container.querySelector('section');
    expect(hero).not.toBeNull();
    await within(hero as HTMLElement).findByText('435');

    const managePanel = screen.getByRole('tabpanel');
    const kpiGrid = managePanel.querySelector(
      'div.grid.gap-3.sm\\:grid-cols-2.xl\\:grid-cols-4',
    );
    expect(kpiGrid).not.toBeNull();
    const kpiScope = within(kpiGrid as HTMLElement);

    expect(kpiScope.getByText('Node ID')).toBeInTheDocument();
    expect(
      kpiScope.getByText('node-4a04009084c915cb51211f3908704867'),
    ).toBeInTheDocument();
    expect(kpiScope.getByText('Agent version')).toBeInTheDocument();
    expect(kpiScope.getByText('1.0.3-canary.88')).toBeInTheDocument();
    expect(kpiScope.getByText('Current config')).toBeInTheDocument();
    expect(kpiScope.getByText('edge-live-r34')).toBeInTheDocument();
    expect(kpiScope.getByText('Last heartbeat')).toBeInTheDocument();
    expect(kpiScope.getByText('N/A')).toBeInTheDocument();
  });
});
