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
import type { CapacityTrendPoint } from '@/lib/services/openflare';

import { formatPercent, formatTrendHour } from './dashboard-utils';

export function CapacityTrendChart({
  points,
  title,
  description,
}: {
  points: CapacityTrendPoint[];
  title?: string;
  description?: string;
}) {
  const t = useTranslations('dashboard.capacityTrend');
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
          yAxisValueFormatter={formatPercent}
          series={[
            {
              label: t('avgCpu'),
              color: '#0f766e',
              fillColor: 'rgba(15, 118, 110, 0.15)',
              variant: 'area',
              values: points.map((point) => point.average_cpu_usage_percent),
              valueFormatter: formatPercent,
            },
            {
              label: t('avgMemory'),
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
