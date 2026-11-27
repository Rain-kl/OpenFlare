'use client';

import dynamic from 'next/dynamic';
import {
  type ComponentType,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslations } from 'next-intl';
import { Activity, Gauge, Globe2, Timer, Zap } from 'lucide-react';

import { EmptyState } from '@/components/layout/empty';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { getFormattingLocale } from '@/i18n/format';
import type { DashboardCountryLatency } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { buildCountryLatency } from './country-heat';
import { formatCompactNumber } from './dashboard-utils';
import {
  formatLatencyMs,
  getLatencyTier,
  type LatencyTier,
} from './latency-stage-map';

const LatencyStageMap = dynamic(
  () => import('./latency-stage-map').then((module) => module.LatencyStageMap),
  { ssr: false },
);

const LATENCY_LEGEND = [
  { dot: 'bg-emerald-500', key: 'legendFast' },
  { dot: 'bg-amber-500', key: 'legendModerate' },
  { dot: 'bg-orange-500', key: 'legendSlow' },
  { dot: 'bg-destructive', key: 'legendCritical' },
] as const;

const TIER_DOT_CLASS: Record<LatencyTier, string> = {
  fast: 'bg-emerald-500',
  moderate: 'bg-amber-500',
  slow: 'bg-orange-500',
  critical: 'bg-destructive',
};

