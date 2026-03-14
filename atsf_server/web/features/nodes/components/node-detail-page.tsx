'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';

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
import type { ConfigVersionItem } from '@/features/config-versions/types';
import { getApplyLogs } from '@/features/apply-logs/api/apply-logs';
import {
  deleteNode,
  getNodeAgentRelease,
  getNodeObservability,
  getNodes,
  requestNodeOpenrestyRestart,
  requestNodeAgentUpdate,
  updateNode,
} from '@/features/nodes/api/nodes';
import type {
  NodeAgentReleaseInfo,
  NodeMutationPayload,
  NodeObservability,
} from '@/features/nodes/types';
import {
  CodeBlock,
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import type { ReleaseChannel } from '@/features/update/types';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';
import {
  buildNodeInstallCommand,
  getApplyLabel,
  getApplyVariant,
  getNodeStatusLabel,
  getNodeStatusVariant,
  getOpenrestyStatusLabel,
  getOpenrestyStatusVariant,
  getServerUrl,
  getUpdateMode,
  isMeaningfulTime,
} from '@/features/nodes/utils';

const nodesQueryKey = ['nodes'];

const nodeSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, '请输入节点名')
    .max(128, '节点名不能超过 128 个字符'),
  auto_update_enabled: z.boolean(),
});

type NodeFormValues = z.infer<typeof nodeSchema>;

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

