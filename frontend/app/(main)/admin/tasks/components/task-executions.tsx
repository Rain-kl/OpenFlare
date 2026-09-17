'use client';

import { useMemo, useState } from 'react';
import { useTranslations } from 'next-intl';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { format } from 'date-fns';
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  RotateCcw,
} from 'lucide-react';

import services from '@/lib/services';
import type {
  TaskExecution,
  TaskExecutionStatus,
  TaskMeta,
} from '@/lib/services/admin';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
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

const STATUS_LABELS_KEYS: Record<TaskExecutionStatus, string> = {
  pending: 'statusPending',
  running: 'statusRunning',
  succeeded: 'statusSucceeded',
  failed: 'statusFailed',
};

const TRIGGER_LABELS_KEYS: Record<string, string> = {
  system: 'triggerSystem',
  manual: 'triggerManual',
  retry: 'triggerRetry',
  schedule: 'triggerSchedule',
};

const PAGE_SIZE = 10;

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
  if (status === 'failed') return 'destructive';
  if (status === 'succeeded') return 'secondary';
  return 'outline';
}

function mappedLabel(
  t: (key: string) => string,
  keys: Record<string, string>,
  value: string,
): string {
  const key = keys[value];
  if (!key) return value || '-';
  return t(key);
}

export function TaskExecutionsManager() {
  const t = useTranslations('admin.tasks');
  const queryClient = useQueryClient();
  const [executionsPage, setExecutionsPage] = useState(1);
  const [executionStatus, setExecutionStatus] = useState<
    TaskExecutionStatus | 'all'
  >('all');
  const [executionTaskType, setExecutionTaskType] = useState<string>('all');
  const [selectedExecutionId, setSelectedExecutionId] = useState<string | null>(
    null,
  );
  const [executionPreview, setExecutionPreview] =
    useState<TaskExecution | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const taskTypesQuery = useQuery({
    queryKey: ['admin', 'task-types'],
    queryFn: () => services.adminTask.getTaskTypes(),
  });

  const executionsQuery = useQuery({
    queryKey: [
      'admin',
      'task-executions',
      executionsPage,
      executionStatus,
      executionTaskType,
    ],
    queryFn: () =>
      services.adminTask.listTaskExecutions({
        page: executionsPage,
        page_size: PAGE_SIZE,
        status: executionStatus === 'all' ? undefined : executionStatus,
        task_type: executionTaskType === 'all' ? undefined : executionTaskType,
      }),
  });

  const detailQuery = useQuery({
    queryKey: ['admin', 'task-execution', selectedExecutionId],
    queryFn: () => services.adminTask.getTaskExecution(selectedExecutionId!),
    enabled: detailOpen && !!selectedExecutionId,
  });

  const retryMutation = useMutation({
    mutationFn: (id: string) => services.adminTask.retryTaskExecution(id),
    onSuccess: (taskID) => {
      toast.success(t('retrySuccess'), {
        description: t('retrySuccessDesc', { taskId: taskID }),
      });
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'task-executions'],
      });
    },
    onError: (err: Error) => {
      toast.error(t('retryFailed'), {
        description: err.message || t('unknownError'),
      });
    },
  });

  const taskTypes: TaskMeta[] = taskTypesQuery.data ?? [];
  const executions = executionsQuery.data?.items ?? [];
  const executionsTotal = executionsQuery.data?.total ?? 0;
  const executionsLoading =
    executionsQuery.isPending || executionsQuery.isFetching;
  const executionsError = executionsQuery.isError
    ? executionsQuery.error
    : null;
  const selectedExecution = detailQuery.data ?? executionPreview;
  const detailLoading = detailQuery.isFetching;

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(executionsTotal / PAGE_SIZE)),
    [executionsTotal],
  );

  const openExecutionDetail = (execution: TaskExecution) => {
    setExecutionPreview(execution);
    setSelectedExecutionId(execution.id);
    setDetailOpen(true);
  };

  const handleStatusChange = (value: TaskExecutionStatus | 'all') => {
    setExecutionStatus(value);
    setExecutionsPage(1);
  };

  const handleTaskTypeChange = (value: string) => {
    setExecutionTaskType(value);
    setExecutionsPage(1);
  };

  return (
    <div className='space-y-6'>
      <br />
      <div className='flex flex-col gap-3 border-b border-border pb-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex flex-wrap items-center gap-2'>
          <Select
            value={executionStatus}
            onValueChange={(value) =>
              handleStatusChange(value as TaskExecutionStatus | 'all')
            }
          >
            <SelectTrigger size='sm' className='w-[120px]'>
              <SelectValue placeholder={t('colStatus')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>{t('allStatuses')}</SelectItem>
              <SelectItem value='pending'>{t('statusPending')}</SelectItem>
              <SelectItem value='running'>{t('statusRunning')}</SelectItem>
              <SelectItem value='succeeded'>{t('statusSucceeded')}</SelectItem>
              <SelectItem value='failed'>{t('statusFailed')}</SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={executionTaskType}
            onValueChange={handleTaskTypeChange}
          >
            <SelectTrigger size='sm' className='w-[180px]'>
              <SelectValue placeholder={t('taskTypeFilter')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>{t('allTasks')}</SelectItem>
              {taskTypes.map((task) => (
                <SelectItem key={task.type} value={task.type}>
                  {task.name || task.type}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void executionsQuery.refetch()}
            disabled={executionsLoading}
          >
            {executionsLoading ? (
              <Spinner className='size-4' />
            ) : (
              <RefreshCw className='size-4' />
            )}
            {t('refresh')}
          </Button>
        </div>
      </div>

      {executionsError ? (
        <div className='p-8 border border-dashed rounded-lg bg-card'>
          <ErrorInline
            error={executionsError}
            onRetry={() => void executionsQuery.refetch()}
            className='justify-center'
          />
        </div>
      ) : executionsLoading && executions.length === 0 ? (
        <LoadingStateWithBorder
          icon={Activity}
          description={t('loadingExecutions')}
        />
      ) : executions.length === 0 ? (
        <EmptyStateWithBorder icon={Activity} description={t('noExecutions')} />
      ) : (
        <div className='rounded-lg border bg-card'>
          <Table className='min-w-[900px]'>
            <TableHeader>
              <TableRow className='hover:bg-transparent'>
                <TableHead className='w-[180px]'>{t('colTask')}</TableHead>
                <TableHead className='w-[100px]'>{t('colStatus')}</TableHead>
                <TableHead className='w-[110px]'>{t('colTrigger')}</TableHead>
                <TableHead className='w-[120px]'>{t('colRetry')}</TableHead>
                <TableHead className='w-[120px]'>{t('colDuration')}</TableHead>
                <TableHead className='min-w-[220px]'>
                  {t('colResult')}
                </TableHead>
                <TableHead className='w-[170px]'>{t('colCreatedAt')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {executions.map((execution) => (
                <TableRow
                  key={execution.id}
                  className='cursor-pointer'
                  onClick={() => openExecutionDetail(execution)}
                >
                  <TableCell>
                    <div className='flex flex-col gap-1'>
                      <span className='text-sm font-medium'>
                        {execution.task_name || execution.task_type}
                      </span>
                      <span className='font-mono text-[11px] text-muted-foreground'>
                        {execution.task_id}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(execution.status)}>
                      {mappedLabel(t, STATUS_LABELS_KEYS, execution.status)}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant='outline'>
                      {mappedLabel(
                        t,
                        TRIGGER_LABELS_KEYS,
                        execution.triggered_by,
                      )}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-mono text-xs text-muted-foreground'>
                    {execution.retry_count}/{execution.max_retry}
                  </TableCell>
                  <TableCell className='font-mono text-xs text-muted-foreground'>
                    {formatDuration(execution.duration)}
                  </TableCell>
                  <TableCell className='max-w-[320px] truncate text-xs text-muted-foreground'>
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

      <div className='flex items-center justify-between'>
        <div className='text-xs text-muted-foreground'>
          {t('paginationInfo', {
            total: executionsTotal,
            current: executionsPage,
            totalPages,
          })}
        </div>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setExecutionsPage((page) => Math.max(1, page - 1))}
            disabled={executionsPage <= 1 || executionsLoading}
          >
            <ChevronLeft className='size-4' />
            {t('prevPage')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() =>
              setExecutionsPage((page) => Math.min(totalPages, page + 1))
            }
            disabled={executionsPage >= totalPages || executionsLoading}
          >
            {t('nextPage')}
            <ChevronRight className='size-4' />
          </Button>
        </div>
      </div>

      <Sheet open={detailOpen} onOpenChange={setDetailOpen}>
        <SheetContent className='w-full p-0 sm:max-w-[640px]'>
          <SheetHeader className='border-b'>
            <SheetTitle>{t('executionDetailTitle')}</SheetTitle>
            <SheetDescription>
              {selectedExecution?.task_name ||
                selectedExecution?.task_type ||
                t('executionRecord')}
            </SheetDescription>
          </SheetHeader>

          <div className='flex-1 overflow-y-auto px-4 pb-4'>
            {detailLoading && !selectedExecution ? (
              <LoadingStateWithBorder
                icon={Activity}
                description={t('loadingDetail')}
              />
            ) : selectedExecution ? (
              <div className='space-y-5 py-4'>
                <div className='grid grid-cols-2 gap-3'>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('detailStatus')}
                    </div>
                    <div className='mt-2'>
                      <Badge variant={statusVariant(selectedExecution.status)}>
                        {mappedLabel(
                          t,
                          STATUS_LABELS_KEYS,
                          selectedExecution.status,
                        )}
                      </Badge>
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('detailTrigger')}
                    </div>
                    <div className='mt-2 text-sm font-medium'>
                      {mappedLabel(
                        t,
                        TRIGGER_LABELS_KEYS,
                        selectedExecution.triggered_by,
                      )}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('detailRetryCount')}
                    </div>
                    <div className='mt-2 font-mono text-sm'>
                      {selectedExecution.retry_count}/
                      {selectedExecution.max_retry}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-xs text-muted-foreground'>
                      {t('detailDuration')}
                    </div>
                    <div className='mt-2 font-mono text-sm'>
                      {formatDuration(selectedExecution.duration)}
                    </div>
                  </div>
                </div>

                <div className='grid gap-2'>
                  <Label>{t('detailTaskId')}</Label>
                  <div className='rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs break-all'>
                    {selectedExecution.task_id}
                  </div>
                </div>

                <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                  <div className='grid gap-2'>
                    <Label>{t('detailCreatedAt')}</Label>
                    <div className='font-mono text-xs text-muted-foreground'>
                      {formatDateTime(selectedExecution.created_at)}
                    </div>
                  </div>
                  <div className='grid gap-2'>
                    <Label>{t('detailStartedAt')}</Label>
                    <div className='font-mono text-xs text-muted-foreground'>
                      {formatDateTime(selectedExecution.started_at)}
                    </div>
                  </div>
                  <div className='grid gap-2'>
                    <Label>{t('detailFinishedAt')}</Label>
                    <div className='font-mono text-xs text-muted-foreground'>
                      {formatDateTime(selectedExecution.finished_at)}
                    </div>
                  </div>
                  <div className='grid gap-2'>
                    <Label>{t('detailUpdatedAt')}</Label>
                    <div className='font-mono text-xs text-muted-foreground'>
                      {formatDateTime(selectedExecution.updated_at)}
                    </div>
                  </div>
                </div>

                <div className='grid gap-2'>
                  <Label>{t('detailResult')}</Label>
                  <div className='min-h-10 rounded-md border bg-muted/30 px-3 py-2 text-sm whitespace-pre-wrap break-all'>
                    {selectedExecution.result || '-'}
                  </div>
                </div>

                {selectedExecution.error_message && (
                  <div className='grid gap-2'>
                    <Label>{t('detailError')}</Label>
                    <div className='rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive whitespace-pre-wrap break-all'>
                      {selectedExecution.error_message}
                    </div>
                  </div>
                )}

                <div className='grid gap-2'>
                  <Label>{t('detailPayload')}</Label>
                  <pre className='max-h-40 overflow-auto rounded-md border bg-muted/40 p-3 text-xs leading-relaxed'>
                    {selectedExecution.payload || '{}'}
                  </pre>
                </div>

                <div className='grid gap-2'>
                  <Label>{t('detailLog')}</Label>
                  <pre className='min-h-48 max-h-[420px] overflow-auto rounded-md border bg-muted/40 p-3 text-xs leading-relaxed whitespace-pre-wrap'>
                    {selectedExecution.log || t('noLog')}
                  </pre>
                </div>
              </div>
            ) : (
              <EmptyStateWithBorder
                icon={Activity}
                description={t('noExecutionSelected')}
              />
            )}
          </div>

          <SheetFooter className='border-t'>
            <Button
              variant='outline'
              onClick={() => void detailQuery.refetch()}
              disabled={!selectedExecutionId || detailLoading}
            >
              {detailLoading ? (
                <Spinner className='size-4' />
              ) : (
                <RefreshCw className='size-4' />
              )}
              {t('refreshDetail')}
            </Button>
            {selectedExecution &&
              selectedExecution.status === 'failed' &&
              selectedExecution.retryable && (
                <Button
                  onClick={() => retryMutation.mutate(selectedExecution.id)}
                  disabled={retryMutation.isPending}
                >
                  {retryMutation.isPending ? (
                    <Spinner className='size-4' />
                  ) : (
                    <RotateCcw className='size-4' />
                  )}
                  {t('retryTask')}
                </Button>
              )}
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  );
}
