import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { NextIntlClientProvider } from 'next-intl';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import { TunnelsPageClient } from '@/app/(main)/tunnels/page-client';
import { TunnelService } from '@/lib/services/openflare/tunnel.service';
import apiClient from '@/lib/services/core/api-client';
import en from '@/messages/en.json';

vi.mock('@/lib/services/core/api-client', () => ({
  default: { get: vi.fn(), post: vi.fn() },
}));

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  cleanup();
});

it('calls new /tunnel-pools, /tunnel-relays, and /tunnels control-plane APIs', async () => {
  vi.mocked(apiClient.get).mockResolvedValue({
    data: { error_msg: '', data: [] },
  } as never);

  await TunnelService.listPools();
  expect(apiClient.get).toHaveBeenLastCalledWith(
    '/api/v1/d/tunnel-pools',
    expect.objectContaining({ params: undefined }),
  );

  await TunnelService.listRelays();
  expect(apiClient.get).toHaveBeenLastCalledWith(
    '/api/v1/d/tunnel-relays',
    expect.objectContaining({ params: undefined }),
  );

  await TunnelService.getTopology();
  expect(apiClient.get).toHaveBeenLastCalledWith(
    '/api/v1/d/tunnels/topology',
    expect.objectContaining({ params: undefined }),
  );
});

it('renders tunnel architecture console with tunnels, relays, pools, and topology tabs', async () => {
  vi.spyOn(TunnelService, 'getTopology').mockResolvedValue({
    pools: [
      {
        id: 1,
        pool_key: 'default',
        name: 'Default Pool',
        description: '',
        region: 'global',
        is_default: true,
        routing_policy: {},
        relay_count: 1,
        tunnel_count: 1,
        created_at: '2026-11-23T00:00:00Z',
        updated_at: '2026-11-23T00:00:00Z',
      },
    ],
    relays: [],
    tunnels: [
      {
        id: 7,
        tunnel_id: 'tun-abc',
        name: 'office-vpc',
        description: '',
        pool_id: 1,
        pool_key: 'default',
        ha_relay_count: 2,
        preferred_transport: 'auto',
        fallback_transports: ['noise', 'websocket'],
        client_proxy_url: '',
        allowed_target_cidrs: [],
        blocked_target_cidrs: ['169.254.169.254/32'],
        max_connectors: 4,
        enabled: true,
        config_revision: 3,
        status: 'online',
        online_connectors: 2,
        active_attachments: 4,
        bound_routes_count: 1,
        auto_update_enabled: false,
        update_requested: false,
        update_channel: 'stable',
        update_tag: '',
        quarantined_at: null,
        quarantine_reason: '',
        metadata: {},
        created_at: '2026-11-23T00:00:00Z',
        updated_at: '2026-11-23T00:00:00Z',
      },
    ],
    connectors: [],
    attachments: [],
  });
  vi.spyOn(TunnelService, 'getRelayBootstrapToken').mockResolvedValue({
    discovery_token: 'disc-tok',
    bootstrap_token: 'oftok1_mock',
    server_url: 'https://panel.example.com',
    deploy_command: 'openflare-relay --token oftok1_mock',
  });

  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  render(
    <NextIntlClientProvider locale='en' messages={en} timeZone='UTC'>
      <QueryClientProvider client={client}>
        <TunnelsPageClient />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );

  await screen.findByText('office-vpc');
  expect(screen.getByRole('tab', { name: 'Relay PoPs' })).toBeInTheDocument();
  expect(screen.getByRole('tab', { name: 'Relay Pools' })).toBeInTheDocument();
  expect(
    screen.getByRole('tab', { name: 'Mesh Topology' }),
  ).toBeInTheDocument();
});
