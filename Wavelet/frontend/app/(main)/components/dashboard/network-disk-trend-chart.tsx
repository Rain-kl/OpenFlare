'use client';

import {TrendChart} from '@/components/data/trend-chart';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {DiskIOTrendPoint, NetworkTrendPoint} from '@/lib/services/openflare';

import {formatBytes, formatBytesPerSecond, formatTrendHour} from './dashboard-utils';

export function NetworkDiskTrendChart({
  networkPoints,
  diskPoints,
}: {
  networkPoints: NetworkTrendPoint[];
  diskPoints: DiskIOTrendPoint[];
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold">24 小时网络与磁盘趋势</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <TrendChart
          labels={networkPoints.map((point) => formatTrendHour(point.bucket_started_at))}
          height={180}
          yAxisValueFormatter={(value) => formatBytesPerSecond(value, 3600)}
          series={[
            {
              label: 'OpenResty 入站',
              color: '#22c55e',
              fillColor: 'rgba(34, 197, 94, 0.14)',
              variant: 'area',
              values: networkPoints.map((point) => point.openresty_rx_bytes),
              valueFormatter: (value) => formatBytesPerSecond(value, 3600),
            },
            {
              label: 'OpenResty 出站',
              color: '#38bdf8',
              values: networkPoints.map((point) => point.openresty_tx_bytes),
              valueFormatter: (value) => formatBytesPerSecond(value, 3600),
            },
          ]}
        />

        <TrendChart
          labels={diskPoints.map((point) => formatTrendHour(point.bucket_started_at))}
          height={180}
          yAxisValueFormatter={formatBytes}
          series={[
            {
              label: '磁盘读',
              color: '#a78bfa',
              fillColor: 'rgba(167, 139, 250, 0.14)',
              variant: 'area',
              values: diskPoints.map((point) => point.disk_read_bytes),
              valueFormatter: formatBytes,
            },
            {
              label: '磁盘写',
              color: '#fb7185',
              values: diskPoints.map((point) => point.disk_write_bytes),
              valueFormatter: formatBytes,
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}