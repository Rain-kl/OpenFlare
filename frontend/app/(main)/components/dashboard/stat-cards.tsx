'use client';

import { Activity, Cpu, Gauge, Server, Users } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import type {
  DashboardCapacity,
  DashboardSummary,
  DashboardTraffic,
} from '@/lib/services/openflare';

import { formatCompactNumber, formatPercent } from './dashboard-utils';

export function DashboardStatCards({
  summary,
  traffic,
  capacity,
}: {
  summary: DashboardSummary;
  traffic: DashboardTraffic;
  capacity: DashboardCapacity;
}) {
  const t = useTranslations('dashboard.stats');
  const onlineRate =
    summary.total_nodes > 0
      ? (summary.online_nodes / summary.total_nodes) * 100
      : 0;

  return (
    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between pb-2'>
          <span className='text-xs font-medium text-muted-foreground'>
            {t('requests24h')}
          </span>
          <Activity className='size-4 text-primary' />
        </CardHeader>
        <CardContent className='space-y-2'>
          <div className='text-2xl font-semibold tracking-tight'>
            {formatCompactNumber(traffic.request_count)}
          </div>
          <p className='text-[10px] text-muted-foreground'>
            {t('requestsMeta', {
              visitors: formatCompactNumber(traffic.unique_visitors),
              errors: formatCompactNumber(traffic.error_count),
              qps: traffic.estimated_qps.toFixed(2),
            })}
          </p>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between pb-2'>
          <span className='text-xs font-medium text-muted-foreground'>
            {t('clusterCapacity')}
          </span>
          <Cpu className='size-4 text-primary' />
        </CardHeader>
        <CardContent className='space-y-2'>
          <div className='text-2xl font-semibold tracking-tight'>
            {formatPercent(capacity.average_cpu_usage_percent)}
            <span className='ml-2 text-sm font-normal text-muted-foreground'>
              CPU
            </span>
          </div>
          <Progress
            value={capacity.average_cpu_usage_percent}
            className='h-1.5'
          />
          <p className='text-[10px] text-muted-foreground'>
            {t('avgMemoryHighLoad', {
              memory: formatPercent(capacity.average_memory_usage_percent),
              cpu: capacity.high_cpu_nodes,
              memoryNodes: capacity.high_memory_nodes,
              storage: capacity.high_storage_nodes,
            })}
          </p>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none md:col-span-2 xl:col-span-1'>
        <CardHeader className='flex flex-row items-center justify-between pb-2'>
          <span className='text-xs font-medium text-muted-foreground'>
            {t('nodeOverview')}
          </span>
          <Server className='size-4 text-primary' />
        </CardHeader>
        <CardContent className='space-y-2'>
          <div className='text-2xl font-semibold tracking-tight'>
            {summary.online_nodes}
            <span className='text-sm font-normal text-muted-foreground'>
              {' '}
              {t('onlineOf', { total: summary.total_nodes })}
            </span>
          </div>
          <Progress value={onlineRate} className='h-1.5' />
          <p className='text-[10px] text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1'>
            <span className='inline-flex items-center gap-1'>
              <Users className='size-3' />
              {t('pending', { count: summary.pending_nodes })}
            </span>
            <span className='inline-flex items-center gap-1'>
              <Gauge className='size-3' />
              {t('unhealthyOffline', {
                unhealthy: summary.unhealthy_nodes,
                offline: summary.offline_nodes,
              })}
            </span>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
