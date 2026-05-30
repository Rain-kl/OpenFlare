'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { AppModal } from '@/components/ui/app-modal';
import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  cleanupApplyLogs,
  getApplyLogs,
} from '@/features/apply-logs/api/apply-logs';
import type {
  ApplyLogCleanupPayload,
  ApplyLogItem,
} from '@/features/apply-logs/types';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';

const applyLogsQueryKey = (
  nodeId: string,
  pageNo: number,
  pageSize: number,
) => ['apply-logs', nodeId, pageNo, pageSize] as const;

const pageSizeOptions = [20, 50, 100];
const emptyApplyLogRows: ApplyLogItem[] = [];

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function getResultMeta(result: string) {
  if (result === 'success') {
    return { label: '成功', variant: 'success' as const };
  }
  if (result === 'warning') {
    return { label: '警告', variant: 'warning' as const };
  }
  return { label: '失败', variant: 'danger' as const };
}

function truncateHash(value: string) {
  if (!value) {
    return '—';
  }
  return value.length > 12 ? `${value.slice(0, 12)}...` : value;
}

function buildSummary(rows: ApplyLogItem[], total: number, current: number, totalPage: number) {
  const nodeIds = new Set(rows.map((item) => item.node_id));
  return [
    { label: '总记录数', value: total },
    { label: '当前页', value: current },
    { label: '总页数', value: totalPage },
    { label: '当前页节点数', value: nodeIds.size },
  ];
}

