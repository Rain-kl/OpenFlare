'use client';

import {useMemo, useState} from 'react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {Loader2, Trash2} from 'lucide-react';
import {toast} from 'sonner';

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {formatDateTime} from '@/lib/utils';
import type {NodeHealthEvent, NodeItem, NodeSystemProfile} from '@/lib/services/openflare';
import {NodeService} from '@/lib/services/openflare';

import {CapacityTrendChart} from '../../components/dashboard/capacity-trend-chart';
import {TrafficTrendChart} from '../../components/dashboard/traffic-trend-chart';
import {DiskIOTrendChart} from './disk-io-trend-chart';
import {DistributionList} from './distribution-list';
import {NetworkTrendChart} from './network-trend-chart';
import {NodeStatusBadge} from './node-status-badge';
import {
  aggregateTrafficBreakdown,
  formatBytes,
  formatBytesPerSecond,
  formatPercent,
  formatRelativeTime,
  formatUptime,
  formatUsageRatio,
  getErrorMessage,
  getHealthEventLabel,
  getHealthEventTone,
  getNodeStatusLabel,
  getNodeStatusTone,
  getOpenrestyStatusLabel,
  getOpenrestyStatusTone,
  isMeaningfulTime,
} from './node-utils';

type HealthEventFilter = 'all' | 'active' | 'resolved';
type NodeObservabilityVariant = 'edge' | 'compact';

function MetricBar({
  label,
  value,
  progress,
  hint,
}: {
  label: string;
  value: string;
  progress?: number | null;
  hint?: string;
}) {
  return (
    <div className="space-y-2 rounded-lg border px-3 py-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">{label}</p>
          {hint ? <p className="mt-1 text-xs text-muted-foreground">{hint}</p> : null}
        </div>
        <p className="text-sm font-medium">{value}</p>
      </div>
      {progress !== null && progress !== undefined ? (
        <div className="h-2 overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-primary transition-[width]"
            style={{ width: `${progress}%` }}
          />
        </div>
      ) : null}
    </div>
  );
}

function SummaryStat({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint: string;
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader className="pb-2">
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-base font-semibold">{value}</CardTitle>
        <p className="text-sm text-muted-foreground">{hint}</p>
      </CardHeader>
    </Card>
  );
}

function SystemProfileCard({
  profile,
  nodeName,
}: {
  profile: NodeSystemProfile;
  nodeName: string;
}) {
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <div className="space-y-4 rounded-lg border px-4 py-4">
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">主机名</p>
          <p className="mt-2 text-sm">{profile.hostname || nodeName}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">操作系统</p>
          <p className="mt-2 text-sm">
            {profile.os_name || 'unknown'}
            {profile.os_version ? ` ${profile.os_version}` : ''}
          </p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">内核 / 架构</p>
          <p className="mt-2 text-sm">
            {profile.kernel_version || 'unknown'} · {profile.architecture || 'unknown'}
          </p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">在线时长</p>
          <p className="mt-2 text-sm">{formatUptime(profile.uptime_seconds)}</p>
        </div>
      </div>

      <div className="space-y-4 rounded-lg border px-4 py-4">
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">CPU</p>
          <p className="mt-2 text-sm">{profile.cpu_model || 'unknown'}</p>
          <p className="mt-1 text-xs text-muted-foreground">{profile.cpu_cores || 0} 核</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">总内存</p>
          <p className="mt-2 text-sm">{formatBytes(profile.total_memory_bytes)}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">总存储</p>
          <p className="mt-2 text-sm">{formatBytes(profile.total_disk_bytes)}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide">上报时间</p>
          <p className="mt-2 text-sm">
            {isMeaningfulTime(profile.reported_at)
              ? formatDateTime(profile.reported_at)
              : '—'}
          </p>
        </div>
      </div>
    </div>
  );
}

