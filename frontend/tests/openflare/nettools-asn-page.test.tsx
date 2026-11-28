import type { ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NetworkToolsAsnPageClient } from '@/app/(main)/network-tools/asn/page-client';
import { openflareSidebarNav } from '@/lib/navigation/openflare-nav';
import { NetworkToolsService } from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    NetworkToolsService: {
      lookupIP: vi.fn(),
      lookupASN: vi.fn(),
      listASNPrefixes: vi.fn(),
      getASNUpstreamGraph: vi.fn(),
      traceASPath: vi.fn(),
    },
  };
});

function renderWithProviders(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <NextIntlClientProvider locale='zh-CN' messages={zhCN}>
        {ui}
      </NextIntlClientProvider>
    </QueryClientProvider>,
  );
}

describe('NetworkToolsAsnPageClient layout and upstream graph zoom', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(NetworkToolsService.lookupASN).mockResolvedValue({
      asn_info: {
        asn: 13335,
        name: 'Cloudflare',
        class: 'Content',
        country: 'US',
        tags: ['anycast'],
        ipv4_prefixes: 100,
        ipv6_prefixes: 50,
        neighbor_count: 1,
      },
      neighbors: [
        {
          peer_asn: 174,
          peer_name: 'Cogent',
          peer_country: 'US',
          peer_class: 'Transit',
          relation: 'provider',
        },
      ],
      attribution: 'Data powered by bgp.tools',
    });

    vi.mocked(NetworkToolsService.listASNPrefixes).mockResolvedValue({
      asn: 13335,
      total: 0,
      results: [],
      attribution: 'Data powered by bgp.tools',
    });

    vi.mocked(NetworkToolsService.getASNUpstreamGraph).mockResolvedValue({
      origin_asn: 13335,
      sampled_prefix: '1.1.1.0/24',
      data_source: 'caida',
      total_paths_parsed: 2,
      cached: false,
      generated_at: '2026-11-28T02:00:00Z',
      nodes: [
        {
          asn: 13335,
          name: 'Cloudflare',
          short_name: 'Cloudflare',
          country: 'US',
          class: 'Content',
          role: 'origin',
          layer: 0,
          is_tier1: false,
          path_hits: 2,
        },
        {
          asn: 174,
          name: 'Cogent',
          short_name: 'Cogent',
          country: 'US',
          class: 'Transit',
          role: 'tier1',
          layer: 2,
          is_tier1: true,
          path_hits: 2,
        },
      ],
      edges: [
        {
          from: 13335,
          to: 174,
          kind: 'upstream',
          path_hits: 2,
        },
      ],
      attribution: 'Data powered by bgp.tools',
    });

    vi.mocked(NetworkToolsService.traceASPath).mockResolvedValue({
      target_asn: 13335,
      nodes: [
        {
          node_id: 'edge-1',
          node_name: 'HK Edge 1',
          node_ip: '203.0.113.10',
          node_geo: 'Hong Kong',
          node_asn: 4134,
          node_asn_name: 'Chinanet',
          as_hops: 1,
          path: [{ asn: 4134, name: 'Chinanet', country: 'CN' }],
          effective_rtt_ms: 24.5,
          quality_score: 88.2,
          confidence: 0.9,
          data_source: 'measured',
        },
      ],
      attribution: 'Data powered by bgp.tools',
    });
  });

  it('defaults upstream graph zoom to 75% and renders path analysis card above the graph', async () => {
    const user = userEvent.setup();
    renderWithProviders(<NetworkToolsAsnPageClient />);

    await user.type(
      screen.getByLabelText(zhCN.networkTools.asn.queryLabel),
      'AS13335',
    );
    await user.click(
      screen.getByRole('button', { name: zhCN.networkTools.asn.search }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(zhCN.networkTools.asn.upstreamGraph.title),
      ).toBeInTheDocument();
    });

    const zoomResetBtn = screen.getByRole('button', { name: '75%' });
    expect(zoomResetBtn).toBeInTheDocument();
    const zoomOutBtn = screen.getByRole('button', {
      name: zhCN.networkTools.asn.upstreamGraph.zoomOut,
    });
    expect(zoomOutBtn).toBeEnabled();

    const svg = screen.getByRole('img', {
      name: zhCN.networkTools.asn.upstreamGraph.title,
    });
    const viewBoxParts = (svg.getAttribute('viewBox') ?? '')
      .split(' ')
      .map(Number);
    expect(Number(svg.getAttribute('width'))).toBe(
      Math.round(viewBoxParts[2] * 0.75),
    );

    await user.click(zoomOutBtn);
    expect(screen.getByRole('button', { name: '60%' })).toBeInTheDocument();
    expect(zoomOutBtn).toBeDisabled();

    await user.click(
      screen.getByRole('button', {
        name: zhCN.networkTools.asn.upstreamGraph.zoomIn,
      }),
    );
    await user.click(
      screen.getByRole('button', {
        name: zhCN.networkTools.asn.upstreamGraph.zoomIn,
      }),
    );
    expect(screen.getByRole('button', { name: '90%' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '90%' }));
    expect(screen.getByRole('button', { name: '75%' })).toBeInTheDocument();

    await user.click(
      screen.getByRole('button', { name: zhCN.networkTools.asn.pathCompare }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(`${zhCN.networkTools.asn.pathCard} (AS13335)`),
      ).toBeInTheDocument();
    });

    const pathCardHeading = screen.getByText(
      `${zhCN.networkTools.asn.pathCard} (AS13335)`,
    );
    const graphHeading = screen.getByText(
      zhCN.networkTools.asn.upstreamGraph.title,
    );
    expect(
      Boolean(
        pathCardHeading.compareDocumentPosition(graphHeading) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ),
    ).toBe(true);
  });

  it('places the networkTools sidebar group right below the dns group', () => {
    const dnsIndex = openflareSidebarNav.findIndex(
      (entry) => entry.kind === 'group' && entry.titleKey === 'dns',
    );
    const networkToolsIndex = openflareSidebarNav.findIndex(
      (entry) => entry.kind === 'group' && entry.titleKey === 'networkTools',
    );
    expect(dnsIndex).toBeGreaterThan(-1);
    expect(networkToolsIndex).toBe(dnsIndex + 1);
  });
});
