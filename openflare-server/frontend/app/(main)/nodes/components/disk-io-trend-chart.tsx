'use client';

import {TrendChart} from '@/components/data/trend-chart';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {DiskIOTrendPoint} from '@/lib/services/openflare';

import {formatBytes, formatTrendHour} from '../../components/dashboard/dashboard-utils';

export function DiskIOTrendChart({
  points,
  title = '24 小时磁盘 IO 趋势',
  description = '观察磁盘读写变化，辅助判断日志放大、缓存抖动或磁盘压力。',
}: {
  points: DiskIOTrendPoint[];
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
          yAxisValueFormatter={formatBytes}
          series={[
            {
              label: '磁盘读',
              color: '#a78bfa',
              fillColor: 'rgba(167, 139, 250, 0.14)',
              variant: 'area',
              values: points.map((point) => point.disk_read_bytes),
              valueFormatter: formatBytes,
            },
            {
              label: '磁盘写',
              color: '#fb7185',
              values: points.map((point) => point.disk_write_bytes),
              valueFormatter: formatBytes,
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}