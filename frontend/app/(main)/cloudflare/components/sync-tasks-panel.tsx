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

const STATUS_LABELS: Record<TaskExecutionStatus, string> = {
  pending: '等待中',
  running: '执行中',
  succeeded: '成功',
  failed: '失败',
};

const TRIGGER_LABELS: Record<string, string> = {
  system: '系统',
  manual: '手动',
  retry: '重试',
  schedule: '定时',
};

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
      toast.success('同步任务已重新下发', {
        description: `新任务 ID：${taskID}`,
      });
      void queryClient.invalidateQueries({
        queryKey: [...cloudflareQueryKey, 'sync-executions'],
      });
    },
    onError: (err: Error) => {
      toast.error('任务重试失败', {
        description: err.message || '未知错误',
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
            同步任务
          </CardTitle>
          <CardDescription>
            仅展示 Cloudflare
            域名同步（sync_member）与分组同步（sync_group）的执行记录。
          </CardDescription>
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
              <SelectValue placeholder='状态' />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>全部状态</SelectItem>
              <SelectItem value='pending'>等待中</SelectItem>
              <SelectItem value='running'>执行中</SelectItem>
              <SelectItem value='succeeded'>成功</SelectItem>
              <SelectItem value='failed'>失败</SelectItem>
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
            刷新
          </Button>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        {executionsQuery.isError ? (
          <ErrorInline
            message={
              executionsQuery.error instanceof Error
                ? executionsQuery.error.message
                : '加载同步任务失败'
            }
            onRetry={() => void executionsQuery.refetch()}
          />
        ) : loading && executions.length === 0 ? (
          <LoadingStateWithBorder
            icon={Activity}
            description='加载同步任务中...'
          />
        ) : executions.length === 0 ? (
          <EmptyStateWithBorder
            icon={Activity}
            title='暂无同步任务'
            description='添加域名或同步分组后，这里会显示 Cloudflare 域名 / 分组同步执行记录。'
          />
        ) : (
          <div className='rounded-lg border'>
            <Table className='min-w-[720px]'>
              <TableHeader>
                <TableRow className='hover:bg-transparent'>
                  <TableHead className='w-[200px]'>任务</TableHead>
                  <TableHead className='w-[100px]'>状态</TableHead>
                  <TableHead className='w-[100px]'>触发</TableHead>
                  <TableHead className='w-[100px]'>耗时</TableHead>
                  <TableHead className='min-w-[180px]'>结果/错误</TableHead>
                  <TableHead className='w-[160px]'>创建时间</TableHead>
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
                        {STATUS_LABELS[execution.status] || execution.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>
                        {TRIGGER_LABELS[execution.triggered_by] ||
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
            共 {total} 条，第 {page}/{totalPages} 页
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1 || loading}
            >
              <ChevronLeft className='size-4' />
              上一页
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages || loading}
            >
              下一页
              <ChevronRight className='size-4' />
            </Button>
          </div>
        </div>
      </CardContent>

      <Sheet open={detailOpen} onOpenChange={setDetailOpen}>
        <SheetContent className='w-full p-0 sm:max-w-[560px]'>
          <SheetHeader className='border-b'>
            <SheetTitle>同步任务详情</SheetTitle>
            <SheetDescription>
              {selected?.task_name || selected?.task_type || 'Cloudflare 同步'}
            </SheetDescription>
          </SheetHeader>
          <div className='flex-1 space-y-4 overflow-y-auto px-4 py-4'>
            {detailQuery.isFetching && !selected ? (
              <LoadingStateWithBorder
                icon={Activity}
                description='加载任务详情中...'
              />
            ) : selected ? (
              <>
                <div className='grid grid-cols-2 gap-3'>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>状态</div>
                    <div className='mt-2'>
                      <Badge variant={statusVariant(selected.status)}>
                        {STATUS_LABELS[selected.status] || selected.status}
                      </Badge>
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>触发</div>
                    <div className='mt-2 text-sm font-medium'>
                      {TRIGGER_LABELS[selected.triggered_by] ||
                        selected.triggered_by}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>重试</div>
                    <div className='mt-2 font-mono text-sm'>
                      {selected.retry_count}/{selected.max_retry}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>耗时</div>
                    <div className='mt-2 font-mono text-sm'>
                      {formatDuration(selected.duration)}
                    </div>
                  </div>
                </div>

                <div className='space-y-1'>
                  <div className='text-xs text-muted-foreground'>任务 ID</div>
                  <div className='rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs break-all'>
                    {selected.task_id}
                  </div>
                </div>

                <div className='grid gap-3 sm:grid-cols-2'>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      创建时间
                    </div>
                    <div className='font-mono text-xs'>
                      {formatDateTime(selected.created_at)}
                    </div>
                  </div>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      结束时间
                    </div>
                    <div className='font-mono text-xs'>
                      {formatDateTime(selected.finished_at)}
                    </div>
                  </div>
                </div>

                <div className='space-y-1'>
                  <div className='text-xs text-muted-foreground'>执行结果</div>
                  <div className='min-h-10 rounded-md border bg-muted/30 px-3 py-2 text-sm whitespace-pre-wrap break-all'>
                    {selected.result || '-'}
                  </div>
                </div>

                {selected.error_message ? (
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      错误信息
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
                      执行日志
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
                重试
              </Button>
            ) : null}
            <Button variant='outline' onClick={() => setDetailOpen(false)}>
              关闭
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </Card>
  );
}