function HealthEventTimeline({
  events,
  allEventsCount,
  healthEventFilter,
  onFilterChange,
  onCleanup,
  cleanupPending,
}: {
  events: NodeHealthEvent[];
  allEventsCount: number;
  healthEventFilter: HealthEventFilter;
  onFilterChange: (filter: HealthEventFilter) => void;
  onCleanup: () => void;
  cleanupPending: boolean;
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle className="text-base font-semibold">健康事件时间线</CardTitle>
          <CardDescription>保留活动与已恢复事件，帮助判断运行状态。</CardDescription>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="h-7 text-xs text-destructive hover:text-destructive"
          disabled={cleanupPending || allEventsCount === 0}
          onClick={onCleanup}
        >
          <Trash2 className="size-3.5 mr-1" />
          清理日志
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        {allEventsCount > 0 ? (
          <>
            <div className="flex flex-wrap gap-2">
              {(
                [
                  ['all', '全部事件'],
                  ['active', '活动中'],
                  ['resolved', '已恢复'],
                ] as const
              ).map(([filter, label]) => (
                <Button
                  key={filter}
                  type="button"
                  variant={healthEventFilter === filter ? 'secondary' : 'outline'}
                  size="sm"
                  className="h-7 text-xs rounded-full"
                  onClick={() => onFilterChange(filter)}
                >
                  {label}
                </Button>
              ))}
            </div>

            {events.length === 0 ? (
              <p className="text-sm text-muted-foreground">当前筛选下没有健康事件。</p>
            ) : null}

            {events.slice(0, 8).map((event) => (
              <div
                key={`${event.event_type}-${event.last_triggered_at}-${event.status}`}
                className="rounded-lg border px-4 py-3"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <NodeStatusBadge
                    label={getHealthEventLabel(event)}
                    tone={getHealthEventTone(event)}
                  />
                  <NodeStatusBadge
                    label={event.status === 'active' ? '活动中' : '已恢复'}
                    tone={event.status === 'active' ? 'warning' : 'success'}
                  />
                </div>
                <p className="mt-2 text-sm text-muted-foreground">
                  {event.message || '暂无详细消息'}
                </p>
                <div className="mt-2 grid gap-1 text-xs text-muted-foreground md:grid-cols-3">
                  <p>
                    首次触发：
                    {isMeaningfulTime(event.first_triggered_at)
                      ? ` ${formatDateTime(event.first_triggered_at)}`
                      : ' —'}
                  </p>
                  <p>
                    最近触发：
                    {isMeaningfulTime(event.last_triggered_at)
                      ? ` ${formatDateTime(event.last_triggered_at)}`
                      : ' —'}
                  </p>
                  <p>
                    恢复时间：
                    {isMeaningfulTime(event.resolved_at) && event.resolved_at
                      ? ` ${formatDateTime(event.resolved_at)}`
                      : ' —'}
                  </p>
                </div>
              </div>
            ))}
          </>
        ) : (
          <p className="text-sm text-muted-foreground">节点当前还没有上报健康事件记录。</p>
        )}
      </CardContent>
    </Card>
  );
}

