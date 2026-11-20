'use client';

import { useTranslations } from 'next-intl';

import { useMemo } from 'react';
import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';

import { cn } from '@/lib/utils';
import { calculateNiceAxisMax, formatCompactNumber } from '@/lib/utils/metrics';

type TrendChartSeries = {
  label: string;
  color: string;
  fillColor?: string;
  values: number[];
  variant?: 'line' | 'area';
  valueFormatter?: (value: number) => string;
};

type TrendChartSummaryScope = 'last-point' | 'total' | 'average';

type TrendChartProps = {
  labels: string[];
  series: TrendChartSeries[];
  height?: number;
  yAxisValueFormatter?: (value: number) => string;
  showSummary?: boolean;
  summaryScope?: TrendChartSummaryScope;
  summaryHint?: string;
  compactSummary?: boolean;
};

type TooltipParam = {
  axisValueLabel?: string;
  color?: string;
  seriesName?: string;
  value?: number | string | Array<number | string>;
};

const defaultFormatter = (value: number) => formatCompactNumber(value);

function resolveSummaryValue(values: number[], scope: TrendChartSummaryScope) {
  if (values.length === 0) {
    return 0;
  }
  if (scope === 'total') {
    return values.reduce((sum, value) => sum + value, 0);
  }
  if (scope === 'average') {
    const sum = values.reduce((acc, value) => acc + value, 0);
    return sum / values.length;
  }
  return values[values.length - 1] ?? 0;
}

export function TrendChart({
  labels,
  series,
  height = 220,
  yAxisValueFormatter,
  showSummary = true,
  summaryScope = 'last-point',
  summaryHint,
  compactSummary = false,
}: TrendChartProps) {
  const t = useTranslations('dataComponents');
  const option = useMemo<EChartsOption>(() => {
    const axisFormatter = yAxisValueFormatter ?? defaultFormatter;
    const maxValue = calculateNiceAxisMax(
      series.flatMap((item) => item.values),
    );

    return {
      animationDuration: 500,
      animationEasing: 'cubicOut',
      grid: {
        left: 16,
        right: 16,
        top: 20,
        bottom: 20,
        containLabel: true,
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
          const items = Array.isArray(params) ? (params as TooltipParam[]) : [];
          if (items.length === 0) {
            return '';
          }

          const header = items[0]?.axisValueLabel ?? '';
          const rows = items.map((item) => {
            const matchedSeries = series.find(
              (seriesItem) => seriesItem.label === item.seriesName,
            );
            const formatter =
              matchedSeries?.valueFormatter ??
              yAxisValueFormatter ??
              defaultFormatter;
            const rawValue = Array.isArray(item.value)
              ? item.value[1]
              : item.value;
            const numericValue =
              typeof rawValue === 'number' ? rawValue : Number(rawValue ?? 0);

            return [
              '<span style="display:inline-flex;align-items:center;gap:8px;">',
              `<span style="display:inline-block;width:8px;height:8px;border-radius:9999px;background:${item.color ?? '#94a3b8'};"></span>`,
              `<span>${item.seriesName ?? ''}</span>`,
              `<strong style="margin-left:8px;">${formatter(numericValue)}</strong>`,
              '</span>',
            ].join('');
          });

          return [header, ...rows].join('<br/>');
        },
      },
      legend: {
        show: false,
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: labels,
        axisLine: {
          lineStyle: {
            color: 'rgba(148, 163, 184, 0.24)',
          },
        },
        axisTick: {
          show: false,
        },
        axisLabel: {
          color: '#94a3b8',
          margin: 14,
        },
      },
      yAxis: {
        type: 'value',
        min: 0,
        max: maxValue,
        splitNumber: 4,
        axisLabel: {
          color: '#94a3b8',
          formatter: (value: number) => axisFormatter(value),
        },
        splitLine: {
          lineStyle: {
            color: 'rgba(148, 163, 184, 0.16)',
            type: 'dashed',
          },
        },
      },
      series: series.map((item) => ({
        name: item.label,
        type: 'line',
        smooth: true,
        showSymbol: false,
        symbol: 'circle',
        symbolSize: 8,
        lineStyle: {
          color: item.color,
          width: 3,
        },
        itemStyle: {
          color: item.color,
        },
        areaStyle:
          item.variant === 'area'
            ? {
                color: item.fillColor ?? `${item.color}33`,
              }
            : undefined,
        emphasis: {
          focus: 'series',
          scale: true,
        },
        data: item.values,
      })),
    };
  }, [labels, series, yAxisValueFormatter]);

  if (labels.length === 0 || series.length === 0) {
    return (
      <div
        className={cn(
          'mt-auto flex items-center justify-center rounded-3xl border border-dashed bg-muted/30 text-sm text-muted-foreground',
          compactSummary ? 'box-content py-4' : 'h-[220px]',
        )}
        style={compactSummary ? { height } : undefined}
      >
        {t('noTrend')}
      </div>
    );
  }

  return (
    <div
      className={cn(
        'flex flex-1 flex-col gap-4',
        compactSummary && '@container',
      )}
    >
      {showSummary ? (
        <div
          className={cn(
            'gap-3',
            compactSummary
              ? 'grid grid-cols-2 @min-[26rem]:grid-cols-4'
              : 'flex flex-wrap',
          )}
        >
          {series.map((item) => {
            const summaryValue = resolveSummaryValue(item.values, summaryScope);
            const formatter = item.valueFormatter ?? defaultFormatter;
            return (
              <div
                key={item.label}
                className={cn(
                  compactSummary
                    ? 'min-w-0'
                    : 'min-w-[140px] rounded-2xl border bg-card px-4 py-3',
                )}
              >
                <div className='flex items-center gap-2'>
                  <span
                    className='size-2.5 shrink-0 rounded-full'
                    style={{ backgroundColor: item.color }}
                  />
                  <p
                    className={cn(
                      'text-xs text-muted-foreground',
                      compactSummary
                        ? 'wrap-anywhere'
                        : 'tracking-[0.18em] uppercase',
                    )}
                  >
                    {item.label}
                  </p>
                </div>
                <p className='mt-2 text-lg font-semibold'>
                  {formatter(summaryValue)}
                </p>
                {summaryHint ? (
                  <p className='mt-1 text-[10px] text-muted-foreground'>
                    {summaryHint}
                  </p>
                ) : null}
              </div>
            );
          })}
        </div>
      ) : null}

      <div
        className={cn(
          'mt-auto overflow-hidden rounded-[28px] border p-4',
          compactSummary
            ? 'border-dashed'
            : 'bg-linear-to-b from-white/3 to-transparent dark:from-white/3',
        )}
      >
        <ReactECharts
          option={option}
          notMerge
          lazyUpdate
          style={{ height, width: '100%' }}
        />
      </div>
    </div>
  );
}
