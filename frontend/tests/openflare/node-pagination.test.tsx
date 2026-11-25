import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import { NodesPageClient } from '@/app/(main)/nodes/page-client';
import { NodeDetailPageClient } from '@/app/(main)/nodes/detail/page-client';
import { NodeService, type NodeItem } from '@/lib/services/openflare';
import en from '@/messages/en.json';

let params = new URLSearchParams();
vi.mock('next/navigation', () => ({ useSearchParams: () => params }));
vi.mock('@/lib/services/openflare', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/services/openflare')>()),
  NodeService: {
    listNodePage: vi.fn(),
    listNodes: vi.fn(),
    getNode: vi.fn(),
    getPingRecords: vi.fn(),
    deleteNode: vi.fn(),
  },
}));
vi.mock('@/app/(main)/nodes/components/node-editor-dialog', () => ({
  NodeEditorDialog: () => null,
}));
vi.mock('@/app/(main)/nodes/components/edge-node-detail', () => ({
  EdgeNodeDetail: ({ node }: { node: NodeItem }) => (
    <div>Node detail {node.id}</div>
  ),
}));

function node(id: number): NodeItem {
  return { id, name: `Edge ${id}`, node_type: 'edge_node' } as NodeItem;
}
function renderPage(detail = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const result = render(
    <NextIntlClientProvider locale='en' messages={en} timeZone='UTC'>
      <QueryClientProvider client={client}>
        {detail ? <NodeDetailPageClient /> : <NodesPageClient />}
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
  return { ...result, client };
}

beforeEach(() => {
  params = new URLSearchParams();
  vi.clearAllMocks();
  vi.mocked(NodeService.listNodePage).mockImplementation(
    async (query = {}) => ({
      page: query.page ?? 1,
      page_size: query.page_size ?? 50,
      total: 101,
      results: [node(query.page ?? 1)],
    }),
  );
});
afterEach(() => {
  cleanup();
});

it('fetches only visible pages and debounces server search while resetting the page', async () => {
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await screen.findByText('Edge 2');
  expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
    expect.objectContaining({
      page: 2,
      page_size: 50,
      include_monitoring: true,
    }),
  );
  expect(NodeService.listNodes).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Search nodes' }));
  fireEvent.change(screen.getByRole('textbox', { name: 'Search nodes' }), {
    target: { value: 'test' },
  });
  await waitFor(() =>
    expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 1, search: 'test' }),
    ),
  );
});

it('refetches the previous valid page after deleting the only node on the last page', async () => {
  let deleted = false;
  vi.mocked(NodeService.listNodePage).mockImplementation(
    async (query = {}) => ({
      page: query.page ?? 1,
      page_size: 50,
      total: deleted ? 50 : 51,
      results:
        deleted && query.page === 2 ? [] : [node(query.page === 2 ? 51 : 1)],
    }),
  );
  vi.mocked(NodeService.deleteNode).mockImplementation(async () => {
    deleted = true;
  });
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await screen.findByText('Edge 51');
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
  fireEvent.click(
    within(await screen.findByRole('alertdialog')).getByRole('button', {
      name: 'Delete',
    }),
  );
  await waitFor(() =>
    expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 1 }),
    ),
  );
  await screen.findByText('Edge 1');
});

it('keeps page errors retryable through refresh', async () => {
  vi.mocked(NodeService.listNodePage).mockRejectedValueOnce(
    new Error('temporary failure'),
  );
  renderPage();
  await screen.findByText('temporary failure');
  fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
  await screen.findByText('Edge 1');
});

it('fetches one detail and refetches it on shared node invalidation', async () => {
  params = new URLSearchParams('id=42');
  vi.mocked(NodeService.getNode).mockResolvedValue(node(42));
  const { client } = renderPage(true);
  await screen.findByText('Node detail 42');
  expect(NodeService.getNode).toHaveBeenCalledWith(42);
  expect(NodeService.listNodes).not.toHaveBeenCalled();
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['openflare', 'nodes'] });
  });
  expect(NodeService.getNode).toHaveBeenCalledTimes(2);
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

it('validates the UTF-8 search limit before requesting and recovers after shortening', async () => {
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Search nodes' }));
  const input = screen.getByRole('textbox', { name: 'Search nodes' });
  for (const search of ['a'.repeat(256), '汉'.repeat(85) + 'a']) {
    fireEvent.change(input, { target: { value: search } });
    await waitFor(() =>
      expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
        expect.objectContaining({ search }),
      ),
    );
  }
  const calls = vi.mocked(NodeService.listNodePage).mock.calls.length;
  fireEvent.change(input, { target: { value: '汉'.repeat(86) } });
  expect(input).toHaveAttribute('aria-invalid', 'true');
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Search text is too long',
  );
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 350));
  });
  expect(NodeService.listNodePage).toHaveBeenCalledTimes(calls);
  fireEvent.change(input, { target: { value: 'short' } });
  await waitFor(() =>
    expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
      expect.objectContaining({ search: 'short' }),
    ),
  );
  expect(input).not.toHaveAttribute('aria-invalid');
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});

