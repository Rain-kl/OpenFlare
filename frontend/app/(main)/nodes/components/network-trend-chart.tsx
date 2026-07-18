'use client';

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
  title = '24 小时网络趋势',
  description = '按小时展示已提供/接收数据（访问日志）与宿主机网卡；摘要为近 24 小时总量。',
}: {
  points: NetworkTrendPoint[];
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
          yAxisValueFormatter={formatBytes}
          series={[
            {
              label: '接收数据',
              color: '#22c55e',
              fillColor: 'rgba(34, 197, 94, 0.14)',
              variant: 'area',
              values: points.map((point) => point.bytes_received),
              valueFormatter: formatBytes,
            },
            {
              label: '已提供数据',
              color: '#38bdf8',
              values: points.map((point) => point.bytes_provided),
              valueFormatter: formatBytes,
            },
            {
              label: '宿主机网卡入站',
              color: '#a3e635',
              values: points.map((point) => point.network_rx_bytes),
              valueFormatter: formatBytes,
            },
            {
              label: '宿主机网卡出站',
              color: '#f97316',
              values: points.map((point) => point.network_tx_bytes),
              valueFormatter: formatBytes,
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
