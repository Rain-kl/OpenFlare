'use client';

import {useQuery} from '@tanstack/react-query';
import {LayoutDashboard, RefreshCw} from 'lucide-react';

import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import {Button} from '@/components/ui/button';
import {DashboardService} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';

import {CapacityTrendChart} from './components/dashboard/capacity-trend-chart';
import {
  SourceDistributionChart,
  StatusCodeDistributionChart,
  TopDomainChart,
} from './components/dashboard/distribution-rank-charts';
import {NetworkDiskTrendChart} from './components/dashboard/network-disk-trend-chart';
import {NodeHealthTable} from './components/dashboard/node-health-table';
import {NodeRankPanel} from './components/dashboard/node-rank-panel';
import {DashboardStatCards} from './components/dashboard/stat-cards';
import {TrafficTrendChart} from './components/dashboard/traffic-trend-chart';
import {getErrorMessage} from './nodes/components/node-utils';

const dashboardQueryKey = ['openflare', 'dashboard', 'overview'];

export default function OpenFlareDashboardPage() {
  const overviewQuery = useQuery({
    queryKey: dashboardQueryKey,
    queryFn: () => DashboardService.getOverview(),
    refetchInterval: 30_000,
  });

  const overview = overviewQuery.data;

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <LayoutDashboard className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">总览</h1>
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {overview?.generated_at ? (
            <span>数据生成于 {formatDateTime(overview.generated_at)}</span>
          ) : null}
          <Button
            variant="outline"
            size="sm"
            className="h-8"
            onClick={() => overviewQuery.refetch()}
            disabled={overviewQuery.isFetching}
          >
            <RefreshCw
              className={`size-3.5 mr-1.5 ${overviewQuery.isFetching ? 'animate-spin' : ''}`}
            />
            刷新
          </Button>
        </div>
      </div>

      {overviewQuery.isLoading ? (
        <LoadingStateWithBorder
          title="加载总览数据"
          description="正在聚合节点健康、流量与容量指标..."
        />
      ) : overviewQuery.isError ? (
        <ErrorInline
          message={`总览看板加载失败：${getErrorMessage(overviewQuery.error)}`}
          onRetry={() => overviewQuery.refetch()}
        />
      ) : !overview ? (
        <EmptyStateWithBorder
          title="暂无总览数据"
          description="系统已经启动，但还没有可展示的总览聚合结果。"
        />
      ) : (
        <>
          <DashboardStatCards
            summary={overview.summary}
            traffic={overview.traffic}
            capacity={overview.capacity}
          />

          <div className="grid gap-6 xl:grid-cols-2">
            <TrafficTrendChart points={overview.trends.traffic_24h} />
            <CapacityTrendChart points={overview.trends.capacity_24h} />
          </div>

          <div className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
            <NetworkDiskTrendChart
              networkPoints={overview.trends.network_24h}
              diskPoints={overview.trends.disk_io_24h}
            />
            <NodeRankPanel nodes={overview.nodes} />
          </div>

          <div className="grid gap-6 xl:grid-cols-3">
            <SourceDistributionChart items={overview.distributions.source_countries} />
            <StatusCodeDistributionChart items={overview.distributions.status_codes} />
            <TopDomainChart items={overview.distributions.top_domains} />
          </div>

          <NodeHealthTable nodes={overview.nodes} />
        </>
      )}
    </div>
  );
}