export function ApplyLogsPage() {
  const queryClient = useQueryClient();
  const [nodeFilterInput, setNodeFilterInput] = useState('');
  const [nodeFilter, setNodeFilter] = useState('');
  const [pageNo, setPageNo] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [selectedLog, setSelectedLog] = useState<ApplyLogItem | null>(null);
  const [isCleanupModalOpen, setCleanupModalOpen] = useState(false);
  const [cleanupMode, setCleanupMode] = useState<'all' | 'custom'>('custom');
  const [customRetentionDays, setCustomRetentionDays] = useState('30');
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  useEffect(() => {
    const nodeID = new URLSearchParams(window.location.search)
      .get('node_id')
      ?.trim();
    if (!nodeID) {
      return;
    }
    setNodeFilterInput(nodeID);
    setNodeFilter(nodeID);
    setPageNo(1);
  }, []);

  const logsQuery = useQuery({
    queryKey: applyLogsQueryKey(nodeFilter, pageNo, pageSize),
    queryFn: () =>
      getApplyLogs({
        node_id: nodeFilter || undefined,
        pageNo,
        pageSize,
      }),
    placeholderData: (previous) => previous,
  });

  const cleanupMutation = useMutation({
    mutationFn: (payload: ApplyLogCleanupPayload) => cleanupApplyLogs(payload),
    onSuccess: async (result) => {
      setCleanupModalOpen(false);
      setPageNo(1);
      setFeedback({
        tone: 'success',
        message: result.delete_all
          ? `已删除全部应用日志，共 ${result.deleted_count} 条。`
          : `已清理保留期之外的应用日志，共 ${result.deleted_count} 条。`,
      });
      await queryClient.invalidateQueries({ queryKey: ['apply-logs'] });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const rows = logsQuery.data?.rows ?? emptyApplyLogRows;
  const current = logsQuery.data?.current ?? pageNo;
  const total = logsQuery.data?.total ?? 0;
  const totalPage = logsQuery.data?.totalPage ?? 0;
  const summary = useMemo(
    () => buildSummary(rows, total, current, totalPage),
    [rows, total, current, totalPage],
  );

  useEffect(() => {
    if (rows.length === 0 && selectedLog) {
      setSelectedLog(null);
      return;
    }
    if (selectedLog && !rows.some((item) => item.id === selectedLog.id)) {
      setSelectedLog(null);
    }
  }, [rows, selectedLog]);

  const handleSearch = () => {
    setFeedback(null);
    setPageNo(1);
    setNodeFilter(nodeFilterInput.trim());
  };

  const handleReset = () => {
    setFeedback(null);
    setNodeFilter('');
    setNodeFilterInput('');
    setPageNo(1);
  };

  const handleRefresh = async () => {
    setFeedback(null);
    await queryClient.invalidateQueries({
      queryKey: applyLogsQueryKey(nodeFilter, pageNo, pageSize),
    });
  };

  const handleCleanupConfirm = () => {
    const payload: ApplyLogCleanupPayload =
      cleanupMode === 'all'
        ? { delete_all: true }
        : {
            retention_days: Number.parseInt(customRetentionDays, 10),
          };
    cleanupMutation.mutate(payload);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="应用日志"
        description="查看节点应用配置的成功、警告和失败记录，支持分页查询、详情弹窗和按保留天数清理。"
        action={
          <div className="flex flex-wrap gap-2">
            <SecondaryButton type="button" onClick={handleRefresh}>
              刷新
            </SecondaryButton>
            <PrimaryButton
              type="button"
              onClick={() => {
                setFeedback(null);
                setCleanupModalOpen(true);
              }}
            >
              删除日志
            </PrimaryButton>
            <Link
              href="/node"
              className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
            >
              返回节点
            </Link>
          </div>
        }
      />

      {feedback ? (
        <InlineMessage tone={feedback.tone} message={feedback.message} />
      ) : null}

      <AppCard title="日志摘要" description="后端按分页返回应用日志，页面仅展示当前页数据和总量信息。">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {summary.map((item) => (
            <div
              key={item.label}
              className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4"
            >
              <p className="text-xs tracking-[0.2em] uppercase text-[var(--foreground-muted)]">
                {item.label}
              </p>
              <p className="mt-2 text-lg font-semibold text-[var(--foreground-primary)]">
                {item.value}
              </p>
            </div>
          ))}
        </div>
      </AppCard>

      <AppCard
        title="过滤与列表"
        description="支持按 node_id 过滤，并按页查看应用结果。默认每页 20 条。"
      >
        <div className="space-y-5">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
            <div className="grid flex-1 gap-3 md:grid-cols-[minmax(0,1fr)_180px]">
              <ResourceField label="Node ID">
                <ResourceInput
                  value={nodeFilterInput}
                  onChange={(event) => setNodeFilterInput(event.target.value)}
                  placeholder="输入 node_id 过滤应用日志"
                />
              </ResourceField>
              <ResourceField label="每页条数">
                <ResourceSelect
                  value={String(pageSize)}
                  onChange={(event) => {
                    setPageSize(Number.parseInt(event.target.value, 10));
                    setPageNo(1);
                  }}
                >
                  {pageSizeOptions.map((option) => (
                    <option key={option} value={option}>
                      {option} 条
                    </option>
                  ))}
                </ResourceSelect>
              </ResourceField>
            </div>
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
              title="应用日志加载失败"
              description={getErrorMessage(logsQuery.error)}
            />
          ) : rows.length === 0 ? (
            <EmptyState
              title="暂无应用日志"
              description="当前筛选条件下没有可展示的应用记录。"
            />
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                  <thead>
                    <tr className="text-[var(--foreground-secondary)]">
                      <th className="px-3 py-3 font-medium">Node ID</th>
                      <th className="px-3 py-3 font-medium">版本</th>
                      <th className="px-3 py-3 font-medium">结果</th>
                      <th className="px-3 py-3 font-medium">Checksum</th>
                      <th className="px-3 py-3 font-medium">时间</th>
                      <th className="px-3 py-3 font-medium">消息</th>
                      <th className="px-3 py-3 font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--border-default)]">
                    {rows.map((log) => {
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
                          <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                            <div className="max-w-72 break-words whitespace-pre-wrap">
                              {log.message || '—'}
                            </div>
                          </td>
                          <td className="px-3 py-4">
                            <SecondaryButton
                              type="button"
                              className="px-3 py-2 text-xs"
                              onClick={() => setSelectedLog(log)}
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

              <div className="flex flex-col gap-3 border-t border-[var(--border-default)] pt-4 md:flex-row md:items-center md:justify-between">
                <p className="text-sm text-[var(--foreground-secondary)]">
                  第 {current} / {Math.max(totalPage, 1)} 页，共 {total} 条记录。
                </p>
                <div className="flex flex-wrap gap-2">
                  <SecondaryButton
                    type="button"
                    disabled={current <= 1}
                    onClick={() => setPageNo((previous) => Math.max(1, previous - 1))}
                  >
                    上一页
                  </SecondaryButton>
                  <SecondaryButton
                    type="button"
                    disabled={totalPage === 0 || current >= totalPage}
                    onClick={() =>
                      setPageNo((previous) =>
                        totalPage > 0 ? Math.min(totalPage, previous + 1) : previous,
                      )
                    }
                  >
                    下一页
                  </SecondaryButton>
                </div>
              </div>
            </>
          )}
        </div>
      </AppCard>

      <AppModal
        isOpen={selectedLog !== null}
        title="应用日志详情"
        description="查看单条应用日志的完整结果、消息和校验信息。"
        size="lg"
        onClose={() => setSelectedLog(null)}
      >
        {selectedLog ? (
          <div className="space-y-4">
            <div className="flex flex-wrap gap-2">
              <StatusBadge {...getResultMeta(selectedLog.result)} />
              <StatusBadge label={`Node：${selectedLog.node_id}`} variant="info" />
              <StatusBadge label={`版本：${selectedLog.version}`} variant="warning" />
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <MetricCard label="创建时间" value={formatDateTime(selectedLog.created_at)} />
              <MetricCard label="相对时间" value={formatRelativeTime(selectedLog.created_at)} />
              <MetricCard label="目标 Checksum" value={selectedLog.checksum || '—'} breakAll />
              <MetricCard
                label="支持文件数"
                value={String(selectedLog.support_file_count)}
              />
              <MetricCard
                label="主配置摘要"
                value={selectedLog.main_config_checksum || '—'}
                breakAll
              />
              <MetricCard
                label="路由配置摘要"
                value={selectedLog.route_config_checksum || '—'}
                breakAll
              />
            </div>

            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] uppercase text-[var(--foreground-muted)]">
                消息
              </p>
              <pre className="mt-3 whitespace-pre-wrap break-words text-sm leading-6 text-[var(--foreground-primary)]">
                {selectedLog.message || '—'}
              </pre>
            </div>
          </div>
        ) : null}
      </AppModal>

      <AppModal
        isOpen={isCleanupModalOpen}
        title="删除应用日志"
        description="可以选择删除全部应用日志，或仅保留最近自定义天数内的记录。"
        onClose={() => setCleanupModalOpen(false)}
        footer={
          <div className="flex flex-wrap justify-end gap-2">
            <SecondaryButton type="button" onClick={() => setCleanupModalOpen(false)}>
              取消
            </SecondaryButton>
            <PrimaryButton
              type="button"
              disabled={cleanupMutation.isPending}
              onClick={handleCleanupConfirm}
            >
              {cleanupMutation.isPending ? '处理中...' : '确认删除'}
            </PrimaryButton>
          </div>
        }
      >
        <div className="space-y-5">
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => setCleanupMode('all')}
              className={`rounded-2xl border px-4 py-3 text-sm transition ${
                cleanupMode === 'all'
                  ? 'border-[var(--brand-primary)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                  : 'border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)]'
              }`}
            >
              全部删除
            </button>
            <button
              type="button"
              onClick={() => setCleanupMode('custom')}
              className={`rounded-2xl border px-4 py-3 text-sm transition ${
                cleanupMode === 'custom'
                  ? 'border-[var(--brand-primary)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                  : 'border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)]'
              }`}
            >
              保留自定义天数
            </button>
          </div>

          {cleanupMode === 'custom' ? (
            <ResourceField label="保留天数" hint="当前支持 1 到 3650 天。">
              <ResourceInput
                value={customRetentionDays}
                onChange={(event) => setCustomRetentionDays(event.target.value)}
                type="number"
                min={1}
                max={3650}
                placeholder="输入保留天数"
              />
            </ResourceField>
          ) : null}

          {cleanupMutation.isError ? (
            <ErrorState
              title="删除应用日志失败"
              description={getErrorMessage(cleanupMutation.error)}
            />
          ) : null}
        </div>
      </AppModal>
    </div>
  );
}

function MetricCard({
  label,
  value,
  breakAll = false,
}: {
  label: string;
  value: string;
  breakAll?: boolean;
}) {
  return (
    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
      <p className="text-xs tracking-[0.2em] uppercase text-[var(--foreground-muted)]">
        {label}
      </p>
      <p
        className={`mt-2 text-sm text-[var(--foreground-primary)] ${
          breakAll ? 'break-all' : ''
        }`}
      >
        {value}
      </p>
    </div>
  );
}
