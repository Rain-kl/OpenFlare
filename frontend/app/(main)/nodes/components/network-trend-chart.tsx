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
import type { NetworkTrendPoint } from '@/lib/services/openflare';

import {
  formatBytes,
  formatTrendHour,
} from '../../components/dashboard/dashboard-utils';

export function NetworkTrendChart({
  points,
  title,
  description,
}: {
  points: NetworkTrendPoint[];
  title?: string;
  description?: string;
}) {
  const t = useTranslations('nodes.networkTrend');
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
          summaryScope='total'
          summaryHint={t('summaryHint')}
          yAxisValueFormatter={formatBytes}
          series={[
            {
              label: t('received'),
              color: '#22c55e',
              fillColor: 'rgba(34, 197, 94, 0.14)',
              variant: 'area',
              values: points.map((point) => point.bytes_received),
              valueFormatter: formatBytes,
            },
            {
              label: t('provided'),
              color: '#38bdf8',
              values: points.map((point) => point.bytes_provided),
              valueFormatter: formatBytes,
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
