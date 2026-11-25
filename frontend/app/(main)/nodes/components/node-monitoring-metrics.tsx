'use client';

import { ArrowDown, ArrowUp } from 'lucide-react';
import dynamic from 'next/dynamic';
import { useFormatter, useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Progress } from '@/components/ui/progress';
import type { NodeItem } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { formatBytesPerSecond } from './node-utils';

const NodePingPanel = dynamic(() =>
  import('./node-ping-panel').then((module) => module.NodePingPanel),
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

const latencyColors = {
  low: { text: 'text-success', strip: 'bg-success/70' },
  elevated: { text: 'text-warning', strip: 'bg-warning/75' },
  high: { text: 'text-destructive', strip: 'bg-destructive/75' },
  missing: { text: 'text-muted-foreground', strip: 'bg-muted' },
} as const;

function getLatencyColors(value?: number) {
  if (value === undefined) return latencyColors.missing;
  if (value < 0 || value >= 250) return latencyColors.high;
  if (value >= 200) return latencyColors.elevated;
  return latencyColors.low;
}

export function NodeLatency({
  node,
  disabled,
}: {
  node: NodeItem;
  disabled?: boolean;
}) {
  const t = useTranslations('nodes');
  const tasks = node.monitoring?.ping;
  if (!tasks?.length) return <MissingMetric />;

  const readouts = tasks.map(({ task_id, samples }) => {
    const value = samples.at(-1)?.value;
    const failed = value !== undefined && value < 0;
    return {
      id: task_id,
      label: failed ? t('monitoring.probeFailed') : undefined,
      text:
        value === undefined
          ? '—'
          : failed
            ? '×'
            : t('monitoring.milliseconds', { value }),
      color: getLatencyColors(value).text,
    };
  });
  const title = t('monitoring.probeHistory', { name: node.name });

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          disabled={disabled}
          className='h-auto w-full min-w-0 flex-col items-stretch gap-1 px-1 py-1.5 text-left text-[11px] whitespace-normal'
        >
          <span className='sr-only'>{title}</span>
          <span className='flex flex-wrap items-center gap-x-0.5 tabular-nums'>
            {readouts.map(({ id, label, text, color }, index) => (
              <span
                key={id}
                className='flex items-center gap-0.5 whitespace-nowrap'
              >
                {index > 0 && (
                  <span className='text-muted-foreground' aria-hidden>
                    ·
                  </span>
                )}
                <span className={color} aria-hidden={label ? true : undefined}>
                  {text}
                </span>
                {label && <span className='sr-only'>{label}</span>}
              </span>
            ))}
          </span>
          <span className='flex flex-col gap-px' aria-hidden>
            {tasks.map(({ task_id, samples }) => (
              <span key={task_id} className='flex h-1 gap-px'>
                {samples.length ? (
                  samples.map((sample, index) => (
                    <span
                      key={`${sample.recorded_at}-${index}`}
                      className={cn(
                        'flex-1 rounded-xs',
                        getLatencyColors(sample.value).strip,
                      )}
                    />
                  ))
                ) : (
                  <span className='flex-1 rounded-xs bg-muted' />
                )}
              </span>
            ))}
          </span>
        </Button>
      </DialogTrigger>
      <DialogContent className='min-w-0 grid-cols-[minmax(0,1fr)] p-4 sm:w-[min(72rem,calc(100%-2rem))] sm:max-w-none sm:p-6 md:max-w-none'>
        <DialogHeader className='pr-7'>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className='sr-only'>
            {t('ping.sectionDesc')}
          </DialogDescription>
        </DialogHeader>
        <NodePingPanel nodeId={node.id} showIntro={false} />
      </DialogContent>
    </Dialog>
  );
}
