'use client';

import { useMemo } from 'react';
import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';
import { Clock } from 'lucide-react';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type { AccessLogOverview } from '@/lib/services/openflare';
import { calculateNiceAxisMax, formatCompactNumber } from '@/lib/utils/metrics';

import {
  formatOverviewTrendLabel,
  type OverviewRangeHours,
} from '../../access-logs/components/access-log-utils';

const BUCKET_SECONDS = 3600;
const RPS_COLOR = '#38bdf8';
const VISITS_COLOR = '#a78bfa';

function formatDateTime(date: Date) {
  const y = date.getFullYear();
  const m = `${date.getMonth() + 1}`.padStart(2, '0');
  const d = `${date.getDate()}`.padStart(2, '0');
  const h = `${date.getHours()}`.padStart(2, '0');
  const min = `${date.getMinutes()}`.padStart(2, '0');
  const s = `${date.getSeconds()}`.padStart(2, '0');
  return `${y}-${m}-${d} ${h}:${min}:${s}`;
}

function formatAxisTime(value: string, hours: number) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—\n—';
  }
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  const hour = `${date.getHours()}`.padStart(2, '0');
  const minute = `${date.getMinutes()}`.padStart(2, '0');
  if (hours <= 24) {
    return `${hour}:${minute}\n`;
  }
  return `${month}-${day}\n${hour}:${minute}`;
}

function formatRps(value: number) {
  if (!Number.isFinite(value)) return '—';
  if (value >= 100) return formatCompactNumber(value);
  return value.toLocaleString('zh-CN', {
    maximumFractionDigits: 2,
    minimumFractionDigits: 0,
  });
}

type RatePressureChartProps = {
  data?: AccessLogOverview;
  hours: OverviewRangeHours;
};

