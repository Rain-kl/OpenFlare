'use client';

import { Gauge } from 'lucide-react';

import { RankCard } from '@/components/data/rank-card';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import type {
  AccessLogOverview,
  DistributionItem,
} from '@/lib/services/openflare';

import {
  formatOverviewRangeHint,
  type OverviewRangeHours,
} from '../../access-logs/components/access-log-utils';
import { OverviewToolbar } from '../../access-logs/components/overview-toolbar';
import { RatePressureChart } from './rate-pressure-chart';

function toAvgRpsItems(items: DistributionItem[] | undefined, hours: number) {
  const windowSeconds = Math.max(hours, 1) * 3600;
  return (items ?? []).map((item) => ({
    label: item.key,
    value: item.value / windowSeconds,
  }));
}

function formatRps(value: number) {
  if (!Number.isFinite(value)) return '—';
  if (value >= 100) {
    return value.toLocaleString('zh-CN', { maximumFractionDigits: 1 });
  }
  return value.toLocaleString('zh-CN', {
    maximumFractionDigits: 3,
    minimumFractionDigits: 0,
  });
}

export function AnalysisTab({
  data,
  loading,
  error,
  hours,
  hosts,
  onHoursChange,
  onHostsChange,
  onRetry,
}: {
  data?: AccessLogOverview;
  loading: boolean;
  error: Error | null;
  hours: OverviewRangeHours;
  hosts: string[];
  onHoursChange: (hours: OverviewRangeHours) => void;
  onHostsChange: (hosts: string[]) => void;
  onRetry: () => void;
}) {
  const rangeHint = formatOverviewRangeHint(hours);
  const hostItems = toAvgRpsItems(data?.top_hosts, hours);
  const ipItems = toAvgRpsItems(data?.top_ips, hours);

  return (
    <div className='space-y-6'>
      <OverviewToolbar
        hours={hours}
        hosts={hosts}
        onHoursChange={onHoursChange}
        onHostsChange={onHostsChange}
      />

      {loading ? (
        <LoadingStateWithBorder icon={Gauge} description='加载请求压力...' />
      ) : error ? (
        <ErrorInline message={error.message || '加载失败'} onRetry={onRetry} />
      ) : !data ? (
        <EmptyStateWithBorder
          icon={Gauge}
          title='暂无数据'
          description='当前时间范围内没有可用的访问日志。'
        />
      ) : (
        <>
          <RatePressureChart data={data} hours={hours} />
          <div className='grid gap-4 lg:grid-cols-2'>
            <RankCard
              title='平均 RPS 最高域名'
              description={`${rangeHint}平均请求速率`}
              items={hostItems}
              color='#38bdf8'
              valueFormatter={(value) => `${formatRps(value)} req/s`}
            />
            <RankCard
              title='平均 RPS 最高 IP'
              description={`${rangeHint}平均请求速率`}
              items={ipItems}
              color='#a78bfa'
              valueFormatter={(value) => `${formatRps(value)} req/s`}
            />
          </div>
        </>
      )}
    </div>
  );
}
