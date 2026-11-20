'use client';

import { useTranslations } from 'next-intl';

import { RankCard } from '@/components/data/rank-card';
import { Skeleton } from '@/components/ui/skeleton';
import { getFormattingLocale } from '@/i18n/format';
import type {
  AccessLogOverview,
  DistributionItem,
} from '@/lib/services/openflare';

import {
  formatOverviewRangeHint,
  type RateLimitRangeHours,
} from '../../access-logs/components/access-log-utils';

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
    return value.toLocaleString(getFormattingLocale(), {
      maximumFractionDigits: 1,
    });
  }
  return value.toLocaleString(getFormattingLocale(), {
    maximumFractionDigits: 3,
    minimumFractionDigits: 0,
  });
}

export function RateRankCards({
  data,
  loading = false,
  error = null,
  hours,
}: {
  data?: AccessLogOverview;
  loading?: boolean;
  error?: Error | null;
  hours: RateLimitRangeHours;
}) {
  const t = useTranslations('rateLimits');
  const tLogs = useTranslations('accessLogs');

  if (loading) {
    return (
      <div className='grid gap-6 lg:grid-cols-2'>
        <Skeleton className='h-64 w-full' />
        <Skeleton className='h-64 w-full' />
      </div>
    );
  }

  if (error) {
    return null;
  }

  const rangeHint = formatOverviewRangeHint(hours, (key, values) =>
    tLogs(key, values),
  );
  const hostItems = toAvgRpsItems(data?.top_hosts, hours);
  const ipItems = toAvgRpsItems(data?.top_ips, hours);

  return (
    <div className='grid gap-6 lg:grid-cols-2'>
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
  );
}
