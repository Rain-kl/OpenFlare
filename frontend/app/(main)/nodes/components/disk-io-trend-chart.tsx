'use client';

import { useTranslations } from 'next-intl';

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
  title,
  description,
}: {
  points: DiskIOTrendPoint[];
  title?: string;
  description?: string;
}) {
  const t = useTranslations('nodes.diskIo');
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>
          {title ?? t('title')}
        </CardTitle>
        <CardDescription className='text-xs'>
          {description ?? t('description')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <TrendChart
          labels={points.map((point) =>
            formatTrendHour(point.bucket_started_at),
          )}
          summaryScope='average'
          summaryHint={t('summaryHint')}
          yAxisValueFormatter={formatDiskRate}
          series={[
            {
              label: t('read'),
              color: '#a78bfa',
              fillColor: 'rgba(167, 139, 250, 0.14)',
              variant: 'area',
              values: points.map((point) =>
                diskBytesToRate(point.disk_read_bytes),
              ),
              valueFormatter: formatDiskRate,
            },
            {
              label: t('write'),
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
