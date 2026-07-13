'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, HardDrive, Users } from 'lucide-react';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart';
import { Skeleton } from '@/components/ui/skeleton';
import {
  ZoneService,
  zoneQueryKey,
  type ZoneOverview,
  type ZoneStatsRange,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { formatBytes, formatCompactNumber } from '@/lib/utils/metrics';
import { cn } from '@/lib/utils';

const rangeOptions: Array<{ value: ZoneStatsRange; label: string }> = [
  { value: '24h', label: '24 小时' },
  { value: '7d', label: '7 天' },
  { value: '30d', label: '30 天' },
];

const visitorsChartConfig = {
  value: { label: '唯一访问者', color: 'hsl(217 91% 60%)' },
} satisfies ChartConfig;

const requestsChartConfig = {
  value: { label: '请求总数', color: 'hsl(217 91% 60%)' },
} satisfies ChartConfig;

const bytesChartConfig = {
  value: { label: '已提供数据', color: 'hsl(217 91% 60%)' },
} satisfies ChartConfig;

function formatAxisLabel(iso: string, range: ZoneStatsRange) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  if (range === '24h') {
    return date.toLocaleTimeString('en-US', {
      hour: 'numeric',
      minute: undefined,
      hour12: true,
    });
  }
  return date.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
  });
}

function formatWindowLabel(startedAt?: string, endedAt?: string) {
  if (!startedAt || !endedAt) {
    return '—';
  }
  const start = new Date(startedAt);
  const end = new Date(endedAt);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    return '—';
  }
  const fmt = (value: Date) =>
    value
      .toLocaleDateString('en-GB', { day: 'numeric', month: 'long' })
      .toUpperCase();
  return `${fmt(start)} — ${fmt(end)}`;
}

