'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';

import { RankChart } from '@/components/data/rank-chart';
import { TrendChart } from '@/components/data/trend-chart';
import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { getDashboardOverview } from '@/features/dashboard/api/overview';
import type { DashboardNodeHealth } from '@/features/dashboard/types';
import { WorldStage } from '@/features/dashboard/components/world-stage';
import {
  getNodeStatusLabel,
  getNodeStatusVariant,
  getOpenrestyStatusLabel,
  getOpenrestyStatusVariant,
} from '@/features/nodes/utils';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';
import {
  formatBytes,
  formatBytesPerSecond,
  formatPercent,
} from '@/lib/utils/metrics';

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function formatTrendHour(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return `${date.getHours().toString().padStart(2, '0')}:00`;
}

function buildNodeDetailHref(id?: number | null) {
  if (!id) {
    return '/node';
  }
  return `/node/detail?id=${id}`;
}

function buildNodeRankItems(
  nodes: DashboardNodeHealth[],
  selector: (node: DashboardNodeHealth) => number,
  limit = 5,
) {
  return [...nodes]
    .sort((left, right) => {
      const leftValue = selector(left);
      const rightValue = selector(right);
      if (leftValue === rightValue) {
        return left.name.localeCompare(right.name, 'zh-CN');
      }
      return rightValue - leftValue;
    })
    .slice(0, limit)
    .filter((node) => selector(node) > 0)
    .map((node) => ({
      label: node.name,
      value: selector(node),
    }));
}

function NodeHealthRow({ node }: { node: DashboardNodeHealth }) {
  return (
    <Link
      href={buildNodeDetailHref(node.id)}
      className="block rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 transition hover:border-[var(--border-strong)] hover:bg-[var(--surface-muted)]"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-[var(--foreground-primary)]">
            {node.name}
          </p>
          <p className="mt-1 text-xs text-[var(--foreground-muted)]">
            {node.node_id}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <StatusBadge
            label={getNodeStatusLabel(node.status)}
            variant={getNodeStatusVariant(node.status)}
          />
          <StatusBadge
            label={getOpenrestyStatusLabel(node.openresty_status)}
            variant={getOpenrestyStatusVariant(node.openresty_status)}
          />
        </div>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-4">
        <div>
          <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
            CPU
          </p>
          <p className="mt-2 text-sm text-[var(--foreground-primary)]">
            {formatPercent(node.cpu_usage_percent)}
          </p>
        </div>
        <div>
          <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
            内存
          </p>
          <p className="mt-2 text-sm text-[var(--foreground-primary)]">
            {formatPercent(node.memory_usage_percent)}
          </p>
        </div>
        <div>
          <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
            最近窗口请求
          </p>
          <p className="mt-2 text-sm text-[var(--foreground-primary)]">
            {node.request_count.toLocaleString('zh-CN')}
          </p>
        </div>
        <div>
          <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
            活动异常
          </p>
          <p className="mt-2 text-sm text-[var(--foreground-primary)]">
            {node.active_event_count}
          </p>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-[var(--foreground-secondary)]">
        <p>当前版本：{node.current_version || '未应用'}</p>
        <p>
          最近心跳：
          {node.last_seen_at
            ? ` ${formatRelativeTime(node.last_seen_at)} · ${formatDateTime(node.last_seen_at)}`
            : ' 暂无'}
        </p>
      </div>
    </Link>
  );
}

