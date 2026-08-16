'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { format } from 'date-fns';
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  RotateCcw,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useMemo, useState } from 'react';
import { toast } from 'sonner';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { AdminTaskService } from '@/lib/services/admin';
import type {
  ListTaskExecutionsResponse,
  TaskExecution,
  TaskExecutionStatus,
} from '@/lib/services/admin';
import { cloudflareQueryKey } from '@/lib/services/openflare';

/**
 * Only domain-member and group sync jobs.
 * Uses exact asynq task_type filters (compatible with existing admin API).
 * Excludes cloudflare:sync_by_node and any unrelated scheduled system tasks.
 */
export const CLOUDFLARE_SYNC_TASK_TYPES = [
  'cloudflare:sync_member',
  'cloudflare:sync_group',
] as const;

const ALLOWED_TASK_TYPES = new Set<string>(CLOUDFLARE_SYNC_TASK_TYPES);

/** Per-type fetch window; merged and paginated on the client. */
const FETCH_PAGE_SIZE = 50;
const PAGE_SIZE = 10;

async function listCloudflareSyncExecutions(options: {
  page: number;
  status?: TaskExecutionStatus;
}): Promise<ListTaskExecutionsResponse> {
  const status = options.status;
  const responses = await Promise.all(
    CLOUDFLARE_SYNC_TASK_TYPES.map((taskType) =>
      AdminTaskService.listTaskExecutions({
        task_type: taskType,
        page: 1,
        page_size: FETCH_PAGE_SIZE,
        status,
      }),
    ),
  );

  const merged = responses
    .flatMap((response) => response.items)
    .filter((item) => ALLOWED_TASK_TYPES.has(item.task_type))
    .sort((a, b) => {
      const timeA = Date.parse(a.created_at) || 0;
      const timeB = Date.parse(b.created_at) || 0;
      if (timeA !== timeB) return timeB - timeA;
      return String(b.id).localeCompare(String(a.id), undefined, {
        numeric: true,
      });
    });

  const start = (options.page - 1) * PAGE_SIZE;
  return {
    items: merged.slice(start, start + PAGE_SIZE),
    total: merged.length,
    page: options.page,
    page_size: PAGE_SIZE,
  };
}

function formatDateTime(value?: string | null) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return format(date, 'yyyy-MM-dd HH:mm:ss');
}

function formatDuration(duration: number) {
  if (!duration) return '-';
  if (duration < 1000) return `${duration}ms`;
  return `${(duration / 1000).toFixed(2)}s`;
}

function statusVariant(status: TaskExecutionStatus) {
  if (status === 'failed') return 'destructive' as const;
  if (status === 'succeeded') return 'secondary' as const;
  return 'outline' as const;
}

