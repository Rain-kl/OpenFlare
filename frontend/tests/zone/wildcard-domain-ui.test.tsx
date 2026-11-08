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
      }),
    );
  });

  it('quick-add previews and submits * as a wildcard under the selected zone', async () => {
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
    fireEvent.submit(document.getElementById('quick-create-zone-domain')!);
    await waitFor(() => {
      expect(ZoneDomainService.create).toHaveBeenCalledWith(1, {
        domain: '*.example.com',
        cert_id: null,
      });
      expect(onCreated).toHaveBeenCalledWith(wildcardDomain);
    });
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
