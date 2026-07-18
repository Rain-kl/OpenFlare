'use client';

import { useMemo } from 'react';

import { cn } from '@/lib/utils';

type RankChartItem = {
  label: string;
  value: number;
};

type RankChartProps = {
  items: RankChartItem[];
  color: string;
  valueFormatter?: (value: number) => string;
  emptyMessage?: string;
  className?: string;
};

const defaultFormatter = (value: number) => {
  if (value >= 1000000) {
    return `${Number((value / 1000000).toFixed(2))}M`;
  }
  if (value >= 1000) {
    return `${Number((value / 1000).toFixed(2))}k`;
  }
  return value.toLocaleString('zh-CN');
};

export function RankChart({
  items,
  color,
  valueFormatter = defaultFormatter,
  emptyMessage = '暂无分布数据',
  className,
}: RankChartProps) {
  const maxValue = useMemo(
    () => Math.max(0, ...items.map((item) => item.value)),
    [items],
  );

  if (items.length === 0) {
    return (
      <div
        className={cn(
          'flex h-[320px] items-center justify-center rounded-md border border-dashed bg-muted/20 text-sm text-muted-foreground',
          className,
        )}
      >
        {emptyMessage}
      </div>
    );
  }

  return (
    <div
      className={cn(
        'h-[320px] space-y-0.5 overflow-y-auto pr-1 hide-scrollbar',
        className,
      )}
    >
      {items.map((item) => {
        const ratio = maxValue > 0 ? (item.value / maxValue) * 100 : 0;
        return (
          <div
            key={item.label}
            className='group flex items-center justify-between gap-3 py-1.5 text-sm'
          >
            <span
              className='min-w-0 flex-1 truncate text-[13px] font-normal text-foreground/90'
              title={item.label}
            >
              {item.label}
            </span>
            <div className='flex items-center gap-3 shrink-0'>
              <div className='h-1.5 w-24 sm:w-28 overflow-hidden rounded-full bg-muted/60'>
                <div
                  className='h-full rounded-full transition-[width] duration-300'
                  style={{
                    width: `${ratio}%`,
                    backgroundColor: color,
                  }}
                />
              </div>
              <span className='w-14 text-right tabular-nums text-foreground/80 text-[13px] font-medium'>
                {valueFormatter(item.value)}
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
