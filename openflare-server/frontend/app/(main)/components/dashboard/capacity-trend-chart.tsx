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

export function CapacityTrendChart({
  points,
  title = '24 小时容量趋势',
  description = '按小时聚合 CPU 与内存使用率，判断整体容量是否持续紧张。',
}: {
  points: CapacityTrendPoint[];
  title?: string;
  description?: string;
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold">{title}</CardTitle>
        <CardDescription className="text-xs">{description}</CardDescription>
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