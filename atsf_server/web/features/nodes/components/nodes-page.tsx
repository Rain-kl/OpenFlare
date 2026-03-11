'use client';

import Link from 'next/link';
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
import {
  createNode,
  deleteNode,
  getNodes,
  requestNodeAgentUpdate,
  updateNode,
} from '@/features/nodes/api/nodes';
import type { NodeItem, NodeMutationPayload } from '@/features/nodes/types';
import {
  CodeBlock,
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';

const nodesQueryKey = ['nodes'];
const installerScriptUrl =
  'https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/scripts/install-agent.sh';

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

function isMeaningfulTime(value: string | null | undefined) {
  return Boolean(value) && !String(value).startsWith('0001-01-01');
}

function getStatusVariant(status: NodeItem['status']) {
  if (status === 'online') {
    return 'success';
  }

  if (status === 'pending') {
    return 'warning';
  }

  return 'danger';
}

function getStatusLabel(status: NodeItem['status']) {
  if (status === 'online') {
    return '在线';
  }

  if (status === 'pending') {
    return '待接入';
  }

  return '离线';
}

function getApplyVariant(result: NodeItem['latest_apply_result']) {
  if (result === 'success') {
    return 'success';
  }

  if (result === 'failed') {
    return 'danger';
  }

  return 'warning';
}

function getApplyLabel(result: NodeItem['latest_apply_result']) {
  if (result === 'success') {
    return '成功';
  }

  if (result === 'failed') {
    return '失败';
  }

  return '暂无';
}

function getUpdateMode(node: NodeItem) {
  if (node.update_requested) {
    return { label: '等待更新', variant: 'warning' as const };
  }

  if (node.auto_update_enabled) {
    return { label: '自动更新', variant: 'success' as const };
  }

  return { label: '手动更新', variant: 'info' as const };
}

function getServerUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}

function buildNodeInstallCommand(serverUrl: string, agentToken: string) {
  return [
    `curl -fsSL ${installerScriptUrl} | bash -s -- \\`,
    `  --server-url ${serverUrl} \\`,
    `  --agent-token ${agentToken}`,
  ].join('\n');
}

async function copyToClipboard(value: string) {
  await navigator.clipboard.writeText(value);
}

