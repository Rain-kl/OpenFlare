'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';

import { TrendChart } from '@/components/data/trend-chart';
import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppModal } from '@/components/ui/app-modal';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  cleanupAccessLogs,
  getAccessLogIPSummaries,
  getAccessLogIPTrend,
  getAccessLogs,
  getFoldedAccessLogs,
} from '@/features/access-logs/api/access-logs';
import type {
  AccessLogCleanupPayload,
  AccessLogIPSummaryItem,
  AccessLogIPSummaryList,
  AccessLogList,
  FoldedAccessLogList,
} from '@/features/access-logs/types';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';
import { formatCompactNumber } from '@/lib/utils/metrics';

type ActiveTab = 'detail' | 'ip';

type SearchDraft = {
  nodeId: string;
  remoteAddr: string;
  host: string;
  path: string;
};

type AppliedSearch = SearchDraft;

const pageSizeOptions = [20, 50, 100, 200];
const detailSortOptions = [
  { value: 'logged_at:desc', label: '时间从新到旧' },
  { value: 'logged_at:asc', label: '时间从旧到新' },
  { value: 'status_code:desc', label: '状态码从高到低' },
  { value: 'status_code:asc', label: '状态码从低到高' },
  { value: 'remote_addr:asc', label: 'IP 正序' },
  { value: 'remote_addr:desc', label: 'IP 倒序' },
  { value: 'host:asc', label: '域名正序' },
  { value: 'host:desc', label: '域名倒序' },
];
const foldedSortOptions = [
  { value: 'bucket_started_at:desc', label: '时间桶从新到旧' },
  { value: 'bucket_started_at:asc', label: '时间桶从旧到新' },
  { value: 'request_count:desc', label: '访问次数从高到低' },
  { value: 'request_count:asc', label: '访问次数从低到高' },
];
const ipSortOptions = [
  { value: 'total_requests:desc', label: '总访问次数从高到低' },
  { value: 'total_requests:asc', label: '总访问次数从低到高' },
  { value: 'recent_requests:desc', label: '3 小时访问次数从高到低' },
  { value: 'recent_requests:asc', label: '3 小时访问次数从低到高' },
  { value: 'last_seen_at:desc', label: '最后访问时间从新到旧' },
  { value: 'last_seen_at:asc', label: '最后访问时间从旧到新' },
];
const foldOptions = [
  { value: '0', label: '不折叠' },
  { value: '3', label: '按 3 分钟折叠' },
  { value: '5', label: '按 5 分钟折叠' },
];
const cleanupPresetOptions = [3, 7, 30];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function parseSortValue(value: string) {
  const [sortBy = 'logged_at', sortOrder = 'desc'] = value.split(':');
  return {
    sortBy,
    sortOrder: sortOrder === 'asc' ? 'asc' : 'desc',
  } as const;
}

function buildSummary(totalRecord = 0, totalIP = 0, activeTab: ActiveTab) {
  return [
    { label: '访问记录', value: formatCompactNumber(totalRecord) },
    { label: '来源 IP', value: formatCompactNumber(totalIP) },
    { label: '当前视图', value: activeTab === 'detail' ? '明细日志' : 'IP 维度' },
  ];
}

