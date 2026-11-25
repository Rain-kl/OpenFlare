'use client';

import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';
import { Info, Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { useChartThemePalette } from '@/components/data/chart-theme';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import type { PingRecord, PingTaskStats } from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

type PingRangeHours = 1 | 6 | 12 | 24 | 168 | 720;

const PING_RANGES: Array<{ hours: PingRangeHours; key: string }> = [
  { hours: 1, key: '1h' },
  { hours: 6, key: '6h' },
  { hours: 12, key: '12h' },
  { hours: 24, key: '24h' },
  { hours: 168, key: '7d' },
  { hours: 720, key: '30d' },
];

function formatPingTimestamp(iso: string, hours: PingRangeHours) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const pad = (n: number) => String(n).padStart(2, '0');
  if (hours <= 12) {
    return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  }
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/**
 * Applies EWMA smoothing while clipping isolated single-point outliers.
 */
function smoothSeries(values: Array<number | null>, alpha = 0.28) {
  let ema: number | null = null;
  return values.map((v, idx) => {
    if (v === null) return null;
    if (ema === null) {
      ema = v;
      return v;
    }
    const prev = idx > 0 ? values[idx - 1] : null;
    const next = idx + 1 < values.length ? values[idx + 1] : null;
    let clamped = v;
    if (
      prev !== null &&
      next !== null &&
      v > Math.max(prev, next) * 1.8 &&
      v - Math.max(prev, next) > 30
    ) {
      clamped = (prev + next) / 2;
    }
    ema = alpha * clamped + (1 - alpha) * ema;
    return Math.round(ema * 10) / 10;
  });
}

function TaskStatCard({
  task,
  color,
  selected,
  onToggle,
}: {
  task: PingTaskStats;
  color: string;
  selected: boolean;
  onToggle: () => void;
}) {
  const t = useTranslations('nodes');

  return (
    <div
      role='button'
      aria-pressed={selected}
      tabIndex={0}
      onClick={onToggle}
      onKeyDown={(e) => {
        if (
          e.target === e.currentTarget &&
          (e.key === 'Enter' || e.key === ' ')
        ) {
          e.preventDefault();
          onToggle();
        }
      }}
      className={cn(
        'flex cursor-pointer select-none items-center gap-3 rounded-xl border border-dashed bg-card p-3 transition-colors hover:border-primary/40',
        !selected && 'opacity-40',
      )}
    >
      <div className='min-w-0 flex-1'>
        <div className='flex items-center gap-2'>
          <span
            className='h-4 w-1 shrink-0 rounded-full'
            style={{ backgroundColor: color }}
          />
          <span className='truncate text-sm font-semibold text-foreground'>
            {task.name}
          </span>
          <div className='flex-1' />
          <Popover>
            <PopoverTrigger asChild>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-6 shrink-0 text-muted-foreground'
                aria-label={t('ping.statsDetails')}
                onClick={(e) => e.stopPropagation()}
              >
                <Info className='size-3.5' />
              </Button>
            </PopoverTrigger>
            <PopoverContent
              align='end'
              className='w-64 p-3 text-xs'
              onClick={(e) => e.stopPropagation()}
            >
              <p className='mb-2 truncate font-semibold text-foreground'>
                {task.name}
              </p>
              <div className='grid grid-cols-2 gap-x-3 gap-y-1.5 tabular-nums'>
                <span className='text-muted-foreground'>{t('ping.min')}</span>
                <span className='text-right font-medium'>{task.min} ms</span>
                <span className='text-muted-foreground'>{t('ping.max')}</span>
                <span className='text-right font-medium'>{task.max} ms</span>
                <span className='text-muted-foreground'>{t('ping.avg')}</span>
                <span className='text-right font-medium'>
                  {Math.round(task.avg)} ms
                </span>
                <span className='text-muted-foreground'>
                  {t('ping.latest')}
                </span>
                <span className='text-right font-medium'>
                  {task.latest >= 0 ? `${task.latest} ms` : '—'}
                </span>
                <span className='text-muted-foreground'>P50</span>
                <span className='text-right font-medium'>{task.p50} ms</span>
                <span className='text-muted-foreground'>P99</span>
                <span className='text-right font-medium'>{task.p99} ms</span>
                <span className='text-muted-foreground'>
                  {t('ping.jitter')}
                </span>
                <span className='text-right font-medium'>
                  {task.p99_p50_ratio.toFixed(2)}
                </span>
                <span className='text-muted-foreground'>
                  {t('ping.interval')}
                </span>
                <span className='text-right font-medium'>{task.interval}s</span>
                <span className='text-muted-foreground'>{t('ping.type')}</span>
                <span className='text-right font-medium'>
                  {task.type.toUpperCase()}
                </span>
                <span className='text-muted-foreground'>
                  {t('ping.samples')}
                </span>
                <span className='text-right font-medium'>{task.total}</span>
              </div>
            </PopoverContent>
          </Popover>
        </div>

        <div className='mt-1.5 flex items-center gap-1.5 text-xs tabular-nums text-muted-foreground'>
          <span className='font-medium text-foreground'>
            {task.total > 0 && task.avg > 0
              ? `${Math.round(task.avg)} ms`
              : '—'}
          </span>
          <span className='opacity-60'>·</span>
          <span>{task.loss.toFixed(2)}%</span>
          <span className='opacity-60'>·</span>
          <span>{task.p99_p50_ratio.toFixed(2)}</span>
        </div>
      </div>
    </div>
  );
}

export function NodePingPanel({
  nodeId,
  showIntro = true,
}: {
  nodeId: number;
  showIntro?: boolean;
}) {
  const t = useTranslations('nodes');
  const palette = useChartThemePalette();
  const [hours, setHours] = useState<PingRangeHours>(1);
  const [selectedTaskIds, setSelectedTaskIds] = useState<number[]>([]);
  const [initializedTaskKey, setInitializedTaskKey] = useState<string>('');
  const [showDelay, setShowDelay] = useState(true);
  const [showLoss, setShowLoss] = useState(true);
  const [cutPeak, setCutPeak] = useState(false);

  const pingQuery = useQuery({
    queryKey: ['openflare', 'node-ping-records', nodeId, hours],
    queryFn: () =>
      NodeService.getPingRecords(nodeId, { hours, max_points: 400 }),
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
  });

  const tasks = useMemo(
    () => pingQuery.data?.tasks ?? [],
    [pingQuery.data?.tasks],
  );
  const records = useMemo(
    () => pingQuery.data?.records ?? [],
    [pingQuery.data?.records],
  );

  const taskSignature = useMemo(
    () => tasks.map((item) => item.id).join(','),
    [tasks],
  );

  useEffect(() => {
    if (taskSignature && taskSignature !== initializedTaskKey) {
      setSelectedTaskIds(tasks.map((item) => item.id));
      setInitializedTaskKey(taskSignature);
    }
  }, [initializedTaskKey, taskSignature, tasks]);

  const toggleTask = (taskId: number) => {
    setSelectedTaskIds((prev) =>
      prev.includes(taskId)
        ? prev.filter((id) => id !== taskId)
        : [...prev, taskId],
    );
  };

  const chartOption = useMemo<EChartsOption>(() => {
    const activeTasks = tasks.filter((item) =>
      selectedTaskIds.includes(item.id),
    );
    if (activeTasks.length === 0 || records.length === 0) {
      return {};
    }

    // Group timestamps into shared time slots (aligned by nearest 30s or proportional to range)
    const bucketMs = hours <= 1 ? 15_000 : hours <= 24 ? 60_000 : 300_000;
    const slotMap = new Map<number, PingRecord[]>();
    for (const rec of records) {
      if (!selectedTaskIds.includes(rec.task_id)) continue;
      const ms = new Date(rec.recorded_at).getTime();
      if (Number.isNaN(ms)) continue;
      const slot = Math.floor(ms / bucketMs) * bucketMs;
      const list = slotMap.get(slot);
      if (list) list.push(rec);
      else slotMap.set(slot, [rec]);
    }
    const slots = Array.from(slotMap.keys()).sort((a, b) => a - b);
    if (slots.length === 0) return {};
    const xLabels = slots.map((ms) =>
      formatPingTimestamp(new Date(ms).toISOString(), hours),
    );

    const series: EChartsOption['series'] = [];
    const dualGrid = showDelay && showLoss;

    if (showDelay) {
      activeTasks.forEach((task) => {
        const taskIdx = tasks.findIndex((item) => item.id === task.id);
        const color = palette.series[taskIdx % palette.series.length];
        const rawValues = slots.map((slot) => {
          const items = (slotMap.get(slot) ?? []).filter(
            (r) => r.task_id === task.id && r.value >= 0,
          );
          if (items.length === 0) return null;
          const avg =
            items.reduce((acc, item) => acc + item.value, 0) / items.length;
          return Math.round(avg * 10) / 10;
        });
        const values = cutPeak ? smoothSeries(rawValues) : rawValues;
        series.push({
          name: task.name,
          type: 'line',
          xAxisIndex: 0,
          yAxisIndex: 0,
          smooth: true,
          connectNulls: true,
          showSymbol: values.filter((value) => value !== null).length === 1,
          lineStyle: { width: 2, color },
          itemStyle: { color },
          data: values,
        });
      });
    }

    if (showLoss) {
      const lossData = slots.map((slot) => {
        const items = slotMap.get(slot) ?? [];
        if (items.length === 0) return 0;
        const lost = items.filter((r) => r.value < 0).length;
        return Math.round((lost / items.length) * 1000) / 10;
      });
      series.push({
        name: t('ping.loss'),
        type: 'bar',
        xAxisIndex: dualGrid ? 1 : 0,
        yAxisIndex: dualGrid ? 1 : 0,
        barMaxWidth: 8,
        itemStyle: { color: palette.destructive, opacity: 0.75 },
        data: lossData,
      });
    }

    const grids = dualGrid
      ? [
          { left: 12, right: 16, top: 18, height: '58%', containLabel: true },
          {
            left: 12,
            right: 16,
            top: '76%',
            height: '16%',
            containLabel: true,
          },
        ]
      : [{ left: 12, right: 16, top: 18, bottom: 26, containLabel: true }];

    const xAxes: EChartsOption['xAxis'] = dualGrid
      ? [
          {
            type: 'category',
            gridIndex: 0,
            boundaryGap: false,
            data: xLabels,
            axisLine: { lineStyle: { color: palette.border } },
            axisTick: { show: false },
            axisLabel: { show: false },
          },
          {
            type: 'category',
            gridIndex: 1,
            boundaryGap: false,
            data: xLabels,
            axisLine: { lineStyle: { color: palette.border } },
            axisTick: { show: false },
            axisLabel: { color: palette.mutedForeground, fontSize: 10 },
          },
        ]
      : {
          type: 'category',
          boundaryGap: false,
          data: xLabels,
          axisLine: { lineStyle: { color: palette.border } },
          axisTick: { show: false },
          axisLabel: { color: palette.mutedForeground, fontSize: 10 },
        };

    const yAxes: EChartsOption['yAxis'] = dualGrid
      ? [
          {
            type: 'value',
            gridIndex: 0,
            min: 0,
            axisLabel: {
              color: palette.mutedForeground,
              fontSize: 10,
              formatter: '{value} ms',
            },
            splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
          },
          {
            type: 'value',
            gridIndex: 1,
            min: 0,
            max: 100,
            splitNumber: 2,
            axisLabel: {
              color: palette.mutedForeground,
              fontSize: 10,
              formatter: '{value}%',
            },
            splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
          },
        ]
      : {
          type: 'value',
          min: 0,
          max: !showDelay && showLoss ? 100 : undefined,
          axisLabel: {
            color: palette.mutedForeground,
            fontSize: 10,
            formatter: !showDelay && showLoss ? '{value}%' : '{value} ms',
          },
          splitLine: { lineStyle: { color: palette.border, type: 'dashed' } },
        };

    const noLoss =
      showLoss &&
      activeTasks.every((task) => task.total > 0 && task.loss === 0);
    return {
      animationDuration: 350,
      graphic: noLoss
        ? {
            type: 'text',
            left: 'center',
            top: dualGrid ? '83%' : '50%',
            silent: true,
            style: {
              text: t('ping.noLoss'),
              fill: palette.mutedForeground,
              fontSize: 12,
            },
          }
        : undefined,
      axisPointer: dualGrid ? { link: [{ xAxisIndex: 'all' }] } : undefined,
      tooltip: {
        trigger: 'axis',
        backgroundColor: palette.popover,
        borderColor: palette.border,
        borderWidth: 1,
        textStyle: { color: palette.popoverForeground, fontSize: 12 },
      },
      grid: grids,
      xAxis: xAxes,
      yAxis: yAxes,
      series,
    };
  }, [
    cutPeak,
    hours,
    palette,
    records,
    selectedTaskIds,
    showDelay,
    showLoss,
    t,
    tasks,
  ]);

  const hasSelection = selectedTaskIds.length > 0 && (showDelay || showLoss);
  const hasChartData = hasSelection && Boolean(chartOption.series);

  return (
    <section
      className='flex min-w-0 flex-col gap-4'
      aria-busy={pingQuery.isFetching}
    >
      {showIntro && (
        <div>
          <h2 className='text-base font-semibold tracking-tight'>
            {t('ping.sectionTitle')}
          </h2>
          <p className='text-xs text-muted-foreground'>
            {t('ping.sectionDesc')}
          </p>
        </div>
      )}
      <div className='flex flex-wrap items-center gap-3'>
        <Tabs
          value={String(hours)}
          onValueChange={(value) => setHours(Number(value) as PingRangeHours)}
        >
          <TabsList className='h-auto min-h-8 flex-wrap justify-start'>
            {PING_RANGES.map((item) => (
              <TabsTrigger
                key={item.key}
                value={String(item.hours)}
                className='h-6.5 px-2.5 text-xs'
              >
                {t(`overview.ranges.${item.key}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {tasks.length > 0 ? (
          <div className='flex items-center gap-1 sm:ml-auto'>
            <Button
              type='button'
              variant={
                selectedTaskIds.length === tasks.length
                  ? 'secondary'
                  : 'outline'
              }
              size='sm'
              className='h-8 text-xs'
              onClick={() => setSelectedTaskIds(tasks.map((item) => item.id))}
            >
              {t('ping.selectAll')}
            </Button>
            <Button
              type='button'
              variant={selectedTaskIds.length === 0 ? 'secondary' : 'outline'}
              size='sm'
              className='h-8 text-xs'
              onClick={() => setSelectedTaskIds([])}
            >
              {t('ping.deselectAll')}
            </Button>
          </div>
        ) : null}
        {pingQuery.isFetching && !pingQuery.isPending && (
          <Loader2
            className='size-3.5 animate-spin text-muted-foreground'
            aria-hidden
          />
        )}
      </div>

      {pingQuery.isPending ? (
        <LoadingStateWithBorder />
      ) : pingQuery.isLoadingError ? (
        <ErrorInline
          message={t('requestFailed')}
          onRetry={() => void pingQuery.refetch()}
        />
      ) : tasks.length === 0 ? (
        <Card>
          <CardContent className='flex items-center justify-center py-10 text-sm text-muted-foreground'>
            {t('ping.noTasksForNode')}
          </CardContent>
        </Card>
      ) : (
        <>
          <div className='grid grid-cols-[repeat(auto-fit,minmax(min(100%,12rem),1fr))] gap-3'>
            {tasks.map((task, idx) => (
              <TaskStatCard
                key={task.id}
                task={task}
                color={palette.series[idx % palette.series.length]}
                selected={selectedTaskIds.includes(task.id)}
                onToggle={() => toggleTask(task.id)}
              />
            ))}
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <Button
              type='button'
              variant={showDelay ? 'secondary' : 'outline'}
              aria-pressed={showDelay}
              size='sm'
              className='h-7 text-xs'
              onClick={() => setShowDelay((v) => !v)}
            >
              {t('ping.toggleDelay')}
            </Button>
            <Button
              type='button'
              variant={showLoss ? 'secondary' : 'outline'}
              aria-pressed={showLoss}
              size='sm'
              className='h-7 text-xs'
              onClick={() => setShowLoss((v) => !v)}
            >
              {t('ping.toggleLoss')}
            </Button>
            <div className='flex items-center gap-1'>
              <Button
                type='button'
                variant={cutPeak ? 'secondary' : 'outline'}
                aria-pressed={cutPeak}
                size='sm'
                className='h-7 text-xs'
                onClick={() => setCutPeak((v) => !v)}
              >
                {t('ping.toggleSmooth')}
              </Button>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='size-7 text-muted-foreground'
                      aria-label={t('ping.smoothHint')}
                    >
                      <Info className='size-3.5' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('ping.smoothHint')}</TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
          </div>

          <Card>
            <CardContent className='p-4'>
              {hasChartData ? (
                <ReactECharts
                  option={chartOption}
                  notMerge
                  lazyUpdate
                  style={{
                    height: showDelay && showLoss ? 360 : 280,
                    width: '100%',
                  }}
                />
              ) : (
                <div className='flex h-64 items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground'>
                  {t(hasSelection ? 'ping.noRecords' : 'ping.noSelection')}
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </section>
  );
}
