import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ThemeProvider } from '@/components/providers/theme-provider';
import { ProxyRouteConfigPage } from '@/features/proxy-routes/components/proxy-route-config-page';
import { ProxyRoutesPage } from '@/features/proxy-routes/components/proxy-routes-page';

const pushMock = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: pushMock,
  }),
}));

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

function buildRoute(overrides: Record<string, unknown> = {}) {
  return {
    id: 9,
    site_name: 'marketing-site',
    domain: 'app.example.com',
    domains: ['app.example.com', 'www.example.com'],
    primary_domain: 'app.example.com',
    domain_count: 2,
    origin_id: null,
    origin_url: 'https://origin-a.internal:443',
    origin_host: '',
    upstreams: JSON.stringify([
      'https://origin-a.internal:443',
      'https://origin-b.internal:443',
    ]),
    upstream_list: [
      'https://origin-a.internal:443',
      'https://origin-b.internal:443',
    ],
    enabled: true,
    enable_https: true,
    cert_id: 1,
    cert_ids: [1],
    domain_cert_ids: [1, 0],
    redirect_http: true,
    limit_conn_per_server: 120,
    limit_conn_per_ip: 12,
    limit_rate: '512k',
    cache_enabled: true,
    cache_policy: 'path_prefix',
    cache_rules: JSON.stringify(['/assets']),
    cache_rule_list: ['/assets'],
    custom_headers: JSON.stringify([{ key: 'X-Site', value: 'marketing' }]),
    custom_header_list: [{ key: 'X-Site', value: 'marketing' }],
    remark: 'Marketing website',
    created_at: '2026-03-20T08:00:00Z',
    updated_at: '2026-03-21T08:00:00Z',
    ...overrides,
  };
}

function buildDiff(overrides: Record<string, unknown> = {}) {
  return {
    active_version: '20260330-001',
    added_sites: [],
    removed_sites: [],
    modified_sites: [],
    added_domains: [],
    removed_domains: [],
    modified_domains: [],
    main_config_changed: false,
    changed_option_keys: [],
    changed_option_details: [],
    current_website_count: 1,
    active_website_count: 1,
    ...overrides,
  };
}

function renderWithProviders(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>{ui}</ThemeProvider>
    </QueryClientProvider>,
  );
}