export function ZoneOverviewPanel({
  overview,
  zoneId,
}: {
  overview: ZoneOverview;
  zoneId: number;
}) {
  const [range, setRange] = useState<ZoneStatsRange>('24h');

  const statsQuery = useQuery({
    queryKey: [...zoneQueryKey, zoneId, 'stats', range],
    queryFn: () => ZoneService.getStats(zoneId, range),
    enabled: zoneId > 0,
  });

  const stats = statsQuery.data;
  const chartData = useMemo(
    () =>
      (stats?.series ?? []).map((point) => ({
        label: formatAxisLabel(point.bucket_started_at, range),
        at: point.bucket_started_at,
        visitors: point.unique_visitors,
        requests: point.request_count,
        bytes: point.bytes_sent,
      })),
    [range, stats?.series],
  );

  return (
    <div className='space-y-6'>
      <div className='space-y-4'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='inline-flex rounded-lg border bg-muted/40 p-0.5'>
            {rangeOptions.map((option) => (
              <Button
                key={option.value}
                type='button'
                size='sm'
                variant='ghost'
                className={cn(
                  'h-7 rounded-md px-3 text-xs font-medium shadow-none',
                  range === option.value
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground',
                )}
                onClick={() => setRange(option.value)}
              >
                {option.label}
              </Button>
            ))}
          </div>
          <p className='text-xs font-medium tracking-wide text-muted-foreground'>
            {formatWindowLabel(
              stats?.window_started_at,
              stats?.window_ended_at,
            )}
          </p>
        </div>

        {statsQuery.isLoading ? (
          <div className='space-y-3'>
            {Array.from({ length: 3 }).map((_, index) => (
              <Skeleton key={index} className='h-36 w-full rounded-xl' />
            ))}
          </div>
        ) : statsQuery.isError ? (
          <div className='rounded-xl border border-dashed p-6 text-center text-sm text-muted-foreground'>
            加载流量统计失败
            <div className='mt-3'>
              <Button
                size='sm'
                variant='outline'
                className='h-7 text-xs'
                onClick={() => void statsQuery.refetch()}
              >
                重试
              </Button>
            </div>
          </div>
        ) : (
          <div className='space-y-3'>
            {!stats?.available ? (
              <p className='text-xs text-muted-foreground'>
                分析存储暂不可用，图表可能为空。请确认 ClickHouse 已启用。
              </p>
            ) : null}

            <MetricTrendCard
              icon={Users}
              label='唯一访问者'
              value={formatCompactNumber(stats?.unique_visitors ?? 0)}
              data={chartData}
              dataKey='visitors'
              config={visitorsChartConfig}
              gradientId='zone-visitors'
            />
            <MetricTrendCard
              icon={Activity}
              label='请求总数'
              value={formatCompactNumber(stats?.request_count ?? 0)}
              data={chartData}
              dataKey='requests'
              config={requestsChartConfig}
              gradientId='zone-requests'
            />
            <MetricTrendCard
              icon={HardDrive}
              label='已提供的数据总计'
              value={formatBytes(stats?.bytes_sent ?? 0, { zeroText: '0 B' })}
              data={chartData}
              dataKey='bytes'
              config={bytesChartConfig}
              gradientId='zone-bytes'
              valueFormatter={(value) =>
                formatBytes(value, { zeroText: '0 B' })
              }
            />
          </div>
        )}
      </div>

      <div className='grid gap-4 md:grid-cols-3'>
        <Card className='border-dashed shadow-none md:col-span-3'>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm'>Zone 信息</CardTitle>
          </CardHeader>
          <CardContent className='grid gap-3 text-sm sm:grid-cols-2'>
            <div>
              <p className='text-xs text-muted-foreground'>根域</p>
              <p className='mt-1 font-medium'>{overview.zone.domain}</p>
            </div>
            <div>
              <p className='text-xs text-muted-foreground'>创建时间</p>
              <p className='mt-1'>{formatDateTime(overview.zone.created_at)}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function MetricTrendCard({
  icon: Icon,
  label,
  value,
  data,
  dataKey,
  config,
  gradientId,
  valueFormatter,
}: {
  icon: typeof Users;
  label: string;
  value: string;
  data: Array<Record<string, string | number>>;
  dataKey: string;
  config: ChartConfig;
  gradientId: string;
  valueFormatter?: (value: number) => string;
}) {
  return (
    <Card className='overflow-hidden border shadow-none'>
      <CardContent className='p-0'>
        <div className='grid gap-0 md:grid-cols-[minmax(140px,200px)_1fr]'>
          <div className='flex flex-col justify-between gap-3 border-b p-4 md:border-b-0 md:border-r'>
            <div className='flex items-center gap-1.5 text-xs text-muted-foreground'>
              <Icon className='size-3.5 text-primary' />
              <span>{label}</span>
            </div>
            <p className='text-3xl font-semibold tracking-tight'>{value}</p>
          </div>
          <div className='h-36 min-h-[9rem] w-full px-2 py-3 md:h-40'>
            <ChartContainer
              config={config}
              className='h-full w-full aspect-auto'
            >
              <AreaChart
                data={data}
                margin={{ top: 8, right: 12, left: 0, bottom: 0 }}
              >
                <defs>
                  <linearGradient id={gradientId} x1='0' y1='0' x2='0' y2='1'>
                    <stop
                      offset='5%'
                      stopColor='var(--color-value)'
                      stopOpacity={0.28}
                    />
                    <stop
                      offset='95%'
                      stopColor='var(--color-value)'
                      stopOpacity={0.02}
                    />
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray='3 3'
                  vertical={false}
                  className='stroke-border/60'
                />
                <XAxis
                  dataKey='label'
                  tickLine={false}
                  axisLine={false}
                  minTickGap={28}
                  tickMargin={8}
                  className='text-[10px]'
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  width={36}
                  tickMargin={4}
                  className='text-[10px]'
                  allowDecimals={false}
                  tickFormatter={(tick) =>
                    valueFormatter
                      ? valueFormatter(Number(tick))
                      : formatCompactNumber(Number(tick))
                  }
                />
                <ChartTooltip
                  content={
                    <ChartTooltipContent
                      formatter={(raw) => {
                        const numeric = Number(raw);
                        return valueFormatter
                          ? valueFormatter(numeric)
                          : formatCompactNumber(numeric);
                      }}
                    />
                  }
                />
                <Area
                  type='monotone'
                  dataKey={dataKey}
                  name='value'
                  stroke='var(--color-value)'
                  strokeWidth={2}
                  fill={`url(#${gradientId})`}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ChartContainer>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
