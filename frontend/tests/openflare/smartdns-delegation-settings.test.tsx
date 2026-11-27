import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { SmartDNSSettingsTab } from '@/app/(main)/dns/smartdns/settings-tab';
import { SmartDNSService } from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    SmartDNSService: {
      ...actual.SmartDNSService,
      getSettings: vi.fn(),
    },
  };
});

function renderSettingsTab() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  render(
    <NextIntlClientProvider
      locale='zh-CN'
      messages={zhCN}
      timeZone='Asia/Shanghai'
    >
      <QueryClientProvider client={client}>
        <SmartDNSSettingsTab />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

describe('SmartDNS delegation settings tab', () => {
  beforeEach(() => {
    vi.mocked(SmartDNSService.getSettings).mockResolvedValue({
      enabled: true,
      delegation_domain: 'sdns.example.com',
      ns_hosts: ['ns1.example.com', 'ns2.example.com'],
      default_ttl: 60,
      snapshot_interval_seconds: 10,
      update_repo: 'Rain-kl/OpenFlare',
      discovery_token: 'test-token',
    });
  });

  it('renders the documentation guide link and out-of-bailiwick NS placeholder', async () => {
    renderSettingsTab();

    expect(await screen.findByText('DNS 委派配置指引')).toBeVisible();
    const docLink = screen.getByRole('link', { name: '查看配置文档' });
    expect(docLink).toHaveAttribute(
      'href',
      'https://openflare.cc/guide/smartdns/',
    );
    expect(docLink).toHaveAttribute('target', '_blank');
    expect(docLink).toHaveAttribute('rel', 'noopener noreferrer');

    const nsTextarea = document.querySelector('#sdns-ns-hosts');
    expect(nsTextarea).toHaveAttribute(
      'placeholder',
      'ns1.example.com\nns2.example.com',
    );
  });
});
