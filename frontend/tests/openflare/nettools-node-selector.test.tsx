import { useState, type ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NetworkToolsDnsPageClient } from '@/app/(main)/network-tools/dns/page-client';
import { NetworkToolsPingPageClient } from '@/app/(main)/network-tools/ping/page-client';
import {
  NodeSelector,
  type NodeSelection,
} from '@/app/(main)/network-tools/shared/node-selector';
import { NetworkToolsTraceroutePageClient } from '@/app/(main)/network-tools/traceroute/page-client';
import {
  NetworkToolsService,
  NodeService,
  type NodeItem,
  type NodeType,
} from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    NodeService: {
      listNodes: vi.fn(),
    },
    NetworkToolsService: {
      dispatchPing: vi.fn(),
      dispatchTraceroute: vi.fn(),
      queryDNS: vi.fn(),
      getTask: vi.fn(),
      listTracerouteHistory: vi.fn(),
    },
  };
});

function makeNode(
  overrides: Partial<NodeItem> & Pick<NodeItem, 'node_id' | 'name'>,
): NodeItem {
  return {
    id: 1,
    node_type: 'edge_node',
    ip: '203.0.113.10',
    ip_manual_override: false,
    geo_name: 'Tokyo, Japan',
    geo_manual_override: false,
    access_token: '',
    auto_update_enabled: false,
    update_requested: false,
    update_channel: 'stable',
    update_tag: '',
    restart_openresty_requested: false,
    version: 'v4.1.0',
    ext_version: '',
    openresty_status: 'healthy',
    openresty_message: '',
    status: 'online',
    last_seen_at: '2026-11-27T13:00:00Z',
    current_version: 'v4.1.0',
    last_error: '',
    latest_apply_result: 'success',
    latest_apply_message: '',
    latest_apply_checksum: '',
    latest_main_config_checksum: '',
    latest_route_config_checksum: '',
    latest_support_file_count: 0,
    created_at: '2026-11-27T00:00:00Z',
    updated_at: '2026-11-27T13:00:00Z',
    ...overrides,
  };
}

const mockNodes: NodeItem[] = [
  makeNode({
    id: 1,
    node_id: 'edge-la-1',
    name: 'US-LA-Edge-01',
    asn: 906,
    geo_name: 'Los Angeles, United States',
    status: 'online',
  }),
  makeNode({
    id: 2,
    node_id: 'edge-jp-1',
    name: 'JP-NB-R',
    asn: 206069,
    geo_name: 'Tokyo, Japan',
    status: 'online',
  }),
  makeNode({
    id: 3,
    node_id: 'edge-de-off',
    name: 'DE-FRA-Offline',
    asn: 24940,
    geo_name: 'Frankfurt, Germany',
    status: 'offline',
  }),
  makeNode({
    id: 4,
    node_id: 'relay-skip-1',
    name: 'Relay-Should-Skip',
    node_type: 'tunnel_relay' as unknown as NodeType,
    status: 'online',
  }),
];

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
      <QueryClientProvider client={client}>{ui}</QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

function ControlledNodeSelector({
  initialSelection = null,
  onChangeSpy,
}: {
  initialSelection?: NodeSelection;
  onChangeSpy?: (next: NodeSelection) => void;
}) {
  const [selection, setSelection] = useState<NodeSelection>(initialSelection);
  return (
    <NodeSelector
      selectedIds={selection}
      onChange={(next) => {
        setSelection(next);
        onChangeSpy?.(next);
      }}
    />
  );
}