const defaultValues: NodeFormValues = {
  name: '',
  auto_update_enabled: false,
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function toPayload(values: NodeFormValues): NodeMutationPayload {
  return {
    name: values.name.trim(),
    auto_update_enabled: values.auto_update_enabled,
  };
}

async function copyToClipboard(value: string) {
  await navigator.clipboard.writeText(value);
}

function formatBytes(value?: number | null) {
  if (!value || value <= 0) {
    return '—';
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let current = value;
  let index = 0;
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024;
    index += 1;
  }
  return `${current.toFixed(current >= 100 || index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatPercent(value?: number | null) {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '—';
  }
  return `${value.toFixed(1)}%`;
}

function formatUsageRatio(used?: number | null, total?: number | null) {
  if (!used || !total || total <= 0) {
    return null;
  }
  return Math.max(0, Math.min(100, (used / total) * 100));
}

function formatUptime(seconds?: number | null) {
  if (!seconds || seconds <= 0) {
    return '—';
  }

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (days > 0) {
    return `${days} 天 ${hours} 小时`;
  }
  if (hours > 0) {
    return `${hours} 小时 ${minutes} 分钟`;
  }
  return `${minutes} 分钟`;
}

function getHealthEventVariant(
  event: NodeObservability['health_events'][number],
): 'success' | 'warning' | 'danger' | 'info' {
  if (event.status === 'resolved') {
    return 'success';
  }
  if (event.severity === 'critical') {
    return 'danger';
  }
  if (event.severity === 'warning') {
    return 'warning';
  }
  return 'info';
}

function getHealthEventLabel(
  event: NodeObservability['health_events'][number],
) {
  return event.event_type.replaceAll('_', ' ');
}

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
    <div className="space-y-2 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
            {label}
          </p>
          {hint ? (
            <p className="mt-1 text-xs text-[var(--foreground-muted)]">
              {hint}
            </p>
          ) : null}
        </div>
        <p className="text-sm font-semibold text-[var(--foreground-primary)]">
          {value}
        </p>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-[var(--surface-muted)]">
        <div
          className="h-full rounded-full bg-[var(--status-info-foreground)] transition-[width]"
          style={{ width: `${progress ?? 0}%` }}
        />
      </div>
    </div>
  );
}

export function NodeDetailPage({ nodeId }: { nodeId: string }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [isAgentUpdateModalOpen, setIsAgentUpdateModalOpen] = useState(false);
  const [isTargetSnapshotOpen, setIsTargetSnapshotOpen] = useState(false);
  const [selectedReleaseChannel, setSelectedReleaseChannel] =
    useState<ReleaseChannel>('stable');
  const [agentUpdateFeedback, setAgentUpdateFeedback] =
    useState<FeedbackState | null>(null);
  const [serverUrl, setServerUrl] = useState('');

  const form = useForm<NodeFormValues>({
    resolver: zodResolver(nodeSchema),
    defaultValues,
  });

  const watchedAutoUpdate = useWatch({
    control: form.control,
    name: 'auto_update_enabled',
  });

  const nodesQuery = useQuery({
    queryKey: nodesQueryKey,
    queryFn: getNodes,
    refetchInterval: 5000,
  });

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

  const node = useMemo(() => {
    return (
      (nodesQuery.data ?? []).find((item) => String(item.id) === nodeId) ?? null
    );
  }, [nodeId, nodesQuery.data]);

  const applyLogsQuery = useQuery({
    queryKey: ['apply-logs', node?.node_id ?? ''],
    queryFn: () => getApplyLogs(node?.node_id),
    enabled: Boolean(node?.node_id),
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
    enabled: Boolean(nodeId),
    refetchInterval: 10000,
  });

  useEffect(() => {
    if (typeof window !== 'undefined' && !serverUrl) {
      setServerUrl(window.location.origin);
    }
  }, [serverUrl]);

  useEffect(() => {
    if (!node || isEditorOpen) {
      return;
    }

    form.reset({
      name: node.name,
      auto_update_enabled: node.auto_update_enabled,
    });
  }, [form, isEditorOpen, node]);

  const saveMutation = useMutation({
    mutationFn: async (values: NodeFormValues) =>
      updateNode(Number(nodeId), toPayload(values)),
    onSuccess: async (updatedNode) => {
      setFeedback({ tone: 'success', message: '节点已更新。' });
      setIsEditorOpen(false);
      form.reset({
        name: updatedNode.name,
        auto_update_enabled: updatedNode.auto_update_enabled,
      });
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
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
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      const message = getErrorMessage(error);
      setFeedback({ tone: 'danger', message });
      setAgentUpdateFeedback({ tone: 'danger', message });
    },
  });

  const restartOpenrestyMutation = useMutation({
    mutationFn: () => requestNodeOpenrestyRestart(Number(nodeId)),
    onSuccess: async (updatedNode) => {
      setFeedback({
        tone: 'success',
        message: `已向节点 ${updatedNode.name} 下发 OpenResty 重启指令。`,
      });
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteNode(Number(nodeId)),
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '节点已删除。' });
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
      router.push('/node');
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const handleSubmit = form.handleSubmit((values) => {
    setFeedback(null);
    saveMutation.mutate(values);
  });

  const handleDelete = () => {
    if (!node) {
      return;
    }

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

  const handleRestartOpenresty = () => {
    if (!node) {
      return;
    }

    if (
      !window.confirm(
        `确认向节点“${node.name}”下发 OpenResty 重启指令吗？该指令会在下一次心跳后执行。`,
      )
    ) {
      return;
    }

    setFeedback(null);
    restartOpenrestyMutation.mutate();
  };

  const handleCopy = async (value: string, message: string) => {
    try {
      await copyToClipboard(value);
      setFeedback({ tone: 'success', message });
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  const activeConfigVersion = useMemo<ConfigVersionItem | null>(() => {
    return (
      (configVersionsQuery.data ?? []).find((item) => item.is_active) ?? null
    );
  }, [configVersionsQuery.data]);

  if (nodesQuery.isLoading) {
    return <LoadingState />;
  }

  if (nodesQuery.isError) {
    return (
      <ErrorState
        title="节点详情加载失败"
        description={getErrorMessage(nodesQuery.error)}
      />
    );
  }

  if (!node) {
    return (
      <EmptyState
        title="节点不存在"
        description="该节点可能已被删除，或当前 ID 无法匹配到节点记录。"
      />
    );
  }

  const normalizedServerUrl = getServerUrl(serverUrl);
  const nodeInstallCommand =
    normalizedServerUrl && node.agent_token
      ? buildNodeInstallCommand(normalizedServerUrl, node.agent_token)
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
  const applyLogs = applyLogsQuery.data ?? [];
  const observability = observabilityQuery.data ?? null;
  const latestMetricSnapshot = observability?.metric_snapshots?.[0] ?? null;
  const latestTrafficReport = observability?.traffic_reports?.[0] ?? null;
  const activeHealthEvents =
    observability?.health_events.filter((event) => event.status === 'active') ??
    [];
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
    nodesQuery.isFetching ||
    applyLogsQuery.isFetching ||
    observabilityQuery.isFetching;

  const handleRefresh = async () => {
    setFeedback(null);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: nodesQueryKey }),
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
          description="节点详情"
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
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <div className="grid gap-6 xl:grid-cols-[1.1fr_1.1fr_0.8fr]">
          <AppCard
            title="系统画像"
            description="展示节点当前上报的主机事实信息，便于快速识别机器环境。"
          >
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
                      {formatBytes(observability.profile.total_memory_bytes)}
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

          <AppCard
            title="实时资源"
            description="读取节点最近一次快照，快速判断资源压力与吞吐情况。"
          >
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
                    )} / ${formatBytes(
                      latestMetricSnapshot.storage_total_bytes,
                    )}`}
                    progress={storageUsageRatio}
                  />
                  <MetricBar
                    label="连接数"
                    value={
                      latestMetricSnapshot.openresty_connections
                        ? `${latestMetricSnapshot.openresty_connections}`
                        : '—'
                    }
                    progress={null}
                    hint="OpenResty 当前连接"
                  />
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                    <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                      网络流量
                    </p>
                    <div className="mt-3 space-y-2 text-sm text-[var(--foreground-secondary)]">
                      <p>
                        入站：
                        {formatBytes(latestMetricSnapshot.network_rx_bytes)}
                      </p>
                      <p>
                        出站：
                        {formatBytes(latestMetricSnapshot.network_tx_bytes)}
                      </p>
                    </div>
                  </div>
                  <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                    <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                      磁盘 IO
                    </p>
                    <div className="mt-3 space-y-2 text-sm text-[var(--foreground-secondary)]">
                      <p>
                        读取：
                        {formatBytes(latestMetricSnapshot.disk_read_bytes)}
                      </p>
                      <p>
                        写入：
                        {formatBytes(latestMetricSnapshot.disk_write_bytes)}
                      </p>
                    </div>
                  </div>
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
            title="运行状态"
            description="汇总节点在线状态、OpenResty 健康与当前异常。"
          >
            <div className="space-y-4">
              <div className="flex flex-wrap gap-3">
                <StatusBadge
                  label={getNodeStatusLabel(node.status)}
                  variant={getNodeStatusVariant(node.status)}
                />
                <StatusBadge
                  label={getOpenrestyStatusLabel(node.openresty_status)}
                  variant={getOpenrestyStatusVariant(node.openresty_status)}
                />
                <StatusBadge
                  label={
                    activeHealthEvents.length
                      ? `${activeHealthEvents.length} 个活动异常`
                      : '无活动异常'
                  }
                  variant={activeHealthEvents.length ? 'warning' : 'success'}
                />
              </div>

              <div className="space-y-2 text-sm text-[var(--foreground-secondary)]">
                <p>
                  最近心跳：
                  {isMeaningfulTime(node.last_seen_at)
                    ? ` ${formatRelativeTime(node.last_seen_at)} · ${formatDateTime(node.last_seen_at)}`
                    : ' 暂无'}
                </p>
                <p>IP：{node.ip || '暂无'}</p>
                <p>当前配置：{node.current_version || '未应用'}</p>
                {latestTrafficReport ? (
                  <p>
                    最近窗口请求：{latestTrafficReport.request_count} · UV{' '}
                    {latestTrafficReport.unique_visitor_count} · 错误{' '}
                    {latestTrafficReport.error_count}
                  </p>
                ) : null}
              </div>

              <div className="space-y-2">
                {activeHealthEvents.slice(0, 3).map((event) => (
                  <div
                    key={`${event.event_type}-${event.last_triggered_at}`}
                    className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <StatusBadge
                        label={getHealthEventLabel(event)}
                        variant={getHealthEventVariant(event)}
                      />
                    </div>
                    <p className="mt-2 text-sm text-[var(--foreground-secondary)]">
                      {event.message || '暂无详细消息'}
                    </p>
                  </div>
                ))}

                {activeHealthEvents.length === 0 ? (
                  <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 text-sm text-[var(--foreground-secondary)]">
                    当前没有活动中的健康事件。
                  </div>
                ) : null}
              </div>
            </div>
          </AppCard>
        </div>

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

          <AppCard title="版本信息">
            <div className="space-y-2 text-sm text-[var(--foreground-secondary)]">
              <p>Agent：{node.agent_version || 'unknown'}</p>
              <p>Nginx：{node.nginx_version || 'unknown'}</p>
              <p>当前配置：{node.current_version || '未应用'}</p>
            </div>
          </AppCard>

          <AppCard title="最近应用">
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
                  <p>目标 Checksum：{node.latest_apply_checksum}</p>
                  <p>支持文件：{node.latest_support_file_count}</p>
                </div>
              ) : null}
            </div>
          </AppCard>
        </div>

        <AppCard
          title="当前目标版本"
          description="展示当前全局激活配置版本，便于直接核对节点应追上的主配置与路由配置。"
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
                    label={
                      isTargetVersionApplied
                        ? '已追平目标版本'
                        : '待追平目标版本'
                    }
                    variant={isTargetVersionApplied ? 'success' : 'warning'}
                  />
                </div>
                <p className="mt-3 text-sm text-[var(--foreground-secondary)]">
                  {isTargetVersionApplied
                    ? '当前节点已应用全局激活配置。'
                    : '当前节点版本落后于全局激活配置，可结合应用记录定位原因。'}
                </p>
              </div>

              <div className="grid gap-4 md:grid-cols-3">
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    目标版本
                  </p>
                  <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                    {activeConfigVersion.version}
                  </p>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    Target Checksum
                  </p>
                  <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                    {activeConfigVersion.checksum}
                  </p>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    激活时间
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
              message="当前还没有全局激活配置版本，无法展示目标快照。"
            />
          )}
        </AppCard>

        <AppCard
          title="OpenResty 健康与控制"
          description="OpenResty 当前健康状态。"
          action={
            <div className="flex flex-wrap gap-3">
              <PrimaryButton
                type="button"
                onClick={handleRestartOpenresty}
                disabled={restartOpenrestyMutation.isPending}
              >
                {restartOpenrestyMutation.isPending
                  ? '下发重启中...'
                  : node.restart_openresty_requested
                    ? '等待 OpenResty 重启'
                    : '重启 OpenResty'}
              </PrimaryButton>
            </div>
          }
        >
          <div className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                健康状态
              </p>
              <div className="mt-3 flex flex-wrap items-center gap-3">
                <StatusBadge
                  label={getOpenrestyStatusLabel(node.openresty_status)}
                  variant={getOpenrestyStatusVariant(node.openresty_status)}
                />
                {node.restart_openresty_requested ? (
                  <StatusBadge label="等待重启执行" variant="warning" />
                ) : null}
              </div>
              <p className="mt-3 text-sm text-[var(--foreground-secondary)]">
                {node.restart_openresty_requested
                  ? '已等待节点在下一次心跳后执行 OpenResty 重启。'
                  : '系统会在每次心跳前自动采集健康状态。'}
              </p>
            </div>

            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                状态消息
              </p>
              <p className="mt-3 text-sm leading-6 break-words whitespace-pre-wrap text-[var(--foreground-secondary)]">
                {node.openresty_message || '当前未上报额外错误。'}
              </p>
            </div>
          </div>
        </AppCard>

        <div className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
          <AppCard
            title="节点标识与部署"
            action={
              nodeInstallCommand ? (
                <PrimaryButton
                  type="button"
                  onClick={() =>
                    void handleCopy(
                      nodeInstallCommand,
                      '节点专属部署命令已复制。',
                    )
                  }
                >
                  复制部署命令
                </PrimaryButton>
              ) : null
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
                    Agent Token
                  </p>
                  <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                    {node.agent_token || '暂无'}
                  </p>
                </div>
              </div>

              <ResourceField
                label="Server URL"
                hint="默认使用当前控制面来源地址，可按需改为外部访问地址。"
              >
                <ResourceInput
                  value={serverUrl}
                  onChange={(event) => setServerUrl(event.target.value)}
                />
              </ResourceField>

              {nodeInstallCommand ? (
                <CodeBlock className="whitespace-pre-wrap">
                  {nodeInstallCommand}
                </CodeBlock>
              ) : null}
            </div>
          </AppCard>

          <AppCard title="运行信息">
            <div className="space-y-4 text-sm text-[var(--foreground-secondary)]">
              <div>
                <p className="font-medium text-[var(--foreground-primary)]">
                  IP 地址
                </p>
                <p className="mt-1">{node.ip || '暂无'}</p>
              </div>
              <div>
                <p className="font-medium text-[var(--foreground-primary)]">
                  最近错误
                </p>
                <p className="mt-1 break-words whitespace-pre-wrap">
                  {node.last_error || '无'}
                </p>
              </div>
              <div>
                <p className="font-medium text-[var(--foreground-primary)]">
                  OpenResty 状态消息
                </p>
                <p className="mt-1 break-words whitespace-pre-wrap">
                  {node.openresty_message || '无'}
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
          title="最近应用记录"
          description="仅展示当前节点的应用历史。"
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
              description="当前节点还没有上报过配置应用结果。"
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">版本</th>
                    <th className="px-3 py-3 font-medium">结果</th>
                    <th className="px-3 py-3 font-medium">Checksum</th>
                    <th className="px-3 py-3 font-medium">时间</th>
                    <th className="px-3 py-3 font-medium">消息</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {applyLogs.slice(0, 10).map((log) => (
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
                        {log.checksum ? `${log.checksum.slice(0, 12)}...` : '—'}
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        {formatRelativeTime(log.created_at)} ·{' '}
                        {formatDateTime(log.created_at)}
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        <div className="max-w-80 space-y-2 break-words whitespace-pre-wrap">
                          <p>
                            {log.main_config_checksum
                              ? `主配置：${log.main_config_checksum}`
                              : ''}
                          </p>
                          <p>
                            {log.route_config_checksum
                              ? `路由配置：${log.route_config_checksum}`
                              : ''}
                          </p>
                          <p>
                            {log.support_file_count
                              ? `支持文件：${log.support_file_count}`
                              : ''}
                          </p>
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
      </div>

      <ConfigVersionSnapshotModal
        version={isTargetSnapshotOpen ? activeConfigVersion : null}
        onClose={() => setIsTargetSnapshotOpen(false)}
      />

      <AppModal
        isOpen={isEditorOpen}
        onClose={() => setIsEditorOpen(false)}
        title="编辑"
        description="更新模式和节点名都在详情页维护。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={() => setIsEditorOpen(false)}
              disabled={saveMutation.isPending}
            >
              取消
            </SecondaryButton>
            <PrimaryButton
              type="submit"
              form="node-detail-editor-form"
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending ? '保存中...' : '保存修改'}
            </PrimaryButton>
          </div>
        }
      >
        <form
          id="node-detail-editor-form"
          className="space-y-5"
          onSubmit={handleSubmit}
        >
          <ResourceField
            label="节点名"
            hint="示例：shanghai-edge-1"
            error={form.formState.errors.name?.message}
          >
            <ResourceInput
              placeholder="shanghai-edge-1"
              {...form.register('name')}
            />
          </ResourceField>

          <ToggleField
            label="启用自动更新"
            description="开启后 Agent 心跳返回会提示节点自动执行自更新。"
            checked={watchedAutoUpdate}
            onChange={(checked) =>
              form.setValue('auto_update_enabled', checked, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
          />
        </form>
      </AppModal>

      <AppModal
        isOpen={isAgentUpdateModalOpen}
        onClose={() => setIsAgentUpdateModalOpen(false)}
        title="Agent 更新"
        description="默认检查正式版；你也可以手动检查 preview 发布，并选择向当前节点下发对应版本的升级指令。"
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
            <AppCard title="当前 Agent 版本">
              <p className="text-sm font-medium text-[var(--foreground-primary)]">
                {node.agent_version || 'unknown'}
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
              title="Agent 版本检查失败"
              description={getErrorMessage(selectedAgentReleaseError)}
            />
          ) : null}
          {!isCheckingAgentRelease &&
          !selectedAgentReleaseError &&
          !selectedAgentRelease ? (
            <EmptyState
              title="尚未检查 Agent 更新"
              description="点击“检查正式版”或“检查预览版”后，会在这里显示对应发布信息。"
            />
          ) : null}

          {selectedAgentRelease ? (
            <AppCard
              title={`GitHub ${selectedReleaseChannel === 'preview' ? '预览版' : '正式版'} · ${selectedAgentRelease.tag_name || '未找到版本'}`}
              description={
                selectedAgentRelease.published_at
                  ? `发布时间：${formatRelativeTime(selectedAgentRelease.published_at)} · ${formatDateTime(selectedAgentRelease.published_at)}`
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
                  {node.update_requested ? (
                    <StatusBadge
                      label={`已下发${node.update_channel === 'preview' ? '预览版' : '正式版'}更新`}
                      variant="warning"
                    />
                  ) : null}
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
                      目标版本
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
