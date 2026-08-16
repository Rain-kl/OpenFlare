'use client';

import { useQuery } from '@tanstack/react-query';
import { LayoutDashboard, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import { DashboardService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import {
  SourceDistributionChart,
  StatusCodeDistributionChart,
  TopDomainChart,
} from './components/dashboard/distribution-rank-charts';
import { NodeHealthTable } from './components/dashboard/node-health-table';
import { TrafficCapacityTrendChart } from './components/dashboard/traffic-capacity-trend-chart';
import { TrafficTrendChart } from './components/dashboard/traffic-trend-chart';
import { WorldStage } from './components/dashboard/world-stage';
import { getErrorMessage } from './nodes/components/node-utils';

const dashboardQueryKey = ['openflare', 'dashboard', 'overview'];

export default function OpenFlareDashboardPage() {
  const t = useTranslations('dashboard');
  const overviewQuery = useQuery({
    queryKey: dashboardQueryKey,
    queryFn: () => DashboardService.getOverview(),
    refetchInterval: 60_000,
  });

  const overview = overviewQuery.data;

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <LayoutDashboard className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex items-center gap-2 text-xs text-muted-foreground'>
          {overview?.generated_at ? (
            <span>
              {t('generatedAt', {
                time: formatDateTime(overview.generated_at),
              })}
            </span>
          ) : null}
          <Button
            variant='outline'
            size='sm'
            className='h-8'
            onClick={() => overviewQuery.refetch()}
            disabled={overviewQuery.isFetching}
          >
            <RefreshCw
              className={`size-3.5 mr-1.5 ${overviewQuery.isFetching ? 'animate-spin' : ''}`}
            />
            {t('refresh')}
          </Button>
        </div>
      </div>

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
            summary={overview.summary}
            traffic={overview.traffic}
            capacity={overview.capacity}
            nodes={overview.nodes}
            sourceCountries={overview.distributions.source_countries}
          />

          <div className='grid gap-6'>
            <TrafficTrendChart points={overview.trends.traffic_24h} />
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
