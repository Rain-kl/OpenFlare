'use client';

import dynamic from 'next/dynamic';
import { type ComponentType, useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import {
  Activity,
  CircleAlert,
  Globe2,
  LayoutDashboard,
  Server,
  ServerCrash,
  Users,
} from 'lucide-react';

import { getFormattingLocale } from '@/i18n/format';
import { EmptyState } from '@/components/layout/empty';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import type {
  DashboardNodeHealth,
  DashboardTraffic,
  DistributionItem,
} from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { formatCompactNumber } from './dashboard-utils';

const WorldStageMap = dynamic(
  () => import('./world-stage-map').then((module) => module.WorldStageMap),
  { ssr: false },
);

const LEGEND_ITEMS = [
  { dot: 'bg-primary', key: 'legendSource' },
  { dot: 'bg-emerald-500', key: 'legendHealthy' },
  { dot: 'bg-amber-500', key: 'legendPressure' },
  { dot: 'bg-destructive', key: 'legendUnhealthy' },
] as const;

function SummaryMetric({
  label,
  value,
  hint,
  icon: Icon,
  className,
}: {
  label: string;
  value: string;
  hint: string;
  icon: ComponentType<{ className?: string }>;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'flex min-w-0 flex-col gap-2 rounded-lg border border-dashed bg-muted/15 p-3',
        className,
      )}
    >
      <div className='flex items-center justify-between gap-2'>
        <span className='truncate text-xs text-muted-foreground'>{label}</span>
        <Icon className='size-3.5 shrink-0 text-primary/60' />
      </div>
      <span className='text-xl font-semibold tabular-nums leading-none tracking-tight'>
        {value}
      </span>
      <span className='truncate text-[10px] text-muted-foreground'>{hint}</span>
    </div>
  );
}

