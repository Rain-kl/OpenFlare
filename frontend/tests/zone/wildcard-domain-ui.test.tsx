import type { ReactElement } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { QuickCreateZoneDomainDialog } from '@/app/(main)/websites/components/quick-create-zone-domain-dialog';
import { ZoneDomainDialog } from '@/app/(main)/websites/[zoneId]/components/zone-domain-dialog';
import { ZoneDomainsTable } from '@/app/(main)/websites/[zoneId]/components/zone-domains-table';
import { ZoneEditorDialog } from '@/app/(main)/websites/[zoneId]/components/zone-editor-dialog';
import {
  TlsCertificateService,
  ZoneDomainService,
  ZoneService,
  type TlsCertificateListItem,
  type ZoneDomainItem,
} from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    TlsCertificateService: { list: vi.fn() },
    ZoneDomainService: { create: vi.fn() },
    ZoneService: { create: vi.fn(), update: vi.fn() },
  };
});

const zone = { id: 1, domain: 'example.com', created_at: '', updated_at: '' };
const wildcardDomain: ZoneDomainItem = {
  id: 7,
  zone_id: 1,
  domain: '*.example.com',
  cert_id: null,
  proxy_route_id: null,
  created_at: '',
  updated_at: '',
};

const wildcardCertificate: TlsCertificateListItem = {
  id: 9,
  name: 'Wildcard',
  provider: 'upload',
  acme_account_id: 0,
  dns_account_id: 0,
  key_algorithm: '',
  auto_renew: false,
  primary_domain: '',
  other_domains: '',
  dns_names: ['*.example.com'],
  disable_cname: false,
  skip_dns: false,
  dns1: '',
  dns2: '',
  apply_status: 'ready',
  apply_message: '',
  not_before: '2020-01-01T00:00:00Z',
  not_after: '2099-01-01T00:00:00Z',
  remark: '',
  created_at: '',
  updated_at: '',
};

