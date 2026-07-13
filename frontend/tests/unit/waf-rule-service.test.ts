import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import type {AxiosResponse} from 'axios';
import {createElement} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';

import WafPage from '@/app/(main)/waf/page';
import apiClient from '@/lib/services/core/api-client';
import {WafService} from '@/lib/services/openflare';
import type {WAFSiteRuleGroups} from '@/lib/services/openflare';
import {WafService as DirectWafService} from '@/lib/services/openflare/waf.service';

const pushMock = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({push: pushMock}),
}));

vi.mock('@/lib/services/core/api-client', () => ({
  default: {
    post: vi.fn(),
  },
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    WafService: {
      ...actual.WafService,
      listRuleGroups: vi.fn(),
      listIPGroups: vi.fn(),
      createRule: vi.fn(),
      deleteRuleGroup: vi.fn(),
    },
  };
});

const graph = {
  schema_version: 1,
  nodes: [
    {id: 'start', type: 'start' as const, position: {x: 0, y: 0}, config: {}},
    {id: 'allow', type: 'allow' as const, position: {x: 200, y: 0}, config: {}},
  ],
  edges: [{id: 'edge', source: 'start', source_handle: 'next', target: 'allow'}],
};

const ruleSummary = {
  id: 17,
  name: '入口防护',
  enabled: false,
  is_global: false,
  graph,
  revision: 1,
  applied_site_ids: [],
  applied_site_count: 0,
  created_at: '',
  updated_at: '',
};

// The binding API returns graph-backed rule summaries, not obsolete fixed-chain fields.
const siteRuleGroupsFixture: WAFSiteRuleGroups = {
  route_id: 9,
  global_rule_group: null,
  rule_groups: [ruleSummary],
  applied_rule_groups: [ruleSummary],
  applied_ids: [17],
};

describe('WafService rule graph API', () => {
  beforeEach(() => {
    vi.mocked(apiClient.post).mockReset();
  });

  it('models site bindings with graph-backed rule summaries', () => {
    expect(siteRuleGroupsFixture.rule_groups[0]?.graph.nodes).toHaveLength(2);
    expect(siteRuleGroupsFixture.applied_ids).toEqual([17]);
  });

  it('creates a rule with a name-only payload', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({
      data: {error_msg: '', data: {id: 17, name: '入口防护', graph}},
    } as AxiosResponse);

    await DirectWafService.createRule({name: '入口防护'});

    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/waf/rule-groups',
      {name: '入口防护'},
      undefined,
    );
  });

  it('saves the graph together with its current revision', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({
      data: {error_msg: '', data: {id: 17, revision: 4, graph}},
    } as AxiosResponse);

    await DirectWafService.saveRuleGraph(17, {revision: 3, graph});

    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/waf/rule-groups/17/graph',
      {revision: 3, graph},
      undefined,
    );
  });

  it('updates rule metadata with the current name and enabled state', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({
      data: {error_msg: '', data: {...ruleSummary, enabled: true}},
    } as AxiosResponse);

    await DirectWafService.updateRuleMeta(17, {
      name: '入口防护',
      enabled: true,
    });

    expect(apiClient.post).toHaveBeenCalledWith(
      '/api/v1/d/waf/rule-groups/17/meta',
      {name: '入口防护', enabled: true},
      undefined,
    );
  });

  it('does not expose the retired rule-to-sites binding service', () => {
    expect(DirectWafService).not.toHaveProperty('updateRuleGroupSites');
  });
});

describe('WAF rule creation flow', () => {
  beforeEach(() => {
    pushMock.mockReset();
    vi.mocked(WafService.listRuleGroups).mockReset();
    vi.mocked(WafService.listRuleGroups).mockResolvedValue([]);
    vi.mocked(WafService.listIPGroups).mockReset();
    vi.mocked(WafService.listIPGroups).mockResolvedValue([]);
    vi.mocked(WafService.createRule).mockReset();
    vi.mocked(WafService.createRule).mockResolvedValue(ruleSummary);
  });

  it('creates by name and navigates directly to the graph editor', async () => {
    const client = new QueryClient({
      defaultOptions: {queries: {retry: false, gcTime: 0}},
    });
    render(
      createElement(
        QueryClientProvider,
        {client},
        createElement(WafPage),
      ),
    );

    fireEvent.click(await screen.findByRole('button', {name: '新建规则'}));
    expect(screen.queryByText('IP 白名单')).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox', {name: '规则名称'}), {
      target: {value: '入口防护'},
    });
    fireEvent.click(screen.getByRole('button', {name: '创建并编排'}));

    await waitFor(() => {
      expect(WafService.createRule).toHaveBeenCalledWith({name: '入口防护'});
      expect(pushMock).toHaveBeenCalledWith('/waf/rules/editor?id=17');
    });
  });
});