export function WorldStage({
  traffic,
  nodes,
  sourceCountries,
}: {
  traffic: DashboardTraffic;
  nodes: DashboardNodeHealth[];
  sourceCountries: DistributionItem[];
}) {
  const td = useTranslations('dashboard');
  const t = useTranslations('dashboard.world');
  const tm = useTranslations('dashboard.map');
  const mapViewportRef = useRef<HTMLDivElement | null>(null);
  const [shouldRenderMap, setShouldRenderMap] = useState(false);
  const [rankingMode, setRankingMode] = useState<'countries' | 'nodes'>(
    'countries',
  );

  useEffect(() => {
    if (shouldRenderMap) {
      return;
    }

    const mapViewport = mapViewportRef.current;
    if (!mapViewport || typeof IntersectionObserver === 'undefined') {
      setShouldRenderMap(true);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setShouldRenderMap(true);
          observer.disconnect();
        }
      },
      { rootMargin: '120px 0px' },
    );

    observer.observe(mapViewport);

    return () => {
      observer.disconnect();
    };
  }, [shouldRenderMap]);

  const status4xxCount =
    traffic.status_4xx_count ?? (traffic.request_count === 0 ? 0 : undefined);
  const requestRate = (count: number | undefined) => {
    if (count === undefined) return '—';
    const rate = traffic.request_count > 0 ? count / traffic.request_count : 0;
    return rate.toLocaleString(getFormattingLocale(), {
      style: 'percent',
      maximumFractionDigits: 2,
    });
  };
  const headlineMetrics = [
    {
      label: t('requests24h'),
      value: formatCompactNumber(traffic.request_count),
      hint: t('averageQps', {
        qps: traffic.estimated_qps.toLocaleString(getFormattingLocale(), {
          maximumFractionDigits: 2,
        }),
      }),
      icon: Activity,
    },
    {
      label: t('uniqueIps'),
      value: formatCompactNumber(traffic.unique_visitors),
      hint: t('uniqueIpsHint'),
      icon: Users,
    },
    {
      label: t('requests4xx'),
      value: formatCompactNumber(status4xxCount),
      hint: t('requestShare', { rate: requestRate(status4xxCount) }),
      icon: CircleAlert,
    },
    {
      label: t('requests5xx'),
      value: formatCompactNumber(traffic.error_count),
      hint: t('requestShare', { rate: requestRate(traffic.error_count) }),
      icon: ServerCrash,
    },
  ] as const;

  const countries = [...sourceCountries]
    .filter((item) => item.key && item.value > 0)
    .sort((a, b) => b.value - a.value)
    .slice(0, 7);
  const maximum = Math.max(1, ...countries.map((item) => item.value));
  const rankedNodes = [...nodes]
    .sort((a, b) => {
      if (b.request_count !== a.request_count) {
        return b.request_count - a.request_count;
      }
      return b.cpu_usage_percent - a.cpu_usage_percent;
    })
    .slice(0, 7);
  const maxNodeRequests = Math.max(
    1,
    ...rankedNodes.map((node) => node.request_count),
  );
  const locale = getFormattingLocale();

  return (
    <Card className='gap-3 overflow-hidden border-dashed shadow-none'>
      <CardHeader className='gap-0'>
        <div className='flex flex-wrap items-center justify-between gap-x-4 gap-y-2'>
          <CardTitle className='flex items-center gap-2'>
            <LayoutDashboard className='size-5 shrink-0 text-primary' />
            <h1 className='text-2xl font-semibold tracking-tight'>
              {td('title')}
            </h1>
          </CardTitle>
          <div className='flex flex-wrap items-center gap-x-3 gap-y-1 rounded-full border border-dashed bg-muted/15 px-3 py-1.5'>
            {LEGEND_ITEMS.map((item) => (
              <span
                key={item.key}
                className='inline-flex items-center gap-1 text-[10px] text-muted-foreground'
              >
                <span className={cn('size-1.5 rounded-full', item.dot)} />
                {t(item.key)}
              </span>
            ))}
          </div>
        </div>
      </CardHeader>

      <CardContent className='flex flex-col gap-4 pt-0'>
        <div className='grid grid-cols-2 gap-3 xl:grid-cols-4'>
          {headlineMetrics.map((metric) => (
            <SummaryMetric key={metric.label} {...metric} />
          ))}
        </div>

        <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem] lg:items-stretch'>
          <div className='min-w-0 rounded-lg border border-dashed bg-muted/10 p-3'>
            <div
              ref={mapViewportRef}
              className='relative h-[360px] w-full overflow-hidden sm:h-[440px] lg:h-[480px]'
            >
              {shouldRenderMap ? (
                <WorldStageMap
                  nodes={nodes}
                  sourceCountries={sourceCountries}
                />
              ) : (
                <div className='flex h-full items-center justify-center px-4'>
                  <EmptyState
                    title={t('mapPreparing')}
                    description={t('mapPreparingDesc')}
                    iconSize='sm'
                  />
                </div>
              )}

              {shouldRenderMap && nodes.length === 0 ? (
                <div className='pointer-events-none absolute inset-x-3 bottom-2 z-10'>
                  <p className='text-center text-xs text-muted-foreground'>
                    {t('noNodesOverlay')}
                  </p>
                </div>
              ) : null}
            </div>
          </div>

          <aside
            className='flex min-w-0 flex-col rounded-lg border border-dashed bg-muted/10 p-4'
            aria-label={
              rankingMode === 'countries'
                ? tm('sourceRanking')
                : tm('nodeLoadRanking')
            }
          >
            <div className='flex items-center justify-between gap-2'>
              <h3 className='truncate text-sm font-medium'>
                {rankingMode === 'countries'
                  ? tm('sourceRanking')
                  : tm('nodeLoadRanking')}
              </h3>
              <ToggleGroup
                type='single'
                variant='outline'
                size='sm'
                value={rankingMode}
                onValueChange={(value) => {
                  if (value === 'countries' || value === 'nodes') {
                    setRankingMode(value);
                  }
                }}
              >
                <ToggleGroupItem value='countries' className='h-7 px-2 text-xs'>
                  {tm('rankingCountriesTab')}
                </ToggleGroupItem>
                <ToggleGroupItem value='nodes' className='h-7 px-2 text-xs'>
                  {tm('rankingNodesTab')}
                </ToggleGroupItem>
              </ToggleGroup>
            </div>

            {rankingMode === 'countries' ? (
              countries.length ? (
                <ol className='flex flex-1 flex-col justify-center gap-5 py-6'>
                  {countries.map((country) => (
                    <li
                      key={country.key}
                      className='flex min-w-0 flex-col gap-2'
                    >
                      <div className='flex items-center justify-between gap-3 text-sm'>
                        <span className='min-w-0 truncate' title={country.key}>
                          {country.key}
                        </span>
                        <span className='shrink-0 font-semibold tabular-nums'>
                          {country.value.toLocaleString(locale)}
                        </span>
                      </div>
                      <Progress
                        value={(country.value / maximum) * 100}
                        aria-label={country.key}
                        className='h-1'
                      />
                    </li>
                  ))}
                </ol>
              ) : (
                <EmptyState
                  title={tm('noSources')}
                  icon={Globe2}
                  iconSize='sm'
                  className='min-h-48 flex-1 px-2'
                />
              )
            ) : rankedNodes.length ? (
              <ol className='flex flex-1 flex-col justify-center gap-4 py-4'>
                {rankedNodes.map((node) => {
                  const displayName = node.name || node.node_id;
                  return (
                    <li
                      key={node.node_id}
                      className='flex min-w-0 flex-col gap-1.5'
                    >
                      <div className='flex items-center justify-between gap-2 text-sm'>
                        <span className='min-w-0 truncate' title={displayName}>
                          {displayName}
                        </span>
                        <span className='shrink-0 font-semibold tabular-nums'>
                          {tm('nodeRequestsCount', {
                            count: node.request_count.toLocaleString(locale),
                          })}
                        </span>
                      </div>
                      <div className='flex items-center justify-between gap-2 text-[11px] text-muted-foreground'>
                        <span className='truncate'>
                          {tm('nodeLoadMeta', {
                            cpu: Math.round(node.cpu_usage_percent),
                            memory: Math.round(node.memory_usage_percent),
                          })}
                        </span>
                      </div>
                      <Progress
                        value={(node.request_count / maxNodeRequests) * 100}
                        aria-label={displayName}
                        className='h-1'
                      />
                    </li>
                  );
                })}
              </ol>
            ) : (
              <EmptyState
                title={tm('noNodes')}
                icon={Server}
                iconSize='sm'
                className='min-h-48 flex-1 px-2'
              />
            )}
          </aside>
        </div>
      </CardContent>
    </Card>
  );
}
