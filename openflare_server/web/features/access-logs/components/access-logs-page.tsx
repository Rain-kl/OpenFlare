'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { getAccessLogs } from '@/features/access-logs/api/access-logs';
import type { AccessLogItem } from '@/features/access-logs/types';
import {
  PrimaryButton,
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';

const accessLogsQueryKey = (nodeId: string) => ['access-logs', nodeId] as const;
const defaultPageSize = 20;
const accessLogPageSizeOptions = [20, 50, 100, 200];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function buildSummary(totalRecord = 0, totalIP = 0) {
  return [
    { label: '访问记录', value: totalRecord },
    { label: '来源 IP', value: totalIP },
  ];
}

function getStatusMeta(statusCode: number) {
  if (statusCode >= 500) {
    return { label: String(statusCode), variant: 'danger' as const };
  }
  if (statusCode >= 400) {
    return { label: String(statusCode), variant: 'warning' as const };
  }
  return { label: String(statusCode), variant: 'success' as const };
}

export function AccessLogsPage() {
  const queryClient = useQueryClient();
  const [nodeFilterInput, setNodeFilterInput] = useState('');
  const [nodeFilter, setNodeFilter] = useState('');
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(defaultPageSize);

  const logsQuery = useQuery({
    queryKey: [...accessLogsQueryKey(nodeFilter), page, pageSize],
    queryFn: () => getAccessLogs(page, nodeFilter, pageSize),
    placeholderData: (previousData) => previousData,
  });

  const logs = useMemo(() => logsQuery.data?.items ?? [], [logsQuery.data]);
  const hasMore = logsQuery.data?.has_more ?? false;
  const summary = useMemo(
    () =>
      buildSummary(
        logsQuery.data?.total_record ?? 0,
        logsQuery.data?.total_ip ?? 0,
      ),
    [logsQuery.data?.total_ip, logsQuery.data?.total_record],
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="日志"
        description="查看最近时间窗口内的访问记录，包含来源 IP、归属地、访问域名、路径、命中节点与响应状态码。"
        action={
          <Link
            href="/node"
            className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
          >
            返回节点页
          </Link>
        }
      />

      <AppCard
        title="日志摘要"
        description="基于当前筛选条件汇总最近时间窗口内的访问记录与来源 IP 规模。"
      >
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-2">
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

      <AppCard
        title="访问记录"
        description="默认展示全部节点的最近访问日志，可按 node_id 快速过滤。"
        action={
          <SecondaryButton
            type="button"
            onClick={() =>
              void queryClient.invalidateQueries({
                queryKey: accessLogsQueryKey(nodeFilter),
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
              placeholder="输入 node_id 过滤访问日志"
              className="lg:max-w-md"
            />
            <ResourceSelect
              value={String(pageSize)}
              onChange={(event) => {
                setPageSize(Number(event.target.value) || defaultPageSize);
                setPage(0);
              }}
              className="lg:w-40"
              aria-label="每页条数"
            >
              {accessLogPageSizeOptions.map((option) => (
                <option key={option} value={option}>
                  每页 {option} 条
                </option>
              ))}
            </ResourceSelect>
            <div className="flex flex-wrap gap-2">
              <PrimaryButton
                type="button"
                onClick={() => {
                  setNodeFilter(nodeFilterInput.trim());
                  setPage(0);
                }}
              >
                筛选
              </PrimaryButton>
              <SecondaryButton
                type="button"
                onClick={() => {
                  setNodeFilter('');
                  setNodeFilterInput('');
                  setPage(0);
                }}
              >
                清空
              </SecondaryButton>
            </div>
          </div>

          {logsQuery.isLoading ? (
            <LoadingState />
          ) : logsQuery.isError ? (
            <ErrorState
              title="访问日志加载失败"
              description={getErrorMessage(logsQuery.error)}
            />
          ) : logs.length === 0 ? (
            <EmptyState
              title="暂无访问日志"
              description="当前时间窗口内还没有可展示的访问记录。"
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">时间</th>
                    <th className="px-3 py-3 font-medium">原 IP</th>
                    <th className="px-3 py-3 font-medium">访问域名</th>
                    <th className="px-3 py-3 font-medium">路径</th>
                    <th className="px-3 py-3 font-medium">节点</th>
                    <th className="px-3 py-3 font-medium">状态码</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {logs.map((item) => {
                    const statusMeta = getStatusMeta(item.status_code);
                    return (
                      <tr key={item.id} className="align-top">
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          <div>{formatDateTime(item.logged_at)}</div>
                          <div className="mt-1 text-xs text-[var(--foreground-muted)]">
                            {formatRelativeTime(item.logged_at)}
                          </div>
                        </td>
                        <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                          <div>{item.remote_addr || '—'}</div>
                          {item.region ? (
                            <div className="mt-2">
                              <span className="inline-flex rounded-full border border-[var(--border-default)] bg-[var(--surface-elevated)] px-2.5 py-1 text-[11px] font-medium text-[var(--foreground-secondary)]">
                                {item.region}
                              </span>
                            </div>
                          ) : null}
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {item.host || '—'}
                        </td>
                        <td
                          className="max-w-[360px] px-3 py-4 text-[var(--foreground-secondary)]"
                          title={item.path}
                        >
                          <span className="break-all">{item.path || '—'}</span>
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          <div>{item.node_name || item.node_id}</div>
                          <div className="mt-1 text-xs text-[var(--foreground-muted)]">
                            {item.node_id}
                          </div>
                        </td>
                        <td className="px-3 py-4">
                          <StatusBadge
                            label={statusMeta.label}
                            variant={statusMeta.variant}
                          />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
          <div className="flex flex-col gap-3 border-t border-[var(--border-default)] pt-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-[var(--foreground-secondary)]">
              第 {page + 1} 页，每页 {pageSize} 条。
            </p>
            <div className="flex gap-2">
              <SecondaryButton
                type="button"
                disabled={page === 0 || logsQuery.isLoading}
                onClick={() => setPage((value) => Math.max(value - 1, 0))}
              >
                上一页
              </SecondaryButton>
              <SecondaryButton
                type="button"
                disabled={!hasMore || logsQuery.isLoading}
                onClick={() => setPage((value) => value + 1)}
              >
                下一页
              </SecondaryButton>
            </div>
          </div>
        </div>
      </AppCard>
    </div>
  );
}
