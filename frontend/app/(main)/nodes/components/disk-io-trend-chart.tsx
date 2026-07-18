'use client';

import { TrendChart } from '@/components/data/trend-chart';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type { DiskIOTrendPoint } from '@/lib/services/openflare';
import { formatBytesPerSecond } from '@/lib/utils/metrics';

import { formatTrendHour } from '../../components/dashboard/dashboard-utils';

/** Backend disk points are per-hour totals; chart displays bytes/s within each hour. */
const DISK_BUCKET_SECONDS = 3600;

function diskBytesToRate(bytes: number) {
  return bytes > 0 ? bytes / DISK_BUCKET_SECONDS : 0;
}

function formatDiskRate(bytesPerSecond: number) {
  return formatBytesPerSecond(bytesPerSecond, 1, { zeroText: '0 B' });
}

export function DiskIOTrendChart({
  points,
  title = '24 小时磁盘 IO 趋势',
  description = '按小时展示磁盘读写速率（B/s），辅助判断日志放大、缓存抖动或磁盘压力。',
}: {
  points: DiskIOTrendPoint[];
  title?: string;
  description?: string;
}) {
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>{title}</CardTitle>
        <CardDescription className='text-xs'>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        <TrendChart
          labels={points.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          summaryScope='average'
          summaryHint='近 24 小时 · 平均速率'
          yAxisValueFormatter={formatDiskRate}
          series={[
            {
              label: '磁盘读',
              color: '#a78bfa',
              fillColor: 'rgba(167, 139, 250, 0.14)',
              variant: 'area',
              values: points.map((point) =>
                diskBytesToRate(point.disk_read_bytes),
              ),
              valueFormatter: formatDiskRate,
            },
            {
              label: '磁盘写',
              color: '#fb7185',
              values: points.map((point) =>
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
