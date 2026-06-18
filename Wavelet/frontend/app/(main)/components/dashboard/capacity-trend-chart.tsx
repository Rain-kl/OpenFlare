'use client';

import {TrendChart} from '@/components/data/trend-chart';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {CapacityTrendPoint} from '@/lib/services/openflare';

import {formatPercent, formatTrendHour} from './dashboard-utils';

export function CapacityTrendChart({points}: {points: CapacityTrendPoint[]}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold">24 小时容量趋势</CardTitle>
        <CardDescription className="text-xs">
          按小时聚合 CPU 与内存使用率，判断整体容量是否持续紧张。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <TrendChart
          labels={points.map((point) => formatTrendHour(point.bucket_started_at))}
          yAxisValueFormatter={formatPercent}
          series={[
            {
              label: '平均 CPU',
              color: '#0f766e',
              fillColor: 'rgba(15, 118, 110, 0.15)',
              variant: 'area',
              values: points.map((point) => point.average_cpu_usage_percent),
              valueFormatter: formatPercent,
            },
            {
              label: '平均内存',
              color: '#2563eb',
              values: points.map((point) => point.average_memory_usage_percent),
              valueFormatter: formatPercent,
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}