'use client';

import { useCallback, useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw, ScrollText, Trash2 } from 'lucide-react';
import { toast } from 'sonner';

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
import { OverviewTab } from './components/overview-tab';

const emptyDraft: SearchDraft = {
  nodeId: '',
  remoteAddr: '',
  host: '',
  path: '',
};

export default function AccessLogsPage() {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<AccessLogTab>('overview');
  const [draft, setDraft] = useState<SearchDraft>(emptyDraft);
  const [filters, setFilters] = useState<SearchDraft>(emptyDraft);
  const [pageSize, setPageSize] = useState(20);
  const [page, setPage] = useState(0);
  const [detailSort, setDetailSort] = useState('logged_at:desc');
  const [overviewHours, setOverviewHours] = useState<OverviewRangeHours>(24);
  const [cleanupOpen, setCleanupOpen] = useState(false);

  const detailSortState = parseSortValue(detailSort);

  const overviewQuery = useQuery({
    queryKey: ['openflare', 'access-logs', 'overview', overviewHours],
    queryFn: () =>
      AccessLogService.getOverview({
        hours: overviewHours,
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

  const refreshActive = () => {
    if (tab === 'overview') void overviewQuery.refetch();
    if (tab === 'list') void listQuery.refetch();
  };

  const isFetching = overviewQuery.isFetching || listQuery.isFetching;

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-2'>
          <ScrollText className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>访问日志</h1>
            <p className='text-sm text-muted-foreground'>
              查看访问概览、排行榜与明细日志。
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

      <Tabs
        value={tab}
        onValueChange={(value) => setTab(value as AccessLogTab)}
      >
        <TabsList className='grid w-full max-w-md grid-cols-2'>
          <TabsTrigger value='overview'>概览</TabsTrigger>
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
            onHoursChange={setOverviewHours}
            onRetry={() => void overviewQuery.refetch()}
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