export function NodesPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [editingNodeId, setEditingNodeId] = useState<number | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [isDeployModalOpen, setIsDeployModalOpen] = useState(false);
  const [selectedNode, setSelectedNode] = useState<NodeItem | null>(null);
  const [serverUrl, setServerUrl] = useState('');
  const [, setRefreshTick] = useState(0);

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
  });

  useEffect(() => {
    if (typeof window !== 'undefined' && !serverUrl) {
      setServerUrl(window.location.origin);
    }
  }, [serverUrl]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void queryClient.invalidateQueries({ queryKey: nodesQueryKey });
      setRefreshTick((value) => value + 1);
    }, 30000);

    return () => {
      window.clearInterval(timer);
    };
  }, [queryClient]);

  const saveMutation = useMutation({
    mutationFn: async (values: NodeFormValues) => {
      const payload = toPayload(values);
      return editingNodeId
        ? updateNode(editingNodeId, payload)
        : createNode(payload);
    },
    onSuccess: async (node) => {
      setFeedback({
        tone: 'success',
        message: editingNodeId ? '节点已更新。' : '节点已创建。',
      });
      setEditingNodeId(null);
      setIsEditorOpen(false);
      setSelectedNode(node);
      form.reset(defaultValues);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const updateAgentMutation = useMutation({
    mutationFn: requestNodeAgentUpdate,
    onSuccess: async (node) => {
      setFeedback({
        tone: 'success',
        message: `已向节点 ${node.name} 下发更新指令。`,
      });
      setSelectedNode(node);
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
      setEditingNodeId(null);
      setSelectedNode(null);
      form.reset(defaultValues);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const nodes = useMemo(() => nodesQuery.data ?? [], [nodesQuery.data]);
  const normalizedServerUrl = getServerUrl(serverUrl);

  const selectedNodeFromList = useMemo(() => {
    if (!selectedNode) {
      return null;
    }

    return nodes.find((item) => item.id === selectedNode.id) ?? selectedNode;
  }, [nodes, selectedNode]);

  const handleReset = () => {
    setFeedback(null);
    setEditingNodeId(null);
    setIsEditorOpen(false);
    form.reset(defaultValues);
  };

  const handleCreate = () => {
    setFeedback(null);
    setEditingNodeId(null);
    form.reset(defaultValues);
    setIsEditorOpen(true);
  };

  const handleEdit = (node: NodeItem) => {
    setFeedback(null);
    setEditingNodeId(node.id);
    setSelectedNode(node);
    form.reset({
      name: node.name,
      auto_update_enabled: node.auto_update_enabled,
    });
    setIsEditorOpen(true);
  };

  const handleOpenDeployModal = (node: NodeItem) => {
    setFeedback(null);
    setSelectedNode(node);
    setIsDeployModalOpen(true);
  };

  const handleDelete = (node: NodeItem) => {
    if (
      !window.confirm(
        `确认删除节点“${node.name}”吗？删除后该节点需要重新创建并重新接入。`,
      )
    ) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(node.id);
  };

  const handleCopy = async (value: string, successMessage: string) => {
    try {
      await copyToClipboard(value);
      setFeedback({ tone: 'success', message: successMessage });
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  const handleSubmit = form.handleSubmit((values) => {
    setFeedback(null);
    saveMutation.mutate(values);
  });

  const nodeInstallCommand =
    normalizedServerUrl && selectedNodeFromList?.agent_token
      ? buildNodeInstallCommand(
          normalizedServerUrl,
          selectedNodeFromList.agent_token,
        )
      : '';

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="节点管理"
          description="查看节点在线状态、最近心跳、部署入口与 Agent 更新动作，并支持预创建节点。"
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
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <AppCard
          title="节点列表"
          description="展示心跳、最近应用结果、版本信息与更新状态，可直接执行管理动作。"
          action={
            <SecondaryButton
              type="button"
              onClick={() =>
                void queryClient.invalidateQueries({ queryKey: nodesQueryKey })
              }
            >
              刷新列表
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
          ) : nodes.length === 0 ? (
            <EmptyState
              title="暂无节点"
              description="请先创建一个节点，然后在列表中打开该节点的部署弹窗。"
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">节点</th>
                    <th className="px-3 py-3 font-medium">状态</th>
                    <th className="px-3 py-3 font-medium">更新模式</th>
                    <th className="px-3 py-3 font-medium">Agent / Nginx</th>
                    <th className="px-3 py-3 font-medium">当前版本</th>
                    <th className="px-3 py-3 font-medium">最近应用</th>
                    <th className="px-3 py-3 font-medium">最近心跳</th>
                    <th className="px-3 py-3 font-medium">错误</th>
                    <th className="px-3 py-3 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {nodes.map((node) => {
                    const updateMode = getUpdateMode(node);

                    return (
                      <tr key={node.id} className="align-top">
                        <td className="px-3 py-4">
                          <div className="space-y-2">
                            <p className="font-medium text-[var(--foreground-primary)]">
                              {node.name}
                            </p>
                            <p className="text-xs text-[var(--foreground-secondary)]">
                              Node ID：{node.node_id}
                            </p>
                            <p className="text-xs break-all text-[var(--foreground-secondary)]">
                              Token：{node.agent_token || '暂无'}
                            </p>
                            <p className="text-xs text-[var(--foreground-secondary)]">
                              IP：{node.ip || '暂无'}
                            </p>
                          </div>
                        </td>
                        <td className="px-3 py-4">
                          <div className="space-y-2">
                            <StatusBadge
                              label={getStatusLabel(node.status)}
                              variant={getStatusVariant(node.status)}
                            />
                            <StatusBadge
                              label={node.pending ? '未占用' : '已绑定'}
                              variant={node.pending ? 'warning' : 'success'}
                            />
                          </div>
                        </td>
                        <td className="px-3 py-4">
                          <StatusBadge
                            label={updateMode.label}
                            variant={updateMode.variant}
                          />
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {node.agent_version || 'unknown'} /{' '}
                          {node.nginx_version || 'unknown'}
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
                            <p className="max-w-56 text-xs leading-5 text-[var(--foreground-secondary)]">
                              {node.latest_apply_message || '暂无记录'}
                            </p>
                            <p className="text-xs text-[var(--foreground-secondary)]">
                              {isMeaningfulTime(node.latest_apply_at)
                                ? `${formatRelativeTime(node.latest_apply_at)} · ${formatDateTime(node.latest_apply_at)}`
                                : '暂无'}
                            </p>
                          </div>
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {isMeaningfulTime(node.last_seen_at)
                            ? `${formatRelativeTime(node.last_seen_at)} · ${formatDateTime(node.last_seen_at)}`
                            : '暂无'}
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          <p className="max-w-56 break-words whitespace-pre-wrap">
                            {node.last_error || '无'}
                          </p>
                        </td>
                        <td className="px-3 py-4">
                          <div className="flex flex-wrap gap-2">
                            <PrimaryButton
                              type="button"
                              onClick={() =>
                                updateAgentMutation.mutate(node.id)
                              }
                              disabled={
                                updateAgentMutation.isPending ||
                                node.update_requested
                              }
                              className="px-3 py-2 text-xs"
                            >
                              立即更新
                            </PrimaryButton>
                            <SecondaryButton
                              type="button"
                              onClick={() => handleOpenDeployModal(node)}
                              className="px-3 py-2 text-xs"
                            >
                              部署
                            </SecondaryButton>
                            <SecondaryButton
                              type="button"
                              onClick={() => handleEdit(node)}
                              className="px-3 py-2 text-xs"
                            >
                              编辑
                            </SecondaryButton>
                            <DangerButton
                              type="button"
                              onClick={() => handleDelete(node)}
                              disabled={deleteMutation.isPending}
                              className="px-3 py-2 text-xs"
                            >
                              删除
                            </DangerButton>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </AppCard>
      </div>
      <AppModal
        isOpen={isDeployModalOpen}
        onClose={() => setIsDeployModalOpen(false)}
        title="节点专属部署命令"
        description="仅展示节点专属 Agent Token 部署方式。Discovery Token 部署入口已从管理端移除。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={() => setIsDeployModalOpen(false)}
            >
              关闭
            </SecondaryButton>
            {nodeInstallCommand ? (
              <PrimaryButton
                type="button"
                onClick={() =>
                  void handleCopy(
                    nodeInstallCommand,
                    '节点专属部署命令已复制。',
                  )
                }
              >
                复制命令
              </PrimaryButton>
            ) : null}
          </div>
        }
      >
        {selectedNodeFromList ? (
          <div className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                    {selectedNodeFromList.name}
                  </p>
                  <StatusBadge
                    label={selectedNodeFromList.pending ? '未占用' : '已绑定'}
                    variant={
                      selectedNodeFromList.pending ? 'warning' : 'success'
                    }
                  />
                </div>
                <p className="mt-2 text-xs text-[var(--foreground-secondary)]">
                  Node ID：{selectedNodeFromList.node_id}
                </p>
                <p className="mt-2 text-xs text-[var(--foreground-secondary)]">
                  IP：{selectedNodeFromList.ip || '暂无'}
                </p>
              </div>
              <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                  Agent Token
                </p>
                <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                  {selectedNodeFromList.agent_token || '暂无'}
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
        ) : (
          <EmptyState
            title="尚未选择节点"
            description="请从节点列表中点击“部署”打开当前节点的专属安装命令。"
          />
        )}
      </AppModal>
      <AppModal
        isOpen={isEditorOpen}
        onClose={handleReset}
        title={editingNodeId ? '编辑节点' : '新增节点'}
        description="预创建节点后会立即生成节点专属 Token，可继续复制专属安装命令。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={handleReset}
              disabled={saveMutation.isPending}
            >
              取消
            </SecondaryButton>
            <PrimaryButton
              type="submit"
              form="node-editor-form"
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending
                ? '保存中...'
                : editingNodeId
                  ? '保存修改'
                  : '新增节点'}
            </PrimaryButton>
          </div>
        }
      >
        <form
          id="node-editor-form"
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
    </>
  );
}
