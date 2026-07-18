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

const defaultFormatter = (value: number) => value.toLocaleString('zh-CN');

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
          'flex h-[450px] items-center justify-center rounded-md border border-dashed bg-muted/20 text-sm text-muted-foreground',
          className,
        )}
      >
        {emptyMessage}
      </div>
    );
  }

  return (
    <div className={cn('h-[450px] space-y-1.5 overflow-y-auto', className)}>
      {items.map((item) => {
        const ratio = maxValue > 0 ? (item.value / maxValue) * 100 : 0;
        return (
          <div key={item.label} className='group space-y-1'>
            <div className='flex items-center justify-between gap-3 text-[13px] leading-none'>
              <span
                className='min-w-0 truncate font-medium text-foreground'
                title={item.label}
              >
                {item.label}
              </span>
              <span className='shrink-0 tabular-nums text-foreground/80'>
                {valueFormatter(item.value)}
              </span>
            </div>
            <div className='h-1.5 overflow-hidden rounded-full bg-muted'>
              <div
                className='h-full rounded-full transition-[width] duration-300'
                style={{
                  width: `${ratio}%`,
                  backgroundColor: color,
                }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
