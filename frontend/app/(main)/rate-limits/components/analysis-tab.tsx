'use client';

import { Gauge } from 'lucide-react';
import { useTranslations } from 'next-intl';

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
  RATE_LIMIT_RANGE_OPTIONS,
  type RateLimitRangeHours,
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
  hours: RateLimitRangeHours;
  hosts: string[];
  onHoursChange: (hours: RateLimitRangeHours) => void;
  onHostsChange: (hosts: string[]) => void;
  onRetry: () => void;
}) {
  const t = useTranslations('rateLimits');
  const tLogs = useTranslations('accessLogs');
  const rangeHint = formatOverviewRangeHint(hours, (key, values) =>
    tLogs(key, values),
  );
  const rangeOptions = RATE_LIMIT_RANGE_OPTIONS.map((option) => ({
    value: option.value,
    label: t(option.labelKey),
  }));
  const hostItems = toAvgRpsItems(data?.top_hosts, hours);
  const ipItems = toAvgRpsItems(data?.top_ips, hours);

  return (
    <div className='space-y-6'>
      <OverviewToolbar
        hours={hours}
        hosts={hosts}
        onHoursChange={(next) => onHoursChange(next as RateLimitRangeHours)}
        onHostsChange={onHostsChange}
        rangeOptions={rangeOptions}
      />

      {loading ? (
        <LoadingStateWithBorder
          icon={Gauge}
          description={t('loadingPressure')}
        />
      ) : error ? (
        <ErrorInline
          message={error.message || t('loadFailed')}
          onRetry={onRetry}
        />
      ) : !data ? (
        <EmptyStateWithBorder
          icon={Gauge}
          title={t('emptyTitle')}
          description={t('emptyDesc')}
        />
      ) : (
        <>
          <RatePressureChart data={data} hours={hours} />
          <div className='grid gap-4 lg:grid-cols-2'>
            <RankCard
              title={t('topHost')}
              description={t('avgRate', { range: rangeHint })}
              items={hostItems}
              color='#38bdf8'
              valueFormatter={(value) => `${formatRps(value)} req/s`}
            />
            <RankCard
              title={t('topIp')}
              description={t('avgRate', { range: rangeHint })}
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
