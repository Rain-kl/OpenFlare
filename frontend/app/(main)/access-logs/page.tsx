'use client';

import { Suspense, useCallback, useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw, ScrollText, Trash2 } from 'lucide-react';
import { toast } from 'sonner';

import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AccessLogService } from '@/lib/services/openflare';

import { AccessLogFilters } from './components/access-log-filters';
import {
  type AccessLogTab,
  type OverviewRangeHours,
  parseSortValue,
  type SearchDraft,
} from './components/access-log-utils';
import { CleanupDialog } from './components/cleanup-dialog';
import { DetailTab } from './components/detail-tab';
import {
  buildIpSummaryTimeParams,
  defaultLocalRangeForHours,
  IpTab,
  type IpTabTimeMode,
} from './components/ip-tab';
import { OverviewTab } from './components/overview-tab';

const emptyDraft: SearchDraft = {
  nodeId: '',
  remoteAddr: '',
  host: '',
  path: '',
};

function resolveTab(value: string | null): AccessLogTab {
  if (value === 'list') return 'list';
  if (value === 'ips') return 'ips';
  return 'overview';
}

function AccessLogsPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const tab = resolveTab(searchParams.get('tab'));
  const [draft, setDraft] = useState<SearchDraft>(emptyDraft);
  const [filters, setFilters] = useState<SearchDraft>(emptyDraft);
  const [pageSize, setPageSize] = useState(20);
  const [page, setPage] = useState(0);
  const [detailSort, setDetailSort] = useState('logged_at:desc');
  const [ipSort, setIpSort] = useState('total_requests:desc');
  const [ipPageSize, setIpPageSize] = useState(20);
  const [ipPage, setIpPage] = useState(0);
  const [ipHours, setIpHours] = useState<OverviewRangeHours>(168);
  const [ipTimeMode, setIpTimeMode] = useState<IpTabTimeMode>('preset');
  const [ipCustomSince, setIpCustomSince] = useState('');
  const [ipCustomUntil, setIpCustomUntil] = useState('');
  const [overviewHours, setOverviewHours] = useState<OverviewRangeHours>(24);
  const [overviewHosts, setOverviewHosts] = useState<string[]>([]);
  const [cleanupOpen, setCleanupOpen] = useState(false);

  const detailSortState = parseSortValue(detailSort);
  const ipSortState = parseSortValue(ipSort);
  const ipTimeParams = buildIpSummaryTimeParams({
    timeMode: ipTimeMode,
    hours: ipHours,
    customSince: ipCustomSince,
    customUntil: ipCustomUntil,
  });
  const ipQueryEnabled = tab === 'ips' && ipTimeParams != null;

  const handleTabChange = (value: string) => {
    const next = resolveTab(value);
    router.replace(
      next === 'overview' ? '/access-logs' : `/access-logs?tab=${next}`,
    );
  };

  const overviewQuery = useQuery({
    queryKey: [
      'openflare',
      'access-logs',
      'overview',
      overviewHours,
      overviewHosts,
    ],
    queryFn: () =>
      AccessLogService.getOverview({
        hours: overviewHours,
        hosts: overviewHosts.length > 0 ? overviewHosts : undefined,
      }),
    enabled: tab === 'overview',
  });

  const listQuery = useQuery({
    queryKey: [
      'openflare',
      'access-logs',
      'list',
      filters,
      page,
      pageSize,
      detailSort,
    ],
    queryFn: () =>
      AccessLogService.list({
        node_id: filters.nodeId || undefined,
        remote_addr: filters.remoteAddr || undefined,
        host: filters.host || undefined,
        path: filters.path || undefined,
        p: page,
        page_size: pageSize,
        sort_by: detailSortState.sortBy,
        sort_order: detailSortState.sortOrder,
      }),
    enabled: tab === 'list',
  });

  const ipQuery = useQuery({
    queryKey: [
      'openflare',
      'access-logs',
      'ip-summary',
      ipTimeParams,
      ipPage,
      ipPageSize,
      ipSort,
    ],
    queryFn: () =>
      AccessLogService.listIPSummaries({
        ...(ipTimeParams as NonNullable<typeof ipTimeParams>),
        p: ipPage,
        page_size: ipPageSize,
        sort_by: ipSortState.sortBy,
        sort_order: ipSortState.sortOrder,
      }),
    enabled: ipQueryEnabled,
  });

  const cleanupMutation = useMutation({
    mutationFn: (retentionDays: number) =>
      AccessLogService.cleanup({ retention_days: retentionDays }),
    onSuccess: async (result) => {
      toast.success(`已清理 ${result.deleted_count} 条日志`);
      setCleanupOpen(false);
      await queryClient.invalidateQueries({
        queryKey: ['openflare', 'access-logs'],
      });
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '清理失败');
    },
  });

  const handleSearch = useCallback(() => {
    setFilters({
      nodeId: draft.nodeId.trim(),
      remoteAddr: draft.remoteAddr.trim(),
      host: draft.host.trim(),
      path: draft.path.trim(),
    });
    setPage(0);
  }, [draft]);

  const handleReset = () => {
    setDraft(emptyDraft);
    setFilters(emptyDraft);
    setPage(0);
  };

  useEffect(() => {
    setPage(0);
  }, [tab, pageSize]);

  useEffect(() => {
    setIpPage(0);
  }, [
    tab,
    ipPageSize,
    ipHours,
    ipTimeMode,
    ipCustomSince,
    ipCustomUntil,
    ipSort,
  ]);

  const refreshActive = () => {
    if (tab === 'overview') void overviewQuery.refetch();
    if (tab === 'list') void listQuery.refetch();
    if (tab === 'ips') void ipQuery.refetch();
  };

  const isFetching =
    overviewQuery.isFetching || listQuery.isFetching || ipQuery.isFetching;

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-2'>
          <ScrollText className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>访问日志</h1>
            <p className='text-sm text-muted-foreground'>
              查看访问概览、IP 明细与请求日志。
            </p>
          </div>
        </div>
        <div className='flex gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={refreshActive}
            disabled={isFetching}
          >
            <RefreshCw
              className={`size-3.5 mr-1 ${isFetching ? 'animate-spin' : ''}`}
            />
            刷新
          </Button>
          <Button
            variant='destructive'
            size='sm'
            onClick={() => setCleanupOpen(true)}
          >
            <Trash2 className='size-3.5 mr-1' />
            清理日志
          </Button>
        </div>
      </div>

      <Tabs value={tab} onValueChange={handleTabChange}>
        <TabsList className='grid w-full max-w-lg grid-cols-3'>
          <TabsTrigger value='overview'>概览</TabsTrigger>
          <TabsTrigger value='ips'>IP 明细</TabsTrigger>
          <TabsTrigger value='list'>日志明细</TabsTrigger>
        </TabsList>

        <TabsContent value='overview' className='mt-4'>
          <OverviewTab
            data={overviewQuery.data}
            loading={overviewQuery.isLoading}
            error={
              overviewQuery.error instanceof Error ? overviewQuery.error : null
            }
            hours={overviewHours}
            hosts={overviewHosts}
            onHoursChange={setOverviewHours}
            onHostsChange={setOverviewHosts}
            onRetry={() => void overviewQuery.refetch()}
          />
        </TabsContent>

        <TabsContent value='ips' className='mt-4'>
          <IpTab
            data={ipQuery.data}
            loading={ipQueryEnabled && ipQuery.isLoading}
            error={ipQuery.error instanceof Error ? ipQuery.error : null}
            page={ipPage}
            pageSize={ipPageSize}
            sort={ipSort}
            hours={ipHours}
            timeMode={ipTimeMode}
            customSince={ipCustomSince}
            customUntil={ipCustomUntil}
            onHoursChange={(next) => {
              setIpTimeMode('preset');
              setIpHours(next);
            }}
            onTimeModeChange={(mode) => {
              setIpTimeMode(mode);
              if (
                mode === 'custom' &&
                (!ipCustomSince.trim() || !ipCustomUntil.trim())
              ) {
                const range = defaultLocalRangeForHours(ipHours);
                setIpCustomSince(range.since);
                setIpCustomUntil(range.until);
              }
            }}
            onApplyCustomRange={(since, until) => {
              setIpCustomSince(since);
              setIpCustomUntil(until);
              setIpTimeMode('custom');
              setIpPage(0);
            }}
            onPageSizeChange={setIpPageSize}
            onSortChange={setIpSort}
            onRetry={() => void ipQuery.refetch()}
            onPrevPage={() => setIpPage((p) => Math.max(0, p - 1))}
            onNextPage={() => setIpPage((p) => p + 1)}
            isFetching={ipQuery.isFetching}
          />
        </TabsContent>

        <TabsContent value='list' className='mt-4 space-y-4'>
          <div className='rounded-lg border border-dashed bg-background p-4'>
            <AccessLogFilters
              draft={draft}
              pageSize={pageSize}
              onDraftChange={setDraft}
              onPageSizeChange={setPageSize}
              onSearch={handleSearch}
              onReset={handleReset}
            />
          </div>
          <DetailTab
            data={listQuery.data}
            loading={listQuery.isLoading}
            error={listQuery.error instanceof Error ? listQuery.error : null}
            page={page}
            detailSort={detailSort}
            onDetailSortChange={setDetailSort}
            onRetry={() => void listQuery.refetch()}
            onPrevPage={() => setPage((p) => Math.max(0, p - 1))}
            onNextPage={() => setPage((p) => p + 1)}
            isFetching={listQuery.isFetching}
          />
        </TabsContent>
      </Tabs>

      <CleanupDialog
        open={cleanupOpen}
        onOpenChange={setCleanupOpen}
        onConfirm={(days) => cleanupMutation.mutate(days)}
        loading={cleanupMutation.isPending}
      />
    </div>
  );
}

export default function AccessLogsPage() {
  return (
    <Suspense
      fallback={
        <LoadingStateWithBorder
          title='加载访问日志'
          description='正在准备页面...'
        />
      }
    >
      <AccessLogsPageContent />
    </Suspense>
  );
}
