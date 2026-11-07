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
vi.mock('@/app/(main)/nodes/components/relay-node-detail', () => ({
  RelayNodeDetail: () => null,
}));
vi.mock('@/app/(main)/nodes/components/tunnel-node-detail', () => ({
  TunnelNodeDetail: () => null,
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
      page_size: query.page_size ?? 20,
      total: 41,
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
    expect.objectContaining({ page: 2, page_size: 20 }),
  );
  expect(NodeService.listNodes).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText('Search nodes'), {
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
      page_size: 20,
      total: deleted ? 20 : 21,
      results:
        deleted && query.page === 2 ? [] : [node(query.page === 2 ? 21 : 1)],
    }),
  );
  vi.mocked(NodeService.deleteNode).mockImplementation(async () => {
    deleted = true;
  });
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await screen.findByText('Edge 21');
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

it('resets the page immediately when the URL node type changes', async () => {
  const page = renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await screen.findByText('Edge 2');
  params = new URLSearchParams('filter=relay');
  page.rerender(
    <NextIntlClientProvider locale='en' messages={en} timeZone='UTC'>
      <QueryClientProvider client={page.client}>
        <NodesPageClient />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
  await waitFor(() =>
    expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 1, node_type: 'tunnel_relay' }),
    ),
  );
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
  const input = screen.getByLabelText('Search nodes');
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
      : { page: 1, page_size: 20, total: 41, results: [node(1)] },
  );
  renderPage();
  await screen.findByText('Edge 1');
  expect(screen.getByRole('link', { name: 'Details' })).toHaveAttribute(
    'href',
    '/nodes/detail?id=1',
  );
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled(),
  );
  expect(screen.getByText('Edge 1')).toBeInTheDocument();
  expect(screen.getByText('Page 1 of 3 · 41 nodes')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Details' })).toBeDisabled();
  expect(
    screen.queryByRole('link', { name: 'Details' }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  expect(NodeService.deleteNode).not.toHaveBeenCalled();
  await act(async () => {
    pending.resolve({ page: 2, page_size: 20, total: 41, results: [node(2)] });
  });
  await screen.findByText('Edge 2');
  expect(screen.queryByText('Edge 1')).not.toBeInTheDocument();
  expect(screen.getByText('Page 2 of 3 · 41 nodes')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Edit' })).toBeEnabled();
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
      : { page: 1, page_size: 20, total: 41, results: [node(1)] },
  );
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.change(screen.getByLabelText('Search nodes'), {
    target: { value: 'another' },
  });
  await waitFor(() =>
    expect(NodeService.listNodePage).toHaveBeenLastCalledWith(
      expect.objectContaining({ search: 'another' }),
    ),
  );
  expect(screen.queryByText('Edge 1')).not.toBeInTheDocument();
  expect(screen.queryByText('Page 1 of 3 · 41 nodes')).not.toBeInTheDocument();
  expect(
    screen.queryByRole('link', { name: 'Details' }),
  ).not.toBeInTheDocument();
  await act(async () => {
    pending.resolve({ page: 1, page_size: 20, total: 1, results: [node(9)] });
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
      : { page: 1, page_size: 20, total: 41, results: [node(1)] },
  );
  renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled(),
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
    page_size: 20,
    total: 41,
    results: [node(2)],
  });
  fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
  await screen.findByText('Edge 2');
  expect(screen.getByText('Page 2 of 3 · 41 nodes')).toBeInTheDocument();
});

it('closes an open node confirmation when a background refresh clamps a depleted page', async () => {
  let removed = false;
  vi.mocked(NodeService.listNodePage).mockImplementation(
    async (query = {}) => ({
      page: query.page ?? 1,
      page_size: 20,
      total: removed ? 20 : 21,
      results:
        removed && query.page === 2 ? [] : [node(query.page === 2 ? 21 : 1)],
    }),
  );
  const { client } = renderPage();
  await screen.findByText('Edge 1');
  fireEvent.click(screen.getByRole('button', { name: 'Next' }));
  await screen.findByText('Edge 21');
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
