import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DeployCommandDialog } from '@/app/(main)/tunnels/components/deploy-command-dialog';
import { SmartDNSInstancesTab } from '@/app/(main)/dns/smartdns/instances-tab';
import {
  SmartDNSService,
  type SmartDNSInstance,
} from '@/lib/services/openflare';
import en from '@/messages/en.json';
import ru from '@/messages/ru.json';
import zhCN from '@/messages/zh-CN.json';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    SmartDNSService: {
      ...actual.SmartDNSService,
      listInstances: vi.fn(),
      createInstance: vi.fn(),
    },
  };
});

const createdInstance: SmartDNSInstance = {
  id: 1,
  instance_id: 'dns-8bb26b1c',
  name: 'hk-dns-01',
  mode: 'standalone',
  public_addr: '203.0.113.53',
  listen_addr: '0.0.0.0:53',
  geo_name: 'Hong Kong',
  version: '0.1.0',
  status: 'offline',
  connected: false,
  snapshot_version: 0,
  snapshot_etag: '',
  qps: 0,
  p50_ms: 0,
  p95_ms: 0,
  p99_ms: 0,
  success_total: 0,
  error_total: 0,
  cache_hit_ratio: 0,
  cpu_usage_percent: 0,
  memory_usage_percent: 0,
  last_seen_at: null,
  last_error: '',
  quarantined_at: null,
  quarantine_reason: '',
  access_token: 'ofdns_test_secret_token',
  created_at: '2026-11-28T00:00:00Z',
  updated_at: '2026-11-28T00:00:00Z',
};

function renderInstancesTab(
  locale: 'zh-CN' | 'en' | 'ru',
  messages: typeof zhCN | typeof en | typeof ru,
  onError: (error: Error) => void,
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  render(
    <NextIntlClientProvider
      locale={locale}
      messages={messages}
      timeZone='Asia/Shanghai'
      onError={onError}
    >
      <QueryClientProvider client={client}>
        <SmartDNSInstancesTab />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

describe('SmartDNS instance credentials dialog i18n', () => {
  beforeEach(() => {
    vi.mocked(SmartDNSService.listInstances).mockResolvedValue([]);
    vi.mocked(SmartDNSService.createInstance).mockResolvedValue(
      createdInstance,
    );
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it.each([
    {
      locale: 'zh-CN' as const,
      messages: zhCN,
      createLabel: zhCN.smartdns.instances.create,
      nameLabel: zhCN.smartdns.instances.nameLabel,
      saveLabel: zhCN.smartdns.instances.save,
      credentialsTitle: zhCN.smartdns.instances.credentialsTitle,
      closeLabel: zhCN.common.close,
    },
    {
      locale: 'en' as const,
      messages: en,
      createLabel: en.smartdns.instances.create,
      nameLabel: en.smartdns.instances.nameLabel,
      saveLabel: en.smartdns.instances.save,
      credentialsTitle: en.smartdns.instances.credentialsTitle,
      closeLabel: en.common.close,
    },
    {
      locale: 'ru' as const,
      messages: ru,
      createLabel: ru.smartdns.instances.create,
      nameLabel: ru.smartdns.instances.nameLabel,
      saveLabel: ru.smartdns.instances.save,
      credentialsTitle: ru.smartdns.instances.credentialsTitle,
      closeLabel: ru.common.close,
    },
  ])(
    'renders translated close button in $locale without missing i18n keys',
    async ({
      locale,
      messages,
      createLabel,
      nameLabel,
      saveLabel,
      credentialsTitle,
      closeLabel,
    }) => {
      const i18nErrors: Error[] = [];
      renderInstancesTab(locale, messages, (error) => i18nErrors.push(error));
      const user = userEvent.setup();

      await user.click(
        await screen.findByRole('button', { name: createLabel }),
      );
      await user.type(screen.getByLabelText(nameLabel), 'hk-dns-01');
      await user.click(screen.getByRole('button', { name: saveLabel }));

      const dialog = await screen.findByRole('dialog', {
        name: credentialsTitle,
      });
      expect(screen.queryByText('common.close')).not.toBeInTheDocument();
      const footer = dialog.querySelector('[data-slot="dialog-footer"]');
      expect(footer).not.toBeNull();
      const closeButton = within(footer as HTMLElement).getByRole('button', {
        name: closeLabel,
      });
      expect(closeButton).toBeVisible();
      expect(i18nErrors).toEqual([]);

      await user.click(closeButton);
      await waitFor(() =>
        expect(
          screen.queryByRole('dialog', { name: credentialsTitle }),
        ).not.toBeInTheDocument(),
      );
    },
  );

  it.each([
    { locale: 'zh-CN' as const, messages: zhCN, closeLabel: zhCN.common.close },
    { locale: 'en' as const, messages: en, closeLabel: en.common.close },
    { locale: 'ru' as const, messages: ru, closeLabel: ru.common.close },
  ])(
    'renders translated close button in tunnel deploy dialog for $locale',
    ({ locale, messages, closeLabel }) => {
      const i18nErrors: Error[] = [];
      render(
        <NextIntlClientProvider
          locale={locale}
          messages={messages}
          timeZone='Asia/Shanghai'
          onError={(error) => i18nErrors.push(error)}
        >
          <DeployCommandDialog
            open
            mode='tunnel'
            title='Deploy'
            accessToken='token'
            bootstrapToken='boot'
            serverUrl='https://panel.example.com'
            deployCommand='openflared --token boot'
            onClose={vi.fn()}
          />
        </NextIntlClientProvider>,
      );

      const dialog = screen.getByRole('dialog');
      const footer = dialog.querySelector('[data-slot="dialog-footer"]');
      expect(footer).not.toBeNull();
      expect(
        within(footer as HTMLElement).getByRole('button', { name: closeLabel }),
      ).toBeVisible();
      expect(screen.queryByText('common.close')).not.toBeInTheDocument();
      expect(i18nErrors).toEqual([]);
    },
  );
});