it('retains the loaded page and total during navigation without leaving stale node actions', async () => {
  const pending =
    deferred<Awaited<ReturnType<typeof NodeService.listNodePage>>>();
  vi.mocked(NodeService.listNodePage).mockImplementation(async (query = {}) =>
    query.page === 2
      ? pending.promise
      : { page: 1, page_size: 50, total: 101, results: [node(1)] },
  );
  renderPage();
  await screen.findByText('Edge 1');
  expect(screen.getByRole('link', { name: 'Details' })).toHaveAttribute(
    'href',
    '/nodes/detail?id=1',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled(),
  );
  expect(screen.getByText('Edge 1')).toBeInTheDocument();
  expect(screen.getByText('Page 1 of 3 · 101 nodes')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Details' })).toBeDisabled();
  expect(
    screen.queryByRole('link', { name: 'Details' }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  expect(NodeService.deleteNode).not.toHaveBeenCalled();
  await act(async () => {
    pending.resolve({ page: 2, page_size: 50, total: 101, results: [node(2)] });
  });
  await screen.findByText('Edge 2');
  expect(screen.queryByText('Edge 1')).not.toBeInTheDocument();
  expect(screen.getByText('Page 2 of 3 · 101 nodes')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Delete' })).toBeEnabled();
  expect(screen.getByRole('link', { name: 'Details' })).toHaveAttribute(
    'href',
    '/nodes/detail?id=2',
  );
});

it('does not reuse nodes or totals from a different search while it is loading', async () => {
  const pending =
    deferred<Awaited<ReturnType<typeof NodeService.listNodePage>>>();
  vi.mocked(NodeService.listNodePage).mockImplementation(async (query = {}) =>
    query.search
      ? pending.promise
      : { page: 1, page_size: 50, total: 101, results: [node(1)] },
  );
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Search nodes' }));
  fireEvent.change(screen.getByRole('textbox', { name: 'Search nodes' }), {
    target: { value: 'another' },
  });
  await waitFor(() =>
    expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
      expect.objectContaining({ search: 'another' }),
    ),
  );
  expect(screen.queryByText('Edge 1')).not.toBeInTheDocument();
  expect(screen.queryByText('Page 1 of 3 · 101 nodes')).not.toBeInTheDocument();
  expect(
    screen.queryByRole('link', { name: 'Details' }),
  ).not.toBeInTheDocument();
  await act(async () => {
    pending.resolve({ page: 1, page_size: 50, total: 1, results: [node(9)] });
  });
  await screen.findByText('Edge 9');
  expect(screen.getByText('Page 1 of 1 · 1 nodes')).toBeInTheDocument();
});

it('drops placeholder actions and unknown totals after a failed page request and allows retry', async () => {
  const pending =
    deferred<Awaited<ReturnType<typeof NodeService.listNodePage>>>();
  vi.mocked(NodeService.listNodePage).mockImplementation(async (query = {}) =>
    query.page === 2
      ? pending.promise
      : { page: 1, page_size: 50, total: 101, results: [node(1)] },
  );
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled(),
  );
  await act(async () => {
    pending.reject(new Error('page request failed'));
  });
  await screen.findByText('page request failed');
  expect(screen.queryByText('Edge 1')).not.toBeInTheDocument();
  expect(screen.queryByText(/Page [0-9]+ of/)).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Delete' }),
  ).not.toBeInTheDocument();
  vi.mocked(NodeService.listNodePage).mockResolvedValue({
    page: 2,
    page_size: 50,
    total: 101,
    results: [node(2)],
  });
  fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
  await screen.findByText('Edge 2');
  expect(screen.getByText('Page 2 of 3 · 101 nodes')).toBeInTheDocument();
});

it('closes an open node confirmation when a background refresh clamps a depleted page', async () => {
  let removed = false;
  vi.mocked(NodeService.listNodePage).mockImplementation(
    async (query = {}) => ({
      page: query.page ?? 1,
      page_size: 50,
      total: removed ? 50 : 51,
      results:
        removed && query.page === 2 ? [] : [node(query.page === 2 ? 51 : 1)],
    }),
  );
  const { client } = renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await screen.findByText('Edge 51');
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
  await screen.findByRole('alertdialog');
  removed = true;
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['openflare', 'nodes'] });
  });
  await screen.findByText('Edge 1');
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  expect(NodeService.deleteNode).not.toHaveBeenCalled();
});