export function RatePressureChart({ data, hours }: RatePressureChartProps) {
  const rangeLabel = useMemo(() => {
    const end = data?.generated_at ? new Date(data.generated_at) : new Date();
    const start = new Date(end.getTime() - hours * 3600 * 1000);
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
      return '—';
    }
    return `${formatDateTime(start)} — ${formatDateTime(end)}`;
  }, [data?.generated_at, hours]);

  const chartModel = useMemo(() => {
    const requests = data?.trends.requests ?? [];
    const visits = data?.trends.visits ?? [];
    const length = Math.max(requests.length, visits.length);
    const labels: string[] = [];
    const rpsValues: number[] = [];
    const visitValues: number[] = [];
    const rawTimes: string[] = [];

    for (let i = 0; i < length; i += 1) {
      const req = requests[i];
      const visit = visits[i];
      const time = req?.bucket_started_at ?? visit?.bucket_started_at ?? '';
      rawTimes.push(time);
      labels.push(formatAxisTime(time, hours));
      rpsValues.push((req?.value ?? 0) / BUCKET_SECONDS);
      visitValues.push(visit?.value ?? 0);
    }

    return { labels, rpsValues, visitValues, rawTimes };
  }, [data?.trends.requests, data?.trends.visits, hours]);

  const option = useMemo<EChartsOption>(() => {
    const rpsMax = calculateNiceAxisMax(chartModel.rpsValues);
    const visitMax = calculateNiceAxisMax(chartModel.visitValues);

    return {
      animationDuration: 500,
      animationEasing: 'cubicOut',
      grid: {
        left: 16,
        right: 16,
        top: 48,
        bottom: 72,
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
          const items = Array.isArray(params) ? params : [];
          if (items.length === 0) return '';
          const first = items[0] as {
            dataIndex?: number;
            axisValueLabel?: string;
          };
          const index =
            typeof first.dataIndex === 'number' ? first.dataIndex : 0;
          const rawTime = chartModel.rawTimes[index] ?? '';
          const header = rawTime
            ? formatOverviewTrendLabel(rawTime, hours)
            : (first.axisValueLabel ?? '');
          const rows = items.map((item) => {
            const row = item as {
              seriesName?: string;
              color?: string;
              value?: number | string;
            };
            const numeric =
              typeof row.value === 'number'
                ? row.value
                : Number(row.value ?? 0);
            const isRps = row.seriesName === '请求速率';
            const formatted = isRps
              ? `${formatRps(numeric)} req/s`
              : formatCompactNumber(numeric);
            return [
              '<span style="display:inline-flex;align-items:center;gap:8px;">',
              `<span style="display:inline-block;width:8px;height:8px;border-radius:9999px;background:${row.color ?? '#94a3b8'};"></span>`,
              `<span>${row.seriesName ?? ''}</span>`,
              `<strong style="margin-left:8px;">${formatted}</strong>`,
              '</span>',
            ].join('');
          });
          return [header, ...rows].join('<br/>');
        },
      },
      legend: {
        show: true,
        top: 8,
        right: 80,
        itemWidth: 10,
        itemHeight: 10,
        icon: 'circle',
        textStyle: {
          color: '#94a3b8',
          fontSize: 12,
        },
        data: ['请求速率', '独立访客'],
      },
      graphic: [
        {
          type: 'text',
          left: 16,
          top: 12,
          style: {
            text: 'RPS',
            fill: '#94a3b8',
            fontSize: 12,
          },
        },
        {
          type: 'text',
          right: 16,
          top: 12,
          style: {
            text: '访客 / 桶',
            fill: '#94a3b8',
            fontSize: 12,
          },
        },
      ],
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: chartModel.labels,
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
          lineHeight: 16,
        },
      },
      yAxis: [
        {
          type: 'value',
          min: 0,
          max: rpsMax,
          splitNumber: 4,
          axisLabel: {
            color: '#94a3b8',
            formatter: (value: number) => formatRps(value),
          },
          splitLine: {
            lineStyle: {
              color: 'rgba(148, 163, 184, 0.16)',
              type: 'dashed',
            },
          },
        },
        {
          type: 'value',
          min: 0,
          max: visitMax,
          splitNumber: 4,
          axisLabel: {
            color: '#94a3b8',
            formatter: (value: number) => formatCompactNumber(value),
          },
          splitLine: {
            show: false,
          },
        },
      ],
      dataZoom: [
        {
          type: 'slider',
          height: 28,
          bottom: 8,
          borderColor: 'rgba(148, 163, 184, 0.2)',
          backgroundColor: 'rgba(148, 163, 184, 0.06)',
          fillerColor: 'rgba(56, 189, 248, 0.12)',
          handleStyle: {
            color: '#94a3b8',
          },
          textStyle: {
            color: '#94a3b8',
            fontSize: 10,
          },
          dataBackground: {
            lineStyle: {
              color: RPS_COLOR,
              width: 1,
            },
            areaStyle: {
              color: `${RPS_COLOR}33`,
            },
          },
        },
        {
          type: 'inside',
        },
      ],
      series: [
        {
          name: '请求速率',
          type: 'line',
          yAxisIndex: 0,
          smooth: true,
          showSymbol: false,
          symbol: 'circle',
          symbolSize: 8,
          lineStyle: {
            color: RPS_COLOR,
            width: 2.5,
          },
          itemStyle: {
            color: RPS_COLOR,
          },
          areaStyle: {
            color: `${RPS_COLOR}33`,
          },
          data: chartModel.rpsValues,
        },
        {
          name: '独立访客',
          type: 'line',
          yAxisIndex: 1,
          smooth: true,
          showSymbol: false,
          symbol: 'circle',
          symbolSize: 8,
          lineStyle: {
            color: VISITS_COLOR,
            width: 2,
          },
          itemStyle: {
            color: VISITS_COLOR,
          },
          areaStyle: {
            color: `${VISITS_COLOR}22`,
          },
          data: chartModel.visitValues,
        },
      ],
    };
  }, [chartModel, hours]);

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between pb-2'>
        <CardTitle className='text-base font-semibold tracking-tight'>
          请求压力
        </CardTitle>
        <div className='flex items-center gap-1.5 text-xs text-muted-foreground font-mono'>
          <Clock className='size-3.5 shrink-0' />
          <span className='break-all'>{rangeLabel}</span>
        </div>
      </CardHeader>
      <CardContent className='pt-0'>
        {chartModel.labels.length === 0 ? (
          <div className='flex h-[360px] items-center justify-center rounded-md border border-dashed bg-muted/20 text-sm text-muted-foreground'>
            暂无请求压力数据
          </div>
        ) : (
          <ReactECharts
            option={option}
            notMerge
            lazyUpdate
            style={{ height: 360, width: '100%' }}
          />
        )}
      </CardContent>
    </Card>
  );
}