export function DashboardOverview() {
  const overviewQuery = useQuery({
    queryKey: ['dashboard', 'overview'],
    queryFn: getDashboardOverview,
    refetchInterval: 10000,
  });

  if (overviewQuery.isLoading) {
    return <LoadingState />;
  }

  if (overviewQuery.isError) {
    return (
      <ErrorState
        title="总览看板加载失败"
        description={getErrorMessage(overviewQuery.error)}
      />
    );
  }

  const overview = overviewQuery.data;
  if (!overview) {
    return (
      <EmptyState
        title="暂无总览数据"
        description="系统已经启动，但还没有可展示的总览聚合结果。"
      />
    );
  }

  return (
    <div className="space-y-6">
      <WorldStage
        summary={overview.summary}
        traffic={overview.traffic}
        capacity={overview.capacity}
        nodes={overview.nodes}
        sourceCountries={overview.distributions.source_countries}
      />

      <div className="grid gap-6 xl:grid-cols-2">
        <AppCard
          title="24 小时请求趋势"
          description="观察整体请求量和错误量是否出现异常抬升。"
        >
          <TrendChart
            labels={overview.trends.traffic_24h.map((point) =>
              formatTrendHour(point.bucket_started_at),
            )}
            series={[
              {
                label: '请求量',
                color: '#f59e0b',
                fillColor: 'rgba(245, 158, 11, 0.18)',
                variant: 'area',
                values: overview.trends.traffic_24h.map(
                  (point) => point.request_count,
                ),
              },
              {
                label: '错误量',
                color: '#ef4444',
                values: overview.trends.traffic_24h.map(
                  (point) => point.error_count,
                ),
              },
            ]}
          />
        </AppCard>

        <AppCard
          title="24 小时容量趋势"
          description="按小时聚合 CPU 与内存使用率，判断整体容量是否持续紧张。"
        >
          <TrendChart
            labels={overview.trends.capacity_24h.map((point) =>
              formatTrendHour(point.bucket_started_at),
            )}
            yAxisValueFormatter={formatPercent}
            series={[
              {
                label: '平均 CPU',
                color: '#0f766e',
                fillColor: 'rgba(15, 118, 110, 0.15)',
                variant: 'area',
                values: overview.trends.capacity_24h.map(
                  (point) => point.average_cpu_usage_percent,
                ),
                valueFormatter: formatPercent,
              },
              {
                label: '平均内存',
                color: '#2563eb',
                values: overview.trends.capacity_24h.map(
                  (point) => point.average_memory_usage_percent,
                ),
                valueFormatter: formatPercent,
              },
            ]}
          />
        </AppCard>
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
        <AppCard
          title="24 小时网络与磁盘趋势"
          description="把 OpenResty 吞吐和磁盘读写放到同一屏，方便判断是流量拉升还是资源抖动。"
        >
          <div className="space-y-6">
            <TrendChart
              labels={overview.trends.network_24h.map((point) =>
                formatTrendHour(point.bucket_started_at),
              )}
              height={180}
              yAxisValueFormatter={(value) => formatBytesPerSecond(value, 3600)}
              series={[
                {
                  label: 'OpenResty 入站',
                  color: '#22c55e',
                  fillColor: 'rgba(34, 197, 94, 0.14)',
                  variant: 'area',
                  values: overview.trends.network_24h.map(
                    (point) => point.openresty_rx_bytes,
                  ),
                  valueFormatter: (value) => formatBytesPerSecond(value, 3600),
                },
                {
                  label: 'OpenResty 出站',
                  color: '#38bdf8',
                  values: overview.trends.network_24h.map(
                    (point) => point.openresty_tx_bytes,
                  ),
                  valueFormatter: (value) => formatBytesPerSecond(value, 3600),
                },
              ]}
            />

            <TrendChart
              labels={overview.trends.disk_io_24h.map((point) =>
                formatTrendHour(point.bucket_started_at),
              )}
              height={180}
              yAxisValueFormatter={formatBytes}
              series={[
                {
                  label: '磁盘读',
                  color: '#a78bfa',
                  fillColor: 'rgba(167, 139, 250, 0.14)',
                  variant: 'area',
                  values: overview.trends.disk_io_24h.map(
                    (point) => point.disk_read_bytes,
                  ),
                  valueFormatter: formatBytes,
                },
                {
                  label: '磁盘写',
                  color: '#fb7185',
                  values: overview.trends.disk_io_24h.map(
                    (point) => point.disk_write_bytes,
                  ),
                  valueFormatter: formatBytes,
                },
              ]}
            />
          </div>
        </AppCard>

        <AppCard
          title="Top 节点榜单"
          description="把流量、错误与容量压力节点并排展示，保留快速定位热点与瓶颈的入口。"
        >
          <div className="grid gap-6 xl:grid-cols-1">
            <div>
              <p className="mb-3 text-xs tracking-[0.22em] text-[var(--foreground-muted)] uppercase">
                流量最高节点
              </p>
              <RankChart
                items={buildNodeRankItems(
                  overview.nodes,
                  (node) => node.request_count,
                )}
                color="#38bdf8"
                emptyMessage="暂无流量榜单"
              />
            </div>
            <div>
              <p className="mb-3 text-xs tracking-[0.22em] text-[var(--foreground-muted)] uppercase">
                容量压力节点
              </p>
              <RankChart
                items={buildNodeRankItems(overview.nodes, (node) =>
                  Math.round(
                    Math.max(
                      node.cpu_usage_percent,
                      node.memory_usage_percent,
                      node.storage_usage_percent,
                    ),
                  ),
                )}
                color="#ef4444"
                valueFormatter={(value) => `${value}%`}
                emptyMessage="暂无容量压力数据"
              />
            </div>
          </div>
        </AppCard>
      </div>

      <div className="grid gap-6 xl:grid-cols-3">
        <AppCard
          title="来源分布"
          description="聚合最近 24 小时主要来源国家，优先识别流量重心变化。"
        >
          <RankChart
            items={overview.distributions.source_countries.map((item) => ({
              label: item.key,
              value: item.value,
            }))}
            color="#38bdf8"
            emptyMessage="暂无来源分布数据"
          />
        </AppCard>

        <AppCard
          title="状态码分布"
          description="快速判断成功响应是否仍是主流，以及错误码是否有抬升。"
        >
          <RankChart
            items={overview.distributions.status_codes.map((item) => ({
              label: `HTTP ${item.key}`,
              value: item.value,
            }))}
            color="#f59e0b"
            emptyMessage="暂无状态码分布"
          />
        </AppCard>

        <AppCard
          title="Top Domain"
          description="观察主要流量集中在哪些域名，方便判断业务热度与异常集中点。"
        >
          <RankChart
            items={overview.distributions.top_domains.map((item) => ({
              label: item.key,
              value: item.value,
            }))}
            color="#34d399"
            emptyMessage="暂无域名分布"
          />
        </AppCard>
      </div>

      <AppCard
        title="节点健康列表"
        description="按异常数量和资源压力排序，优先显示最需要关注的节点。"
        action={
          <Link
            href="/node"
            className="inline-flex items-center rounded-full border border-[var(--border-default)] px-3 py-1.5 text-xs text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
          >
            进入节点页
          </Link>
        }
      >
        {overview.nodes.length === 0 ? (
          <EmptyState
            title="暂无节点"
            description="节点接入后，这里会展示系统健康与容量摘要。"
          />
        ) : (
          <div className="space-y-4">
            {overview.nodes.slice(0, 8).map((node) => (
              <NodeHealthRow key={node.node_id} node={node} />
            ))}
          </div>
        )}
      </AppCard>
    </div>
  );
}
