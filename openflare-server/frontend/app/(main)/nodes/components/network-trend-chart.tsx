'use client';

import {TrendChart} from '@/components/data/trend-chart';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {NetworkTrendPoint} from '@/lib/services/openflare';

import {
  formatBytesPerSecond,
  formatTrendHour,
} from '../../components/dashboard/dashboard-utils';

export function NetworkTrendChart({
  points,
  title = '24 小时网络趋势',
  description = '观察 OpenResty 入站/出站吞吐的变化，辅助识别回源压力、突发流量或出口异常。',
}: {
  points: NetworkTrendPoint[];
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
          yAxisValueFormatter={(value) => formatBytesPerSecond(value, 3600)}
          series={[
            {
              label: 'OpenResty 入站',
              color: '#22c55e',
              fillColor: 'rgba(34, 197, 94, 0.14)',
              variant: 'area',
              values: points.map((point) => point.openresty_rx_bytes),
              valueFormatter: (value) => formatBytesPerSecond(value, 3600),
            },
            {
              label: 'OpenResty 出站',
              color: '#38bdf8',
              values: points.map((point) => point.openresty_tx_bytes),
              valueFormatter: (value) => formatBytesPerSecond(value, 3600),
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}