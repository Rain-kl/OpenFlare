'use client';

import dynamic from 'next/dynamic';
import { type ComponentType, useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import {
  Activity,
  Cpu,
  Globe2,
  HardDrive,
  MemoryStick,
  Network,
  Server,
  ShieldCheck,
} from 'lucide-react';

import { EmptyState } from '@/components/layout/empty';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import type {
  DashboardCapacity,
  DashboardNodeHealth,
  DashboardSummary,
  DashboardTraffic,
  DistributionItem,
} from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { formatCompactNumber, formatPercent } from './dashboard-utils';

const WorldStageMap = dynamic(
  () => import('./world-stage-map').then((module) => module.WorldStageMap),
  { ssr: false },
);

const LEGEND_ITEMS = [
  { dot: 'bg-blue-500', key: 'legendSource' },
  { dot: 'bg-emerald-500', key: 'legendHealthy' },
  { dot: 'bg-amber-500', key: 'legendPressure' },
  { dot: 'bg-destructive', key: 'legendUnhealthy' },
] as const;

function SummaryMetric({
  label,
  value,
  hint,
  icon: Icon,
  progress,
  className,
}: {
  label: string;
  value: string;
  hint: string;
  icon: ComponentType<{ className?: string }>;
  progress?: number;
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
      {typeof progress === 'number' ? (
        <Progress value={progress} className='h-1' />
      ) : null}
      <span className='truncate text-[10px] text-muted-foreground'>{hint}</span>
    </div>
  );
}

function DetailMetric({
  label,
  value,
  hint,
  icon: Icon,
  progress,
}: {
  label: string;
  value: string;
  hint: string;
  icon: ComponentType<{ className?: string }>;
  progress?: number;
}) {
  return (
    <div className='flex min-w-0 flex-col gap-1 rounded-lg border border-dashed bg-background/60 px-3 py-2.5'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate text-[11px] text-muted-foreground'>{label}</p>
          <p className='mt-1 text-lg font-semibold tabular-nums leading-none tracking-tight'>
            {value}
          </p>
        </div>
        <Icon className='size-3.5 shrink-0 text-muted-foreground' />
      </div>
      {typeof progress === 'number' ? (
        <Progress value={progress} className='h-1' />
      ) : null}
      <p className='truncate text-[10px] text-muted-foreground'>{hint}</p>
    </div>
  );
}

export function WorldStage({
  summary,
  traffic,
  capacity,
  nodes,
  sourceCountries,
}: {
  summary: DashboardSummary;
  traffic: DashboardTraffic;
  capacity: DashboardCapacity;
  nodes: DashboardNodeHealth[];
  sourceCountries: DistributionItem[];
}) {
  const t = useTranslations('dashboard.world');
  const mapViewportRef = useRef<HTMLDivElement | null>(null);
  const [shouldRenderMap, setShouldRenderMap] = useState(false);

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

  const onlineRate =
    summary.total_nodes > 0
      ? (summary.online_nodes / summary.total_nodes) * 100
      : 0;
  const healthyNodes = Math.max(
    0,
    summary.online_nodes - summary.unhealthy_nodes,
  );
  const healthyRate =
    summary.total_nodes > 0 ? (healthyNodes / summary.total_nodes) * 100 : 0;
  const geoConfiguredNodes = nodes.filter(
    (node) =>
      typeof node.geo_latitude === 'number' &&
      typeof node.geo_longitude === 'number',
  ).length;

  const mapModeLabel =
    sourceCountries.length > 0
      ? t('modeVisitorSource')
      : geoConfiguredNodes > 0
        ? t('modeNodeCoords')
        : t('modeCoverage');

  const headlineMetrics = [
    {
      label: t('onlineCoverage'),
      value: formatPercent(onlineRate),
      hint: t('onlineHint', {
        online: summary.online_nodes,
        total: summary.total_nodes,
      }),
      icon: Server,
      progress: onlineRate,
    },
    {
      label: t('runHealth'),
      value: formatPercent(healthyRate),
      hint: t('unhealthyCount', { count: summary.unhealthy_nodes }),
      icon: ShieldCheck,
      progress: healthyRate,
    },
    {
      label: t('requests24h'),
      value: formatCompactNumber(traffic.request_count),
      hint: t('qpsReported', {
        qps: traffic.estimated_qps.toFixed(1),
        count: traffic.reported_nodes,
      }),
      icon: Activity,
    },
    {
      label: t('nodeCoords'),
      value: formatCompactNumber(geoConfiguredNodes),
      hint:
        geoConfiguredNodes > 0
          ? t('sourceCountries', { count: sourceCountries.length })
          : t('fallbackPins'),
      icon: Network,
    },
  ] as const;

  const detailMetrics = [
    {
      label: t('avgCpu'),
      value: formatPercent(capacity.average_cpu_usage_percent),
      hint: t('highCount', { count: capacity.high_cpu_nodes }),
      icon: Cpu,
      progress: capacity.average_cpu_usage_percent,
    },
    {
      label: t('avgMemory'),
      value: formatPercent(capacity.average_memory_usage_percent),
      hint: t('highCount', { count: capacity.high_memory_nodes }),
      icon: MemoryStick,
      progress: capacity.average_memory_usage_percent,
    },
    {
      label: t('highStorage'),
      value: formatCompactNumber(capacity.high_storage_nodes),
      hint: t('offlinePending', {
        offline: summary.offline_nodes,
        pending: summary.pending_nodes,
      }),
      icon: HardDrive,
    },
  ] as const;

  return (
    <Card className='overflow-hidden border-dashed shadow-none'>
      <CardHeader className='gap-3 space-y-0 pb-3'>
        <div className='flex flex-wrap items-start justify-between gap-x-4 gap-y-3'>
          <div className='min-w-0'>
            <CardTitle className='flex items-center gap-1.5 text-sm font-semibold'>
              <Globe2 className='size-4 shrink-0 text-primary' />
              {t('title')}
              <Badge
                variant='outline'
                className='ml-1 text-[10px] font-normal text-muted-foreground'
              >
                {mapModeLabel}
              </Badge>
            </CardTitle>
            <CardDescription className='mt-1 text-xs'>
              {t('description')}
            </CardDescription>
          </div>
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
        <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
          {headlineMetrics.map((metric) => (
            <SummaryMetric key={metric.label} {...metric} />
          ))}
        </div>

        <div className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_18rem] xl:items-stretch'>
          <div className='flex min-w-0 items-center justify-center rounded-lg border border-dashed bg-muted/10 p-3'>
            <div
              ref={mapViewportRef}
              className='relative aspect-[5/3] min-h-[260px] w-full max-w-[720px] overflow-hidden rounded-md bg-background/60 sm:min-h-0'
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
                  <p className='rounded-md border border-dashed bg-background/90 px-2.5 py-1.5 text-[10px] text-muted-foreground backdrop-blur-sm'>
                    {t('noNodesOverlay')}
                  </p>
                </div>
              ) : null}
            </div>
          </div>

          <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-1'>
            {detailMetrics.map((metric) => (
              <DetailMetric key={metric.label} {...metric} />
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