it('shows monitoring in both layouts and restores focus when closing search', async () => {
  const monitoredNode = {
    ...node(1),
    status: 'online',
    openresty_status: 'unhealthy',
    version: 'v1.0.0',
    current_version: '1aef2e8',
    monitoring: {
      system: 'Debian 12',
      metric: {
        captured_at: '2026-11-25T00:00:00Z',
        cpu_usage_percent: 15,
        memory_used_bytes: 0,
        memory_total_bytes: 4,
        storage_used_bytes: 1,
        storage_total_bytes: 4,
        net_in: 1024,
        net_out: 0,
      },
      ping: [
        {
          task_id: 1,
          name: 'Tokyo',
          samples: [{ recorded_at: '2026-11-25T00:00:00Z', value: 120 }],
        },
        {
          task_id: 2,
          name: 'Singapore',
          samples: [
            { recorded_at: '2026-11-24T23:59:00Z', value: 121 },
            { recorded_at: '2026-11-25T00:00:00Z', value: 250 },
          ],
        },
        {
          task_id: 3,
          name: 'Osaka',
          samples: [{ recorded_at: '2026-11-25T00:00:00Z', value: 251 }],
        },
      ],
    },
  } satisfies NodeItem;
  vi.mocked(NodeService.listNodePage).mockResolvedValue({
    page: 1,
    page_size: 50,
    total: 2,
    results: [monitoredNode, { ...node(2), status: 'offline' }],
  });
  vi.mocked(NodeService.getPingRecords).mockResolvedValue({
    node_id: 'edge-1',
    hours: 1,
    tasks: [],
    records: [],
  });
  renderPage();

  await screen.findByText('Edge 1');
  expect(
    screen.getByRole('columnheader', { name: 'Versions' }),
  ).toBeInTheDocument();
  const versionCell = screen.getByText('v1.0.0').closest('td');
  expect(versionCell?.querySelectorAll('dt.sr-only')).toHaveLength(2);
  expect(versionCell).toHaveTextContent('1aef2e8');
  expect(screen.getByText('Edge 1').previousElementSibling).toHaveClass(
    'bg-success',
  );
  const onlineRow = screen.getByRole('row', { name: /Edge 1/ });
  expect(within(onlineRow).getByText('Unhealthy')).toBeInTheDocument();
  expect(within(onlineRow).queryByText(/Online/)).not.toBeInTheDocument();
  expect(onlineRow).not.toHaveClass('grayscale');
  expect(screen.getByRole('row', { name: /Edge 2/ })).toHaveClass('grayscale');
  expect(screen.getByText('Edge 2').previousElementSibling).toBeNull();
  expect(
    screen.queryByRole('textbox', { name: 'Search nodes' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('progressbar', { name: 'Memory' })).toHaveAttribute(
    'aria-valuenow',
    '0',
  );
  expect(screen.getByText('120ms')).toHaveClass('text-success');
  expect(screen.getByText('250ms')).toHaveClass('text-warning');
  expect(screen.getByText('251ms')).toHaveClass('text-destructive');
  const sampleStrip = within(onlineRow).getByRole('button', {
    name: /Edge 1 · Latency and packet loss/,
  }).lastElementChild;
  expect(sampleStrip?.children[1].firstElementChild).toHaveClass(
    'bg-warning/75',
  );
  expect(screen.queryByText('Tokyo')).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Edit' }),
  ).not.toBeInTheDocument();
  expect(NodeService.getPingRecords).not.toHaveBeenCalled();

  fireEvent.click(
    screen.getByRole('button', { name: /Edge 1 · Latency and packet loss/ }),
  );
  const dialog = await screen.findByRole('dialog', {
    name: 'Edge 1 · Latency and packet loss',
  });
  await waitFor(() =>
    expect(NodeService.getPingRecords).toHaveBeenCalledWith(1, {
      hours: 1,
      max_points: 400,
    }),
  );
  fireEvent.keyDown(dialog, { key: 'Escape' });
  await waitFor(() => expect(dialog).not.toBeInTheDocument());

  fireEvent.click(screen.getByRole('radio', { name: 'Card view' }));
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(screen.getByText('Debian 12')).toBeInTheDocument();
  expect(screen.getByText('Edge 2').closest('[data-slot="card"]')).toHaveClass(
    'grayscale',
  );
  expect(
    screen.queryByRole('button', { name: 'Edit' }),
  ).not.toBeInTheDocument();
  expect(screen.getByText('1aef2e8')).toBeInTheDocument();
  expect(screen.getByRole('progressbar', { name: 'CPU' })).toHaveAttribute(
    'aria-valuenow',
    '15',
  );
  expect(NodeService.listNodePage).toHaveBeenCalledTimes(1);

  fireEvent.click(screen.getByRole('button', { name: 'Search nodes' }));
  const search = screen.getByRole('textbox', { name: 'Search nodes' });
  expect(search).toHaveFocus();
  fireEvent.keyDown(search, { key: 'Escape' });
  expect(
    screen.queryByRole('textbox', { name: 'Search nodes' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Search nodes' })).toHaveFocus();
});