export function SyncTasksPanel() {
  const t = useTranslations('cloudflare.tasks');
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState<TaskExecutionStatus | 'all'>('all');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [preview, setPreview] = useState<TaskExecution | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const executionsQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'sync-executions', page, status],
    queryFn: () =>
      listCloudflareSyncExecutions({
        page,
        status: status === 'all' ? undefined : status,
      }),
    refetchInterval: (query) => {
      const items = query.state.data?.items ?? [];
      const active = items.some(
        (item) => item.status === 'pending' || item.status === 'running',
      );
      return active ? 3000 : 15000;
    },
  });

  const detailQuery = useQuery({
    queryKey: ['admin', 'task-execution', selectedId],
    queryFn: () => AdminTaskService.getTaskExecution(selectedId!),
    enabled: detailOpen && !!selectedId,
  });

  const retryMutation = useMutation({
    mutationFn: (id: string) => AdminTaskService.retryTaskExecution(id),
    onSuccess: (taskID) => {
      toast.success(t('retried'), {
        description: t('newTaskId', { id: taskID }),
      });
      void queryClient.invalidateQueries({
        queryKey: [...cloudflareQueryKey, 'sync-executions'],
      });
    },
    onError: (err: Error) => {
      toast.error(t('retryFailed'), {
        description: err.message || t('unknownError'),
      });
    },
  });

  const executions = executionsQuery.data?.items ?? [];
  const total = executionsQuery.data?.total ?? 0;
  const loading = executionsQuery.isPending || executionsQuery.isFetching;
  const selected = detailQuery.data ?? preview;
  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(total / PAGE_SIZE)),
    [total],
  );
  const statusLabels: Record<TaskExecutionStatus, string> = {
    pending: t('pending'),
    running: t('running'),
    succeeded: t('succeeded'),
    failed: t('failed'),
  };
  const triggerLabels: Record<string, string> = {
    system: t('system'),
    manual: t('manual'),
    retry: t('retry'),
    schedule: t('schedule'),
  };

  const openDetail = (execution: TaskExecution) => {
    setPreview(execution);
    setSelectedId(execution.id);
    setDetailOpen(true);
  };

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
        <div className='space-y-1'>
          <CardTitle className='flex items-center gap-2 text-base'>
            <Activity className='size-4 text-primary' />
            {t('title')}
          </CardTitle>
          <CardDescription>{t('description')}</CardDescription>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Select
            value={status}
            onValueChange={(value) => {
              setStatus(value as TaskExecutionStatus | 'all');
              setPage(1);
            }}
          >
            <SelectTrigger size='sm' className='w-[120px]'>
              <SelectValue placeholder={t('statusPlaceholder')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>{t('allStatus')}</SelectItem>
              <SelectItem value='pending'>{t('pending')}</SelectItem>
              <SelectItem value='running'>{t('running')}</SelectItem>
              <SelectItem value='succeeded'>{t('succeeded')}</SelectItem>
              <SelectItem value='failed'>{t('failed')}</SelectItem>
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void executionsQuery.refetch()}
            disabled={loading}
          >
            {loading ? (
              <Spinner className='size-4' />
            ) : (
              <RefreshCw data-icon='inline-start' />
            )}
            {t('refresh')}
          </Button>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        {executionsQuery.isError ? (
          <ErrorInline
            message={
              executionsQuery.error instanceof Error
                ? executionsQuery.error.message
                : t('loadFailed')
            }
            onRetry={() => void executionsQuery.refetch()}
          />
        ) : loading && executions.length === 0 ? (
          <LoadingStateWithBorder
            icon={Activity}
            description={t('loading')}
          />
        ) : executions.length === 0 ? (
          <EmptyStateWithBorder
            icon={Activity}
            title={t('emptyTitle')}
            description={t('emptyDesc')}
          />
        ) : (
          <div className='rounded-lg border'>
            <Table className='min-w-[720px]'>
              <TableHeader>
                <TableRow className='hover:bg-transparent'>
                  <TableHead className='w-[200px]'>
                    {t('columns.task')}
                  </TableHead>
                  <TableHead className='w-[100px]'>
                    {t('columns.status')}
                  </TableHead>
                  <TableHead className='w-[100px]'>
                    {t('columns.trigger')}
                  </TableHead>
                  <TableHead className='w-[100px]'>
                    {t('columns.duration')}
                  </TableHead>
                  <TableHead className='min-w-[180px]'>
                    {t('columns.result')}
                  </TableHead>
                  <TableHead className='w-[160px]'>
                    {t('columns.createdAt')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {executions.map((execution) => (
                  <TableRow
                    key={execution.id}
                    className='cursor-pointer'
                    onClick={() => openDetail(execution)}
                  >
                    <TableCell>
                      <div className='flex flex-col gap-1'>
                        <span className='text-sm font-medium'>
                          {execution.task_name || execution.task_type}
                        </span>
                        <span className='font-mono text-[11px] text-muted-foreground'>
                          {execution.task_type}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(execution.status)}>
                        {statusLabels[execution.status] || execution.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>
                        {triggerLabels[execution.triggered_by] ||
                          execution.triggered_by}
                      </Badge>
                    </TableCell>
                    <TableCell className='font-mono text-xs text-muted-foreground'>
                      {formatDuration(execution.duration)}
                    </TableCell>
                    <TableCell className='max-w-[280px] truncate text-xs text-muted-foreground'>
                      {execution.error_message || execution.result || '-'}
                    </TableCell>
                    <TableCell className='font-mono text-[11px] text-muted-foreground'>
                      {formatDateTime(execution.created_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        <div className='flex items-center justify-between gap-2'>
          <div className='text-xs text-muted-foreground'>
            {t('pageSummary', { total, page, pages: totalPages })}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1 || loading}
            >
              <ChevronLeft className='size-4' />
              {t('prev')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages || loading}
            >
              {t('next')}
              <ChevronRight className='size-4' />
            </Button>
          </div>
        </div>
      </CardContent>

      <Sheet open={detailOpen} onOpenChange={setDetailOpen}>
        <SheetContent className='w-full p-0 sm:max-w-[560px]'>
          <SheetHeader className='border-b'>
            <SheetTitle>{t('detailTitle')}</SheetTitle>
            <SheetDescription>
              {selected?.task_name || selected?.task_type || t('defaultName')}
            </SheetDescription>
          </SheetHeader>
          <div className='flex-1 space-y-4 overflow-y-auto px-4 py-4'>
            {detailQuery.isFetching && !selected ? (
              <LoadingStateWithBorder
                icon={Activity}
                description={t('loadingDetail')}
              />
            ) : selected ? (
              <>
                <div className='grid grid-cols-2 gap-3'>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('status')}
                    </div>
                    <div className='mt-2'>
                      <Badge variant={statusVariant(selected.status)}>
                        {statusLabels[selected.status] || selected.status}
                      </Badge>
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('trigger')}
                    </div>
                    <div className='mt-2 text-sm font-medium'>
                      {triggerLabels[selected.triggered_by] ||
                        selected.triggered_by}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('retries')}
                    </div>
                    <div className='mt-2 font-mono text-sm'>
                      {selected.retry_count}/{selected.max_retry}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('duration')}
                    </div>
                    <div className='mt-2 font-mono text-sm'>
                      {formatDuration(selected.duration)}
                    </div>
                  </div>
                </div>

                <div className='space-y-1'>
                  <div className='text-xs text-muted-foreground'>
                    {t('taskId')}
                  </div>
                  <div className='rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs break-all'>
                    {selected.task_id}
                  </div>
                </div>

                <div className='grid gap-3 sm:grid-cols-2'>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('createdAt')}
                    </div>
                    <div className='font-mono text-xs'>
                      {formatDateTime(selected.created_at)}
                    </div>
                  </div>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('finishedAt')}
                    </div>
                    <div className='font-mono text-xs'>
                      {formatDateTime(selected.finished_at)}
                    </div>
                  </div>
                </div>

                <div className='space-y-1'>
                  <div className='text-xs text-muted-foreground'>
                    {t('result')}
                  </div>
                  <div className='min-h-10 rounded-md border bg-muted/30 px-3 py-2 text-sm whitespace-pre-wrap break-all'>
                    {selected.result || '-'}
                  </div>
                </div>

                {selected.error_message ? (
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('error')}
                    </div>
                    <div className='rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive whitespace-pre-wrap break-all'>
                      {selected.error_message}
                    </div>
                  </div>
                ) : null}

                <div className='space-y-1'>
                  <div className='text-xs text-muted-foreground'>Payload</div>
                  <pre className='max-h-36 overflow-auto rounded-md border bg-muted/40 p-3 text-xs leading-relaxed'>
                    {selected.payload || '{}'}
                  </pre>
                </div>

                {selected.log ? (
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('logs')}
                    </div>
                    <pre className='max-h-48 overflow-auto rounded-md border bg-muted/40 p-3 text-xs leading-relaxed'>
                      {selected.log}
                    </pre>
                  </div>
                ) : null}
              </>
            ) : null}
          </div>
          <SheetFooter className='border-t'>
            {selected?.retryable && selected.status === 'failed' ? (
              <Button
                variant='outline'
                disabled={retryMutation.isPending}
                onClick={() => retryMutation.mutate(selected.id)}
              >
                {retryMutation.isPending ? (
                  <Spinner className='size-4' />
                ) : (
                  <RotateCcw data-icon='inline-start' />
                )}
                {t('retry')}
              </Button>
            ) : null}
            <Button variant='outline' onClick={() => setDetailOpen(false)}>
              {t('close')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </Card>
  );
}
