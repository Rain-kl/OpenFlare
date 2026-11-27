import type { ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextIntlClientProvider } from 'next-intl';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NetworkToolsQualityPageClient } from '@/app/(main)/network-tools/quality/page-client';
import { NetworkToolsService } from '@/lib/services/openflare';
import zhCN from '@/messages/zh-CN.json';

vi.mock('@/lib/services/openflare', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/lib/services/openflare')>();
  return {
    ...actual,
    NetworkToolsService: {
      listQualityMatrix: vi.fn(),
      predictQualityMatrix: vi.fn(),
      listProbeTargets: vi.fn(),
      getNetIntelStatus: vi.fn(),
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

describe('NetworkToolsQualityPageClient tabs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(NetworkToolsService.listQualityMatrix).mockResolvedValue({
      total: 1,
      results: [
        {
          id: 1,
          node_id: 'edge-hk-1',
          node_asn: 13335,
          node_name: 'HK Edge 1',
          node_geo: 'Hong Kong',
          node_status: 'healthy',
          client_asn: 4134,
          client_country: 'CN',
          client_region: 'CN-GD',
          ip_family: 'ipv4',
          aggregation_level: 'region',
          client_asn_name: 'Chinanet',
          tcp_rtt_p50_ms: 22.5,
          tcp_rtt_p95_ms: 34.0,
          tcp_min_rtt_ms: 19.0,
          tcp_retrans_rate: 0.002,
          tcp_sample_count: 120,
          trace_rtt_p50_ms: 0,
          trace_loss_rate: 0,
          trace_hop_count: 0,
          trace_as_path: '',
          trace_sample_count: 0,
          effective_rtt_ms: 22.5,
          predicted_rtt_ms: 22.5,
          quality_score: 91.5,
          confidence: 0.92,
          measurement_source: 'tcp_passive',
          data_source: 'measured',
          last_observed_at: '2026-11-27T05:00:00Z',
          updated_at: '2026-11-27T05:00:00Z',
        },
      ],
    });

    vi.mocked(NetworkToolsService.predictQualityMatrix).mockResolvedValue({
      target_asn: 4134,
      target_country: 'CN',
      target_region: 'CN-GD',
      ip_family: 'ipv4',
      nodes: [
        {
          node_id: 'edge-hk-1',
          node_name: 'HK Edge 1',
          node_ip: '203.0.113.10',
          node_geo: 'Hong Kong',
          node_asn: 13335,
          node_asn_name: 'Cloudflare',
          effective_rtt_ms: 18.4,
          rtt_p95_ms: 26.2,
          candidate_dispersion_ms: 3.1,
          prediction_error_ms: 1.8,
          quality_score: 94.2,
          confidence: 0.95,
          measurement_source: 'tcp_passive',
          prediction_method: 'exact_region',
          geo_base_rtt_ms: 15.0,
          geo_base_known: true,
          sample_count: 120,
          distinct_prefixes: 4,
          data_source: 'measured',
        },
      ],
      attribution: 'bgp.tools',
    });
  });

  it('separates client quality prediction simulator into its own tab', async () => {
    const user = userEvent.setup();
    renderWithProviders(<NetworkToolsQualityPageClient />);

    // Default matrix tab shows matrix table, NOT predictor panel
    await waitFor(() => {
      expect(screen.getByText('HK Edge 1')).toBeInTheDocument();
    });
    expect(
      screen.queryByText('客户端网络质量模拟预测'),
    ).not.toBeInTheDocument();

    // Switch to predictor tab
    const predictorTab = screen.getByRole('tab', { name: '质量模拟预测' });
    await user.click(predictorTab);

    expect(screen.getByText('客户端网络质量模拟预测')).toBeInTheDocument();
    expect(screen.queryByText('AS4134')).not.toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('访客 ASN，如 4134'), '4134');
    await user.click(screen.getByRole('button', { name: '计算预测' }));

    await waitFor(() => {
      expect(NetworkToolsService.predictQualityMatrix).toHaveBeenCalledWith({
        target_ip: undefined,
        target_asn: 4134,
        target_country: undefined,
        target_region: undefined,
        ip_family: undefined,
      });
    });

    expect(screen.getByText('L1 精确区域')).toBeInTheDocument();
    expect(screen.getByText('94.2')).toBeInTheDocument();
  });
});
