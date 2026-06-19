'use client';

import {TrendChart} from '@/components/data/trend-chart';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {TrafficTrendPoint} from '@/lib/services/openflare';

import {formatTrendHour} from './dashboard-utils';

export function TrafficTrendChart({
  points,
  title = '24 小时请求趋势',
  description = '观察整体请求量和错误量是否出现异常抬升。',
}: {
  points: TrafficTrendPoint[];
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
          series={[
            {
              label: '请求量',
              color: '#f59e0b',
              fillColor: 'rgba(245, 158, 11, 0.18)',
              variant: 'area',
              values: points.map((point) => point.request_count),
            },
            {
              label: '错误量',
              color: '#ef4444',
              values: points.map((point) => point.error_count),
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}