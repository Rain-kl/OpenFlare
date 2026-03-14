'use client';

import { useMemo } from 'react';
import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';

type TrendChartSeries = {
  label: string;
  color: string;
  fillColor?: string;
  values: number[];
  variant?: 'line' | 'area';
  valueFormatter?: (value: number) => string;
};

type TrendChartProps = {
  labels: string[];
  series: TrendChartSeries[];
  height?: number;
};

const defaultFormatter = (value: number) => value.toLocaleString('zh-CN');

export function TrendChart({ labels, series, height = 220 }: TrendChartProps) {
  const option = useMemo<EChartsOption>(() => {
    const maxValue =
      Math.max(
        1,
        ...series.flatMap((item) => item.values.map((value) => value || 0)),
      ) * 1.1;

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
  }, [labels, series]);

  if (labels.length === 0 || series.length === 0) {
    return (
      <div className="flex h-[220px] items-center justify-center rounded-3xl border border-dashed border-[var(--border-default)] bg-[var(--surface-muted)] text-sm text-[var(--foreground-secondary)]">
        暂无趋势数据
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-3">
        {series.map((item) => {
          const latestValue = item.values[item.values.length - 1] ?? 0;
          const formatter = item.valueFormatter ?? defaultFormatter;
          return (
            <div
              key={item.label}
              className="min-w-[140px] rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-3"
            >
              <div className="flex items-center gap-2">
                <span
                  className="h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: item.color }}
                />
                <p className="text-xs tracking-[0.18em] text-[var(--foreground-muted)] uppercase">
                  {item.label}
                </p>
              </div>
              <p className="mt-2 text-lg font-semibold text-[var(--foreground-primary)]">
                {formatter(latestValue)}
              </p>
            </div>
          );
        })}
      </div>

      <div className="overflow-hidden rounded-[28px] border border-[var(--border-default)] bg-[linear-gradient(180deg,rgba(255,255,255,0.03),rgba(255,255,255,0))] px-4 py-4">
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
