import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { toast } from 'sonner';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ZoneDomainsTable } from '@/app/(main)/websites/[zoneId]/components/zone-domains-table';
import {
  type ProxyRouteItem,
  type TlsCertificateItem,
  ZoneDomainService,
  type ZoneDomainItem,
} from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    ZoneDomainService: {
      create: vi.fn(),
      update: vi.fn(),
      deleteById: vi.fn(),
      cleanupUnlinked: vi.fn(),
    },
  };
});

const certificates: TlsCertificateItem[] = [
  {
    id: 1,
    name: 'example.com',
    provider: 'manual',
    acme_account_id: 0,
    dns_account_id: 0,
    key_algorithm: 'EC256',
    auto_renew: false,
    primary_domain: 'example.com',
    other_domains: '*.example.com',
    disable_cname: false,
    skip_dns: false,
    dns1: '',
    dns2: '',
    apply_status: 'issued',
    apply_message: '',
    not_before: '2026-01-01T00:00:00Z',
    not_after: '2027-01-01T00:00:00Z',
    remark: '',
    created_at: '',
    updated_at: '',
  },
];

const routes: ProxyRouteItem[] = [
  {
    id: 10,
    site_name: 'app',
    zone_domain_ids: [1],
    zone_domains: [
      {
        id: 1,
        zone_id: 5,
        domain: 'app.example.com',
        cert_id: 1,
      },
    ],
    origin_id: null,
    origin_url: 'http://127.0.0.1:8080',
    origin_host: '',
    upstreams: '["http://127.0.0.1:8080"]',
    upstream_list: ['http://127.0.0.1:8080'],
    enabled: true,
    enable_https: true,
    enable_http2: true,
    redirect_http: false,
    limit_conn_per_server: 0,
    limit_conn_per_ip: 0,
    limit_rate: '',
    limit_req_per_ip: '',
    cache_enabled: false,
    cache_policy: 'off',
    cache_rules: '[]',
    cache_rule_list: [],
    custom_headers: '[]',
    custom_header_list: [],
    basic_auth_enabled: false,
    basic_auth_username: '',
    basic_auth_password: '',
    upstream_type: 'direct',
    created_at: '',
    updated_at: '',
  },
];

function makeDomain(overrides: Partial<ZoneDomainItem>): ZoneDomainItem {
  return {
    id: 1,
    zone_id: 5,
    proxy_route_id: null,
    domain: 'idle.example.com',
    cert_id: 1,
    created_at: '',
    updated_at: '',
    ...overrides,
  };
}

function renderDomainsTable(
  domains: ZoneDomainItem[],
  onChanged = vi.fn().mockResolvedValue(undefined),
) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  });
  return {
    onChanged,
    ...render(
      <NextIntlClientProvider
        locale='zh-CN'
        messages={zhCN}
        timeZone='Asia/Shanghai'
      >
        <QueryClientProvider client={client}>
          <ZoneDomainsTable
            zoneId={5}
            zoneRoot='example.com'
            domains={domains}
            certificates={certificates}
            routes={routes}
            onChanged={onChanged}
          />
        </QueryClientProvider>
      </NextIntlClientProvider>,
    ),
  };
}

describe('ZoneDomainsTable unlinked cleanup', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('disables the cleanup button when every domain is linked to a route', () => {
    renderDomainsTable([
      makeDomain({ id: 1, proxy_route_id: 10, domain: 'app.example.com' }),
    ]);

    const button = screen.getByRole('button', { name: '清理未关联域名' });
    expect(button).toBeDisabled();
  });

  it('confirms and cleans up all unlinked domains in the zone', async () => {
    const user = userEvent.setup();
    vi.mocked(ZoneDomainService.cleanupUnlinked).mockResolvedValue({
      deleted_count: 2,
    });

    const { onChanged } = renderDomainsTable([
      makeDomain({ id: 1, proxy_route_id: 10, domain: 'app.example.com' }),
      makeDomain({ id: 2, proxy_route_id: null, domain: 'old1.example.com' }),
      makeDomain({ id: 3, proxy_route_id: null, domain: 'old2.example.com' }),
    ]);

    const trigger = screen.getByRole('button', {
      name: '清理未关联域名 (2)',
    });
    expect(trigger).toBeEnabled();

    await user.click(trigger);

    expect(screen.getByText('确认清理未关联域名')).toBeInTheDocument();
    expect(
      screen.getByText(/将删除当前 Zone 下 2 个未关联代理路由的域名/),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '确认清理' }));

    await waitFor(() => {
      expect(ZoneDomainService.cleanupUnlinked).toHaveBeenCalledWith(5);
    });
    expect(toast.success).toHaveBeenCalledWith('已清理 2 个未关联域名');
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it('shows empty feedback when nothing was deleted', async () => {
    const user = userEvent.setup();
    vi.mocked(ZoneDomainService.cleanupUnlinked).mockResolvedValue({
      deleted_count: 0,
    });

    renderDomainsTable([
      makeDomain({ id: 2, proxy_route_id: null, domain: 'old1.example.com' }),
    ]);

    await user.click(
      screen.getByRole('button', { name: '清理未关联域名 (1)' }),
    );
    await user.click(screen.getByRole('button', { name: '确认清理' }));

    await waitFor(() => {
      expect(ZoneDomainService.cleanupUnlinked).toHaveBeenCalledWith(5);
    });
    expect(toast.success).toHaveBeenCalledWith('当前没有可清理的未关联域名');
  });
});
