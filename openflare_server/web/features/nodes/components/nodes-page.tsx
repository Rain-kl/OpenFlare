'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { getConfigVersions } from '@/features/config-versions/api/config-versions';
import {
  createNode,
  deleteNode,
  getNodes,
  updateNode,
} from '@/features/nodes/api/nodes';
import { NodeEditorModal } from '@/features/nodes/components/node-editor-modal';
import type { NodeItem, NodeMutationPayload } from '@/features/nodes/types';
import {
  DangerButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';
import {
  getApplyLabel,
  getApplyVariant,
  getNodeStatusLabel,
  getNodeStatusVariant,
  getOpenrestyStatusLabel,
  getOpenrestyStatusVariant,
  getRelayStatusLabel,
  getRelayStatusVariant,
  isMeaningfulTime,
  isWSConnectedLastSeen,
} from '@/features/nodes/utils';

const nodesQueryKey = ['nodes'];
const supportedRiskFilters = [
  'all',
  'offline',
  'unhealthy',
  'lagging',
] as const;

type NodeRiskFilter = (typeof supportedRiskFilters)[number];

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

export function NodesPage() {
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [editingNode, setEditingNode] = useState<NodeItem | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);

  const nodesQuery = useQuery({
    queryKey: nodesQueryKey,
    queryFn: getNodes,
    refetchInterval: 5000,
  });

  const configVersionsQuery = useQuery({
    queryKey: ['config-versions'],
    queryFn: getConfigVersions,
    refetchInterval: 5000,
  });

  const saveMutation = useMutation({
    mutationFn: async (payload: NodeMutationPayload) => {
      return editingNode
        ? updateNode(editingNode.id, payload)
        : createNode(payload);
    },
    onSuccess: async () => {
      setFeedback({
        tone: 'success',
        message: editingNode ? '节点已更新。' : '节点已创建。',
      });
      setEditingNode(null);
      setIsEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteNode,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '节点已删除。' });
      setEditingNode(null);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const nodes = useMemo(() => nodesQuery.data ?? [], [nodesQuery.data]);
  const activeVersion = useMemo(
    () =>
      (configVersionsQuery.data ?? []).find((item) => item.is_active)
        ?.version ?? '',
    [configVersionsQuery.data],
  );
  const riskFilter = useMemo<NodeRiskFilter>(() => {
    const current = searchParams.get('risk')?.trim().toLowerCase() ?? 'all';
    return supportedRiskFilters.includes(current as NodeRiskFilter)
      ? (current as NodeRiskFilter)
      : 'all';
  }, [searchParams]);
  const filteredNodes = useMemo(() => {
    switch (riskFilter) {
      case 'offline':
        return nodes.filter((node) => node.status === 'offline');
      case 'unhealthy':
        return nodes.filter((node) => node.openresty_status === 'unhealthy');
      case 'lagging':
        return nodes.filter((node) => {
          if (!activeVersion) {
            return false;
          }
          return node.current_version !== activeVersion;
        });
      default:
        return nodes;
    }
  }, [activeVersion, nodes, riskFilter]);

  const filterDescription = useMemo(() => {
    switch (riskFilter) {
      case 'offline':
        return '当前仅展示离线节点。';
      case 'unhealthy':
        return '当前仅展示 OpenResty 异常节点。';
      case 'lagging':
        return activeVersion
          ? `当前仅展示未追平激活版本 ${activeVersion} 的节点。`
          : '当前没有激活版本，无法筛选配置落后节点。';
      default:
        return '列表每 5 秒自动刷新一次。';
    }
  }, [activeVersion, riskFilter]);

  const handleReset = () => {
    setFeedback(null);
    setEditingNode(null);
    setIsEditorOpen(false);
  };

  const handleCreate = () => {
    setFeedback(null);
    setEditingNode(null);
    setIsEditorOpen(true);
  };

  const handleEdit = (node: NodeItem) => {
    setFeedback(null);
    setEditingNode(node);
    setIsEditorOpen(true);
  };

  const handleDelete = (nodeId: number, nodeName: string) => {
    if (
      !window.confirm(
        `确认删除节点“${nodeName}”吗？删除后该节点需要重新创建并重新接入。`,
      )
    ) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(nodeId);
  };
  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="节点管理"
          description="显示节点相关信息。"
          action={
            <>
              <SecondaryButton type="button" onClick={handleCreate}>
                新增节点
              </SecondaryButton>
              <Link
                href="/apply-log"
                className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
              >
                应用记录
              </Link>
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

        <AppCard
          title="节点列表"
          description={filterDescription}
          action={
            <SecondaryButton
              type="button"
              onClick={() =>
                void queryClient.invalidateQueries({ queryKey: nodesQueryKey })
              }
            >
              立即刷新
            </SecondaryButton>
          }
        >
          {nodesQuery.isLoading ? (
            <LoadingState />
          ) : nodesQuery.isError ? (
            <ErrorState
              title="节点列表加载失败"
              description={getErrorMessage(nodesQuery.error)}
            />
          ) : filteredNodes.length === 0 ? (
            <EmptyState
              title={riskFilter === 'all' ? '暂无节点' : '当前筛选无结果'}
              description={
                riskFilter === 'all'
                  ? '请先创建一个节点，然后进入详情页查看专属部署命令。'
                  : '可以返回总览继续排查，或切换到全部节点查看完整列表。'
              }
            />
          ) : (
            <div className="space-y-4">
              <div className="flex flex-wrap gap-2">
                <Link
                  href="/node"
                  className={`inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
                    riskFilter === 'all'
                      ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                      : 'border-[var(--border-default)] text-[var(--foreground-secondary)] hover:bg-[var(--control-background-hover)]'
                  }`}
                >
                  全部节点
                </Link>
                <Link
                  href="/node?risk=offline"
                  className={`inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
                    riskFilter === 'offline'
                      ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                      : 'border-[var(--border-default)] text-[var(--foreground-secondary)] hover:bg-[var(--control-background-hover)]'
                  }`}
                >
                  离线节点
                </Link>
                <Link
                  href="/node?risk=unhealthy"
                  className={`inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
                    riskFilter === 'unhealthy'
                      ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                      : 'border-[var(--border-default)] text-[var(--foreground-secondary)] hover:bg-[var(--control-background-hover)]'
                  }`}
                >
                  OpenResty 异常
                </Link>
                <Link
                  href="/node?risk=lagging"
                  className={`inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
                    riskFilter === 'lagging'
                      ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                      : 'border-[var(--border-default)] text-[var(--foreground-secondary)] hover:bg-[var(--control-background-hover)]'
                  }`}
                >
                  配置落后
                </Link>
              </div>

              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                  <thead>
                    <tr className="text-[var(--foreground-secondary)]">
                      <th className="px-3 py-3 font-medium">节点</th>
                      <th className="px-3 py-3 font-medium">状态</th>
                      <th className="px-3 py-3 font-medium">
                        Version
                      </th>
                      <th className="px-3 py-3 font-medium">运行健康</th>
                      <th className="px-3 py-3 font-medium">当前版本</th>
                      <th className="px-3 py-3 font-medium">最近应用</th>
                      <th className="px-3 py-3 font-medium">最近心跳</th>
                      <th className="px-3 py-3 font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--border-default)]">
                    {filteredNodes.map((node) => (
                      <tr key={node.id} className="align-top">
                        <td className="px-3 py-4">
                          <div className="space-y-1">
                            <div className="flex items-center gap-2">
                              <p className="font-medium text-[var(--foreground-primary)]">
                                {node.name}
                              </p>
                              {node.node_type === 'tunnel_relay' ? (
                                <span className="rounded-full border border-blue-500/20 bg-blue-50 px-2 py-0.5 text-xs text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                                  Relay
                                </span>
                              ) : node.node_type === 'tunnel_client' ? (
                                <span className="rounded-full border border-purple-500/20 bg-purple-50 px-2 py-0.5 text-xs text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">
                                  Client
                                </span>
                              ) : (
                                <span className="rounded-full border border-gray-500/20 bg-gray-50 px-2 py-0.5 text-xs text-gray-700 dark:bg-gray-800/50 dark:text-gray-300">
                                  Edge
                                </span>
                              )}
                            </div>
                            <p className="text-xs text-[var(--foreground-secondary)]">
                              IP：{node.ip || 'null'}
                              {node.ip_manual_override ? '（已锁定）' : ''}
                            </p>
                            <p className="text-xs text-[var(--foreground-secondary)]">
                              位置：{node.geo_name || '未配置地图点位'}
                            </p>
                          </div>
                        </td>
                        <td className="px-3 py-4">
                          <StatusBadge
                            label={getNodeStatusLabel(node.status)}
                            variant={getNodeStatusVariant(node.status)}
                          />
                        </td>
                        <td className="px-3 py-4 text-sm text-[var(--foreground-secondary)]">
                          {node.version || 'unknown'}
                        </td>
                        <td className="px-3 py-4">
                          <div className="space-y-2">
                            {node.node_type === 'tunnel_relay' ? (
                              <StatusBadge
                                label={getRelayStatusLabel(node.relay_status)}
                                variant={getRelayStatusVariant(node.relay_status)}
                              />
                            ) : node.node_type === 'tunnel_client' ? (
                              <span className="text-sm text-[var(--foreground-secondary)]">-</span>
                            ) : (
                              <StatusBadge
                                label={getOpenrestyStatusLabel(
                                  node.openresty_status,
                                )}
                                variant={getOpenrestyStatusVariant(
                                  node.openresty_status,
                                )}
                              />
                            )}
                          </div>
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {node.current_version || '未应用'}
                        </td>
                        <td className="px-3 py-4">
                          <div className="space-y-2">
                            <StatusBadge
                              label={getApplyLabel(node.latest_apply_result)}
                              variant={getApplyVariant(
                                node.latest_apply_result,
                              )}
                            />
                          </div>
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {isWSConnectedLastSeen(node.last_seen_at)
                            ? 'WS 已连接'
                            : isMeaningfulTime(node.last_seen_at)
                              ? `${formatRelativeTime(
                                  node.last_seen_at,
                                )} · ${formatDateTime(node.last_seen_at)}`
                              : '暂无'}
                        </td>
                        <td className="px-3 py-4">
                          <div className="flex flex-wrap gap-2">
                            <Link
                              href={`/node/detail?id=${node.id}`}
                              className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-3 py-2 text-xs font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
                            >
                              详情
                            </Link>
                            <SecondaryButton
                              type="button"
                              onClick={() => handleEdit(node)}
                              className="px-3 py-2 text-xs"
                            >
                              编辑
                            </SecondaryButton>
                            <DangerButton
                              type="button"
                              onClick={() => handleDelete(node.id, node.name)}
                              disabled={deleteMutation.isPending}
                              className="px-3 py-2 text-xs"
                            >
                              删除
                            </DangerButton>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </AppCard>
      </div>
      <NodeEditorModal
        isOpen={isEditorOpen}
        node={editingNode}
        isSubmitting={saveMutation.isPending}
        title={editingNode ? '编辑节点' : '新增节点'}
        onClose={handleReset}
        description="预创建节点后可在详情页查看专属 Token、部署命令与更新控制。"
        submitLabel={editingNode ? '保存修改' : '新增节点'}
        onSubmit={(payload) => {
          setFeedback(null);
          saveMutation.mutate(payload);
        }}
      />
    </>
  );
}
