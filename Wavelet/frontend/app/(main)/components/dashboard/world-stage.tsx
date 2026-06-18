'use client';

import dynamic from 'next/dynamic';
import {useEffect, useRef, useState, type ComponentType} from 'react';
import {
  Activity,
  Cpu,
  Globe2,
  HardDrive,
  MemoryStick,
  Server,
  ShieldCheck,
} from 'lucide-react';

import {EmptyState} from '@/components/layout/empty';
import {Badge} from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {Progress} from '@/components/ui/progress';
import type {
  DashboardCapacity,
  DashboardNodeHealth,
  DashboardSummary,
  DashboardTraffic,
  DistributionItem,
} from '@/lib/services/openflare';
import {cn} from '@/lib/utils';

import {formatCompactNumber, formatPercent} from './dashboard-utils';

const WorldStageMap = dynamic(
  () => import('./world-stage-map').then((module) => module.WorldStageMap),
  {ssr: false},
);

const LEGEND_ITEMS = [
  {dot: 'bg-blue-500', label: '来源'},
  {dot: 'bg-emerald-500', label: '正常'},
  {dot: 'bg-amber-500', label: '承压'},
  {dot: 'bg-destructive', label: '异常'},
] as const;

function CompactMetric({
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
  icon: ComponentType<{className?: string}>;
  progress?: number;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'flex min-w-0 flex-col gap-1 rounded-lg border border-dashed bg-muted/15 px-2.5 py-2 lg:rounded-none lg:border-0 lg:bg-transparent lg:px-3 lg:py-2.5',
        className,
      )}
    >
      <div className="flex items-center justify-between gap-1.5">
        <span className="truncate text-[10px] text-muted-foreground">{label}</span>
        <Icon className="size-3 shrink-0 text-primary/60" />
      </div>
      <span className="text-base font-semibold tabular-nums leading-none tracking-tight">
        {value}
      </span>
      {typeof progress === 'number' ? (
        <Progress value={progress} className="h-0.5" />
      ) : null}
      <span className="truncate text-[10px] text-muted-foreground">{hint}</span>
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
      {rootMargin: '120px 0px'},
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
      ? '访客来源'
      : geoConfiguredNodes > 0
        ? '节点坐标'
        : '覆盖信号';

  const metrics = [
    {
      label: '在线覆盖',
      value: formatPercent(onlineRate),
      hint: `${summary.online_nodes}/${summary.total_nodes} 在线`,
      icon: Server,
      progress: onlineRate,
    },
    {
      label: '运行健康',
      value: formatPercent(healthyRate),
      hint: `${summary.unhealthy_nodes} 个异常`,
      icon: ShieldCheck,
      progress: healthyRate,
    },
    {
      label: '窗口请求',
      value: formatCompactNumber(traffic.request_count),
      hint: `QPS ${traffic.estimated_qps.toFixed(1)}`,
      icon: Activity,
    },
    {
      label: '平均 CPU',
      value: formatPercent(capacity.average_cpu_usage_percent),
      hint: `${capacity.high_cpu_nodes} 个偏高`,
      icon: Cpu,
      progress: capacity.average_cpu_usage_percent,
    },
    {
      label: '平均内存',
      value: formatPercent(capacity.average_memory_usage_percent),
      hint: `${capacity.high_memory_nodes} 个偏高`,
      icon: MemoryStick,
      progress: capacity.average_memory_usage_percent,
    },
    {
      label: '高存储',
      value: formatCompactNumber(capacity.high_storage_nodes),
      hint: `${summary.offline_nodes} 离线 · ${summary.pending_nodes} 待接入`,
      icon: HardDrive,
    },
  ] as const;

  return (
    <Card className="border-dashed shadow-none">
      <CardHeader className="gap-2 space-y-0 pb-3">
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
          <div className="min-w-0">
            <CardTitle className="flex items-center gap-1.5 text-sm font-semibold">
              <Globe2 className="size-4 shrink-0 text-primary" />
              全球态势板
              <Badge
                variant="outline"
                className="ml-1 text-[10px] font-normal text-muted-foreground"
              >
                {mapModeLabel}
              </Badge>
            </CardTitle>
            <CardDescription className="text-xs">
              节点分布与来源热度
            </CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            {LEGEND_ITEMS.map((item) => (
              <span
                key={item.label}
                className="inline-flex items-center gap-1 text-[10px] text-muted-foreground"
              >
                <span className={cn('size-1.5 rounded-full', item.dot)} />
                {item.label}
              </span>
            ))}
          </div>
        </div>
      </CardHeader>

      <CardContent className="pt-0">
        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_12.5rem] lg:gap-4">
          <div
            ref={mapViewportRef}
            className="relative h-[200px] min-h-[200px] min-w-0 overflow-hidden rounded-lg border border-dashed bg-muted/20 sm:h-[240px] sm:min-h-[240px] lg:h-[260px] lg:min-h-[260px]"
          >
            {shouldRenderMap ? (
              <WorldStageMap nodes={nodes} sourceCountries={sourceCountries} />
            ) : (
              <div className="flex h-full items-center justify-center px-4">
                <EmptyState
                  title="地图准备中"
                  description="进入可视区域后加载"
                  iconSize="sm"
                />
              </div>
            )}

            {shouldRenderMap && nodes.length === 0 ? (
              <div className="pointer-events-none absolute inset-x-3 bottom-2 z-10">
                <p className="rounded-md border border-dashed bg-background/90 px-2.5 py-1.5 text-[10px] text-muted-foreground backdrop-blur-sm">
                  暂无节点，接入后将展示地理分布
                </p>
              </div>
            ) : null}
          </div>

          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1 lg:gap-0 lg:divide-y lg:divide-dashed lg:overflow-hidden lg:rounded-lg lg:border lg:border-dashed">
            {metrics.map((metric) => (
              <CompactMetric key={metric.label} {...metric} />
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}