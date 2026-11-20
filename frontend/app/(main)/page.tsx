'use client';

import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { AccessLogService, DashboardService } from '@/lib/services/openflare';

import {
  RATE_LIMIT_RANGE_OPTIONS,
  type RateLimitRangeHours,
} from './access-logs/components/access-log-utils';
import { OverviewToolbar } from './access-logs/components/overview-toolbar';
import {
  SourceDistributionChart,
  StatusCodeDistributionChart,
  TopDomainChart,
} from './components/dashboard/distribution-rank-charts';
import { NodeHealthTable } from './components/dashboard/node-health-table';
import { RatePressureChart } from './components/dashboard/rate-pressure-chart';
import { RateRankCards } from './components/dashboard/rate-rank-cards';
import { TrafficCapacityTrendChart } from './components/dashboard/traffic-capacity-trend-chart';
import { TrafficTrendChart } from './components/dashboard/traffic-trend-chart';
import { WorldStage } from './components/dashboard/world-stage';
import { getErrorMessage } from './nodes/components/node-utils';

const dashboardQueryKey = ['openflare', 'dashboard', 'overview'];

export default function OpenFlareDashboardPage() {
  const t = useTranslations('dashboard');
  const tRate = useTranslations('rateLimits');
  const [hours, setHours] = useState<RateLimitRangeHours>(24);
  const [hosts, setHosts] = useState<string[]>([]);

  const overviewQuery = useQuery({
    queryKey: dashboardQueryKey,
    queryFn: () => DashboardService.getOverview(),
    refetchInterval: 60_000,
  });

  const pressureQuery = useQuery({
    queryKey: ['openflare', 'dashboard', 'rate-pressure', hours, hosts, 3],
    queryFn: () =>
      AccessLogService.getOverview({
        hours,
        hosts: hosts.length > 0 ? hosts : undefined,
        bucket_minutes: 3,
      }),
    refetchInterval: 60_000,
  });

  const overview = overviewQuery.data;
  const rangeOptions = RATE_LIMIT_RANGE_OPTIONS.map((option) => ({
    value: option.value,
    label: tRate(option.labelKey),
  }));

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      {overviewQuery.isLoading ? (
        <LoadingStateWithBorder
          title={t('loadingTitle')}
          description={t('loadingDesc')}
        />
      ) : overviewQuery.isError ? (
        <ErrorInline
          message={t('loadFailed', {
            error: getErrorMessage(overviewQuery.error, t('requestFailed')),
          })}
          onRetry={() => overviewQuery.refetch()}
        />
      ) : !overview ? (
        <EmptyStateWithBorder
          title={t('emptyTitle')}
          description={t('emptyDesc')}
        />
      ) : (
        <>
          <WorldStage
            traffic={overview.traffic}
            nodes={overview.nodes}
            sourceCountries={overview.distributions.source_countries}
          />

          <div className='grid gap-6'>
            <TrafficTrendChart points={overview.trends.traffic_24h} />
          </div>

          <div className='grid gap-6'>
            <RatePressureChart
              data={pressureQuery.data}
              hours={hours}
              loading={pressureQuery.isLoading}
              error={
                pressureQuery.error instanceof Error
                  ? pressureQuery.error
                  : null
              }
              onRetry={() => void pressureQuery.refetch()}
              toolbar={
                <OverviewToolbar
                  hours={hours}
                  hosts={hosts}
                  onHoursChange={(next) =>
                    setHours(next as RateLimitRangeHours)
                  }
                  onHostsChange={setHosts}
                  rangeOptions={rangeOptions}
                />
              }
            />
          </div>

          <div className='grid gap-6 xl:grid-cols-3'>
            <SourceDistributionChart
              items={overview.distributions.source_countries}
            />
            <StatusCodeDistributionChart
              items={overview.distributions.status_codes}
            />
            <TopDomainChart items={overview.distributions.top_domains} />
          </div>

          <RateRankCards
            data={pressureQuery.data}
            loading={pressureQuery.isLoading}
            error={
              pressureQuery.error instanceof Error ? pressureQuery.error : null
            }
            hours={hours}
          />

          <div className='grid gap-6'>
            <TrafficCapacityTrendChart
              networkPoints={overview.trends.network_24h}
              capacityPoints={overview.trends.capacity_24h}
            />
          </div>

          <NodeHealthTable nodes={overview.nodes} />
        </>
      )}
    </div>
  );
}
