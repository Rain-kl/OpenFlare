'use client';

import { useTranslations } from 'next-intl';

import { TrendChart } from '@/components/data/trend-chart';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type {
  CapacityTrendPoint,
  NetworkTrendPoint,
} from '@/lib/services/openflare';

import { formatBytes, formatPercent, formatTrendHour } from './dashboard-utils';

/** 业务流量（来自访问日志）与容量趋势（节点 Agent 宿主机指标）合并展示。 */
export function TrafficCapacityTrendChart({
  networkPoints,
  capacityPoints,
}: {
  networkPoints: NetworkTrendPoint[];
  capacityPoints: CapacityTrendPoint[];
}) {
  const t = useTranslations('dashboard.trafficCapacity');
  const tc = useTranslations('dashboard.capacityTrend');
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>{t('title')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-6'>
        <TrendChart
          labels={networkPoints.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          height={180}
          summaryScope='total'
          summaryHint={t('networkHint')}
          yAxisValueFormatter={formatBytes}
          series={[
            {
              label: t('received'),
              color: '#22c55e',
              fillColor: 'rgba(34, 197, 94, 0.14)',
              variant: 'area',
              values: networkPoints.map((point) => point.bytes_received),
              valueFormatter: formatBytes,
            },
            {
              label: t('provided'),
              color: '#38bdf8',
              values: networkPoints.map((point) => point.bytes_provided),
              valueFormatter: formatBytes,
            },
          ]}
        />

        <TrendChart
          labels={capacityPoints.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          height={180}
          summaryScope='average'
          summaryHint={t('capacityHint')}
          yAxisValueFormatter={formatPercent}
          series={[
            {
              label: tc('avgCpu'),
              color: '#0f766e',
              fillColor: 'rgba(15, 118, 110, 0.15)',
              variant: 'area',
              values: capacityPoints.map(
                (point) => point.average_cpu_usage_percent,
              ),
              valueFormatter: formatPercent,
            },
            {
              label: tc('avgMemory'),
              color: '#2563eb',
              values: capacityPoints.map(
                (point) => point.average_memory_usage_percent,
              ),
              valueFormatter: formatPercent,
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
