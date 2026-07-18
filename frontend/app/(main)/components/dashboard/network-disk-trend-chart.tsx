'use client';

import { TrendChart } from '@/components/data/trend-chart';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type {
  DiskIOTrendPoint,
  NetworkTrendPoint,
} from '@/lib/services/openflare';

import { formatBytes, formatTrendHour } from './dashboard-utils';

export function NetworkDiskTrendChart({
  networkPoints,
  diskPoints,
}: {
  networkPoints: NetworkTrendPoint[];
  diskPoints: DiskIOTrendPoint[];
}) {
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>
          24 小时业务流量与宿主机磁盘
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
          labels={networkPoints.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          height={180}
          summaryScope='total'
          summaryHint='近 24 小时 · 宿主机网卡'
          yAxisValueFormatter={formatBytes}
          series={[
            {
              label: '宿主机网卡入站',
              color: '#a3e635',
              fillColor: 'rgba(163, 230, 53, 0.12)',
              variant: 'area',
              values: networkPoints.map((point) => point.network_rx_bytes),
              valueFormatter: formatBytes,
            },
            {
              label: '宿主机网卡出站',
              color: '#f97316',
              values: networkPoints.map((point) => point.network_tx_bytes),
              valueFormatter: formatBytes,
            },
          ]}
        />

        <TrendChart
          labels={diskPoints.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          height={180}
          summaryScope='total'
          summaryHint='近 24 小时 · 宿主机磁盘'
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