describe('NetworkTools searchable edge node dropdown & merged form cards', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.HTMLElement.prototype.hasPointerCapture = vi.fn(() => false);
    window.HTMLElement.prototype.scrollIntoView = vi.fn();
    vi.mocked(NodeService.listNodes).mockResolvedValue(mockNodes);
    vi.mocked(NetworkToolsService.dispatchPing).mockResolvedValue({
      task_id: 'ping-1',
      kind: 'ping_batch',
      target: '1.1.1.1',
      status: 'completed',
      total_nodes: 2,
      completed_nodes: 2,
      created_at: '2026-11-27T13:00:00Z',
      expires_at: '2026-11-27T14:00:00Z',
      nodes: [],
    });
    vi.mocked(NetworkToolsService.dispatchTraceroute).mockResolvedValue({
      task_id: 'trace-1',
      kind: 'traceroute',
      target: '1.1.1.1',
      status: 'completed',
      total_nodes: 2,
      completed_nodes: 2,
      created_at: '2026-11-27T13:00:00Z',
      expires_at: '2026-11-27T14:00:00Z',
      nodes: [],
    });
    vi.mocked(NetworkToolsService.getTask).mockImplementation(
      async (taskId) => ({
        task_id: taskId,
        kind: taskId.startsWith('trace') ? 'traceroute' : 'ping_batch',
        target: '1.1.1.1',
        status: 'completed',
        total_nodes: 2,
        completed_nodes: 2,
        created_at: '2026-11-27T13:00:00Z',
        expires_at: '2026-11-27T14:00:00Z',
        nodes: [],
      }),
    );
    vi.mocked(NetworkToolsService.queryDNS).mockResolvedValue({
      mode: 'nodes',
      task: {
        task_id: 'dns-1',
        kind: 'dns_query',
        target: 'example.com',
        status: 'completed',
        total_nodes: 2,
        completed_nodes: 2,
        created_at: '2026-11-27T13:00:00Z',
        expires_at: '2026-11-27T14:00:00Z',
        nodes: [],
      },
    });
  });

  it('defaults NodeSelector to all online edge nodes and supports search, toggle, clear, and select-all', async () => {
    const user = userEvent.setup();
    const onChangeSpy = vi.fn();
    renderWithProviders(<ControlledNodeSelector onChangeSpy={onChangeSpy} />);

    const trigger = await screen.findByRole('button', {
      name: /执行探测的边缘节点/,
    });
    await waitFor(() => expect(trigger).toHaveTextContent('全部在线 (2)'));

    await user.click(trigger);

    expect(await screen.findByText('在线边缘节点')).toBeInTheDocument();
    expect(screen.getByText('离线 / 待接入节点')).toBeInTheDocument();
    expect(screen.getByText('US-LA-Edge-01')).toBeInTheDocument();
    expect(screen.getByText('JP-NB-R')).toBeInTheDocument();
    expect(screen.queryByText('Relay-Should-Skip')).not.toBeInTheDocument();

    // Search filters by ASN and reveals matching node
    const searchInput = screen.getByPlaceholderText('搜索节点名称、地区或 ASN');
    await user.type(searchInput, 'AS206069');
    expect(screen.getByText('JP-NB-R')).toBeInTheDocument();
    expect(screen.queryByText('US-LA-Edge-01')).not.toBeInTheDocument();

    await user.clear(searchInput);
    expect(screen.getByText('US-LA-Edge-01')).toBeInTheDocument();

    // Unchecking one online node transitions to explicit subset
    await user.click(
      screen.getByRole('checkbox', { name: '选择节点 US-LA-Edge-01' }),
    );
    expect(onChangeSpy).toHaveBeenLastCalledWith(['edge-jp-1']);
    expect(trigger).toHaveTextContent('已选 1 / 在线 2');

    // Clear resets to empty array
    await user.click(screen.getByRole('button', { name: '清空选择' }));
    expect(onChangeSpy).toHaveBeenLastCalledWith([]);
    expect(trigger).toHaveTextContent('已选 0 / 在线 2');

    // Select all online restores default null state
    await user.click(screen.getByRole('button', { name: '全选在线节点' }));
    expect(onChangeSpy).toHaveBeenLastCalledWith(null);
    expect(trigger).toHaveTextContent('全部在线 (2)');
  });

  it('merges ping form and node dropdown into a single card and defaults to all online nodes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<NetworkToolsPingPageClient />);

    const trigger = await screen.findByRole('button', {
      name: /执行探测的边缘节点/,
    });
    await waitFor(() => expect(trigger).toHaveTextContent('全部在线 (2)'));

    const form = screen.getByLabelText('目标地址').closest('form');
    expect(form).not.toBeNull();
    expect(form).toContainElement(trigger);

    await user.type(screen.getByLabelText('目标地址'), '1.1.1.1');
    const submitBtn = screen.getByRole('button', { name: '开始探测' });
    await user.click(submitBtn);

    await waitFor(() =>
      expect(NetworkToolsService.dispatchPing).toHaveBeenCalledWith(
        expect.objectContaining({
          target: '1.1.1.1',
          node_ids: undefined,
        }),
      ),
    );

    // Clearing all selected nodes disables submission
    await user.click(trigger);
    await user.click(screen.getByRole('button', { name: '清空选择' }));
    expect(submitBtn).toBeDisabled();
  });

  it('merges traceroute form and node dropdown into a single card and defaults to all online nodes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<NetworkToolsTraceroutePageClient />);

    const trigger = await screen.findByRole('button', {
      name: /执行探测的边缘节点/,
    });
    await waitFor(() => expect(trigger).toHaveTextContent('全部在线 (2)'));

    const form = screen.getByLabelText('目标地址').closest('form');
    expect(form).not.toBeNull();
    expect(form).toContainElement(trigger);

    await user.type(screen.getByLabelText('目标地址'), '1.1.1.1');
    const submitBtn = screen.getByRole('button', { name: '开始探测' });
    await user.click(submitBtn);

    await waitFor(() =>
      expect(NetworkToolsService.dispatchTraceroute).toHaveBeenCalledWith(
        expect.objectContaining({
          target: '1.1.1.1',
          node_ids: undefined,
        }),
      ),
    );
  });

  it('passes resolved online node IDs in multi-node DNS mode inside the merged card', async () => {
    const user = userEvent.setup();
    renderWithProviders(<NetworkToolsDnsPageClient />);

    // Default direct mode hides node selector
    expect(
      screen.queryByRole('button', { name: /执行探测的边缘节点/ }),
    ).not.toBeInTheDocument();

    // Switch execution mode to multi-node
    const modeTrigger = screen
      .getAllByRole('combobox')
      .find((el) => el.textContent?.includes('主控直接查询'));
    expect(modeTrigger).toBeDefined();
    await user.click(modeTrigger!);
    await user.click(
      await screen.findByRole('option', { name: '多边缘节点并发查询' }),
    );

    const trigger = await screen.findByRole('button', {
      name: /执行探测的边缘节点/,
    });
    await waitFor(() => expect(trigger).toHaveTextContent('全部在线 (2)'));

    const form = screen.getByLabelText('查询域名').closest('form');
    expect(form).not.toBeNull();
    expect(form).toContainElement(trigger);

    await user.type(screen.getByLabelText('查询域名'), 'example.com');
    await user.click(screen.getByRole('button', { name: '执行查询' }));

    await waitFor(() =>
      expect(NetworkToolsService.queryDNS).toHaveBeenCalledWith(
        expect.objectContaining({
          domain: 'example.com',
          node_ids: ['edge-la-1', 'edge-jp-1'],
        }),
      ),
    );
  });
});
