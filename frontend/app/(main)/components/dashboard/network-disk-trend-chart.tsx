'use client';

import { TrendChart } from '@/components/data/trend-chart';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type {
  DiskIOTrendPoint,
  NetworkTrendPoint,
} from '@/lib/services/openflare';

import {
  formatBytes,
  formatBytesPerSecond,
  formatTrendHour,
} from './dashboard-utils';

/** Backend disk points are per-hour totals; chart displays bytes/s within each hour. */
const DISK_BUCKET_SECONDS = 3600;

function diskBytesToRate(bytes: number) {
  return bytes > 0 ? bytes / DISK_BUCKET_SECONDS : 0;
}

function formatDiskRate(bytesPerSecond: number) {
  return formatBytesPerSecond(bytesPerSecond, 1, { zeroText: '0 B' });
}

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
          labels={diskPoints.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          height={180}
          summaryScope='average'
          summaryHint='近 24 小时 · 宿主机磁盘 · 平均速率'
          yAxisValueFormatter={formatDiskRate}
          series={[
            {
              label: '磁盘读',
              color: '#a78bfa',
              fillColor: 'rgba(167, 139, 250, 0.14)',
              variant: 'area',
              values: diskPoints.map((point) =>
                diskBytesToRate(point.disk_read_bytes),
              ),
              valueFormatter: formatDiskRate,
            },
            {
              label: '磁盘写',
              color: '#fb7185',
              values: diskPoints.map((point) =>
                diskBytesToRate(point.disk_write_bytes),
              ),
              valueFormatter: formatDiskRate,
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
