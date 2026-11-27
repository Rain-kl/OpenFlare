'use client';

import { useQuery } from '@tanstack/react-query';
import { Activity, Globe2, RefreshCw, Server, ShieldCheck } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useMemo, useState } from 'react';

import { useChartThemePalette } from '@/components/data/chart-theme';
import { TrendChart } from '@/components/data/trend-chart';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSOverviewWindow,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { getErrorMessage } from '../../websites/components/website-utils';

const WINDOWS: SmartDNSOverviewWindow[] = ['1h', '6h', '24h', '7d'];

export function SmartDNSOverviewTab({
  onOpenInstances,
}: {
  onOpenInstances: () => void;
}) {
  const t = useTranslations('smartdns');
  const palette = useChartThemePalette();
  const [windowRange, setWindowRange] = useState<SmartDNSOverviewWindow>('24h');

  const overviewQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'overview', windowRange],
    queryFn: () => SmartDNSService.getOverview(windowRange),
    refetchInterval: 15000,
  });

  const overview = overviewQuery.data;
  const chartData = useMemo(() => {
    const series = overview?.series ?? [];
    const labels = series.map((pt) => formatDateTime(pt.timestamp));
    return {
      labels,
      qps: series.map((pt) => Number(pt.qps.toFixed(2))),
      p50: series.map((pt) => Number(pt.latency_p50_ms.toFixed(2))),
      p95: series.map((pt) => Number(pt.latency_p95_ms.toFixed(2))),
      p99: series.map((pt) => Number(pt.latency_p99_ms.toFixed(2))),
    };
  }, [overview?.series]);

  return (
    <div className='flex flex-col gap-5'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        {overview && (
          <div className='flex flex-wrap items-center gap-2'>
            <Badge variant={overview.enabled ? 'default' : 'secondary'}>
              {overview.enabled ? t('serviceEnabled') : t('serviceDisabled')}
            </Badge>
            {overview.delegation_domain ? (
              <Badge variant='outline' className='font-mono'>
                {overview.delegation_domain}
              </Badge>
            ) : null}
            <Badge variant='outline' className='font-mono'>
              {t('snapshotVersion', { version: overview.snapshot_version })}
            </Badge>
          </div>
        )}
        <div className='ml-auto flex flex-wrap items-center gap-2'>
          <ToggleGroup
            aria-label={t('monitoringWindow')}
            type='single'
            variant='outline'
            size='sm'
            value={windowRange}
            onValueChange={(v) => {
              if (v) setWindowRange(v as SmartDNSOverviewWindow);
            }}
          >
            {WINDOWS.map((w) => (
              <ToggleGroupItem key={w} value={w}>
                {w}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void overviewQuery.refetch()}
            disabled={overviewQuery.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={overviewQuery.isFetching ? 'animate-spin' : ''}
            />
            {t('refresh')}
          </Button>
        </div>
      </div>

      {overviewQuery.isLoading ? (
        <LoadingStateWithBorder
          icon={Globe2}
          description={t('loadingOverview')}
        />
      ) : overviewQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(overviewQuery.error)}
          onRetry={() => void overviewQuery.refetch()}
        />
      ) : overview ? (
        <>
          <dl className='grid grid-cols-2 gap-x-4 gap-y-5 rounded-lg border border-dashed p-4 xl:grid-cols-4'>
            {[
              {
                label: t('kpiInstances'),
                value: `${overview.online_instances} / ${overview.total_instances}`,
                icon: Server,
              },
              {
                label: t('kpiCoverage'),
                value: overview.managed_domains,
                detail: t('kpiPoliciesCount', {
                  count: overview.active_policies,
                }),
                icon: Globe2,
              },
              {
                label: t('kpiQps'),
                value:
                  overview.online_instances > 0
                    ? overview.total_qps.toFixed(1)
                    : '—',
                detail:
                  overview.online_instances > 0
                    ? `P50 ${overview.avg_p50_ms.toFixed(2)} ms · P95 ${overview.avg_p95_ms.toFixed(2)} ms · P99 ${overview.avg_p99_ms.toFixed(2)} ms`
                    : undefined,
                icon: Activity,
              },
              {
                label: t('kpiCacheHit'),
                value:
                  overview.online_instances > 0
                    ? `${(overview.avg_cache_hit_ratio * 100).toFixed(1)}%`
                    : '—',
                icon: ShieldCheck,
              },
            ].map(({ label, value, detail, icon: Icon }) => (
              <div key={label} className='flex min-w-0 flex-col gap-1'>
                <dt className='flex items-center gap-2 text-sm text-muted-foreground'>
                  <Icon className='size-4 shrink-0 text-primary' />
                  {label}
                </dt>
                <dd className='text-xl font-semibold tabular-nums'>{value}</dd>
                {detail && (
                  <dd className='text-xs text-muted-foreground tabular-nums'>
                    {detail}
                  </dd>
                )}
              </div>
            ))}
          </dl>
          {chartData.labels.length === 0 ? (
            <div className='flex flex-col gap-2 rounded-lg border border-dashed p-5'>
              <h2 className='text-base font-semibold'>{t('trendsTitle')}</h2>
              <p className='text-sm text-muted-foreground'>
                {t('chartNoSeries')}
              </p>
              {overview.total_instances === 0 && (
                <Button
                  variant='link'
                  className='h-auto self-start p-0'
                  onClick={onOpenInstances}
                >
                  {t('openInstances')}
                </Button>
              )}
            </div>
          ) : (
            <div className='grid gap-4 lg:grid-cols-2'>
              <Card>
                <CardHeader>
                  <CardTitle className='text-base'>
                    {t('chartQpsTitle')}
                  </CardTitle>
                  <CardDescription>{t('chartQpsDesc')}</CardDescription>
                </CardHeader>
                <CardContent>
                  <TrendChart
                    labels={chartData.labels}
                    summaryScope='average'
                    yAxisValueFormatter={(v) => `${v.toFixed(1)} qps`}
                    series={[
                      {
                        label: 'QPS',
                        color: palette.primary,
                        values: chartData.qps,
                        variant: 'area',
                      },
                    ]}
                  />
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className='text-base'>
                    {t('chartLatencyTitle')}
                  </CardTitle>
                  <CardDescription>{t('chartLatencyDesc')}</CardDescription>
                </CardHeader>
                <CardContent>
                  <TrendChart
                    labels={chartData.labels}
                    summaryScope='average'
                    yAxisValueFormatter={(v) => `${v.toFixed(1)} ms`}
                    series={[
                      {
                        label: 'P50',
                        color: palette.series[1],
                        values: chartData.p50,
                        variant: 'line',
                      },
                      {
                        label: 'P95',
                        color: palette.series[2],
                        values: chartData.p95,
                        variant: 'line',
                      },
                      {
                        label: 'P99',
                        color: palette.destructive,
                        values: chartData.p99,
                        variant: 'line',
                      },
                    ]}
                  />
                </CardContent>
              </Card>
            </div>
          )}
        </>
      ) : null}
    </div>
  );
}