function renderUI(ui: ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
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

describe('wildcard domain entry points', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(TlsCertificateService.list).mockResolvedValue([]);
    vi.mocked(ZoneDomainService.create).mockImplementation(
      async (_zoneId, payload) => ({
        ...wildcardDomain,
        domain: payload.domain,
        cert_id: payload.cert_id ?? null,
      }),
    );
  });

  it('quick-add previews and submits * with its matching wildcard certificate', async () => {
    vi.mocked(TlsCertificateService.list).mockResolvedValue([
      wildcardCertificate,
    ]);
    const onCreated = vi.fn();
    renderUI(
      <QuickCreateZoneDomainDialog
        open
        onOpenChange={vi.fn()}
        zones={[zone]}
        fixedZoneId={zone.id}
        onCreated={onCreated}
      />,
    );
    fireEvent.change(screen.getByLabelText('域名'), {
      target: { value: '*' },
    });
    expect(screen.getByText('*.example.com')).toBeVisible();
    const certificateSelect = screen.getByRole('combobox', {
      name: '证书（可选）',
    });
    await waitFor(() =>
      expect(certificateSelect).toHaveTextContent('Wildcard'),
    );
    fireEvent.click(screen.getByRole('button', { name: '添加域名' }));
    await waitFor(() => {
      expect(ZoneDomainService.create).toHaveBeenCalledWith(1, {
        domain: '*.example.com',
        cert_id: wildcardCertificate.id,
      });
      expect(onCreated).toHaveBeenCalledWith({
        ...wildcardDomain,
        cert_id: wildcardCertificate.id,
      });
    });
  });

  it('prefers a valid exact match and warns when a manual choice does not cover the domain', async () => {
    const exactCertificate = {
      ...wildcardCertificate,
      id: 10,
      name: 'Exact',
      dns_names: ['example.com', 'API.EXAMPLE.COM'],
      not_after: '2098-01-01T00:00:00Z',
    };
    vi.mocked(TlsCertificateService.list).mockResolvedValue([
      wildcardCertificate,
      exactCertificate,
      {
        ...exactCertificate,
        id: 11,
        name: 'Expired',
        not_after: '2021-01-01T00:00:00Z',
      },
    ]);
    renderUI(
      <QuickCreateZoneDomainDialog
        open
        onOpenChange={vi.fn()}
        zones={[zone]}
        onCreated={vi.fn()}
      />,
    );

    const input = screen.getByLabelText('域名');
    fireEvent.change(input, { target: { value: 'api' } });
    fireEvent.keyDown(screen.getByRole('combobox', { name: '域名区域' }), {
      key: 'ArrowDown',
    });
    fireEvent.click(screen.getByRole('option', { name: zone.domain }));

    const certificateSelect = screen.getByRole('combobox', {
      name: '证书（可选）',
    });
    await waitFor(() => expect(certificateSelect).toHaveTextContent('Exact'));
    fireEvent.change(input, { target: { value: 'api.s3.example.com' } });
    expect(certificateSelect).toHaveTextContent('不绑定证书');
    fireEvent.change(input, { target: { value: 'api' } });
    expect(certificateSelect).toHaveTextContent('Exact');

    fireEvent.keyDown(certificateSelect, { key: 'ArrowDown' });
    fireEvent.click(screen.getByRole('option', { name: /Wildcard/ }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    fireEvent.change(input, { target: { value: '@' } });
    expect(certificateSelect).toHaveTextContent('Wildcard');
    expect(screen.getByRole('alert')).toHaveTextContent(
      '所选证书未覆盖域名 example.com',
    );

    fireEvent.keyDown(certificateSelect, { key: 'ArrowDown' });
    fireEvent.click(screen.getByRole('option', { name: '不绑定证书' }));
    expect(certificateSelect).toHaveTextContent('不绑定证书');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    fireEvent.change(input, { target: { value: 'api' } });
    expect(certificateSelect).toHaveTextContent('不绑定证书');
  });

  it('the Zone details dialog uses the same nested wildcard resolver', async () => {
    renderUI(
      <ZoneDomainDialog
        open
        onOpenChange={vi.fn()}
        zoneId={zone.id}
        zoneRoot={zone.domain}
        onSaved={vi.fn()}
      />,
    );
    fireEvent.change(screen.getByLabelText('域名'), {
      target: { value: '*.S3.Example.COM' },
    });
    expect(screen.getByText('*.s3.example.com')).toBeVisible();
    fireEvent.submit(document.getElementById('quick-create-zone-domain')!);
    await waitFor(() => {
      expect(ZoneDomainService.create).toHaveBeenCalledWith(1, {
        domain: '*.s3.example.com',
        cert_id: null,
      });
    });
  });

  it.each(['*.s3', '*.other.com', 'a.*.example.com', '*.exa mple.com'])(
    'blocks invalid quick-add input without calling the API: %s',
    async (input) => {
      renderUI(
        <ZoneDomainDialog
          open
          onOpenChange={vi.fn()}
          zoneId={zone.id}
          zoneRoot={zone.domain}
          onSaved={vi.fn()}
        />,
      );
      fireEvent.change(screen.getByLabelText('域名'), {
        target: { value: input },
      });
      fireEvent.submit(document.getElementById('quick-create-zone-domain')!);
      await waitFor(() => {
        expect(document.querySelector('.text-destructive')).toBeInTheDocument();
      });
      expect(ZoneDomainService.create).not.toHaveBeenCalled();
    },
  );

  it.each([false, true])(
    'root domain still rejects wildcards (edit=%s)',
    async (edit) => {
      renderUI(
        <ZoneEditorDialog
          open
          onOpenChange={vi.fn()}
          zone={edit ? zone : undefined}
        />,
      );
      fireEvent.change(screen.getByLabelText('根域'), {
        target: { value: '*.example.com' },
      });
      fireEvent.submit(document.getElementById('zone-editor')!);
      expect(
        await screen.findByText('请输入不含协议或通配符的根域'),
      ).toBeVisible();
      expect(ZoneService.create).not.toHaveBeenCalled();
      expect(ZoneService.update).not.toHaveBeenCalled();
    },
  );

  it('lists wildcard domains as text, never as a clickable wildcard URL', () => {
    renderUI(
      <ZoneDomainsTable
        zoneId={zone.id}
        zoneRoot={zone.domain}
        domains={[wildcardDomain]}
        certificates={[]}
        routes={[]}
        onChanged={vi.fn()}
      />,
    );
    expect(screen.getByText('*.example.com')).toBeVisible();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });
});
