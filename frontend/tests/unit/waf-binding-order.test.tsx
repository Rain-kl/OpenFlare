import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createElement } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  reorderRuleIDs,
  WafSection,
} from '@/app/(main)/proxy-routes/detail/components/waf-section';
import type { ProxyRouteItem, WAFRule } from '@/lib/services/openflare';
import { WafService } from '@/lib/services/openflare';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    WafService: {
      listSiteRuleGroups: vi.fn(),
      updateSiteRuleGroups: vi.fn(),
    },
  };
});

const graph = {
  schema_version: 1,
  nodes: [
    {
      id: 'start',
      type: 'start' as const,
      position: { x: 0, y: 0 },
      config: {},
    },
    {
      id: 'allow',
      type: 'allow' as const,
      position: { x: 100, y: 0 },
      config: {},
    },
  ],
  edges: [
    { id: 'edge', source: 'start', source_handle: 'next', target: 'allow' },
  ],
};

function rule(id: number, name: string, isGlobal = false): WAFRule {
  return {
    id,
    name,
    enabled: true,
    is_global: isGlobal,
    graph,
    revision: 1,
    applied_site_ids: [],
    applied_site_count: 0,
    created_at: '',
    updated_at: '',
  };
}

describe('WAF route binding order', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    );
    vi.mocked(WafService.listSiteRuleGroups).mockReset();
    vi.mocked(WafService.updateSiteRuleGroups).mockReset();
    vi.mocked(WafService.listSiteRuleGroups).mockResolvedValue({
      route_id: 9,
      global_rule_group: rule(99, '全局规则', true),
      rule_groups: [rule(1, '规则 A'), rule(2, '规则 B'), rule(3, '规则 C')],
      applied_rule_groups: [rule(1, '规则 A'), rule(2, '规则 B')],
      applied_ids: [1, 2],
    });
    vi.mocked(WafService.updateSiteRuleGroups).mockImplementation(
      async (_routeID, ids) => ({
        route_id: 9,
        global_rule_group: rule(99, '全局规则', true),
        rule_groups: [rule(1, '规则 A'), rule(2, '规则 B'), rule(3, '规则 C')],
        applied_rule_groups: ids.map((id) => rule(id, `规则 ${id}`)),
        applied_ids: ids,
      }),
    );
  });

  it('reorders by the active and target IDs reported by drag end', () => {
    expect(reorderRuleIDs([1, 2, 3], 3, 1)).toEqual([3, 1, 2]);
  });

  it('keeps the global rule fixed and submits custom rules in UI order', async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      createElement(
        QueryClientProvider,
        { client },
        createElement(WafSection, { route: { id: 9 } as ProxyRouteItem }),
      ),
    );

    expect(await screen.findByText('全局规则')).toBeInTheDocument();
    expect(screen.getByText('始终生效')).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: '上移全局规则' }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('checkbox', { name: '选择规则 C' }));
    fireEvent.click(screen.getByRole('button', { name: '上移规则 C' }));
    fireEvent.click(screen.getByRole('button', { name: '上移规则 C' }));
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => {
      expect(WafService.updateSiteRuleGroups).toHaveBeenCalledWith(
        9,
        [3, 1, 2],
      );
    });
  });
});
