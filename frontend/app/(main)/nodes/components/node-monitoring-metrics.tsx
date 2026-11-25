'use client';

import { ArrowDown, ArrowUp } from 'lucide-react';
import dynamic from 'next/dynamic';
import { useFormatter, useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Progress } from '@/components/ui/progress';
import type { NodeMonitoringPing } from '@/lib/services/openflare/types';
import { cn } from '@/lib/utils';

import { formatBytesPerSecond } from './node-utils';

const TrendChart = dynamic(() =>
  import('@/components/data/trend-chart').then((module) => module.TrendChart),
);

export function NodeUsageBar({
  label,
  value,
}: {
  label: string;
  value?: number | null;
}) {
  const format = useFormatter();
  const percent =
    value == null || !Number.isFinite(value)
      ? null
      : Math.min(100, Math.max(0, value));

  if (percent === null) return <MissingMetric />;

  const displayValue = format.number(percent / 100, {
    style: 'percent',
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });

  return (
    <div className='flex min-w-16 flex-col gap-1.5'>
      <span className='text-xs font-medium tabular-nums'>{displayValue}</span>
      <Progress
        aria-label={label}
        aria-valuetext={displayValue}
        value={percent}
        className={cn(
          'h-1.5 bg-muted',
          percent >= 90 && '[&_[data-slot=progress-indicator]]:bg-destructive',
          percent >= 75 &&
            percent < 90 &&
            '[&_[data-slot=progress-indicator]]:bg-chart-4',
        )}
      />
    </div>
  );
}

export function MissingMetric() {
  return <span className='text-xs text-muted-foreground'>—</span>;
}

export function NodeNetworkRate({
  upload,
  download,
}: {
  upload?: number;
  download?: number;
}) {
  const t = useTranslations('nodes.monitoring');
  if (upload == null && download == null) return <MissingMetric />;

  return (
    <div className='flex flex-col gap-1 text-xs tabular-nums'>
      <div className='flex items-center gap-1.5'>
        <ArrowUp className='size-3.5 text-primary' aria-hidden />
        <span className='sr-only'>{t('upload')}</span>
        {formatBytesPerSecond(upload)}
      </div>
      <div className='flex items-center gap-1.5'>
        <ArrowDown className='size-3.5 text-chart-2' aria-hidden />
        <span className='sr-only'>{t('download')}</span>
        {formatBytesPerSecond(download)}
      </div>
    </div>
  );
}

function NodePingHistory({ task }: { task: NodeMonitoringPing }) {
  const t = useTranslations('nodes.monitoring');
  const format = useFormatter();
  const latest = task.samples.at(-1);
  const readout = latest
    ? latest.value < 0
      ? t('probeFailed')
      : t('milliseconds', { value: latest.value })
    : '—';
  const peak = Math.max(1, ...task.samples.map((sample) => sample.value));

  if (!latest) {
    return (
      <div className='flex items-center justify-between gap-2 rounded px-1.5 py-0.5 text-xs'>
        <span className='truncate text-muted-foreground' title={task.name}>
          {task.name}
        </span>
        <MissingMetric />
      </div>
    );
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant='ghost'
          size='sm'
          aria-label={t('probeHistory', { name: task.name, value: readout })}
          title={t('probeHistory', { name: task.name, value: readout })}
          className='grid h-5 w-full min-w-0 grid-cols-[minmax(0,1fr)_auto_2.25rem] gap-1.5 px-1.5 text-xs'
        >
          <span className='truncate text-left text-muted-foreground'>
            {task.name}
          </span>
          <span
            className={cn(
              'tabular-nums',
              latest.value < 0 && 'text-destructive',
            )}
          >
            {readout}
          </span>
          <span className='flex h-2.5 items-end gap-px' aria-hidden>
            {task.samples.map((sample, index) => (
              <span
                key={`${sample.recorded_at}-${index}`}
                className={cn(
                  'flex-1 rounded-xs',
                  sample.value < 0 ? 'bg-destructive' : 'bg-primary/65',
                )}
                style={{
                  height: `${sample.value < 0 ? 40 : 30 + (70 * sample.value) / peak}%`,
                }}
              />
            ))}
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align='end'
        aria-label={task.name}
        className='flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2'
      >
        <p className='text-sm font-medium break-all'>{task.name}</p>
        <p className='text-xs text-muted-foreground'>
          {format.dateTime(new Date(latest.recorded_at), {
            dateStyle: 'short',
            timeStyle: 'medium',
          })}
          {' · '}
          {readout}
        </p>
        {task.samples.some((sample) => sample.value >= 0) ? (
          <TrendChart
            labels={task.samples.map((sample) =>
              format.dateTime(new Date(sample.recorded_at), {
                timeStyle: 'medium',
              }),
            )}
            series={[
              {
                label: t('latency'),
                color: 'var(--primary)',
                values: task.samples.map((sample) =>
                  sample.value >= 0 ? sample.value : null,
                ),
              },
            ]}
            height={110}
            yAxisValueFormatter={(value) => t('milliseconds', { value })}
            showSummary={false}
            compactSummary
          />
        ) : null}
      </PopoverContent>
    </Popover>
  );
}

export function NodeLatency({ tasks }: { tasks?: NodeMonitoringPing[] }) {
  if (!tasks?.length) return <MissingMetric />;
  return (
    <div className='flex min-w-0 flex-col gap-0.5'>
      {tasks.map((task) => (
        <NodePingHistory key={task.task_id} task={task} />
      ))}
    </div>
  );
}