export function NodeObservability({
  nodeId,
  node,
  variant = 'edge',
  connectionHint = 'OpenResty 当前连接',
}: {
  nodeId: number;
  node?: NodeItem;
  variant?: NodeObservabilityVariant;
  connectionHint?: string;
}) {
  const queryClient = useQueryClient();
  const [healthEventFilter, setHealthEventFilter] = useState<HealthEventFilter>('all');
  const [cleanupOpen, setCleanupOpen] = useState(false);

  const observabilityQuery = useQuery({
    queryKey: ['openflare', 'node-observability', nodeId],
    queryFn: () => NodeService.getObservability(nodeId, { hours: 24, limit: 48 }),
    refetchInterval: 10000,
  });

  const cleanupMutation = useMutation({
    mutationFn: () => NodeService.cleanupHealthEvents(nodeId),
    onSuccess: async (result) => {
      toast.success(
        result.deleted_count > 0
          ? `已清理 ${result.deleted_count} 条健康事件日志`
          : '当前没有可清理的健康事件日志',
      );
      setCleanupOpen(false);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'node-observability', nodeId],
        }),
        queryClient.invalidateQueries({ queryKey: ['openflare', 'dashboard', 'overview'] }),
      ]);
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const observability = observabilityQuery.data ?? null;
  const profile = observability?.profile ?? null;
  const latestMetric = observability?.metric_snapshots?.[0] ?? null;
  const activeHealthEvents = useMemo(
    () => observability?.health_events.filter((event) => event.status === 'active') ?? [],
    [observability?.health_events],
  );
  const resolvedHealthEvents = useMemo(
    () => observability?.health_events.filter((event) => event.status === 'resolved') ?? [],
    [observability?.health_events],
  );
  const filteredHealthEvents = useMemo(() => {
    switch (healthEventFilter) {
      case 'active':
        return activeHealthEvents;
      case 'resolved':
        return resolvedHealthEvents;
      default:
        return observability?.health_events ?? [];
    }
  }, [activeHealthEvents, healthEventFilter, observability?.health_events, resolvedHealthEvents]);

  const statusCodeDistribution = useMemo(
    () => aggregateTrafficBreakdown(observability?.traffic_reports, 'status_codes_json'),
    [observability?.traffic_reports],
  );
  const topDomains = useMemo(
    () => aggregateTrafficBreakdown(observability?.traffic_reports, 'top_domains_json'),
    [observability?.traffic_reports],
  );
  const trafficSummary = observability?.analytics?.traffic ?? null;
  const healthSummary = observability?.analytics?.health ?? null;
  const topSourceCountry =
    observability?.analytics?.distributions?.source_countries?.[0] ?? null;
  const latestHealthEvent = activeHealthEvents[0] ?? null;
  const dominantStatusCode = statusCodeDistribution[0] ?? null;
  const dominantDomain = topDomains[0] ?? null;
  const memoryUsageRatio = formatUsageRatio(
    latestMetric?.memory_used_bytes,
    latestMetric?.memory_total_bytes,
  );
  const storageUsageRatio = formatUsageRatio(
    latestMetric?.storage_used_bytes,
    latestMetric?.storage_total_bytes,
  );
  const trends = observability?.trends;
  const nodeName = node?.name ?? observability?.node_id ?? String(nodeId);

  if (observabilityQuery.isLoading) {
    return (
      <Card className="border-dashed shadow-none">
        <CardContent className="flex items-center justify-center py-10 text-sm text-muted-foreground">
          <Loader2 className="size-4 mr-2 animate-spin" />
          加载运行观测数据中...
        </CardContent>
      </Card>
    );
  }

  if (observabilityQuery.isError) {
    return (
      <Card className="border-dashed shadow-none">
        <CardContent className="py-6">
          <p className="text-sm text-destructive">{getErrorMessage(observabilityQuery.error)}</p>
        </CardContent>
      </Card>
    );
  }

  if (variant === 'compact') {
    return (
      <div className="space-y-6">
        <div className="grid gap-4 md:grid-cols-3">
          <SummaryStat
            label="运行诊断"
            value={
              activeHealthEvents.length
                ? `${activeHealthEvents.length} 个活动异常`
                : '运行稳定'
            }
            hint={
              latestHealthEvent
                ? `${getHealthEventLabel(latestHealthEvent)} · ${latestHealthEvent.message || '等待处理'}`
                : '当前没有活动中的健康事件'
            }
          />
          <SummaryStat
            label="系统核心"
            value={profile?.hostname || '—'}
            hint={profile ? `${profile.os_name || '—'} · ${profile.architecture || '—'}` : '—'}
          />
          <SummaryStat
            label="在线时长"
            value={formatUptime(profile?.uptime_seconds)}
            hint="来自节点系统画像上报"
          />
        </div>

        <div className="grid gap-6 xl:grid-cols-2">
          <Card className="border-dashed shadow-none">
            <CardHeader>
              <CardTitle className="text-base font-semibold">系统画像</CardTitle>
              <CardDescription>节点上报的主机与硬件信息</CardDescription>
            </CardHeader>
            <CardContent>
              {profile ? (
                <SystemProfileCard profile={profile} nodeName={nodeName} />
              ) : (
                <p className="text-sm text-muted-foreground">
                  节点已接入，但尚未上报完整系统画像。
                </p>
              )}
            </CardContent>
          </Card>

          <Card className="border-dashed shadow-none">
            <CardHeader>
              <CardTitle className="text-base font-semibold">实时资源快照</CardTitle>
              <CardDescription>最近一次 metrics 上报</CardDescription>
            </CardHeader>
            <CardContent>
              {latestMetric ? (
                <div className="grid gap-3 sm:grid-cols-2">
                  <MetricBar
                    label="CPU"
                    value={formatPercent(latestMetric.cpu_usage_percent)}
                    progress={latestMetric.cpu_usage_percent}
                    hint={
                      isMeaningfulTime(latestMetric.captured_at)
                        ? `快照 ${formatRelativeTime(latestMetric.captured_at)}`
                        : undefined
                    }
                  />
                  <MetricBar
                    label="内存"
                    value={`${formatBytes(latestMetric.memory_used_bytes)} / ${formatBytes(latestMetric.memory_total_bytes)}`}
                    progress={memoryUsageRatio}
                  />
                  <MetricBar
                    label="存储"
                    value={`${formatBytes(latestMetric.storage_used_bytes)} / ${formatBytes(latestMetric.storage_total_bytes)}`}
                    progress={storageUsageRatio}
                  />
                  <MetricBar
                    label="连接数"
                    value={
                      latestMetric.openresty_connections
                        ? String(latestMetric.openresty_connections)
                        : '—'
                    }
                    progress={null}
                    hint={connectionHint}
                  />
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">节点已接入，但尚未上报资源快照。</p>
              )}
            </CardContent>
          </Card>
        </div>

        <HealthEventTimeline
          events={filteredHealthEvents}
          allEventsCount={observability?.health_events.length ?? 0}
          healthEventFilter={healthEventFilter}
          onFilterChange={setHealthEventFilter}
          onCleanup={() => setCleanupOpen(true)}
          cleanupPending={cleanupMutation.isPending}
        />

        <AlertDialog open={cleanupOpen} onOpenChange={setCleanupOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>清理健康事件日志</AlertDialogTitle>
              <AlertDialogDescription>
                该操作会删除此节点在控制端记录的所有健康诊断事件历史，不会影响后续新事件上报。
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={cleanupMutation.isPending}>取消</AlertDialogCancel>
              <AlertDialogAction
                className="bg-destructive text-white hover:bg-destructive/90"
                disabled={cleanupMutation.isPending}
                onClick={() => cleanupMutation.mutate()}
              >
                {cleanupMutation.isPending ? '清理中...' : '确认清理'}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <SummaryStat
          label="运行诊断"
          value={
            activeHealthEvents.length
              ? `${activeHealthEvents.length} 个活动异常`
              : '运行稳定'
          }
          hint={
            latestHealthEvent
              ? `${getHealthEventLabel(latestHealthEvent)} · ${latestHealthEvent.message || '等待处理'}`
              : '当前没有活动中的健康事件'
          }
        />
        <SummaryStat
          label="当前窗口请求"
          value={trafficSummary ? trafficSummary.request_count.toLocaleString('zh-CN') : '—'}
          hint={
            trafficSummary
              ? `QPS ${trafficSummary.estimated_qps.toFixed(1)} · 错误率 ${trafficSummary.error_rate_percent.toFixed(1)}%`
              : '当前没有可展示的请求窗口摘要'
          }
        />
        <SummaryStat
          label="容量压力"
          value={healthSummary?.has_capacity_risk ? '需要关注' : '正常范围'}
          hint={
            latestMetric
              ? `CPU ${formatPercent(latestMetric.cpu_usage_percent)} · 存储 ${formatPercent(storageUsageRatio)}`
              : '当前没有资源快照'
          }
        />
        <SummaryStat
          label="来源信号"
          value={topSourceCountry?.key ?? '—'}
          hint={
            topSourceCountry
              ? `${topSourceCountry.value.toLocaleString('zh-CN')} 次请求`
              : '当前没有来源分布数据'
          }
        />
      </div>

      <div className="grid gap-6 xl:grid-cols-3">
        <Card className="border-dashed shadow-none">
          <CardHeader>
            <CardTitle className="text-base font-semibold">系统信息</CardTitle>
          </CardHeader>
          <CardContent>
            {profile ? (
              <SystemProfileCard profile={profile} nodeName={nodeName} />
            ) : (
              <p className="text-sm text-muted-foreground">
                节点已经接入，但还没有上报完整系统画像。
              </p>
            )}
          </CardContent>
        </Card>

        <Card className="border-dashed shadow-none">
          <CardHeader>
            <CardTitle className="text-base font-semibold">实时资源</CardTitle>
          </CardHeader>
          <CardContent>
            {latestMetric ? (
              <div className="grid gap-3 sm:grid-cols-2">
                <MetricBar
                  label="CPU"
                  value={formatPercent(latestMetric.cpu_usage_percent)}
                  progress={latestMetric.cpu_usage_percent}
                  hint={
                    isMeaningfulTime(latestMetric.captured_at)
                      ? `快照 ${formatRelativeTime(latestMetric.captured_at)}`
                      : undefined
                  }
                />
                <MetricBar
                  label="内存"
                  value={`${formatBytes(latestMetric.memory_used_bytes)} / ${formatBytes(latestMetric.memory_total_bytes)}`}
                  progress={memoryUsageRatio}
                />
                <MetricBar
                  label="存储"
                  value={`${formatBytes(latestMetric.storage_used_bytes)} / ${formatBytes(latestMetric.storage_total_bytes)}`}
                  progress={storageUsageRatio}
                />
                <MetricBar
                  label="连接数"
                  value={
                    latestMetric.openresty_connections
                      ? String(latestMetric.openresty_connections)
                      : '—'
                  }
                  progress={null}
                  hint={connectionHint}
                />
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">节点已经接入，但还没有上报资源快照。</p>
            )}
          </CardContent>
        </Card>

        <Card className="border-dashed shadow-none">
          <CardHeader>
            <CardTitle className="text-base font-semibold">网络流量</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {latestMetric ? (
              <>
                {node ? (
                  <div className="flex flex-wrap gap-2">
                    <NodeStatusBadge
                      label={getNodeStatusLabel(node.status)}
                      tone={getNodeStatusTone(node.status)}
                    />
                    <NodeStatusBadge
                      label={getOpenrestyStatusLabel(node.openresty_status)}
                      tone={getOpenrestyStatusTone(node.openresty_status)}
                    />
                    <NodeStatusBadge
                      label={
                        activeHealthEvents.length
                          ? `${activeHealthEvents.length} 个活动异常`
                          : '无活动异常'
                      }
                      tone={activeHealthEvents.length ? 'warning' : 'success'}
                    />
                  </div>
                ) : null}

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-lg border px-3 py-3">
                    <p className="text-xs text-muted-foreground uppercase tracking-wide">
                      OpenResty 吞吐
                    </p>
                    <div className="mt-3 space-y-2 text-sm text-muted-foreground">
                      <p>入站：{formatBytesPerSecond(latestMetric.openresty_rx_bytes, 60)}</p>
                      <p>出站：{formatBytesPerSecond(latestMetric.openresty_tx_bytes, 60)}</p>
                    </div>
                  </div>
                  <div className="rounded-lg border px-3 py-3">
                    <p className="text-xs text-muted-foreground uppercase tracking-wide">
                      节点网络
                    </p>
                    <div className="mt-3 space-y-2 text-sm text-muted-foreground">
                      <p>入站：{formatBytes(latestMetric.network_rx_bytes)}</p>
                      <p>出站：{formatBytes(latestMetric.network_tx_bytes)}</p>
                    </div>
                  </div>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-lg border px-3 py-3">
                    <p className="text-xs text-muted-foreground uppercase tracking-wide">
                      最近窗口请求
                    </p>
                    <p className="mt-3 text-2xl font-semibold">
                      {trafficSummary
                        ? trafficSummary.request_count.toLocaleString('zh-CN')
                        : '—'}
                    </p>
                    <p className="mt-2 text-sm text-muted-foreground">
                      {trafficSummary
                        ? `QPS ${trafficSummary.estimated_qps.toFixed(1)} · UV ${trafficSummary.unique_visitor_count}`
                        : '暂无窗口流量摘要'}
                    </p>
                  </div>
                  <div className="rounded-lg border px-3 py-3">
                    <p className="text-xs text-muted-foreground uppercase tracking-wide">
                      最近窗口错误
                    </p>
                    <p className="mt-3 text-2xl font-semibold">
                      {trafficSummary ? trafficSummary.error_count.toLocaleString('zh-CN') : '—'}
                    </p>
                    <p className="mt-2 text-sm text-muted-foreground">
                      {trafficSummary
                        ? `错误率 ${trafficSummary.error_rate_percent.toFixed(1)}%`
                        : '暂无错误率摘要'}
                    </p>
                  </div>
                </div>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">
                节点已经接入，但还没有上报网络流量相关快照。
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <TrafficTrendChart
          points={trends?.traffic_24h ?? []}
          description="按小时聚合该节点的请求量和错误量。"
        />
        <CapacityTrendChart
          points={trends?.capacity_24h ?? []}
          description="观察该节点 CPU 与内存使用率在 24 小时内的变化。"
        />
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <NetworkTrendChart points={trends?.network_24h ?? []} />
        <DiskIOTrendChart points={trends?.disk_io_24h ?? []} />
      </div>

      <div className="grid gap-6 xl:grid-cols-[0.95fr_1.05fr]">
        <Card className="border-dashed shadow-none">
          <CardHeader>
            <CardTitle className="text-base font-semibold">请求结构分布</CardTitle>
            <CardDescription>
              聚合最近 24 小时窗口上报，帮助判断错误集中在哪些状态码、流量集中在哪些域名。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="mb-6 grid gap-4 md:grid-cols-3">
              <div className="rounded-lg border px-4 py-4">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">主状态码</p>
                <p className="mt-3 text-2xl font-semibold">{dominantStatusCode?.label ?? '—'}</p>
                <p className="mt-2 text-sm text-muted-foreground">
                  {dominantStatusCode ? `${dominantStatusCode.value} 次` : '暂无状态码分布'}
                </p>
              </div>
              <div className="rounded-lg border px-4 py-4">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Top Domain</p>
                <p className="mt-3 truncate text-2xl font-semibold">
                  {dominantDomain?.label ?? '—'}
                </p>
                <p className="mt-2 text-sm text-muted-foreground">
                  {dominantDomain ? `${dominantDomain.value} 次` : '暂无域名分布'}
                </p>
              </div>
              <div className="rounded-lg border px-4 py-4">
                <p className="text-xs text-muted-foreground uppercase tracking-wide">已恢复事件</p>
                <p className="mt-3 text-2xl font-semibold">{resolvedHealthEvents.length}</p>
                <p className="mt-2 text-sm text-muted-foreground">最近 24 小时已恢复健康事件</p>
              </div>
            </div>

            <div className="grid gap-6 xl:grid-cols-2">
              <div>
                <p className="mb-4 text-xs text-muted-foreground uppercase tracking-wide">
                  状态码分布
                </p>
                <DistributionList
                  items={statusCodeDistribution}
                  emptyMessage="暂无状态码分布"
                />
              </div>
              <div>
                <p className="mb-4 text-xs text-muted-foreground uppercase tracking-wide">
                  Top Domain
                </p>
                <DistributionList items={topDomains} emptyMessage="暂无域名分布" />
              </div>
            </div>
          </CardContent>
        </Card>

        <HealthEventTimeline
          events={filteredHealthEvents}
          allEventsCount={observability?.health_events.length ?? 0}
          healthEventFilter={healthEventFilter}
          onFilterChange={setHealthEventFilter}
          onCleanup={() => setCleanupOpen(true)}
          cleanupPending={cleanupMutation.isPending}
        />
      </div>

      <AlertDialog open={cleanupOpen} onOpenChange={setCleanupOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>清理健康事件日志</AlertDialogTitle>
            <AlertDialogDescription>
              该操作会删除此节点在控制端记录的所有健康诊断事件历史，不会影响后续新事件上报。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cleanupMutation.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              disabled={cleanupMutation.isPending}
              onClick={() => cleanupMutation.mutate()}
            >
              {cleanupMutation.isPending ? '清理中...' : '确认清理'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}