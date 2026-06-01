import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ThemeProvider } from '@/components/providers/theme-provider';
import { WAFIPGroupsPage } from '@/features/waf/components/ip-groups-page';
import { WAFPage } from '@/features/waf/components/waf-page';

const pushMock = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: pushMock,
  }),
}));

function renderWithProviders(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>{ui}</ThemeProvider>
    </QueryClientProvider>,
  );
}

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

function buildIPGroup(overrides: Record<string, unknown> = {}) {
  return {
    id: 3,
    name: 'edge blacklist',
    type: 'manual',
    enabled: true,
    ip_list: ['203.0.113.10'],
    auto_config: {},
    subscription_url: '',
    subscription_format: 'text',
    subscription_mapping_rule: '',
    sync_interval_minutes: 1440,
    last_sync_status: '',
    last_sync_message: '',
    remark: '',
    referenced_by_rule_count: 0,
    created_at: '2026-06-01T00:00:00Z',
    updated_at: '2026-06-01T00:00:00Z',
    ...overrides,
  };
}

function buildRuleGroup(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: '全局规则组',
    enabled: true,
    is_global: true,
    block_status_code: 418,
    block_response_body: '',
    ip_whitelist: [],
    ip_blacklist: [],
    ip_whitelist_group_ids: [],
    ip_blacklist_group_ids: [],
    country_whitelist: [],
    country_blacklist: [],
    region_whitelist: [],
    region_blacklist: [],
    pow_enabled: false,
    pow_config: {
      difficulty: 4,
      algorithm: 'fast',
      session_ttl: 600,
      challenge_ttl: 300,
      whitelist: {
        ips: [],
        ip_cidrs: [],
        paths: [],
        path_regexes: [],
        user_agents: [],
      },
      blacklist: {
        ips: [],
        ip_cidrs: [],
        paths: [],
        path_regexes: [],
        user_agents: [],
      },
    },
    remark: '',
    applied_site_ids: [],
    applied_site_count: 0,
    created_at: '2026-06-01T00:00:00Z',
    updated_at: '2026-06-01T00:00:00Z',
    ...overrides,
  };
}

describe('WAF IP groups', () => {
  beforeEach(() => {
    pushMock.mockReset();
    stubMatchMedia();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders empty state and saves a manual IP group', async () => {
    let groups: Array<Record<string, unknown>> = [];

    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = init?.method?.toUpperCase() ?? 'GET';

        if (url.includes('/waf/ip-groups') && method === 'POST') {
          const payload = JSON.parse(String(init?.body));
          const created = buildIPGroup({
            id: 7,
            name: payload.name,
            ip_list: payload.ip_list,
          });
          groups = [created];
          return Promise.resolve(
            new Response(
              JSON.stringify({ success: true, message: '', data: created }),
            ),
          );
        }

        if (url.includes('/waf/ip-groups')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({ success: true, message: '', data: groups }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderWithProviders(<WAFIPGroupsPage />);

    expect(await screen.findByText('暂无 IP 组')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /新建 IP 组/ }));
    await userEvent.clear(screen.getByLabelText('IP 组名称'));
    await userEvent.type(screen.getByLabelText('IP 组名称'), 'blocked edge');
    await userEvent.type(
      screen.getByPlaceholderText(/203\.0\.113\.10/),
      '203.0.113.10',
    );
    await userEvent.click(screen.getByRole('button', { name: /保存 IP 组/ }));

    expect(await screen.findByText('IP 组已保存。')).toBeInTheDocument();
    await waitFor(() => expect(groups).toHaveLength(1));
  });

  it('opens IP group management from WAF page and references an IP group', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = init?.method?.toUpperCase() ?? 'GET';

        if (url.includes('/waf/rule-groups/1/update') && method === 'POST') {
          const payload = JSON.parse(String(init?.body));
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: buildRuleGroup({
                  ip_blacklist_group_ids: payload.ip_blacklist_group_ids,
                }),
              }),
            ),
          );
        }

        if (url.includes('/waf/rule-groups')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [buildRuleGroup()],
              }),
            ),
          );
        }

        if (url.includes('/waf/ip-groups')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [buildIPGroup()],
              }),
            ),
          );
        }

        if (url.includes('/proxy-routes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({ success: true, message: '', data: [] }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderWithProviders(<WAFPage />);

    await userEvent.click(
      await screen.findByRole('button', { name: /管理 IP 组/ }),
    );
    expect(pushMock).toHaveBeenCalledWith('/waf/ip-groups');

    await userEvent.click(screen.getByRole('button', { name: /黑白名单/ }));
    await userEvent.click(screen.getByRole('button', { name: /添加/ }));
    await userEvent.click(screen.getByRole('button', { name: 'IP 组' }));
    await userEvent.click(await screen.findByLabelText(/edge blacklist/));
    const addButtons = screen.getAllByRole('button', { name: '添加' });
    await userEvent.click(addButtons[addButtons.length - 1]);

    expect(await screen.findByText(/IP组: edge blacklist/)).toBeInTheDocument();
  });

  it('defaults new rule entries to blacklist', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);

        if (url.includes('/waf/rule-groups')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [buildRuleGroup()],
              }),
            ),
          );
        }

        if (url.includes('/waf/ip-groups') || url.includes('/proxy-routes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({ success: true, message: '', data: [] }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderWithProviders(<WAFPage />);

    await userEvent.click(
      await screen.findByRole('button', { name: /黑白名单/ }),
    );
    await userEvent.click(screen.getByRole('button', { name: /添加/ }));

    expect(
      await screen.findByRole('heading', { name: '添加黑名单规则' }),
    ).toBeInTheDocument();
  });
});