describe('Proxy route website pages', () => {
  beforeEach(() => {
    pushMock.mockReset();
    stubMatchMedia();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders website list summary with config entry', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);

        if (url.includes('/proxy-routes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [buildRoute()],
              }),
            ),
          );
        }

        if (url.includes('/config-versions/diff')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: buildDiff({
                  modified_sites: ['marketing-site'],
                }),
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderWithProviders(<ProxyRoutesPage />);

    expect(await screen.findByText('marketing-site')).toBeInTheDocument();
    expect(screen.getAllByText(/app\.example\.com/).length).toBeGreaterThan(0);
    expect(screen.getByRole('link')).toHaveAttribute(
      'href',
      '/proxy-route/detail?id=9&section=domains',
    );
  });

  it('creates a website and navigates to config page', async () => {
    const routes: Array<Record<string, unknown>> = [];

    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = init?.method?.toUpperCase() ?? 'GET';

        if (url.includes('/proxy-routes/') && method === 'POST') {
          const payload = JSON.parse(String(init?.body));
          const created = buildRoute({
            id: 21,
            site_name: payload.site_name,
            domain: payload.domain,
            domains: payload.domains,
            primary_domain: payload.domain,
            domain_count: payload.domains.length,
            origin_url: payload.origin_url,
            upstreams: JSON.stringify([payload.origin_url, ...payload.upstreams]),
            upstream_list: [payload.origin_url, ...payload.upstreams],
            enabled: payload.enabled,
            enable_https: payload.enable_https,
            cert_id: payload.cert_id,
            cert_ids: payload.cert_ids ?? [],
            domain_cert_ids: payload.domain_cert_ids ?? [],
            redirect_http: payload.redirect_http,
            limit_conn_per_server: 0,
            limit_conn_per_ip: 0,
            limit_rate: '',
            cache_enabled: false,
            cache_policy: 'url',
            cache_rules: '[]',
            cache_rule_list: [],
            custom_headers: '[]',
            custom_header_list: [],
            remark: payload.remark,
          });
          routes.splice(0, routes.length, created);

          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: created,
              }),
            ),
          );
        }

        if (url.includes('/proxy-routes/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: routes,
              }),
            ),
          );
        }

        if (url.includes('/config-versions/diff')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: buildDiff(),
              }),
            ),
          );
        }

        if (url.includes('/managed-domains/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, domain: '*.example.com', cert_id: 1, enabled: true }],
              }),
            ),
          );
        }

        if (url.includes('/tls-certificates/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, name: 'example-cert', not_after: null }],
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderWithProviders(<ProxyRoutesPage />);

    const user = userEvent.setup();
    const pageButtons = await screen.findAllByRole('button');
    await user.click(pageButtons[1]);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toBeInTheDocument();

    await user.type(
      within(dialog).getByPlaceholderText('marketing-site'),
      'launch-site',
    );

    const primaryDomainInput = within(dialog).getByLabelText('域名 1');
    await user.type(primaryDomainInput, 'app.exam');
    await user.click(
      await within(dialog).findByRole('button', { name: 'app.example.com' }),
    );

    await user.click(within(dialog).getByLabelText('新增域名输入框'));
    await user.type(within(dialog).getByLabelText('域名 2'), 'www.example.com');

    await user.type(
      within(dialog).getByLabelText('上游地址'),
      'https://origin-a.internal:443{enter}https://origin-b.internal:443',
    );

    const submitButton = document.querySelector(
      'button[form="create-website-form"]',
    ) as HTMLButtonElement | null;
    expect(submitButton).toBeInstanceOf(HTMLButtonElement);
    if (!submitButton) {
      throw new Error('missing create submit button');
    }
    await user.click(submitButton);

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith(
        '/proxy-route/detail?id=21&section=domains',
      );
    });
  });

  it('saves domain settings from config page by section', async () => {
    const updateRequests: Array<Record<string, unknown>> = [];

    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = init?.method?.toUpperCase() ?? 'GET';

        if (url.includes('/proxy-routes/9/update') && method === 'POST') {
          const payload = JSON.parse(String(init?.body)) as Record<
            string,
            unknown
          >;
          updateRequests.push(payload);

          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: buildRoute({
                  site_name: payload.site_name,
                  domain: (payload.domains as string[])[0],
                  domains: payload.domains,
                  primary_domain: (payload.domains as string[])[0],
                  domain_count: (payload.domains as string[]).length,
                  enabled: payload.enabled,
                  enable_https: payload.enable_https,
                  cert_id: payload.cert_id,
                  cert_ids: payload.cert_ids,
                  domain_cert_ids: payload.domain_cert_ids,
                  redirect_http: payload.redirect_http,
                }),
              }),
            ),
          );
        }

        if (url.includes('/proxy-routes/9')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: buildRoute(),
              }),
            ),
          );
        }

        if (url.includes('/tls-certificates/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, name: 'example-cert', not_after: null }],
              }),
            ),
          );
        }

        if (url.includes('/managed-domains/')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                message: '',
                data: [{ id: 1, domain: '*.example.com', cert_id: 1, enabled: true }],
              }),
            ),
          );
        }

        return Promise.reject(new Error(`Unhandled fetch: ${url}`));
      }),
    );

    renderWithProviders(
      <ProxyRouteConfigPage routeId="9" initialSection="domains" />,
    );

    const user = userEvent.setup();
    expect(await screen.findByText('marketing-site')).toBeInTheDocument();

    const siteNameInput = screen.getByPlaceholderText('marketing-site');
    await user.clear(siteNameInput);
    await user.type(siteNameInput, 'brand-site');

    const primaryDomainInput = screen.getByLabelText('域名 1');
    await user.clear(primaryDomainInput);
    await user.type(primaryDomainInput, 'brand.example.com');

    const secondaryDomainInput = screen.getByLabelText('域名 2');
    await user.clear(secondaryDomainInput);
    await user.type(secondaryDomainInput, 'www.brand.example.com');

    await user.selectOptions(screen.getByLabelText('证书 1'), '1');
    await user.selectOptions(screen.getByLabelText('证书 2'), '');

    const saveButton = document.querySelector(
      'button[form="proxy-route-domains-form"]',
    ) as HTMLButtonElement | null;
    expect(saveButton).toBeInstanceOf(HTMLButtonElement);
    if (!saveButton) {
      throw new Error('missing domain save button');
    }
    await user.click(saveButton);

    await waitFor(() => {
      expect(updateRequests).toHaveLength(1);
    });

    expect(updateRequests[0]).toMatchObject({
      site_name: 'brand-site',
      domain: 'brand.example.com',
      domains: ['brand.example.com', 'www.brand.example.com'],
      enabled: true,
      enable_https: true,
      cert_id: 1,
      cert_ids: [1],
      domain_cert_ids: [1, 0],
      redirect_http: true,
    });
  });
});
