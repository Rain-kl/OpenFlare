'use client';

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
  title = '24 小时请求趋势',
  description = '按小时拆分请求总量与 2xx/4xx/5xx 状态码请求量，判断各状态是否异常抬升。',
}: {
  points: TrafficTrendPoint[];
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
          summaryScope='total'
          summaryHint='近 24 小时'
          series={[
            {
              label: '请求量',
              color: '#f59e0b',
              fillColor: 'rgba(245, 158, 11, 0.18)',
              variant: 'area',
              values: points.map((point) => point.request_count),
            },
            {
              label: '2xx 请求',
              color: '#22c55e',
              values: points.map((point) => point.status_2xx_count),
            },
            {
              label: '4xx 请求',
              color: '#f97316',
              values: points.map((point) => point.status_4xx_count),
            },
            {
              label: '5xx 请求',
              color: '#ef4444',
              values: points.map((point) => point.status_5xx_count),
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
