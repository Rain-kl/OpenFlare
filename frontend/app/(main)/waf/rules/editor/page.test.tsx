import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {AxiosError, type AxiosResponse} from 'axios';
import {beforeEach, expect, it, vi} from 'vitest';

import type {WAFRule, WAFRuleGraph} from '@/lib/services';

import WAFRuleEditorPage from './page';

const {getRule, saveRuleGraph, updateRuleMeta, listIPGroups, toastError} = vi.hoisted(() => ({
  getRule: vi.fn(),
  saveRuleGraph: vi.fn(),
  updateRuleMeta: vi.fn(),
  listIPGroups: vi.fn().mockResolvedValue([]),
  toastError: vi.fn(),
}));

vi.mock('next/navigation', () => ({useRouter: () => ({push: vi.fn()}), useSearchParams: () => new URLSearchParams('id=9')}));
vi.mock('sonner', () => ({toast: {success: vi.fn(), error: toastError}}));
vi.mock('@/lib/services', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services')>();
  return {...actual, services: {...actual.services, openflareWaf: {getRule, saveRuleGraph, updateRuleMeta, listIPGroups}}};
});
vi.mock('./components/rule-flow-canvas', () => ({RuleFlowCanvas: ({graph, focusTarget, onGraphChange}: {graph: WAFRuleGraph; focusTarget?: {kind: string; id: string}; onGraphChange: (graph: WAFRuleGraph) => void}) => <div><button onClick={() => onGraphChange({...graph, nodes: graph.nodes.map((node) => node.id === 'start' ? {...node, position: {x: 10, y: 0}} : node)})}>修改画布</button>{focusTarget && <span>focus:{focusTarget.kind}:{focusTarget.id}</span>}</div>}));
vi.mock('./components/node-properties', () => ({NodeProperties: () => null}));

const graph: WAFRuleGraph = {schema_version: 1, nodes: [
  {id: 'start', type: 'start', position: {x: 0, y: 0}, config: {}},
  {id: 'allow', type: 'allow', position: {x: 200, y: 0}, config: {}},
], edges: [{id: 'start-allow', source: 'start', source_handle: 'next', target: 'allow'}]};
const rule = {id: 9, name: '边缘防护', enabled: true, is_global: false, graph, revision: 1, applied_site_ids: [], applied_site_count: 0, created_at: '', updated_at: ''} satisfies WAFRule;

function renderPage() {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}, mutations: {retry: false}}});
  return render(<QueryClientProvider client={client}><WAFRuleEditorPage/></QueryClientProvider>);
}

beforeEach(() => {
  getRule.mockReset();
  saveRuleGraph.mockReset();
  updateRuleMeta.mockReset();
  listIPGroups.mockClear();
  toastError.mockReset();
});

it('renders a query error with a working retry action', async () => {
  getRule.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(rule);
  renderPage();
  fireEvent.click(await screen.findByRole('button', {name: '重新加载'}));
  expect(await screen.findByRole('heading', {name: '边缘防护'})).toBeInTheDocument();
  expect(getRule).toHaveBeenCalledTimes(2);
});

it('exposes conflict reload and maps typed server node errors to canvas focus', async () => {
  getRule.mockResolvedValue(rule);
  const conflict = new AxiosError('conflict');
  conflict.response = {status: 409, data: {}, headers: {}, config: {headers: {}}} as AxiosResponse;
  saveRuleGraph.mockRejectedValueOnce(conflict).mockRejectedValueOnce(new Error('规则图无效: 节点 start 的 next 出口未连接'));
  renderPage();
  await screen.findByRole('heading', {name: '边缘防护'});
  fireEvent.click(screen.getByRole('button', {name: '修改画布'}));
  fireEvent.click(screen.getByRole('button', {name: '保存'}));
  expect(await screen.findByRole('button', {name: '重新加载'})).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', {name: '保存'}));
  await waitFor(() => expect(screen.getByText('focus:node:start')).toBeInTheDocument());
});

it('optimistically enables a saved valid rule with its current name', async () => {
  const disabledRule = {...rule, enabled: false};
  let resolveUpdate: (value: WAFRule) => void = () => undefined;
  updateRuleMeta.mockImplementation(() => new Promise<WAFRule>((resolve) => {
    resolveUpdate = resolve;
  }));
  getRule.mockResolvedValue(disabledRule);

  renderPage();
  expect(await screen.findByText('已停用')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('switch', {name: '启用规则'}));

  await waitFor(() => {
    expect(updateRuleMeta).toHaveBeenCalledWith(9, {
      name: '边缘防护',
      enabled: true,
    });
    expect(screen.getByText('已启用')).toBeInTheDocument();
  });

  await act(async () => resolveUpdate({...disabledRule, enabled: true}));
});

it('rolls back an optimistic enabled change when metadata update fails', async () => {
  const disabledRule = {...rule, enabled: false};
  let rejectUpdate: (reason: Error) => void = () => undefined;
  updateRuleMeta.mockImplementation(() => new Promise<WAFRule>((_resolve, reject) => {
    rejectUpdate = reject;
  }));
  getRule.mockResolvedValue(disabledRule);

  renderPage();
  expect(await screen.findByText('已停用')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('switch', {name: '启用规则'}));
  await waitFor(() => expect(screen.getByText('已启用')).toBeInTheDocument());

  await act(async () => rejectUpdate(new Error('网络不可用')));

  await waitFor(() => expect(screen.getByText('已停用')).toBeInTheDocument());
  expect(toastError).toHaveBeenCalledWith('网络不可用');
});

it('requires graph changes to be saved before changing enabled state', async () => {
  const disabledRule = {...rule, enabled: false};
  getRule.mockResolvedValue(disabledRule);
  saveRuleGraph.mockResolvedValue({...disabledRule, revision: 2});

  renderPage();
  const enabledSwitch = await screen.findByRole('switch', {name: '启用规则'});
  expect(enabledSwitch).toBeEnabled();

  fireEvent.click(screen.getByRole('button', {name: '修改画布'}));
  expect(enabledSwitch).toBeDisabled();
  fireEvent.click(screen.getByRole('button', {name: '保存'}));

  await waitFor(() => expect(enabledSwitch).toBeEnabled());
});