function SummaryTile({
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

export function LatencyStage({
  countryLatencies,
}: {
  countryLatencies: DashboardCountryLatency[];
}) {
  const t = useTranslations('dashboard.latency');
  const tw = useTranslations('dashboard.world');
  const mapViewportRef = useRef<HTMLDivElement | null>(null);
  const [shouldRenderMap, setShouldRenderMap] = useState(false);
  const [sortOrder, setSortOrder] = useState<'slowest' | 'fastest'>('slowest');

  useEffect(() => {
    if (shouldRenderMap) {
      return;
    }

    const mapViewport = mapViewportRef.current;
    if (!mapViewport || typeof IntersectionObserver === 'undefined') {
      const id = window.setTimeout(() => setShouldRenderMap(true), 0);
      return () => window.clearTimeout(id);
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

  const countryMap = useMemo(
    () => buildCountryLatency(countryLatencies),
    [countryLatencies],
  );

  const entries = useMemo(() => [...countryMap.values()], [countryMap]);

  const sortedEntries = useMemo(() => {
    const copy = [...entries];
    copy.sort((a, b) =>
      sortOrder === 'slowest'
        ? b.avgRequestTimeMs - a.avgRequestTimeMs
        : a.avgRequestTimeMs - b.avgRequestTimeMs,
    );
    return copy;
  }, [entries, sortOrder]);

  const {
    totalRequests,
    globalAvgLatency,
    globalAvgTcpRtt,
    fastest,
    slowest,
    maxLatency,
  } = useMemo(() => {
    if (entries.length === 0) {
      return {
        totalRequests: 0,
        globalAvgLatency: 0,
        globalAvgTcpRtt: 0,
        fastest: null,
        slowest: null,
        maxLatency: 1,
      };
    }

    let reqSum = 0;
    let latencyWeighted = 0;
    let tcpWeight = 0;
    let tcpWeighted = 0;
    let minEntry = entries[0];
    let maxEntry = entries[0];

    for (const item of entries) {
      reqSum += item.requestCount;
      latencyWeighted += item.avgRequestTimeMs * item.requestCount;
      if (item.avgTcpRttMs > 0) {
        tcpWeight += item.requestCount;
        tcpWeighted += item.avgTcpRttMs * item.requestCount;
      }
      if (item.avgRequestTimeMs < minEntry.avgRequestTimeMs) {
        minEntry = item;
      }
      if (item.avgRequestTimeMs > maxEntry.avgRequestTimeMs) {
        maxEntry = item;
      }
    }

    return {
      totalRequests: reqSum,
      globalAvgLatency: reqSum > 0 ? latencyWeighted / reqSum : 0,
      globalAvgTcpRtt: tcpWeight > 0 ? tcpWeighted / tcpWeight : 0,
      fastest: minEntry,
      slowest: maxEntry,
      maxLatency: Math.max(1, maxEntry.avgRequestTimeMs),
    };
  }, [entries]);

  const headlineMetrics = [
    {
      label: t('summary.avgLatency'),
      value: entries.length > 0 ? formatLatencyMs(globalAvgLatency) : '—',
      hint:
        entries.length > 0
          ? t('summary.avgLatencyHint', {
              count: formatCompactNumber(totalRequests),
            })
          : t('summary.noRegionHint'),
      icon: Activity,
    },
    {
      label: t('summary.fastestRegion'),
      value: fastest ? formatLatencyMs(fastest.avgRequestTimeMs) : '—',
      hint: fastest
        ? `${fastest.name} · ${t('requestsCount', {
            count: formatCompactNumber(fastest.requestCount),
          })}`
        : t('summary.noRegionHint'),
      icon: Zap,
    },
    {
      label: t('summary.slowestRegion'),
      value: slowest ? formatLatencyMs(slowest.avgRequestTimeMs) : '—',
      hint: slowest
        ? `${slowest.name} · ${t('requestsCount', {
            count: formatCompactNumber(slowest.requestCount),
          })}`
        : t('summary.noRegionHint'),
      icon: Timer,
    },
    {
      label: t('summary.coveredRegions'),
      value: entries.length.toLocaleString(getFormattingLocale()),
      hint:
        globalAvgTcpRtt > 0
          ? t('summary.tcpRttHint', {
              value: formatLatencyMs(globalAvgTcpRtt),
            })
          : t('summary.windowHint'),
      icon: Globe2,
    },
  ] as const;

  return (
    <Card className='gap-3 overflow-hidden border-dashed shadow-none'>
      <CardHeader className='gap-0'>
        <div className='flex flex-wrap items-start justify-between gap-x-4 gap-y-2'>
          <div className='min-w-0 space-y-1'>
            <CardTitle className='flex items-center gap-2 text-base font-semibold tracking-tight'>
              <Gauge className='size-4 shrink-0 text-primary' />
              <span>{t('title')}</span>
            </CardTitle>
            <p className='text-xs text-muted-foreground'>{t('description')}</p>
          </div>
          <div className='flex flex-wrap items-center gap-x-3 gap-y-1 rounded-full border border-dashed bg-muted/15 px-3 py-1.5'>
            {LATENCY_LEGEND.map((item) => (
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
            <SummaryTile key={metric.label} {...metric} />
          ))}
        </div>

        <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem] lg:items-stretch'>
          <div className='flex min-w-0 flex-col gap-3 rounded-lg border border-dashed bg-muted/10 p-3'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <span className='text-sm font-medium'>{t('mapTitle')}</span>
              <span className='text-xs text-muted-foreground'>
                {t('mapHint')}
              </span>
            </div>
            <div
              ref={mapViewportRef}
              className='relative h-[340px] w-full overflow-hidden sm:h-[400px] lg:h-[440px]'
            >
              {shouldRenderMap ? (
                <LatencyStageMap countryLatency={countryMap} />
              ) : (
                <div className='flex h-full items-center justify-center px-4'>
                  <EmptyState
                    title={tw('mapPreparing')}
                    description={tw('mapPreparingDesc')}
                    iconSize='sm'
                  />
                </div>
              )}

              {shouldRenderMap && entries.length === 0 ? (
                <div className='pointer-events-none absolute inset-x-3 bottom-2 z-10'>
                  <p className='text-center text-xs text-muted-foreground'>
                    {t('noLatencyOverlay')}
                  </p>
                </div>
              ) : null}
            </div>
          </div>

          <aside
            className='flex min-w-0 flex-col rounded-lg border border-dashed bg-muted/10 p-4'
            aria-label={t('countryRanking')}
          >
            <div className='mb-3 flex items-center justify-between gap-2'>
              <h3 className='text-sm font-medium'>{t('countryRanking')}</h3>
              <ToggleGroup
                type='single'
                variant='outline'
                size='sm'
                value={sortOrder}
                onValueChange={(value) => {
                  if (value === 'slowest' || value === 'fastest') {
                    setSortOrder(value);
                  }
                }}
              >
                <ToggleGroupItem value='slowest' className='h-7 px-2 text-xs'>
                  {t('sortSlowest')}
                </ToggleGroupItem>
                <ToggleGroupItem value='fastest' className='h-7 px-2 text-xs'>
                  {t('sortFastest')}
                </ToggleGroupItem>
              </ToggleGroup>
            </div>

            {sortedEntries.length > 0 ? (
              <ol className='flex max-h-[410px] flex-1 flex-col gap-3.5 overflow-y-auto py-1 pr-1'>
                {sortedEntries.map((item) => {
                  const tier = getLatencyTier(item.avgRequestTimeMs);
                  return (
                    <li
                      key={item.name}
                      className='flex min-w-0 flex-col gap-1.5'
                    >
                      <div className='flex items-center justify-between gap-2 text-sm'>
                        <span
                          className='inline-flex min-w-0 items-center gap-1.5 truncate'
                          title={item.name}
                        >
                          <span
                            className={cn(
                              'size-1.5 shrink-0 rounded-full',
                              TIER_DOT_CLASS[tier],
                            )}
                          />
                          <span className='truncate'>{item.name}</span>
                        </span>
                        <span className='shrink-0 font-mono text-xs font-semibold tabular-nums'>
                          {formatLatencyMs(item.avgRequestTimeMs)}
                        </span>
                      </div>
                      <div className='flex items-center justify-between gap-2 text-[11px] text-muted-foreground'>
                        <span className='truncate'>
                          {t('requestsCount', {
                            count: formatCompactNumber(item.requestCount),
                          })}
                        </span>
                        {item.avgTcpRttMs > 0 ? (
                          <span className='shrink-0 font-mono tabular-nums'>
                            {t('tcpRttShort', {
                              value: formatLatencyMs(item.avgTcpRttMs),
                            })}
                          </span>
                        ) : null}
                      </div>
                      <Progress
                        value={(item.avgRequestTimeMs / maxLatency) * 100}
                        aria-label={item.name}
                        className='h-1'
                      />
                    </li>
                  );
                })}
              </ol>
            ) : (
              <EmptyState
                title={t('noData')}
                icon={Globe2}
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
