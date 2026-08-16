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
import type { TrafficTrendPoint } from '@/lib/services/openflare';

import { formatTrendHour } from './dashboard-utils';

export function TrafficTrendChart({
  points,
  title,
  description,
}: {
  points: TrafficTrendPoint[];
  title?: string;
  description?: string;
}) {
  const t = useTranslations('dashboard.trafficTrend');
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
          series={[
            {
              label: t('requests'),
              color: '#f59e0b',
              fillColor: 'rgba(245, 158, 11, 0.18)',
              variant: 'area',
              values: points.map((point) => point.request_count),
            },
            {
              label: t('status2xx'),
              color: '#22c55e',
              values: points.map((point) => point.status_2xx_count),
            },
            {
              label: t('status4xx'),
              color: '#f97316',
              values: points.map((point) => point.status_4xx_count),
            },
            {
              label: t('status5xx'),
              color: '#ef4444',
              values: points.map((point) => point.status_5xx_count),
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
