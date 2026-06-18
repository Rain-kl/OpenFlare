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

type LegendTone = 'source' | 'healthy' | 'warning' | 'danger';

const legendToneClass: Record<LegendTone, string> = {
  source: 'bg-blue-500/10 border-blue-500/20 text-blue-600 dark:text-blue-400',
  healthy:
    'bg-emerald-500/10 border-emerald-500/20 text-emerald-600 dark:text-emerald-400',
  warning:
    'bg-amber-500/10 border-amber-500/20 text-amber-600 dark:text-amber-400',
  danger:
    'bg-destructive/10 border-destructive/20 text-destructive',
};

function LegendBadge({label, tone}: {label: string; tone: LegendTone}) {
  return (
    <Badge
      variant="outline"
      className={cn('text-[10px] font-medium', legendToneClass[tone])}
    >
      {label}
    </Badge>
  );
}

function StageMetricCard({
  label,
  value,
  hint,
  icon: Icon,
  progress,
}: {
  label: string;
  value: string;
  hint: string;
  icon: ComponentType<{className?: string}>;
  progress?: number;
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
        <Icon className="size-4 text-primary" />
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="text-2xl font-semibold tracking-tight">{value}</div>
        {typeof progress === 'number' ? (
          <Progress value={progress} className="h-1.5" />
        ) : null}
        <p className="text-[10px] text-muted-foreground">{hint}</p>
      </CardContent>
    </Card>
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
      {rootMargin: '180px 0px'},
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
      ? '访客来源热度'
      : geoConfiguredNodes > 0
        ? '节点地理坐标'
        : '节点覆盖信号';

  return (
    <div className="grid gap-6 xl:grid-cols-[1.32fr_0.88fr]">
      <Card className="border-dashed shadow-none">
        <CardHeader className="space-y-3">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
                <Globe2 className="size-4 text-primary" />
                全球态势板
              </CardTitle>
              <CardDescription className="text-xs">
                节点地理分布与访客来源热度一览
              </CardDescription>
            </div>
            <Badge variant="outline" className="text-[10px] text-muted-foreground">
              {mapModeLabel}
            </Badge>
          </div>
          <div className="flex flex-wrap gap-1.5">
            <LegendBadge label="国家底色: 来源热度" tone="source" />
            <LegendBadge label="绿色: 运行正常" tone="healthy" />
            <LegendBadge label="黄色: 资源承压" tone="warning" />
            <LegendBadge label="红色: 异常待处理" tone="danger" />
          </div>
        </CardHeader>
        <CardContent>
          <div
            ref={mapViewportRef}
            className="relative min-h-[480px] overflow-hidden rounded-lg border border-dashed bg-muted/20 lg:min-h-[540px]"
          >
            <div className="absolute inset-0 flex items-center justify-center p-4">
              {shouldRenderMap ? (
                <WorldStageMap nodes={nodes} sourceCountries={sourceCountries} />
              ) : (
                <EmptyState
                  title="全球地图准备中"
                  description="地图会在进入可视区域后再加载，避免首页滚动和交互被首屏图表拖慢。"
                  iconSize="sm"
                />
              )}
            </div>

            {shouldRenderMap && nodes.length === 0 ? (
              <div className="pointer-events-none absolute inset-x-4 bottom-4 z-10">
                <div className="rounded-lg border border-dashed bg-background/90 px-4 py-3 text-xs text-muted-foreground backdrop-blur-sm">
                  当前暂无节点接入。地图已完成初始化，后续会在这里展示节点位置与健康状态。
                </div>
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 sm:grid-cols-2">
        <StageMetricCard
          label="在线覆盖"
          value={formatPercent(onlineRate)}
          hint={`${summary.online_nodes}/${summary.total_nodes} 个节点在线`}
          icon={Server}
          progress={onlineRate}
        />
        <StageMetricCard
          label="运行健康"
          value={formatPercent(healthyRate)}
          hint={`${summary.unhealthy_nodes} 个节点存在 OpenResty 异常`}
          icon={ShieldCheck}
          progress={healthyRate}
        />
        <StageMetricCard
          label="最近窗口请求"
          value={formatCompactNumber(traffic.request_count)}
          hint={`QPS ${traffic.estimated_qps.toFixed(1)} · ${traffic.reported_nodes} 个节点已上报`}
          icon={Activity}
        />
        <StageMetricCard
          label="平均 CPU"
          value={formatPercent(capacity.average_cpu_usage_percent)}
          hint={`${capacity.high_cpu_nodes} 个节点 CPU 偏高`}
          icon={Cpu}
          progress={capacity.average_cpu_usage_percent}
        />
        <StageMetricCard
          label="平均内存"
          value={formatPercent(capacity.average_memory_usage_percent)}
          hint={`${capacity.high_memory_nodes} 个高内存节点`}
          icon={MemoryStick}
          progress={capacity.average_memory_usage_percent}
        />
        <StageMetricCard
          label="高存储节点"
          value={formatCompactNumber(capacity.high_storage_nodes)}
          hint={`${summary.offline_nodes} 离线 · ${summary.pending_nodes} 待接入`}
          icon={HardDrive}
        />
      </div>
    </div>
  );
}