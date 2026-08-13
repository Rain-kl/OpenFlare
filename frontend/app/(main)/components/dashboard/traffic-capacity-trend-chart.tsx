'use client';

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
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>
          24 小时业务流量与容量趋势
        </CardTitle>
      </CardHeader>
      <CardContent className='space-y-6'>
        <TrendChart
          labels={networkPoints.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          height={180}
          summaryScope='total'
          summaryHint='近 24 小时 · 来自访问日志'
          yAxisValueFormatter={formatBytes}
          series={[
            {
              label: '接收数据',
              color: '#22c55e',
              fillColor: 'rgba(34, 197, 94, 0.14)',
              variant: 'area',
              values: networkPoints.map((point) => point.bytes_received),
              valueFormatter: formatBytes,
            },
            {
              label: '已提供数据',
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
          summaryHint='近 24 小时 · 宿主机容量 · 平均值'
          yAxisValueFormatter={formatPercent}
          series={[
            {
              label: '平均 CPU',
              color: '#0f766e',
              fillColor: 'rgba(15, 118, 110, 0.15)',
              variant: 'area',
              values: capacityPoints.map(
                (point) => point.average_cpu_usage_percent,
              ),
              valueFormatter: formatPercent,
            },
            {
              label: '平均内存',
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
