'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppModal } from '@/components/ui/app-modal';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { getConfigVersions } from '@/features/config-versions/api/config-versions';
import { ConfigVersionSnapshotModal } from '@/features/config-versions/components/config-version-snapshot-modal';
import type { ConfigVersionSummary } from '@/features/config-versions/types';
import { getApplyLogs } from '@/features/apply-logs/api/apply-logs';
import {
  cleanupNodeHealthEvents,
  deleteNode,
  getNodeAgentRelease,
  getNodeObservability,
  requestNodeForceSync,
  requestNodeAgentUpdate,
  updateNode,
} from '@/features/nodes/api/nodes';
import { NodeEditorModal } from '@/features/nodes/components/node-editor-modal';
import type { NodeItem, NodeAgentReleaseInfo } from '@/features/nodes/types';
import {
  CodeBlock,
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import type { ReleaseChannel } from '@/features/update/types';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';
import { formatBytes, formatPercent } from '@/lib/utils/metrics';
import {
  buildTunnelInstallCommand,
  buildTunnelDockerInstallCommand,
  getApplyLabel,
  getApplyVariant,
  getNodeStatusLabel,
  getServerUrl,
  getUpdateMode,
  isMeaningfulTime,
} from '@/features/nodes/utils';
import {
  copyToClipboard,
  formatUsageRatio,
  formatUptime,
  getErrorMessage,
  getHealthEventLabel,
  getHealthEventVariant,
  FeedbackState,
  HealthEventFilter,
  NodeDetailTab,
  MetricBar,
  SummaryStat,
} from './node-shared';

export function TunnelDetailPage({ node }: { node: NodeItem }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const nodeId = String(node.id);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [isAgentUpdateModalOpen, setIsAgentUpdateModalOpen] = useState(false);
  const [isTargetSnapshotOpen, setIsTargetSnapshotOpen] = useState(false);
  const [selectedReleaseChannel, setSelectedReleaseChannel] =
    useState<ReleaseChannel>('stable');
  const [agentUpdateFeedback, setAgentUpdateFeedback] =
    useState<FeedbackState | null>(null);
  const [serverUrl, setServerUrl] = useState('');
  const [healthEventFilter, setHealthEventFilter] =
    useState<HealthEventFilter>('all');
  const [activeTab, setActiveTab] = useState<NodeDetailTab>('dashboard');
  const [isHealthEventCleanupModalOpen, setHealthEventCleanupModalOpen] =
    useState(false);

  const stableAgentReleaseQuery = useQuery({
    queryKey: ['node-agent-release', nodeId, 'stable'],
    queryFn: () => getNodeAgentRelease(Number(nodeId), 'stable'),
    enabled: false,
  });

  const previewAgentReleaseQuery = useQuery({
    queryKey: ['node-agent-release', nodeId, 'preview'],
    queryFn: () => getNodeAgentRelease(Number(nodeId), 'preview'),
    enabled: false,
  });

  const applyLogsQuery = useQuery({
    queryKey: ['apply-logs', node.node_id, 1, 10],
    queryFn: () =>
      getApplyLogs({
        node_id: node.node_id,
        pageNo: 1,
        pageSize: 10,
      }),
    refetchInterval: 5000,
  });

  const configVersionsQuery = useQuery({
    queryKey: ['config-versions'],
    queryFn: getConfigVersions,
    refetchInterval: 5000,
  });

  const observabilityQuery = useQuery({
    queryKey: ['node-observability', nodeId],
    queryFn: () =>
      getNodeObservability(Number(nodeId), { hours: 24, limit: 48 }),
    refetchInterval: 10000,
  });

  useEffect(() => {
    if (typeof window !== 'undefined' && !serverUrl) {
      setServerUrl(window.location.origin);
    }
  }, [serverUrl]);

  const saveMutation = useMutation({
    mutationFn: async (payload: Parameters<typeof updateNode>[1]) =>
      updateNode(Number(nodeId), payload),
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '节点已更新。' });
      setIsEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: ['nodes'] });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const updateAgentMutation = useMutation({
    mutationFn: (release: NodeAgentReleaseInfo | null) =>
      requestNodeAgentUpdate(Number(nodeId), {
        channel: release?.channel ?? selectedReleaseChannel,
        tag_name:
          release?.channel === 'preview'
            ? release.tag_name || undefined
            : undefined,
      }),
    onSuccess: async (updatedNode) => {
      setFeedback({
        tone: 'success',
        message: `已向节点 ${updatedNode.name} 下发${updatedNode.update_channel === 'preview' ? '预览版' : '正式版'}更新指令。`,
      });
      setAgentUpdateFeedback({
        tone: 'success',
        message: `节点将在下一次心跳后执行${updatedNode.update_channel === 'preview' ? '预览版' : '正式版'} Agent 更新。`,
      });
      await queryClient.invalidateQueries({ queryKey: ['nodes'] });
    },
    onError: (error) => {
      const message = getErrorMessage(error);
      setFeedback({ tone: 'danger', message });
      setAgentUpdateFeedback({ tone: 'danger', message });
    },
  });

  const forceSyncMutation = useMutation({
    mutationFn: () => requestNodeForceSync(Number(nodeId)),
    onSuccess: async (updatedNode) => {
      setFeedback({
        tone: 'success',
        message: `已向节点 ${updatedNode.name} 下发强制同步指令，无视当前错误拦截。`,
      });
      await queryClient.invalidateQueries({ queryKey: ['nodes'] });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteNode(Number(nodeId)),
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '节点已删除。' });
      await queryClient.invalidateQueries({ queryKey: ['nodes'] });
      router.push('/node');
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const cleanupHealthEventsMutation = useMutation({
    mutationFn: () => cleanupNodeHealthEvents(Number(nodeId)),
    onSuccess: async (result) => {
      setFeedback({
        tone: 'success',
        message:
          result.deleted_count > 0
            ? `已清理 ${result.deleted_count} 条健康事件日志。`
            : '当前没有可清理的健康事件日志。',
      });
      setHealthEventCleanupModalOpen(false);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['node-observability', nodeId],
        }),
        queryClient.invalidateQueries({ queryKey: ['dashboard', 'overview'] }),
      ]);
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const handleDelete = () => {
    if (
      !window.confirm(
        `确认删除节点“${node.name}”吗？删除后该节点需要重新创建并重新接入。`,
      )
    ) {
      return;
    }
    setFeedback(null);
    deleteMutation.mutate();
  };

  const handleCopy = async (value: string, message: string) => {
    try {
      await copyToClipboard(value);
      setFeedback({ tone: 'success', message });
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  const activeConfigVersion = useMemo<ConfigVersionSummary | null>(() => {
    return (
      (configVersionsQuery.data ?? []).find((item) => item.is_active) ?? null
    );
  }, [configVersionsQuery.data]);

  const observability = observabilityQuery.data ?? null;
  const latestMetricSnapshot = observability?.metric_snapshots?.[0] ?? null;
  const activeHealthEvents = useMemo(
    () =>
      observability?.health_events.filter(
        (event) => event.status === 'active',
      ) ?? [],
    [observability?.health_events],
  );
  const resolvedHealthEvents = useMemo(
    () =>
      observability?.health_events.filter(
        (event) => event.status === 'resolved',
      ) ?? [],
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
  }, [
    activeHealthEvents,
    healthEventFilter,
    observability?.health_events,
    resolvedHealthEvents,
  ]);

  const tabs = useMemo(
    () =>
      [
        {
          key: 'dashboard' as const,
          label: '数据看板',
          description: '系统画像、资源快照与健康事件。',
        },
        {
          key: 'info' as const,
          label: '节点信息',
          description: '更新模式、版本状态、部署信息与应用记录。',
        },
      ] satisfies Array<{
        key: NodeDetailTab;
        label: string;
        description: string;
      }>,
    [],
  );

  const normalizedServerUrl = getServerUrl(serverUrl);
  const tunnelInstallCommand =
    normalizedServerUrl && node.access_token
      ? buildTunnelInstallCommand(normalizedServerUrl, node.access_token)
      : '';
  const tunnelDockerInstallCommand =
    normalizedServerUrl && node.access_token
      ? buildTunnelDockerInstallCommand(normalizedServerUrl, node.access_token)
      : '';

  const updateMode = getUpdateMode(node);
  const selectedAgentRelease =
    selectedReleaseChannel === 'preview'
      ? previewAgentReleaseQuery.data
      : stableAgentReleaseQuery.data;
  const selectedAgentReleaseError =
    selectedReleaseChannel === 'preview'
      ? previewAgentReleaseQuery.error
      : stableAgentReleaseQuery.error;
  const isCheckingAgentRelease =
    selectedReleaseChannel === 'preview'
      ? previewAgentReleaseQuery.isFetching
      : stableAgentReleaseQuery.isFetching;

  const applyLogs = applyLogsQuery.data?.rows ?? [];
  const latestHealthEvent = activeHealthEvents[0] ?? null;
  const memoryUsageRatio = formatUsageRatio(
    latestMetricSnapshot?.memory_used_bytes,
    latestMetricSnapshot?.memory_total_bytes,
  );
  const storageUsageRatio = formatUsageRatio(
    latestMetricSnapshot?.storage_used_bytes,
    latestMetricSnapshot?.storage_total_bytes,
  );
  const isTargetVersionApplied =
    activeConfigVersion !== null &&
    activeConfigVersion.version === node.current_version;

  const handleOpenAgentUpdateModal = () => {
    setAgentUpdateFeedback(null);
    setSelectedReleaseChannel('stable');
    setIsAgentUpdateModalOpen(true);
    void stableAgentReleaseQuery.refetch();
  };

  const handleCheckStableAgentRelease = () => {
    setAgentUpdateFeedback(null);
    setSelectedReleaseChannel('stable');
    void stableAgentReleaseQuery.refetch();
  };

  const handleCheckPreviewAgentRelease = () => {
    setAgentUpdateFeedback(null);
    setSelectedReleaseChannel('preview');
    void previewAgentReleaseQuery.refetch();
  };

  const handleRequestAgentUpdate = () => {
    updateAgentMutation.mutate(selectedAgentRelease ?? null);
  };

  const isRefreshing =
    applyLogsQuery.isFetching ||
    observabilityQuery.isFetching ||
    configVersionsQuery.isFetching;

  const handleRefresh = async () => {
    setFeedback(null);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['nodes'] }),
      queryClient.invalidateQueries({
        queryKey: ['apply-logs', node.node_id],
      }),
      queryClient.invalidateQueries({
        queryKey: ['config-versions'],
      }),
      queryClient.invalidateQueries({
        queryKey: ['node-observability', nodeId],
      }),
    ]);
  };

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title={node.name}
          description="隧道客户端节点详情 (openflared)"
          action={
            <>
              <Link
                href="/node"
                className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
              >
                返回
              </Link>
              <SecondaryButton
                type="button"
                onClick={() => setIsEditorOpen(true)}
              >
                编辑节点
              </SecondaryButton>
              <SecondaryButton
                type="button"
                onClick={() => void handleRefresh()}
                disabled={isRefreshing}
              >
                {isRefreshing ? '刷新中...' : '刷新'}
              </SecondaryButton>
              <SecondaryButton
                type="button"
                onClick={() => {
                  setFeedback(null);
                  forceSyncMutation.mutate();
                }}
                disabled={forceSyncMutation.isPending}
              >
                {forceSyncMutation.isPending ? '同步中...' : '同步'}
              </SecondaryButton>
              <PrimaryButton
                type="button"
                onClick={handleOpenAgentUpdateModal}
                disabled={updateAgentMutation.isPending}
              >
                {node.update_requested ? '查看升级' : '升级'}
              </PrimaryButton>
              <DangerButton
                type="button"
                onClick={handleDelete}
                disabled={deleteMutation.isPending}
              >
                删除
              </DangerButton>
            </>
          }
        />

        {feedback ? (
          <InlineMessage
            tone={feedback.tone}
            message={feedback.message}
            onClear={() => setFeedback(null)}
          />
        ) : null}

        <div className="flex flex-wrap gap-3">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              type="button"
              onClick={() => setActiveTab(tab.key)}
              className={[
                'rounded-2xl border px-4 py-3 text-left transition',
                activeTab === tab.key
                  ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                  : 'border-[var(--border-default)] bg-[var(--surface-muted)] text-[var(--foreground-secondary)] hover:border-[var(--border-strong)] hover:text-[var(--foreground-primary)]',
              ].join(' ')}
            >
              <p className="text-sm font-semibold">{tab.label}</p>
              <p className="mt-1 text-xs leading-5 text-inherit/80">
                {tab.description}
              </p>
            </button>
          ))}
        </div>

        {activeTab === 'dashboard' ? (
          <>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
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
                label="当前状态"
                value={getNodeStatusLabel(node.status)}
                hint={`最后心跳上报: ${node.last_seen_at ? formatRelativeTime(node.last_seen_at) : '—'}`}
              />
              <SummaryStat
                label="系统核心"
                value={observability?.profile?.hostname || node.name}
                hint={`${observability?.profile?.os_name || '—'} · ${observability?.profile?.architecture || '—'}`}
              />
            </div>

            <div className="grid gap-6 xl:grid-cols-3">
              <AppCard title="系统画像">
                {observabilityQuery.isLoading ? (
                  <LoadingState />
                ) : observabilityQuery.isError ? (
                  <InlineMessage
                    tone="danger"
                    message={getErrorMessage(observabilityQuery.error)}
                  />
                ) : observability?.profile ? (
                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-4 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          主机名
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {observability.profile.hostname || node.name}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          操作系统
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {observability.profile.os_name || 'unknown'}
                          {observability.profile.os_version
                            ? ` ${observability.profile.os_version}`
                            : ''}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          内核 / 架构
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {observability.profile.kernel_version || 'unknown'} ·{' '}
                          {observability.profile.architecture || 'unknown'}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          在线时长
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {formatUptime(observability.profile.uptime_seconds)}
                        </p>
                      </div>
                    </div>

                    <div className="space-y-4 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          CPU
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {observability.profile.cpu_model || 'unknown'}
                        </p>
                        <p className="mt-1 text-xs text-[var(--foreground-muted)]">
                          {observability.profile.cpu_cores || 0} 核
                        </p>
                      </div>
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          总内存
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {formatBytes(
                            observability.profile.total_memory_bytes,
                          )}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          总存储
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {formatBytes(observability.profile.total_disk_bytes)}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                          上报时间
                        </p>
                        <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                          {isMeaningfulTime(observability.profile.reported_at)
                            ? formatDateTime(observability.profile.reported_at)
                            : '—'}
                        </p>
                      </div>
                    </div>
                  </div>
                ) : (
                  <EmptyState
                    title="暂无系统画像"
                    description="节点已经接入，但还没有上报完整系统画像。"
                  />
                )}
              </AppCard>

              <AppCard title="实时资源快照">
                {observabilityQuery.isLoading ? (
                  <LoadingState />
                ) : observabilityQuery.isError ? (
                  <InlineMessage
                    tone="danger"
                    message={getErrorMessage(observabilityQuery.error)}
                  />
                ) : latestMetricSnapshot ? (
                  <div className="space-y-4">
                    <div className="grid gap-4 md:grid-cols-2">
                      <MetricBar
                        label="CPU"
                        value={formatPercent(
                          latestMetricSnapshot.cpu_usage_percent,
                        )}
                        progress={latestMetricSnapshot.cpu_usage_percent}
                        hint={
                          isMeaningfulTime(latestMetricSnapshot.captured_at)
                            ? `快照 ${formatRelativeTime(latestMetricSnapshot.captured_at)}`
                            : undefined
                        }
                      />
                      <MetricBar
                        label="内存"
                        value={`${formatBytes(
                          latestMetricSnapshot.memory_used_bytes,
                        )} / ${formatBytes(latestMetricSnapshot.memory_total_bytes)}`}
                        progress={memoryUsageRatio}
                      />
                      <MetricBar
                        label="存储"
                        value={`${formatBytes(
                          latestMetricSnapshot.storage_used_bytes,
                        )} / ${formatBytes(latestMetricSnapshot.storage_total_bytes)}`}
                        progress={storageUsageRatio}
                      />
                      <MetricBar
                        label="活动连接"
                        value={
                          latestMetricSnapshot.openresty_connections
                            ? `${latestMetricSnapshot.openresty_connections}`
                            : '—'
                        }
                        progress={null}
                        hint="隧道并发连接数"
                      />
                    </div>
                  </div>
                ) : (
                  <EmptyState
                    title="暂无资源快照"
                    description="节点已经接入，但还没有上报资源快照。"
                  />
                )}
              </AppCard>

              <AppCard
                title="健康事件时间线"
                description="保留活动与已恢复事件，用于运行异常诊断。"
                action={
                  <DangerButton
                    type="button"
                    disabled={
                      cleanupHealthEventsMutation.isPending ||
                      !observability?.health_events.length
                    }
                    onClick={() => setHealthEventCleanupModalOpen(true)}
                  >
                    {cleanupHealthEventsMutation.isPending
                      ? '清理中...'
                      : '清理日志'}
                  </DangerButton>
                }
              >
                {observability?.health_events.length ? (
                  <div className="space-y-4">
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => setHealthEventFilter('all')}
                        className={`inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
                          healthEventFilter === 'all'
                            ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                            : 'border-[var(--border-default)] text-[var(--foreground-secondary)] hover:bg-[var(--control-background-hover)]'
                        }`}
                      >
                        全部
                      </button>
                      <button
                        type="button"
                        onClick={() => setHealthEventFilter('active')}
                        className={`inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
                          healthEventFilter === 'active'
                            ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                            : 'border-[var(--border-default)] text-[var(--foreground-secondary)] hover:bg-[var(--control-background-hover)]'
                        }`}
                      >
                        活动中
                      </button>
                      <button
                        type="button"
                        onClick={() => setHealthEventFilter('resolved')}
                        className={`inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
                          healthEventFilter === 'resolved'
                            ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                            : 'border-[var(--border-default)] text-[var(--foreground-secondary)] hover:bg-[var(--control-background-hover)]'
                        }`}
                      >
                        已恢复
                      </button>
                    </div>

                    {filteredHealthEvents.slice(0, 8).map((event) => (
                      <div
                        key={`${event.event_type}-${event.last_triggered_at}-${event.status}`}
                        className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4"
                      >
                        <div className="flex flex-wrap items-center gap-2">
                          <StatusBadge
                            label={getHealthEventLabel(event)}
                            variant={getHealthEventVariant(event)}
                          />
                          <StatusBadge
                            label={
                              event.status === 'active' ? '活动中' : '已恢复'
                            }
                            variant={
                              event.status === 'active' ? 'warning' : 'success'
                            }
                          />
                        </div>
                        <p className="mt-3 text-sm text-[var(--foreground-secondary)]">
                          {event.message || '暂无详细消息'}
                        </p>
                        <div className="mt-3 grid gap-2 text-xs text-[var(--foreground-muted)] md:grid-cols-3">
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
                            {isMeaningfulTime(event.resolved_at)
                              ? ` ${formatDateTime(event.resolved_at)}`
                              : ' —'}
                          </p>
                        </div>
                      </div>
                    ))}
                    {filteredHealthEvents.length === 0 ? (
                      <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 text-sm text-[var(--foreground-secondary)]">
                        当前筛选下没有健康事件。
                      </div>
                    ) : null}
                  </div>
                ) : (
                  <EmptyState
                    title="暂无健康日志"
                    description="节点当前还没有上报健康事件记录。"
                  />
                )}
              </AppCard>
            </div>
          </>
        ) : null}

        {activeTab === 'info' ? (
          <>
            <div className="grid gap-4 xl:grid-cols-3">
              <AppCard title="更新模式">
                <div className="space-y-3">
                  <StatusBadge
                    label={updateMode.label}
                    variant={updateMode.variant}
                  />
                  <p className="text-sm text-[var(--foreground-secondary)]">
                    {node.update_requested
                      ? `已等待节点在下一次心跳后执行${node.update_channel === 'preview' ? '预览版' : '正式版'}更新。`
                      : node.auto_update_enabled
                        ? '节点已启用正式版自动更新。'
                        : '当前仅支持手动触发更新。'}
                  </p>
                </div>
              </AppCard>

              <AppCard title="版本与状态">
                <div className="space-y-2 text-sm text-[var(--foreground-secondary)]">
                  <p>Client (openflared): {node.version || 'unknown'}</p>
                  <p>当前配置版本：{node.current_version || '未应用'}</p>
                  <p>节点类型：隧道客户端 (tunnel_client)</p>
                </div>
              </AppCard>

              <AppCard title="最近应用配置">
                <div className="space-y-3">
                  <StatusBadge
                    label={getApplyLabel(node.latest_apply_result)}
                    variant={getApplyVariant(node.latest_apply_result)}
                  />
                  <p className="text-sm text-[var(--foreground-secondary)]">
                    {isMeaningfulTime(node.latest_apply_at)
                      ? `${formatRelativeTime(
                          node.latest_apply_at,
                        )} · ${formatDateTime(node.latest_apply_at)}`
                      : '暂无应用记录'}
                  </p>
                  {node.latest_apply_checksum ? (
                    <div className="space-y-1 text-sm text-[var(--foreground-secondary)]">
                      <p>同步文件：{node.latest_support_file_count} 个</p>
                    </div>
                  ) : null}
                </div>
              </AppCard>
            </div>

            <AppCard
              title="追平配置版本"
              description="当前全局激活配置版本追平状态。"
              action={
                activeConfigVersion ? (
                  <SecondaryButton
                    type="button"
                    onClick={() => setIsTargetSnapshotOpen(true)}
                  >
                    查看目标快照
                  </SecondaryButton>
                ) : null
              }
            >
              {configVersionsQuery.isLoading ? (
                <LoadingState />
              ) : configVersionsQuery.isError ? (
                <InlineMessage
                  tone="danger"
                  message={getErrorMessage(configVersionsQuery.error)}
                />
              ) : activeConfigVersion ? (
                <div className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
                  <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                    <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                      追平状态
                    </p>
                    <div className="mt-3 flex flex-wrap items-center gap-3">
                      <StatusBadge
                        label={isTargetVersionApplied ? '已追平' : '同步落后'}
                        variant={isTargetVersionApplied ? 'success' : 'warning'}
                      />
                    </div>
                    <p className="mt-3 text-sm text-[var(--foreground-secondary)]">
                      {isTargetVersionApplied
                        ? '当前节点已成功应用最新全局配置。'
                        : '当前节点版本落后于全局激活配置，请检查最新应用记录排查问题。'}
                    </p>
                  </div>

                  <div className="grid gap-4 md:grid-cols-3">
                    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                      <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                        目标配置版本
                      </p>
                      <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                        {activeConfigVersion.version}
                      </p>
                    </div>
                    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                      <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                        配置校验和
                      </p>
                      <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                        {activeConfigVersion.checksum}
                      </p>
                    </div>
                    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                      <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                        全局激活时间
                      </p>
                      <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                        {formatDateTime(activeConfigVersion.created_at)}
                      </p>
                    </div>
                  </div>
                </div>
              ) : (
                <InlineMessage
                  tone="info"
                  message="当前全局没有激活的配置版本。"
                />
              )}
            </AppCard>

            <div className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
              <AppCard
                title="节点部署指引"
                action={
                  <div className="flex gap-2">
                    {tunnelInstallCommand ? (
                      <PrimaryButton
                        type="button"
                        onClick={() =>
                          void handleCopy(
                            tunnelInstallCommand,
                            '部署脚本命令已复制。',
                          )
                        }
                      >
                        复制脚本命令
                      </PrimaryButton>
                    ) : null}
                    {tunnelDockerInstallCommand ? (
                      <PrimaryButton
                        type="button"
                        onClick={() =>
                          void handleCopy(
                            tunnelDockerInstallCommand,
                            'Docker 部署命令已复制。',
                          )
                        }
                      >
                        复制 Docker 命令
                      </PrimaryButton>
                    ) : null}
                  </div>
                }
              >
                <div className="space-y-4">
                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                      <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                        Node ID
                      </p>
                      <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                        {node.node_id}
                      </p>
                    </div>
                    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                      <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                        Tunnel Token
                      </p>
                      <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                        {node.access_token || '暂无'}
                      </p>
                    </div>
                  </div>

                  <ResourceField
                    label="Server URL (控制端地址)"
                    hint="部署脚本和容器以此地址接入 OpenFlare 控制端。"
                  >
                    <ResourceInput
                      value={serverUrl}
                      onChange={(event) => setServerUrl(event.target.value)}
                    />
                  </ResourceField>

                  {tunnelInstallCommand ? (
                    <div>
                      <p className="mb-2 text-sm font-medium text-[var(--foreground-primary)]">
                        一键脚本部署 (Linux / macOS)
                      </p>
                      <CodeBlock className="whitespace-pre-wrap">
                        {tunnelInstallCommand}
                      </CodeBlock>
                    </div>
                  ) : null}

                  {tunnelDockerInstallCommand ? (
                    <div>
                      <p className="mb-2 text-sm font-medium text-[var(--foreground-primary)]">
                        Docker 容器部署
                      </p>
                      <CodeBlock className="whitespace-pre-wrap">
                        {tunnelDockerInstallCommand}
                      </CodeBlock>
                    </div>
                  ) : null}
                </div>
              </AppCard>

              <AppCard title="运行网络与信息">
                <div className="space-y-4 text-sm text-[var(--foreground-secondary)]">
                  <div>
                    <p className="font-medium text-[var(--foreground-primary)]">
                      接入 IP 地址
                    </p>
                    <p className="mt-1">
                      {node.ip || '暂无上报'}
                      {node.ip_manual_override ? '（管理端锁定锁定）' : ''}
                    </p>
                  </div>
                  <div>
                    <p className="font-medium text-[var(--foreground-primary)]">
                      最近心跳错误
                    </p>
                    <p className="mt-1 break-words whitespace-pre-wrap">
                      {node.last_error || '正常'}
                    </p>
                  </div>
                  <div>
                    <p className="font-medium text-[var(--foreground-primary)]">
                      创建时间
                    </p>
                    <p className="mt-1">{formatDateTime(node.created_at)}</p>
                  </div>
                  <div>
                    <p className="font-medium text-[var(--foreground-primary)]">
                      更新时间
                    </p>
                    <p className="mt-1">{formatDateTime(node.updated_at)}</p>
                  </div>
                </div>
              </AppCard>
            </div>

            <AppCard
              title="配置应用历史"
              description="仅展示此节点的局部配置应用结果。"
              action={
                <Link
                  href={`/apply-log?node_id=${encodeURIComponent(node.node_id)}`}
                  className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
                >
                  查看完整记录
                </Link>
              }
            >
              {applyLogsQuery.isLoading ? (
                <LoadingState />
              ) : applyLogsQuery.isError ? (
                <ErrorState
                  title="应用记录加载失败"
                  description={getErrorMessage(applyLogsQuery.error)}
                />
              ) : applyLogs.length === 0 ? (
                <EmptyState
                  title="暂无应用记录"
                  description="当前节点还没有同步应用过任何配置。"
                />
              ) : (
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                    <thead>
                      <tr className="text-[var(--foreground-secondary)]">
                        <th className="px-3 py-3 font-medium">配置版本</th>
                        <th className="px-3 py-3 font-medium">结果</th>
                        <th className="px-3 py-3 font-medium">校验和</th>
                        <th className="px-3 py-3 font-medium">应用时间</th>
                        <th className="px-3 py-3 font-medium">日志消息</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--border-default)]">
                      {applyLogs.map((log) => (
                        <tr key={log.id} className="align-top">
                          <td className="px-3 py-4 text-[var(--foreground-primary)]">
                            {log.version}
                          </td>
                          <td className="px-3 py-4">
                            <StatusBadge
                              label={log.result === 'success' ? '成功' : '失败'}
                              variant={
                                log.result === 'success' ? 'success' : 'danger'
                              }
                            />
                          </td>
                          <td
                            className="px-3 py-4 text-[var(--foreground-secondary)]"
                            title={log.checksum}
                          >
                            {log.checksum
                              ? `${log.checksum.slice(0, 12)}...`
                              : '—'}
                          </td>
                          <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                            {formatRelativeTime(log.created_at)} ·{' '}
                            {formatDateTime(log.created_at)}
                          </td>
                          <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                            <div className="max-w-80 space-y-2 break-words whitespace-pre-wrap">
                              <p>{log.message || '—'}</p>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </AppCard>
          </>
        ) : null}
      </div>

      <ConfigVersionSnapshotModal
        version={isTargetSnapshotOpen ? activeConfigVersion : null}
        onClose={() => setIsTargetSnapshotOpen(false)}
      />

      <NodeEditorModal
        isOpen={isEditorOpen}
        node={node}
        isSubmitting={saveMutation.isPending}
        onClose={() => setIsEditorOpen(false)}
        title="编辑隧道客户端"
        description="可以修改节点名称、IP等标识信息。"
        submitLabel="保存修改"
        onSubmit={(payload) => {
          setFeedback(null);
          saveMutation.mutate(payload);
        }}
      />

      <AppModal
        isOpen={isHealthEventCleanupModalOpen}
        onClose={() => setHealthEventCleanupModalOpen(false)}
        title="清理运行诊断日志"
        description={`确认清理节点“${node.name}”的健康诊断事件吗？`}
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={() => setHealthEventCleanupModalOpen(false)}
            >
              取消
            </SecondaryButton>
            <DangerButton
              type="button"
              disabled={cleanupHealthEventsMutation.isPending}
              onClick={() => {
                setFeedback(null);
                cleanupHealthEventsMutation.mutate();
              }}
            >
              {cleanupHealthEventsMutation.isPending ? '清理中...' : '确认清理'}
            </DangerButton>
          </div>
        }
      >
        {cleanupHealthEventsMutation.isError ? (
          <ErrorState
            title="健康事件清理失败"
            description={getErrorMessage(cleanupHealthEventsMutation.error)}
          />
        ) : (
          <div className="space-y-3 text-sm text-[var(--foreground-secondary)]">
            <p>该操作会物理删除此节点在控制端记录的所有健康诊断事件历史。</p>
            <p>这不会影响节点在后续运行中继续捕获并上报新的故障。</p>
          </div>
        )}
      </AppModal>

      <AppModal
        isOpen={isAgentUpdateModalOpen}
        onClose={() => setIsAgentUpdateModalOpen(false)}
        title="客户端 Agent 升级"
        description="管理当前客户端 (flared/agent) 软件版本。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={handleCheckStableAgentRelease}
              disabled={isCheckingAgentRelease || updateAgentMutation.isPending}
            >
              {isCheckingAgentRelease && selectedReleaseChannel === 'stable'
                ? '检查中...'
                : '检查正式版'}
            </SecondaryButton>
            <SecondaryButton
              type="button"
              onClick={handleCheckPreviewAgentRelease}
              disabled={isCheckingAgentRelease || updateAgentMutation.isPending}
            >
              {isCheckingAgentRelease && selectedReleaseChannel === 'preview'
                ? '检查中...'
                : '检查预览版'}
            </SecondaryButton>
            <PrimaryButton
              type="button"
              onClick={handleRequestAgentUpdate}
              disabled={
                !selectedAgentRelease?.has_update ||
                updateAgentMutation.isPending ||
                isCheckingAgentRelease ||
                node.update_requested
              }
            >
              {updateAgentMutation.isPending
                ? '下发中...'
                : selectedReleaseChannel === 'preview'
                  ? '升级到预览版'
                  : '升级到正式版'}
            </PrimaryButton>
          </div>
        }
      >
        <div className="space-y-6">
          {agentUpdateFeedback ? (
            <InlineMessage
              tone={agentUpdateFeedback.tone}
              message={agentUpdateFeedback.message}
            />
          ) : null}

          <div className="grid gap-4 md:grid-cols-3">
            <AppCard title="当前版本">
              <p className="text-sm font-medium text-[var(--foreground-primary)]">
                {node.version || 'unknown'}
              </p>
            </AppCard>
            <AppCard title="检查通道">
              <div className="flex flex-wrap items-center gap-3">
                <p className="text-sm font-medium text-[var(--foreground-primary)]">
                  {selectedReleaseChannel === 'preview' ? '预览版' : '正式版'}
                </p>
                <StatusBadge
                  label={
                    selectedReleaseChannel === 'preview' ? 'Preview' : 'Stable'
                  }
                  variant={
                    selectedReleaseChannel === 'preview' ? 'warning' : 'info'
                  }
                />
              </div>
            </AppCard>
            <AppCard title="更新状态">
              <StatusBadge
                label={
                  node.update_requested
                    ? node.update_channel === 'preview'
                      ? '等待预览更新'
                      : '等待更新'
                    : '未下发'
                }
                variant={node.update_requested ? 'warning' : 'info'}
              />
            </AppCard>
          </div>

          {isCheckingAgentRelease && !selectedAgentRelease ? (
            <LoadingState />
          ) : null}
          {!isCheckingAgentRelease && selectedAgentReleaseError ? (
            <ErrorState
              title="版本检查失败"
              description={getErrorMessage(selectedAgentReleaseError)}
            />
          ) : null}
          {!isCheckingAgentRelease &&
          !selectedAgentReleaseError &&
          !selectedAgentRelease ? (
            <EmptyState
              title="尚未检查客户端更新"
              description="选择上面的通道进行客户端版本检查。"
            />
          ) : null}

          {selectedAgentRelease ? (
            <AppCard
              title={`GitHub ${selectedReleaseChannel === 'preview' ? '预览版' : '正式版'} · ${selectedAgentRelease.tag_name || '未找到版本'}`}
              description={
                selectedAgentRelease.published_at
                  ? `发布时间：${formatRelativeTime(selectedAgentRelease.published_at)}`
                  : '未提供发布时间'
              }
            >
              <div className="space-y-4">
                <div className="flex flex-wrap items-center gap-3">
                  <StatusBadge
                    label={
                      selectedAgentRelease.has_update
                        ? '发现可升级版本'
                        : '当前已是最新版本'
                    }
                    variant={
                      selectedAgentRelease.has_update ? 'warning' : 'success'
                    }
                  />
                  {selectedAgentRelease.prerelease ? (
                    <StatusBadge label="Preview 发布" variant="warning" />
                  ) : (
                    <StatusBadge label="正式发布" variant="info" />
                  )}
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div>
                    <p className="text-xs text-[var(--foreground-secondary)]">
                      当前版本
                    </p>
                    <p className="mt-1 text-sm font-medium text-[var(--foreground-primary)]">
                      {selectedAgentRelease.current_version || 'unknown'}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-[var(--foreground-secondary)]">
                      最新发布版本
                    </p>
                    <p className="mt-1 text-sm font-medium text-[var(--foreground-primary)]">
                      {selectedAgentRelease.tag_name || '未找到'}
                    </p>
                  </div>
                </div>

                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 text-sm leading-6 whitespace-pre-wrap text-[var(--foreground-secondary)]">
                  {selectedAgentRelease.body || '暂无更新说明'}
                </div>

                {selectedAgentRelease.html_url ? (
                  <a
                    href={selectedAgentRelease.html_url}
                    target="_blank"
                    rel="noreferrer"
                    className="text-sm font-medium text-[var(--brand-primary)] transition hover:opacity-80"
                  >
                    查看发布详情
                  </a>
                ) : null}
              </div>
            </AppCard>
          ) : null}
        </div>
      </AppModal>
    </>
  );
}
