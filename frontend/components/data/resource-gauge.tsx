'use client';

import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';

import { cn } from '@/lib/utils';

export type ResourceGaugeTone =
  | 'primary'
  | 'chart-1'
  | 'chart-2'
  | 'chart-3'
  | 'chart-4'
  | 'chart-5'
  | 'destructive';

const TONE_COLOR_VAR: Record<ResourceGaugeTone, string> = {
  primary: 'var(--primary)',
  'chart-1': 'var(--chart-1)',
  'chart-2': 'var(--chart-2)',
  'chart-3': 'var(--chart-3)',
  'chart-4': 'var(--chart-4)',
  'chart-5': 'var(--chart-5)',
  destructive: 'var(--destructive)',
};

export interface ResourceGaugeProps {
  label: string;
  /** Percentage value in 0..100; null/undefined renders inactive state */
  value: number | null | undefined;
  /** Primary formatted readout inside the dial (defaults to percentage) */
  displayValue?: string;
  /** Secondary subtitle shown beneath the gauge (e.g. "2.4 GB / 8.0 GB") */
  subtitle?: string;
  /** Optional extra footer hint */
  detail?: ReactNode;
  icon?: LucideIcon;
  tone?: ResourceGaugeTone;
  warnThreshold?: number;
  dangerThreshold?: number;
  className?: string;
}

const SVG_SIZE = 132;
const CENTER = SVG_SIZE / 2;
const RADIUS = 50;
const STROKE_WIDTH = 9;
const START_ANGLE = 135;
const SWEEP_ANGLE = 270;

function polarToCartesian(cx: number, cy: number, r: number, angleDeg: number) {
  const rad = ((angleDeg - 90) * Math.PI) / 180;
  return {
    x: cx + r * Math.cos(rad),
    y: cy + r * Math.sin(rad),
  };
}

function describeArc(
  cx: number,
  cy: number,
  r: number,
  startAngle: number,
  endAngle: number,
) {
  const start = polarToCartesian(cx, cy, r, endAngle);
  const end = polarToCartesian(cx, cy, r, startAngle);
  const largeArcFlag = endAngle - startAngle <= 180 ? '0' : '1';
  return [
    'M',
    start.x.toFixed(2),
    start.y.toFixed(2),
    'A',
    r,
    r,
    0,
    largeArcFlag,
    0,
    end.x.toFixed(2),
    end.y.toFixed(2),
  ].join(' ');
}

/**
 * Reusable flat dashed dashboard radial gauge styled with OpenFlare CSS theme variables.
 */
export function ResourceGauge({
  label,
  value,
  displayValue,
  subtitle,
  detail,
  icon: Icon,
  tone = 'primary',
  warnThreshold = 75,
  dangerThreshold = 90,
  className,
}: ResourceGaugeProps) {
  const hasValue =
    value !== null && value !== undefined && Number.isFinite(value);
  const clamped = hasValue ? Math.max(0, Math.min(100, value)) : 0;

  let resolvedTone: ResourceGaugeTone = tone;
  if (hasValue && clamped >= dangerThreshold) {
    resolvedTone = 'destructive';
  } else if (hasValue && clamped >= warnThreshold) {
    resolvedTone = 'chart-4';
  }

  const activeStroke = TONE_COLOR_VAR[resolvedTone];
  const trackPath = describeArc(
    CENTER,
    CENTER,
    RADIUS,
    START_ANGLE,
    START_ANGLE + SWEEP_ANGLE,
  );
  const progressSweep = Math.max(0.5, (clamped / 100) * SWEEP_ANGLE);
  const progressPath = describeArc(
    CENTER,
    CENTER,
    RADIUS,
    START_ANGLE,
    START_ANGLE + progressSweep,
  );

  const centerText =
    displayValue ?? (hasValue ? `${clamped.toFixed(1)}%` : '—');

  return (
    <div
      role='meter'
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={hasValue ? Math.round(clamped) : undefined}
      className={cn(
        'flex flex-col items-center justify-between rounded-xl border border-dashed bg-card p-4 transition-colors',
        className,
      )}
    >
      <div className='flex w-full items-center justify-between gap-2 text-xs font-medium text-muted-foreground'>
        <span className='truncate'>{label}</span>
        {Icon ? <Icon className='size-3.5 shrink-0 text-primary' /> : null}
      </div>

      <div className='relative my-1 flex items-center justify-center'>
        <svg
          width={SVG_SIZE}
          height={SVG_SIZE - 16}
          viewBox={`0 0 ${SVG_SIZE} ${SVG_SIZE - 10}`}
          className='overflow-visible'
          aria-hidden='true'
        >
          {/* Subtle inner dashed scale ring */}
          <circle
            cx={CENTER}
            cy={CENTER}
            r={RADIUS - 11}
            fill='none'
            stroke='var(--border)'
            strokeWidth={1}
            strokeDasharray='3 4'
          />
          {/* Background arc track */}
          <path
            d={trackPath}
            fill='none'
            stroke='var(--muted)'
            strokeWidth={STROKE_WIDTH}
            strokeLinecap='round'
          />
          {/* Active arc fill */}
          {hasValue && clamped > 0 ? (
            <path
              d={progressPath}
              fill='none'
              stroke={activeStroke}
              strokeWidth={STROKE_WIDTH}
              strokeLinecap='round'
              className='transition-all duration-300 ease-out'
            />
          ) : null}
        </svg>

        <div className='pointer-events-none absolute inset-0 flex flex-col items-center justify-center pt-2 text-center'>
          <span className='text-xl font-semibold tracking-tight tabular-nums text-foreground'>
            {centerText}
          </span>
        </div>
      </div>

      <div className='w-full space-y-0.5 text-center'>
        {subtitle ? (
          <p className='truncate text-xs font-medium tabular-nums text-foreground/85'>
            {subtitle}
          </p>
        ) : null}
        {detail ? (
          <div className='truncate text-[11px] text-muted-foreground tabular-nums'>
            {detail}
          </div>
        ) : null}
      </div>
    </div>
  );
}
