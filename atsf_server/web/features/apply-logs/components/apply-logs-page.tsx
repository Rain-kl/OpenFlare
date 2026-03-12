'use client';

import Link from 'next/link';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { getApplyLogs } from '@/features/apply-logs/api/apply-logs';
import type { ApplyLogItem } from '@/features/apply-logs/types';
import {
  PrimaryButton,
  ResourceInput,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';

const applyLogsQueryKey = (nodeId: string) => ['apply-logs', nodeId] as const;

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function getResultMeta(result: string) {
  if (result === 'success') {
    return { label: '成功', variant: 'success' as const };
  }

  return { label: '失败', variant: 'danger' as const };
}

function buildSummary(logs: ApplyLogItem[]) {
  const nodeIds = new Set(logs.map((item) => item.node_id));

  return [
    { label: '记录总数', value: logs.length },
    {
      label: '成功',
      value: logs.filter((item) => item.result === 'success').length,
    },
    {
      label: '失败',
      value: logs.filter((item) => item.result !== 'success').length,
    },
    { label: '节点数', value: nodeIds.size },
  ];
}

function truncateHash(value: string) {
  if (!value) {
    return '—';
  }

  return value.length > 12 ? `${value.slice(0, 12)}...` : value;
}

export function ApplyLogsPage() {
  const queryClient = useQueryClient();
  const [nodeFilterInput, setNodeFilterInput] = useState('');
  const [nodeFilter, setNodeFilter] = useState('');
  const [selectedLogId, setSelectedLogId] = useState<number | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);

  const logsQuery = useQuery({
    queryKey: applyLogsQueryKey(nodeFilter),
    queryFn: () => getApplyLogs(nodeFilter),
  });

  const logs = useMemo(() => logsQuery.data ?? [], [logsQuery.data]);
  const summary = useMemo(() => buildSummary(logs), [logs]);

  useEffect(() => {
    if (logs.length === 0) {
      setSelectedLogId(null);
      return;
    }

    if (!logs.some((item) => item.id === selectedLogId)) {
      setSelectedLogId(logs[0].id);
    }
  }, [logs, selectedLogId]);

  const selectedLog = logs.find((item) => item.id === selectedLogId) ?? null;

  const handleSearch = () => {
    setFeedback(null);
    setNodeFilter(nodeFilterInput.trim());
  };

  const handleReset = () => {
    setFeedback(null);
    setNodeFilter('');
    setNodeFilterInput('');
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="应用记录"
        description="查看节点应用版本的成功或失败记录，支持按 node_id 过滤并查看单条详情。"
        action={
          <Link
            href="/node"
            className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
          >
            返回节点页
          </Link>
        }
      />

      {feedback ? <InlineMessage tone="info" message={feedback} /> : null}

      <AppCard
        title="记录摘要"
        description="帮助快速识别失败趋势和受影响节点范围。"
      >
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {summary.map((item) => (
            <div
              key={item.label}
              className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4"
            >
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                {item.label}
              </p>
              <p className="mt-2 text-lg font-semibold text-[var(--foreground-primary)]">
                {item.value}
              </p>
            </div>
          ))}
        </div>
      </AppCard>

      <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <AppCard
          title="过滤与列表"
          description="默认展示全部记录，可按 node_id 快速筛选单节点应用结果。"
          action={
            <SecondaryButton
              type="button"
              onClick={() =>
                void queryClient.invalidateQueries({
                  queryKey: applyLogsQueryKey(nodeFilter),
                })
              }
            >
              刷新
            </SecondaryButton>
          }
        >
          <div className="space-y-5">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
              <ResourceInput
                value={nodeFilterInput}
                onChange={(event) => setNodeFilterInput(event.target.value)}
                placeholder="输入 node_id 过滤应用记录"
                className="lg:max-w-md"
              />
              <div className="flex flex-wrap gap-2">
                <PrimaryButton type="button" onClick={handleSearch}>
                  筛选
                </PrimaryButton>
                <SecondaryButton type="button" onClick={handleReset}>
                  清空
                </SecondaryButton>
              </div>
            </div>

            {logsQuery.isLoading ? (
              <LoadingState />
            ) : logsQuery.isError ? (
              <ErrorState
                title="应用记录加载失败"
                description={getErrorMessage(logsQuery.error)}
              />
            ) : logs.length === 0 ? (
              <EmptyState
                title="暂无应用记录"
                description="当前筛选条件下没有可展示的应用结果。"
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                  <thead>
                    <tr className="text-[var(--foreground-secondary)]">
                      <th className="px-3 py-3 font-medium">Node ID</th>
                      <th className="px-3 py-3 font-medium">版本</th>
                      <th className="px-3 py-3 font-medium">结果</th>
                      <th className="px-3 py-3 font-medium">Checksum</th>
                      <th className="px-3 py-3 font-medium">时间</th>
                      <th className="px-3 py-3 font-medium">详情</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--border-default)]">
                    {logs.map((log) => {
                      const resultMeta = getResultMeta(log.result);
                      return (
                        <tr key={log.id} className="align-top">
                          <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                            {log.node_id}
                          </td>
                          <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                            {log.version}
                          </td>
                          <td className="px-3 py-4">
                            <StatusBadge
                              label={resultMeta.label}
                              variant={resultMeta.variant}
                            />
                          </td>
                          <td
                            className="px-3 py-4 text-[var(--foreground-secondary)]"
                            title={log.checksum}
                          >
                            {truncateHash(log.checksum)}
                          </td>
                          <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                            <div className="space-y-1">
                              <p>{formatDateTime(log.created_at)}</p>
                              <p className="text-xs">
                                {formatRelativeTime(log.created_at)}
                              </p>
                            </div>
                          </td>
                          <td className="px-3 py-4">
                            <SecondaryButton
                              type="button"
                              onClick={() => {
                                setSelectedLogId(log.id);
                                setFeedback(
                                  `已选中节点 ${log.node_id} 的版本 ${log.version} 记录。`,
                                );
                              }}
                              className="px-3 py-2 text-xs"
                            >
                              查看详情
                            </SecondaryButton>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </AppCard>

        <AppCard
          title="记录详情"
          description="展示所选应用记录的完整结果与错误信息。"
        >
          {selectedLog ? (
            <div className="space-y-4">
              <div className="flex flex-wrap gap-2">
                <StatusBadge {...getResultMeta(selectedLog.result)} />
                <StatusBadge
                  label={`Node：${selectedLog.node_id}`}
                  variant="info"
                />
                <StatusBadge
                  label={`版本：${selectedLog.version}`}
                  variant="warning"
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    创建时间
                  </p>
                  <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                    {formatDateTime(selectedLog.created_at)}
                  </p>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    相对时间
                  </p>
                  <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                    {formatRelativeTime(selectedLog.created_at)}
                  </p>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    目标 Checksum
                  </p>
                  <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                    {selectedLog.checksum || '无'}
                  </p>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    支持文件数
                  </p>
                  <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                    {selectedLog.support_file_count}
                  </p>
                </div>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    主配置摘要
                  </p>
                  <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                    {selectedLog.main_config_checksum || '无'}
                  </p>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    路由配置摘要
                  </p>
                  <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                    {selectedLog.route_config_checksum || '无'}
                  </p>
                </div>
              </div>

              <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                  应用信息
                </p>
                <p className="mt-3 text-sm leading-6 break-words whitespace-pre-wrap text-[var(--foreground-primary)]">
                  {selectedLog.message || '无附加信息'}
                </p>
              </div>
            </div>
          ) : (
            <EmptyState
              title="未选择记录"
              description="请先从左侧列表中选择一条应用记录查看详情。"
            />
          )}
        </AppCard>
      </div>
    </div>
  );
}
