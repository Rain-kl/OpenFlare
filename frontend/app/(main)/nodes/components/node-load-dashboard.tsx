'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';
import {
  ArrowDown,
  ArrowDownUp,
  ArrowUp,
  Cpu,
  HardDrive,
  Loader2,
  MemoryStick,
} from 'lucide-react';
import { useTranslations } from 'next-intl';

import { useChartThemePalette } from '@/components/data/chart-theme';
import { ResourceGauge } from '@/components/data/resource-gauge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type {
  NodeMetricSnapshot,
  NodeSystemProfile,
} from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';

import {
  formatBytes,
  formatBytesPerSecond,
  formatPercent,
  formatUsageRatio,
} from './node-utils';

type LoadRangeKey = 'realtime' | '1h' | '6h' | '24h' | '7d' | '30d';

const LOAD_RANGES: Array<{
  key: LoadRangeKey;
  hours?: number;
  realtime?: boolean;
}> = [
  { key: 'realtime', realtime: true },
  { key: '1h', hours: 1 },
  { key: '6h', hours: 6 },
  { key: '24h', hours: 24 },
  { key: '7d', hours: 168 },
  { key: '30d', hours: 720 },
];

function formatAxisTimestamp(iso: string, range: LoadRangeKey) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const pad = (n: number) => String(n).padStart(2, '0');
  if (range === 'realtime' || range === '1h' || range === '6h') {
    return `${pad(date.getHours())}:${pad(date.getMinutes())}${
      range === 'realtime' ? `:${pad(date.getSeconds())}` : ''
    }`;
  }
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function LoadMetricChartCard({
  title,
  readout,
  option,
  emptyText,
  hasData,
}: {
  title: string;
  readout: React.ReactNode;
  option: EChartsOption;
  emptyText: string;
  hasData: boolean;
}) {
  return (
    <Card>
      <CardHeader className='flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='text-sm font-semibold'>{title}</CardTitle>
        <div className='text-xs font-medium tabular-nums text-muted-foreground'>
          {readout}
        </div>
      </CardHeader>
      <CardContent className='pt-1'>
        {hasData ? (
          <ReactECharts
            option={option}
            notMerge
            lazyUpdate
            style={{ height: 192, width: '100%' }}
          />
        ) : (
          <div className='flex h-48 items-center justify-center rounded-lg border border-dashed text-xs text-muted-foreground'>
            {emptyText}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function NodeLoadDashboard({
  nodeId,
  profile,
}: {
  nodeId: number;
  profile: NodeSystemProfile | null;
}) {
  const t = useTranslations('nodes');
  const palette = useChartThemePalette();
  const [range, setRange] = useState<LoadRangeKey>('realtime');

  const selectedConfig =
    LOAD_RANGES.find((item) => item.key === range) ?? LOAD_RANGES[0];

  const loadQuery = useQuery({
    queryKey: ['openflare', 'node-load-metrics', nodeId, range],
    queryFn: () =>
      NodeService.getObservability(nodeId, {
        realtime: selectedConfig.realtime,
        hours: selectedConfig.hours,
        limit: selectedConfig.realtime ? 180 : 240,
      }),
    refetchInterval: selectedConfig.realtime ? 10000 : 60000,
    refetchIntervalInBackground: false,
  });

  const snapshots = useMemo<NodeMetricSnapshot[]>(() => {
    const raw = loadQuery.data?.metric_snapshots ?? [];
    return [...raw].sort(
      (a, b) =>
        new Date(a.captured_at).getTime() - new Date(b.captured_at).getTime(),
    );
  }, [loadQuery.data?.metric_snapshots]);

  const latest = snapshots.length > 0 ? snapshots[snapshots.length - 1] : null;
  const labels = useMemo(
    () => snapshots.map((item) => formatAxisTimestamp(item.captured_at, range)),
    [snapshots, range],
  );

  const memTotal =
    latest?.memory_total_bytes || profile?.total_memory_bytes || 0;
  const swapTotal = latest?.swap_total_bytes ?? profile?.swap_total_bytes ?? 0;
  const diskTotal =
    latest?.storage_total_bytes || profile?.total_disk_bytes || 0;

  const memPct = formatUsageRatio(latest?.memory_used_bytes, memTotal);
  const swapPct =
    swapTotal > 0
      ? formatUsageRatio(latest?.swap_used_bytes ?? 0, swapTotal)
      : null;
  const diskPct = formatUsageRatio(latest?.storage_used_bytes, diskTotal);

  const baseAxisAndGrid = useMemo(
    () => ({
      animationDuration: 350,
      grid: { left: 10, right: 12, top: 14, bottom: 24, containLabel: true },
      tooltip: {
        trigger: 'axis' as const,
        backgroundColor: palette.popover,
        borderColor: palette.border,
        borderWidth: 1,
        textStyle: { color: palette.popoverForeground, fontSize: 12 },
      },
      xAxis: {
        type: 'category' as const,
        boundaryGap: false,
        data: labels,
        axisLine: { lineStyle: { color: palette.border } },
        axisTick: { show: false },
        axisLabel: { color: palette.mutedForeground, fontSize: 10 },
      },
      legend: {
        bottom: 0,
        itemWidth: 10,
        itemHeight: 6,
        textStyle: { color: palette.mutedForeground, fontSize: 11 },
      },
    }),
    [labels, palette],
  );

  const cpuOption = useMemo<EChartsOption>(
    () => ({
      ...baseAxisAndGrid,
      yAxis: [
        {
          type: 'value',
          min: 0,
          max: 100,
          splitNumber: 4,
          axisLabel: {
            color: palette.mutedForeground,
            fontSize: 10,
            formatter: '{value}%',
          },
          splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
        },
        {
          type: 'value',
          min: 0,
          splitNumber: 4,
          axisLabel: { color: palette.mutedForeground, fontSize: 10 },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: 'CPU',
          type: 'line',
          smooth: true,
          showSymbol: false,
          yAxisIndex: 0,
          lineStyle: { width: 2, color: palette.series[0] },
          itemStyle: { color: palette.series[0] },
          areaStyle: { opacity: 0.14, color: palette.series[0] },
          data: snapshots.map((s) =>
            Number((s.cpu_usage_percent ?? 0).toFixed(1)),
          ),
        },
        {
          name: t('overview.load1'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          yAxisIndex: 1,
          lineStyle: { width: 1.5, type: 'dashed', color: palette.series[1] },
          itemStyle: { color: palette.series[1] },
          data: snapshots.map((s) => Number((s.load1 ?? 0).toFixed(2))),
        },
      ],
    }),
    [baseAxisAndGrid, palette, snapshots, t],
  );

  const memoryOption = useMemo<EChartsOption>(
    () => ({
      ...baseAxisAndGrid,
      yAxis: {
        type: 'value',
        min: 0,
        max: 100,
        splitNumber: 4,
        axisLabel: {
          color: palette.mutedForeground,
          fontSize: 10,
          formatter: '{value}%',
        },
        splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
      },
      series: [
        {
          name: t('obs.memory'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: palette.series[1] },
          itemStyle: { color: palette.series[1] },
          areaStyle: { opacity: 0.14, color: palette.series[1] },
          data: snapshots.map((s) => {
            const total = s.memory_total_bytes || memTotal;
            return total > 0
              ? Number(((s.memory_used_bytes / total) * 100).toFixed(1))
              : 0;
          }),
        },
        {
          name: t('overview.swap'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 1.5, type: 'dashed', color: palette.series[4] },
          itemStyle: { color: palette.series[4] },
          data: snapshots.map((s) => {
            const total = s.swap_total_bytes || swapTotal;
            return total > 0
              ? Number((((s.swap_used_bytes ?? 0) / total) * 100).toFixed(1))
              : 0;
          }),
        },
      ],
    }),
    [baseAxisAndGrid, memTotal, palette, snapshots, swapTotal, t],
  );

  const diskOption = useMemo<EChartsOption>(
    () => ({
      ...baseAxisAndGrid,
      yAxis: {
        type: 'value',
        min: 0,
        max: 100,
        splitNumber: 4,
        axisLabel: {
          color: palette.mutedForeground,
          fontSize: 10,
          formatter: '{value}%',
        },
        splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
      },
      series: [
        {
          name: t('overview.disk'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: palette.series[2] },
          itemStyle: { color: palette.series[2] },
          areaStyle: { opacity: 0.14, color: palette.series[2] },
          data: snapshots.map((s) => {
            const total = s.storage_total_bytes || diskTotal;
            return total > 0
              ? Number(((s.storage_used_bytes / total) * 100).toFixed(1))
              : 0;
          }),
        },
      ],
    }),
    [baseAxisAndGrid, diskTotal, palette, snapshots, t],
  );

  const networkOption = useMemo<EChartsOption>(
    () => ({
      ...baseAxisAndGrid,
      yAxis: {
        type: 'value',
        min: 0,
        splitNumber: 4,
        axisLabel: {
          color: palette.mutedForeground,
          fontSize: 10,
          formatter: (val: number) => formatBytesPerSecond(val),
        },
        splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
      },
      series: [
        {
          name: t('overview.netOut'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: palette.series[0] },
          itemStyle: { color: palette.series[0] },
          data: snapshots.map((s) => s.net_out ?? 0),
        },
        {
          name: t('overview.netIn'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: palette.series[3] },
          itemStyle: { color: palette.series[3] },
          data: snapshots.map((s) => s.net_in ?? 0),
        },
      ],
    }),
    [baseAxisAndGrid, palette, snapshots, t],
  );

  const connectionsOption = useMemo<EChartsOption>(
    () => ({
      ...baseAxisAndGrid,
      yAxis: {
        type: 'value',
        min: 0,
        minInterval: 1,
        splitNumber: 4,
        axisLabel: { color: palette.mutedForeground, fontSize: 10 },
        splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
      },
      series: [
        {
          name: 'TCP',
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: palette.series[3] },
          itemStyle: { color: palette.series[3] },
          data: snapshots.map((s) => s.tcp_connections ?? 0),
        },
        {
          name: 'UDP',
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: palette.series[4] },
          itemStyle: { color: palette.series[4] },
          data: snapshots.map((s) => s.udp_connections ?? 0),
        },
      ],
    }),
    [baseAxisAndGrid, palette, snapshots],
  );

  const processOption = useMemo<EChartsOption>(
    () => ({
      ...baseAxisAndGrid,
      yAxis: {
        type: 'value',
        min: 0,
        minInterval: 1,
        splitNumber: 4,
        axisLabel: { color: palette.mutedForeground, fontSize: 10 },
        splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
      },
      series: [
        {
          name: t('overview.processes'),
          type: 'line',
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: palette.series[1] },
          itemStyle: { color: palette.series[1] },
          areaStyle: { opacity: 0.14, color: palette.series[1] },
          data: snapshots.map((s) => s.process_count ?? 0),
        },
      ],
    }),
    [baseAxisAndGrid, palette, snapshots, t],
  );

  const hasData = snapshots.length > 0;

  return (
    <section className='space-y-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <h2 className='text-base font-semibold tracking-tight'>
            {t('overview.loadTitle')}
          </h2>
          <p className='text-xs text-muted-foreground'>
            {t('overview.loadDesc')}
          </p>
        </div>

        <div className='flex items-center gap-2'>
          {loadQuery.isFetching ? (
            <Loader2 className='size-3.5 animate-spin text-muted-foreground' />
          ) : null}
          <Tabs
            value={range}
            onValueChange={(value) => setRange(value as LoadRangeKey)}
          >
            <TabsList className='h-8'>
              {LOAD_RANGES.map((item) => (
                <TabsTrigger
                  key={item.key}
                  value={item.key}
                  className='h-6.5 px-2.5 text-xs'
                >
                  {t(`overview.ranges.${item.key}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
      </div>

      {/* Reusable Dashboard Resource Gauges */}
      <div className='grid grid-cols-2 gap-4 xl:grid-cols-4'>
        <ResourceGauge
          label={t('obs.cpu')}
          icon={Cpu}
          tone='chart-1'
          value={latest?.cpu_usage_percent ?? null}
          subtitle={
            latest
              ? `${t('overview.loadShort')} ${(latest.load1 ?? 0).toFixed(2)} · ${(latest.load5 ?? 0).toFixed(2)} · ${(latest.load15 ?? 0).toFixed(2)}`
              : '—'
          }
          detail={
            latest?.temp && latest.temp > 0
              ? `${latest.temp.toFixed(1)} °C`
              : profile?.cpu_cores
                ? t('obs.cpuCores', { count: profile.cpu_cores })
                : undefined
          }
        />
        <ResourceGauge
          label={t('obs.memory')}
          icon={MemoryStick}
          tone='chart-2'
          value={memPct}
          subtitle={
            latest
              ? `${formatBytes(latest.memory_used_bytes)} / ${formatBytes(memTotal)}`
              : '—'
          }
          detail={
            latest?.gpu_usage_percent && latest.gpu_usage_percent > 0
              ? `GPU ${formatPercent(latest.gpu_usage_percent)}`
              : undefined
          }
        />
        <ResourceGauge
          label={t('overview.swap')}
          icon={ArrowDownUp}
          tone='chart-4'
          value={swapPct}
          displayValue={
            swapTotal > 0 && swapPct !== null
              ? `${swapPct.toFixed(1)}%`
              : t('overview.swapInactive')
          }
          subtitle={
            swapTotal > 0
              ? `${formatBytes(latest?.swap_used_bytes ?? 0)} / ${formatBytes(swapTotal)}`
              : '0 B / 0 B'
          }
        />
        <ResourceGauge
          label={t('overview.disk')}
          icon={HardDrive}
          tone='chart-3'
          value={diskPct}
          subtitle={
            latest
              ? `${formatBytes(latest.storage_used_bytes)} / ${formatBytes(diskTotal)}`
              : '—'
          }
          detail={
            latest
              ? `TCP ${latest.tcp_connections ?? 0} · UDP ${latest.udp_connections ?? 0}`
              : undefined
          }
        />
      </div>

      {/* 6-Card Performance Charts Grid (emerald-insights layout) */}
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
        <LoadMetricChartCard
          title={t('obs.cpu')}
          readout={
            latest ? (
              <span>
                {formatPercent(latest.cpu_usage_percent)} ·{' '}
                {(latest.load1 ?? 0).toFixed(2)}
              </span>
            ) : (
              '—'
            )
          }
          option={cpuOption}
          emptyText={t('overview.noLoadData')}
          hasData={hasData}
        />
        <LoadMetricChartCard
          title={t('obs.memory')}
          readout={
            latest
              ? `${formatBytes(latest.memory_used_bytes)} / ${formatBytes(memTotal)}`
              : '—'
          }
          option={memoryOption}
          emptyText={t('overview.noLoadData')}
          hasData={hasData}
        />
        <LoadMetricChartCard
          title={t('overview.disk')}
          readout={
            latest
              ? `${formatBytes(latest.storage_used_bytes)} / ${formatBytes(diskTotal)}`
              : '—'
          }
          option={diskOption}
          emptyText={t('overview.noLoadData')}
          hasData={hasData}
        />
        <LoadMetricChartCard
          title={t('overview.networkChart')}
          readout={
            latest ? (
              <span className='inline-flex items-center gap-2'>
                <span className='inline-flex items-center gap-0.5'>
                  <ArrowUp className='size-3' />
                  {formatBytesPerSecond(latest.net_out ?? 0)}
                </span>
                <span className='inline-flex items-center gap-0.5'>
                  <ArrowDown className='size-3' />
                  {formatBytesPerSecond(latest.net_in ?? 0)}
                </span>
              </span>
            ) : (
              '—'
            )
          }
          option={networkOption}
          emptyText={t('overview.noLoadData')}
          hasData={hasData}
        />
        <LoadMetricChartCard
          title={t('obs.connections')}
          readout={
            latest
              ? `TCP ${latest.tcp_connections ?? 0} · UDP ${latest.udp_connections ?? 0}`
              : '—'
          }
          option={connectionsOption}
          emptyText={t('overview.noLoadData')}
          hasData={hasData}
        />
        <LoadMetricChartCard
          title={t('overview.processes')}
          readout={latest ? String(latest.process_count ?? 0) : '—'}
          option={processOption}
          emptyText={t('overview.noLoadData')}
          hasData={hasData}
        />
      </div>
    </section>
  );
}