function buildTrendLabels(points: Array<{ bucket_started_at: string }>) {
  return points.map((point) => {
    const date = new Date(point.bucket_started_at);
    if (Number.isNaN(date.getTime())) {
      return '--';
    }
    return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(
      date.getDate(),
    ).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(
      date.getMinutes(),
    ).padStart(2, '0')}`;
  });
}

export function AccessLogsPage() {
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<ActiveTab>('detail');
  const [draft, setDraft] = useState<SearchDraft>({
    nodeId: '',
    remoteAddr: '',
    host: '',
    path: '',
  });
  const [filters, setFilters] = useState<AppliedSearch>({
    nodeId: '',
    remoteAddr: '',
    host: '',
    path: '',
  });
  const [detailPage, setDetailPage] = useState(0);
  const [ipPage, setIPPage] = useState(0);
  const [pageSize, setPageSize] = useState(20);
  const [foldMinutes, setFoldMinutes] = useState<0 | 3 | 5>(0);
  const [detailSort, setDetailSort] = useState('logged_at:desc');
  const [foldedSort, setFoldedSort] = useState('bucket_started_at:desc');
  const [ipSort, setIPSort] = useState('total_requests:desc');
  const [selectedIP, setSelectedIP] = useState<AccessLogIPSummaryItem | null>(null);
  const [cleanupDays, setCleanupDays] = useState<string>('7');
  const [customCleanupDays, setCustomCleanupDays] = useState('14');
  const [isCleanupModalOpen, setCleanupModalOpen] = useState(false);

  const detailSortState = parseSortValue(detailSort);
  const foldedSortState = parseSortValue(foldedSort);
  const ipSortState = parseSortValue(ipSort);

  const detailQuery = useQuery<AccessLogList | FoldedAccessLogList>({
    queryKey: [
      'access-logs',
      'detail',
      filters,
      detailPage,
      pageSize,
      detailSort,
      foldMinutes,
      foldedSort,
    ],
    queryFn: () => {
      if (foldMinutes > 0) {
        return getFoldedAccessLogs({
          node_id: filters.nodeId || undefined,
          remote_addr: filters.remoteAddr || undefined,
          host: filters.host || undefined,
          path: filters.path || undefined,
          p: detailPage,
          page_size: pageSize,
          sort_by: foldedSortState.sortBy,
          sort_order: foldedSortState.sortOrder,
          fold_minutes: foldMinutes as 3 | 5,
        });
      }
      return getAccessLogs({
        node_id: filters.nodeId || undefined,
        remote_addr: filters.remoteAddr || undefined,
        host: filters.host || undefined,
        path: filters.path || undefined,
        p: detailPage,
        page_size: pageSize,
        sort_by: detailSortState.sortBy,
        sort_order: detailSortState.sortOrder,
      });
    },
    placeholderData: (previousData: AccessLogList | FoldedAccessLogList | undefined) =>
      previousData,
  });

  const ipSummaryQuery = useQuery<AccessLogIPSummaryList>({
    queryKey: ['access-logs', 'ip-summary', filters, ipPage, pageSize, ipSort],
    queryFn: () =>
      getAccessLogIPSummaries({
        node_id: filters.nodeId || undefined,
        remote_addr: filters.remoteAddr || undefined,
        host: filters.host || undefined,
        p: ipPage,
        page_size: pageSize,
        sort_by: ipSortState.sortBy,
        sort_order: ipSortState.sortOrder,
      }),
    placeholderData: (previousData: AccessLogIPSummaryList | undefined) => previousData,
  });

  const ipTrendQuery = useQuery({
    queryKey: ['access-logs', 'ip-trend', selectedIP?.remote_addr, filters.nodeId, filters.host],
    queryFn: () =>
      getAccessLogIPTrend({
        node_id: filters.nodeId || undefined,
        remote_addr: selectedIP?.remote_addr ?? '',
        host: filters.host || undefined,
        hours: 24,
        bucket_minutes: 30,
      }),
    enabled: Boolean(selectedIP?.remote_addr),
  });

  const cleanupMutation = useMutation({
    mutationFn: (payload: AccessLogCleanupPayload) => cleanupAccessLogs(payload),
    onSuccess: async () => {
      setCleanupModalOpen(false);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['access-logs', 'detail'] }),
        queryClient.invalidateQueries({ queryKey: ['access-logs', 'ip-summary'] }),
        queryClient.invalidateQueries({ queryKey: ['access-logs', 'ip-trend'] }),
      ]);
    },
  });

  const detailSummaryData = detailQuery.data as AccessLogList | FoldedAccessLogList | undefined;
  const summary = useMemo(
    () =>
      buildSummary(
        detailSummaryData?.total_record ?? 0,
        detailSummaryData?.total_ip ?? 0,
        activeTab,
      ),
    [activeTab, detailSummaryData?.total_ip, detailSummaryData?.total_record],
  );

  const trendLabels = useMemo(
    () => buildTrendLabels(ipTrendQuery.data?.points ?? []),
    [ipTrendQuery.data?.points],
  );
  const trendValues = useMemo(
    () => (ipTrendQuery.data?.points ?? []).map((point) => point.request_count),
    [ipTrendQuery.data?.points],
  );

  const handleSearch = () => {
    setFilters({
      nodeId: draft.nodeId.trim(),
      remoteAddr: draft.remoteAddr.trim(),
      host: draft.host.trim(),
      path: draft.path.trim(),
    });
    setDetailPage(0);
    setIPPage(0);
  };

  const handleReset = () => {
    const empty = { nodeId: '', remoteAddr: '', host: '', path: '' };
    setDraft(empty);
    setFilters(empty);
    setDetailPage(0);
    setIPPage(0);
    setSelectedIP(null);
  };

  const handleCleanupConfirm = () => {
    const retentionDays =
      cleanupDays === 'custom'
        ? Number.parseInt(customCleanupDays, 10)
        : Number.parseInt(cleanupDays, 10);
    cleanupMutation.mutate({ retention_days: retentionDays });
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="日志"
        description="升级后的日志中心支持按节点、IP、域名与路径检索，支持时间折叠、IP 维度聚合与按保留天数清理旧日志。"
        action={
          <PrimaryButton type="button" onClick={() => setCleanupModalOpen(true)}>
            清理日志
          </PrimaryButton>
        }
      />

      <AppCard title="日志摘要" description="所有汇总、排序、折叠与分页都由后端计算，前端仅展示当前页结果。">
        <div className="grid gap-4 md:grid-cols-3">
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
        title="筛选与视图"
        description="支持 IP、访问域名、路径、分页大小、排序与时间折叠。"
        action={
          <SecondaryButton
            type="button"
            onClick={() =>
              void queryClient.invalidateQueries({
                queryKey: ['access-logs'],
              })
            }
          >
            刷新
          </SecondaryButton>
        }
      >
        <div className="space-y-5">
          <div className="flex flex-wrap gap-2">
            {[
              { key: 'detail', label: '明细日志', description: '按请求明细查看与折叠聚合' },
              { key: 'ip', label: 'IP 维度', description: '查看来源 IP 汇总与趋势' },
            ].map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => setActiveTab(tab.key as ActiveTab)}
                className={`min-w-[180px] rounded-2xl border px-4 py-3 text-left transition ${
                  activeTab === tab.key
                    ? 'border-[var(--brand-primary)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                    : 'border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)] hover:border-[var(--border-strong)]'
                }`}
              >
                <p className="text-sm font-semibold">{tab.label}</p>
                <p className="mt-1 text-xs leading-5">{tab.description}</p>
              </button>
            ))}
          </div>

          <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-4">
            <ResourceField label="节点 ID">
              <ResourceInput
                value={draft.nodeId}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, nodeId: event.target.value }))
                }
                placeholder="按 node_id 搜索"
              />
            </ResourceField>
            <ResourceField label="来源 IP">
              <ResourceInput
                value={draft.remoteAddr}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, remoteAddr: event.target.value }))
                }
                placeholder="按 IP 搜索"
              />
            </ResourceField>
            <ResourceField label="访问域名">
              <ResourceInput
                value={draft.host}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, host: event.target.value }))
                }
                placeholder="按域名搜索"
              />
            </ResourceField>
            <ResourceField
              label="请求路径"
              hint={activeTab === 'ip' ? 'IP 维度页暂不使用路径过滤。' : '支持按路径模糊过滤。'}
            >
              <ResourceInput
                value={draft.path}
                disabled={activeTab === 'ip'}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, path: event.target.value }))
                }
                placeholder="按路径搜索"
              />
            </ResourceField>
          </div>

          <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-4">
            <ResourceField label="每页条数">
              <ResourceSelect
                value={String(pageSize)}
                onChange={(event) => {
                  setPageSize(Number(event.target.value) || 20);
                  setDetailPage(0);
                  setIPPage(0);
                }}
              >
                {pageSizeOptions.map((option) => (
                  <option key={option} value={option}>
                    每页 {option} 条
                  </option>
                ))}
              </ResourceSelect>
            </ResourceField>

            <ResourceField label={activeTab === 'detail' && foldMinutes > 0 ? '折叠排序' : '排序'}>
              <ResourceSelect
                value={
                  activeTab === 'detail' && foldMinutes > 0
                    ? foldedSort
                    : activeTab === 'detail'
                      ? detailSort
                      : ipSort
                }
                onChange={(event) => {
                  if (activeTab === 'detail' && foldMinutes > 0) {
                    setFoldedSort(event.target.value);
                    setDetailPage(0);
                    return;
                  }
                  if (activeTab === 'detail') {
                    setDetailSort(event.target.value);
                    setDetailPage(0);
                    return;
                  }
                  setIPSort(event.target.value);
                  setIPPage(0);
                }}
              >
                {(activeTab === 'detail' && foldMinutes > 0
                  ? foldedSortOptions
                  : activeTab === 'detail'
                    ? detailSortOptions
                    : ipSortOptions
                ).map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </ResourceSelect>
            </ResourceField>

            {activeTab === 'detail' ? (
              <ResourceField label="时间折叠">
                <ResourceSelect
                  value={String(foldMinutes)}
                  onChange={(event) => {
                    setFoldMinutes(Number(event.target.value) as 0 | 3 | 5);
                    setDetailPage(0);
                  }}
                >
                  {foldOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </ResourceSelect>
              </ResourceField>
            ) : (
              <div />
            )}

            <div className="flex items-end gap-2">
              <PrimaryButton type="button" onClick={handleSearch}>
                应用筛选
              </PrimaryButton>
              <SecondaryButton type="button" onClick={handleReset}>
                重置
              </SecondaryButton>
            </div>
          </div>
        </div>
      </AppCard>

      {activeTab === 'detail' ? (
        <DetailTab
          detailPage={detailPage}
          pageSize={pageSize}
          foldMinutes={foldMinutes}
          query={detailQuery}
          onPrevPage={() => setDetailPage((value) => Math.max(value - 1, 0))}
          onNextPage={() => setDetailPage((value) => value + 1)}
        />
      ) : (
        <IPTab
          pageSize={pageSize}
          ipPage={ipPage}
          query={ipSummaryQuery}
          onPrevPage={() => setIPPage((value) => Math.max(value - 1, 0))}
          onNextPage={() => setIPPage((value) => value + 1)}
          onSelectIP={setSelectedIP}
        />
      )}

      <AppModal
        isOpen={Boolean(selectedIP)}
        title={selectedIP ? `IP 趋势 · ${selectedIP.remote_addr}` : 'IP 趋势'}
        description="展示该来源 IP 最近 24 小时的访问次数曲线，帮助判断是否存在突增、持续轰击或间歇异常。"
        size="xl"
        onClose={() => setSelectedIP(null)}
      >
        {ipTrendQuery.isLoading ? (
          <LoadingState />
        ) : ipTrendQuery.isError ? (
          <ErrorState title="IP 趋势加载失败" description={getErrorMessage(ipTrendQuery.error)} />
        ) : ipTrendQuery.data ? (
          <TrendChart
            labels={trendLabels}
            series={[
              {
                label: '访问次数',
                color: '#0f766e',
                fillColor: 'rgba(15, 118, 110, 0.16)',
                values: trendValues,
                variant: 'area',
              },
            ]}
            yAxisValueFormatter={(value) => formatCompactNumber(value)}
          />
        ) : (
          <EmptyState title="暂无趋势数据" description="当前 IP 在最近 24 小时内没有可展示的访问曲线。" />
        )}
      </AppModal>

      <AppModal
        isOpen={isCleanupModalOpen}
        title="清理访问日志"
        description="选择日志保留范围后，将删除更早的访问日志。该操作会影响当前日志检索与 IP 维度统计，请谨慎执行。"
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
              {cleanupMutation.isPending ? '清理中...' : '确认清理'}
            </PrimaryButton>
          </div>
        }
      >
        <div className="space-y-5">
          <div className="flex flex-wrap gap-2">
            {cleanupPresetOptions.map((days) => (
              <button
                key={days}
                type="button"
                onClick={() => setCleanupDays(String(days))}
                className={`rounded-2xl border px-4 py-3 text-sm transition ${
                  cleanupDays === String(days)
                    ? 'border-[var(--brand-primary)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                    : 'border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)]'
                }`}
              >
                保留最近 {days} 天
              </button>
            ))}
            <button
              type="button"
              onClick={() => setCleanupDays('custom')}
              className={`rounded-2xl border px-4 py-3 text-sm transition ${
                cleanupDays === 'custom'
                  ? 'border-[var(--brand-primary)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                  : 'border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)]'
              }`}
            >
              自定义天数
            </button>
          </div>

          {cleanupDays === 'custom' ? (
            <ResourceField label="自定义保留天数" hint="当前支持 1 到 90 天。">
              <ResourceInput
                value={customCleanupDays}
                onChange={(event) => setCustomCleanupDays(event.target.value)}
                placeholder="输入保留天数"
                type="number"
                min={1}
                max={90}
              />
            </ResourceField>
          ) : null}

          {cleanupMutation.isError ? (
            <ErrorState
              title="日志清理失败"
              description={getErrorMessage(cleanupMutation.error)}
            />
          ) : null}
        </div>
      </AppModal>
    </div>
  );
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

function DetailTab({
  detailPage,
  pageSize,
  foldMinutes,
  query,
  onPrevPage,
  onNextPage,
}: {
  detailPage: number;
  pageSize: number;
  foldMinutes: 0 | 3 | 5;
  query: {
    isLoading: boolean;
    isError: boolean;
    isFetching: boolean;
    error: unknown;
    data?: AccessLogList | FoldedAccessLogList;
  };
  onPrevPage: () => void;
  onNextPage: () => void;
}) {
  if (query.isLoading) {
    return (
      <AppCard title="访问日志" description="加载中...">
        <LoadingState />
      </AppCard>
    );
  }

  if (query.isError) {
    return (
      <AppCard title="访问日志" description="日志查询失败。">
        <ErrorState title="访问日志加载失败" description={getErrorMessage(query.error)} />
      </AppCard>
    );
  }

  if (foldMinutes > 0) {
    const data = query.data as FoldedAccessLogList;
    return (
      <AppCard
        title="折叠日志"
        description={`当前按 ${foldMinutes} 分钟时间桶折叠，适合在高频刷新时快速定位异常波段。`}
      >
        <div className="space-y-4">
          {data.items.length === 0 ? (
            <EmptyState title="暂无折叠日志" description="当前筛选条件下没有可展示的时间桶。" />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">时间桶</th>
                    <th className="px-3 py-3 font-medium">总访问</th>
                    <th className="px-3 py-3 font-medium">来源 IP</th>
                    <th className="px-3 py-3 font-medium">域名数</th>
                    <th className="px-3 py-3 font-medium">2xx</th>
                    <th className="px-3 py-3 font-medium">4xx</th>
                    <th className="px-3 py-3 font-medium">5xx</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {data.items.map((item) => (
                    <tr key={item.bucket_started_at} className="align-top">
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        <div>{formatDateTime(item.bucket_started_at)}</div>
                        <div className="mt-1 text-xs text-[var(--foreground-muted)]">
                          {formatRelativeTime(item.bucket_started_at)}
                        </div>
                      </td>
                      <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                        {formatCompactNumber(item.request_count)}
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        {formatCompactNumber(item.unique_ip_count)}
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        {formatCompactNumber(item.unique_host_count)}
                      </td>
                      <td className="px-3 py-4 text-emerald-600">
                        {formatCompactNumber(item.success_count)}
                      </td>
                      <td className="px-3 py-4 text-amber-600">
                        {formatCompactNumber(item.client_error_count)}
                      </td>
                      <td className="px-3 py-4 text-rose-600">
                        {formatCompactNumber(item.server_error_count)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <Pager
            page={detailPage}
            pageSize={pageSize}
            hasMore={data.has_more}
            isFetching={query.isFetching}
            onPrev={onPrevPage}
            onNext={onNextPage}
          />
        </div>
      </AppCard>
    );
  }

  const data = query.data as AccessLogList;
  return (
    <AppCard
      title="访问明细"
      description="展示原始访问明细，支持按时间、状态码、IP 与域名排序。"
    >
      <div className="space-y-4">
        {data.items.length === 0 ? (
          <EmptyState title="暂无访问日志" description="当前筛选条件下没有可展示的访问明细。" />
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
              <thead>
                <tr className="text-[var(--foreground-secondary)]">
                  <th className="px-3 py-3 font-medium">时间</th>
                  <th className="px-3 py-3 font-medium">来源 IP</th>
                  <th className="px-3 py-3 font-medium">访问域名</th>
                  <th className="px-3 py-3 font-medium">路径</th>
                  <th className="px-3 py-3 font-medium">节点</th>
                  <th className="px-3 py-3 font-medium">状态码</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-default)]">
                {data.items.map((item) => {
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
                        <StatusBadge label={statusMeta.label} variant={statusMeta.variant} />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
        <Pager
          page={detailPage}
          pageSize={pageSize}
          hasMore={data.has_more}
          isFetching={query.isFetching}
          onPrev={onPrevPage}
          onNext={onNextPage}
        />
      </div>
    </AppCard>
  );
}

function IPTab({
  pageSize,
  ipPage,
  query,
  onPrevPage,
  onNextPage,
  onSelectIP,
}: {
  pageSize: number;
  ipPage: number;
  query: {
    isLoading: boolean;
    isError: boolean;
    isFetching: boolean;
    error: unknown;
    data?: {
      items: AccessLogIPSummaryItem[];
      has_more: boolean;
    };
  };
  onPrevPage: () => void;
  onNextPage: () => void;
  onSelectIP: (item: AccessLogIPSummaryItem) => void;
}) {
  const items = query.data?.items ?? [];
  const hasMore = query.data?.has_more ?? false;

  return (
    <AppCard
      title="IP 维度日志"
      description="聚合展示访问过系统的来源 IP，可按访问次数或最后访问时间排序，点击行查看 24 小时趋势曲线。"
    >
      {query.isLoading ? (
        <LoadingState />
      ) : query.isError ? (
        <ErrorState title="IP 汇总加载失败" description={getErrorMessage(query.error)} />
      ) : items.length === 0 ? (
        <EmptyState title="暂无 IP 访问记录" description="当前筛选条件下没有可展示的来源 IP。" />
      ) : (
        <div className="space-y-4">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
              <thead>
                <tr className="text-[var(--foreground-secondary)]">
                  <th className="px-3 py-3 font-medium">IP</th>
                  <th className="px-3 py-3 font-medium">总访问次数</th>
                  <th className="px-3 py-3 font-medium">3 小时内访问次数</th>
                  <th className="px-3 py-3 font-medium">最后访问时间</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-default)]">
                {items.map((item) => (
                  <tr
                    key={item.remote_addr}
                    className="cursor-pointer align-top transition hover:bg-[var(--surface-elevated)]"
                    onClick={() => onSelectIP(item)}
                  >
                    <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                      {item.remote_addr}
                    </td>
                    <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                      {formatCompactNumber(item.total_requests)}
                    </td>
                    <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                      {formatCompactNumber(item.recent_requests)}
                    </td>
                    <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                      <div>{formatDateTime(item.last_seen_at)}</div>
                      <div className="mt-1 text-xs text-[var(--foreground-muted)]">
                        {formatRelativeTime(item.last_seen_at)}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pager
            page={ipPage}
            pageSize={pageSize}
            hasMore={hasMore}
            isFetching={query.isFetching}
            onPrev={onPrevPage}
            onNext={onNextPage}
          />
        </div>
      )}
    </AppCard>
  );
}

function Pager({
  page,
  pageSize,
  hasMore,
  isFetching,
  onPrev,
  onNext,
}: {
  page: number;
  pageSize: number;
  hasMore: boolean;
  isFetching: boolean;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className="flex flex-col gap-3 border-t border-[var(--border-default)] pt-4 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-sm text-[var(--foreground-secondary)]">
        第 {page + 1} 页，每页 {pageSize} 条。
      </p>
      <div className="flex gap-2">
        <SecondaryButton type="button" disabled={page === 0 || isFetching} onClick={onPrev}>
          上一页
        </SecondaryButton>
        <SecondaryButton type="button" disabled={!hasMore || isFetching} onClick={onNext}>
          下一页
        </SecondaryButton>
      </div>
    </div>
  );
}
