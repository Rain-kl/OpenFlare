'use client';

import { useMemo } from 'react';
import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';
import { Cell, Pie, PieChart } from 'recharts';

import { RankCard } from '@/components/data/rank-card';
import { TrendChart } from '@/components/data/trend-chart';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart';
import type {
  AccessLogOverview,
  DistributionItem,
} from '@/lib/services/openflare';
import { formatBytes, formatCompactNumber } from '@/lib/utils/metrics';

import {
  formatOverviewRangeHint,
  formatOverviewTrendLabel,
  type OverviewRangeHours,
} from './access-log-utils';
import { OverviewToolbar } from './overview-toolbar';

const DEVICE_COLORS = [
  '#38bdf8',
  '#34d399',
  '#f59e0b',
  '#a78bfa',
  '#f472b6',
  '#94a3b8',
];

function SparklineMetricCard({
  title,
  value,
  hint,
  color,
  fillColor,
  labels,
  values,
  valueFormatter,
}: {
  title: string;
  value: string;
  hint: string;
  color: string;
  fillColor: string;
  labels: string[];
  values: number[];
  valueFormatter?: (value: number) => string;
}) {
  const option = useMemo<EChartsOption>(
    () => ({
      animationDuration: 400,
      grid: {
        left: 0,
        right: 0,
        top: 8,
        bottom: 0,
      },
      xAxis: {
        type: 'category',
        show: false,
        boundaryGap: false,
        data: labels,
      },
      yAxis: {
        type: 'value',
        show: false,
        min: 0,
      },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(15, 23, 42, 0.92)',
        borderWidth: 0,
        textStyle: {
          color: '#e2e8f0',
          fontSize: 12,
        },
        formatter: (params: unknown) => {
          const items = Array.isArray(params) ? params : [];
          const item = items[0] as
            { axisValueLabel?: string; value?: number } | undefined;
          if (!item) return '';
          const raw = typeof item.value === 'number' ? item.value : 0;
          const formatted = valueFormatter
            ? valueFormatter(raw)
            : formatCompactNumber(raw);
          return `${item.axisValueLabel ?? ''}<br/>${formatted}`;
        },
      },
      series: [
        {
          type: 'line',
          data: values,
          smooth: true,
          showSymbol: false,
          lineStyle: {
            color,
            width: 2,
          },
          areaStyle: {
            color: fillColor,
          },
        },
      ],
    }),
    [color, fillColor, labels, valueFormatter, values],
  );

  return (
    <Card className='border-dashed shadow-none'>
      <CardContent className='p-4'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0 space-y-1'>
            <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
              {title}
            </p>
            <p className='text-2xl font-semibold tracking-tight'>{value}</p>
            <p className='text-[11px] text-muted-foreground'>{hint}</p>
          </div>
          <div className='h-16 w-28 shrink-0 sm:w-36'>
            {labels.length > 0 ? (
              <ReactECharts
                option={option}
                notMerge
                lazyUpdate
                style={{ height: '100%', width: '100%' }}
              />
            ) : null}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function toRankItems(items: DistributionItem[] | undefined) {
  return (items ?? []).map((item) => ({
    label: item.key,
    value: item.value,
  }));
}

function PieDistributionCard({
  title,
  description,
  items,
  emptyMessage,
}: {
  title: string;
  description: string;
  items: DistributionItem[];
  emptyMessage: string;
}) {
  const chartData = useMemo(
    () =>
      items.map((item, index) => ({
        name: item.key,
        value: item.value,
        fill: DEVICE_COLORS[index % DEVICE_COLORS.length],
      })),
    [items],
  );

  const chartConfig = useMemo(() => {
    const config: ChartConfig = {};
    chartData.forEach((item) => {
      config[item.name] = {
        label: item.name,
        color: item.fill,
      };
    });
    return config;
  }, [chartData]);

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='pb-3'>
        <CardTitle className='text-sm font-semibold text-foreground'>
          {title}
        </CardTitle>
        <CardDescription className='text-xs text-muted-foreground'>
          {description}
        </CardDescription>
      </CardHeader>
      <CardContent className='pt-0'>
        {chartData.length === 0 ? (
          <div className='flex h-[300px] items-center justify-center rounded-md border border-dashed bg-muted/20 text-sm text-muted-foreground'>
            {emptyMessage}
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            className='mx-auto h-[300px] w-full'
          >
            <PieChart>
              <Pie
                data={chartData}
                dataKey='value'
                nameKey='name'
                cx='50%'
                cy='46%'
                innerRadius={50}
                outerRadius={80}
                paddingAngle={2}
              >
                {chartData.map((entry) => (
                  <Cell key={entry.name} fill={entry.fill} />
                ))}
              </Pie>
              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    hideLabel
                    formatter={(value, name) => (
                      <>
                        <span className='text-muted-foreground'>{name}</span>
                        <span className='ml-auto font-mono font-medium tabular-nums text-foreground'>
                          {formatCompactNumber(Number(value ?? 0))}
                        </span>
                      </>
                    )}
                  />
                }
              />
              <ChartLegend
                content={<ChartLegendContent nameKey='name' />}
                className='flex-wrap justify-center gap-x-4 gap-y-1 pt-2 text-[11px]'
              />
            </PieChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}

export function OverviewTab({
  data,
  loading,
  error,
  hours,
  hosts,
  onHoursChange,
  onHostsChange,
  onRetry,
}: {
  data?: AccessLogOverview;
  loading: boolean;
  error: Error | null;
  hours: OverviewRangeHours;
  hosts: string[];
  onHoursChange: (hours: OverviewRangeHours) => void;
  onHostsChange: (hosts: string[]) => void;
  onRetry: () => void;
}) {
  return (
    <div className='space-y-6'>
      <OverviewToolbar
        hours={hours}
        hosts={hosts}
        onHoursChange={onHoursChange}
        onHostsChange={onHostsChange}
      />

      {loading ? (
        <LoadingStateWithBorder
          title='加载访问概览'
          description='正在聚合请求量、访问量与带宽趋势...'
        />
      ) : error ? (
        <ErrorInline
          message={error.message || '加载访问概览失败'}
          onRetry={onRetry}
        />
      ) : !data ? (
        <EmptyStateWithBorder
          title='暂无概览数据'
          description='当前时间范围内没有可展示的访问统计。'
        />
      ) : (
        <OverviewContent data={data} hours={hours} />
      )}
    </div>
  );
}

function OverviewContent({
  data,
  hours,
}: {
  data: AccessLogOverview;
  hours: number;
}) {
  const requestLabels = data.trends.requests.map((point) =>
    formatOverviewTrendLabel(point.bucket_started_at, hours),
  );
  const requestValues = data.trends.requests.map((point) => point.value);
  const visitValues = data.trends.visits.map((point) => point.value);
  const bandwidthValues = data.trends.bandwidth.map((point) => point.value);
  const hint = formatOverviewRangeHint(hours);

  return (
    <>
      <div className='grid gap-4 lg:grid-cols-3'>
        <SparklineMetricCard
          title='Total Requests'
          value={formatCompactNumber(data.summary.total_requests)}
          hint={hint}
          color='#f59e0b'
          fillColor='rgba(245, 158, 11, 0.18)'
          labels={requestLabels}
          values={requestValues}
        />
        <SparklineMetricCard
          title='Total Visits'
          value={formatCompactNumber(data.summary.total_visits)}
          hint={`${hint} · 独立访客`}
          color='#38bdf8'
          fillColor='rgba(56, 189, 248, 0.16)'
          labels={requestLabels}
          values={visitValues}
        />
        <SparklineMetricCard
          title='Bandwidth Served'
          value={formatBytes(data.summary.bandwidth_served)}
          hint={`${hint} · 已提供数据`}
          color='#34d399'
          fillColor='rgba(52, 211, 153, 0.16)'
          labels={requestLabels}
          values={bandwidthValues}
          valueFormatter={formatBytes}
        />
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-sm font-semibold'>
            Requests over time
          </CardTitle>
          <CardDescription className='text-xs'>
            观察请求量是否出现异常抬升或回落。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <TrendChart
            labels={requestLabels}
            height={280}
            showSummary={false}
            series={[
              {
                label: '请求量',
                color: '#f59e0b',
                fillColor: 'rgba(245, 158, 11, 0.18)',
                variant: 'area',
                values: requestValues,
              },
            ]}
          />
        </CardContent>
      </Card>

      <div className='grid gap-6 xl:grid-cols-2'>
        <PieDistributionCard
          title='Requests by device type'
          description='按设备类型统计请求占比。'
          items={data.device_types ?? []}
          emptyMessage='暂无设备类型数据'
        />
        <PieDistributionCard
          title='Status code'
          description='按 HTTP 状态码统计请求占比。'
          items={data.status_codes ?? []}
          emptyMessage='暂无状态码分布数据'
        />
      </div>

      <div className='grid gap-6 xl:grid-cols-3'>
        <RankCard
          title='Top Paths'
          description='访问量最高的请求路径。'
          items={toRankItems(data.top_paths)}
        />
        <RankCard
          title='Top Hosts'
          description='流量集中的访问域名。'
          items={toRankItems(data.top_hosts)}
        />
        <RankCard
          title='Top IPs'
          description='请求次数最多的来源 IP。'
          items={toRankItems(data.top_ips)}
        />
      </div>

      <div className='grid gap-6 xl:grid-cols-3'>
        <RankCard
          title='Top browsers'
          description='按浏览器聚合的请求排行。'
          items={toRankItems(data.top_browsers)}
        />
        <RankCard
          title='Top Operating System'
          description='按操作系统聚合的请求排行。'
          items={toRankItems(data.top_operating_systems)}
        />
        <RankCard
          title='Top User-Agent'
          description='原始 User-Agent 请求排行。'
          items={toRankItems(data.top_user_agents)}
        />
      </div>
    </>
  );
}
