'use client';

import { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Download, Pencil, RefreshCw, RotateCcw } from 'lucide-react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Spinner } from '@/components/ui/spinner';
import { AdminTaskService } from '@/lib/services/admin';
import {
  type PagesSource,
  type PagesSourceActionReceipt,
  type PagesSourceStatus,
  PagesService,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import {
  deploymentsQueryKey,
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../../components/pages-utils';
import { PagesSourceDialog } from './pages-source-dialog';

const ACTION_POLL_INTERVAL = 2_000;
const ACTION_MAX_WAIT = 16 * 60 * 1_000;

const SOURCE_STATUS: Record<
  PagesSourceStatus,
  {
    label: string;
    variant: 'default' | 'secondary' | 'destructive' | 'outline';
  }
> = {
  idle: { label: '空闲', variant: 'outline' },
  checking: { label: '检查中', variant: 'secondary' },
  update_available: { label: '有可用更新', variant: 'default' },
  syncing: { label: '同步中', variant: 'secondary' },
  failed: { label: '最近同步失败', variant: 'destructive' },
  attention: { label: '需要确认', variant: 'destructive' },
};

interface ActiveAction {
  receipt: PagesSourceActionReceipt;
  startedAt: number;
}

function revisionSummary(source: PagesSource) {
  if (source.source_type === 'manual' || !source.last_applied)
    return '尚未应用';
  return `${source.last_applied.label} · ${source.last_applied.revision.slice(0, 12)}`;
}

export function PagesSourceCard({ projectId }: { projectId: number }) {
  const queryClient = useQueryClient();
  const handledExecutionID = useRef<string | null>(null);
  const sourcePollingStartedAt = useRef<number | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<'manual' | 'remote_url'>(
    'manual',
  );
  const [activeAction, setActiveAction] = useState<ActiveAction | null>(null);
  const [actionTimedOut, setActionTimedOut] = useState(false);

  const sourceQuery = useQuery({
    queryKey: sourceQueryKey(projectId),
    queryFn: () => PagesService.getSource(projectId),
    refetchInterval: (query) => {
      const source = query.state.data;
      if (
        source &&
        source.source_type !== 'manual' &&
        (source.sync_status === 'checking' || source.sync_status === 'syncing')
      ) {
        sourcePollingStartedAt.current ??= Date.now();
        return Date.now() - sourcePollingStartedAt.current < ACTION_MAX_WAIT
          ? ACTION_POLL_INTERVAL
          : false;
      }
      sourcePollingStartedAt.current = null;
      return false;
    },
  });

  const executionQuery = useQuery({
    queryKey: [
      'admin',
      'task-execution',
      activeAction?.receipt.execution_id ?? '',
    ],
    queryFn: () =>
      AdminTaskService.getTaskExecution(activeAction!.receipt.execution_id),
    enabled: Boolean(activeAction) && !actionTimedOut,
    refetchInterval: (query) => {
      if (actionTimedOut) return false;
      const status = query.state.data?.status;
      return status === 'pending' || status === 'running'
        ? ACTION_POLL_INTERVAL
        : false;
    },
  });

  useEffect(() => {
    if (!activeAction || actionTimedOut) return;
    const elapsed = Date.now() - activeAction.startedAt;
    const remaining = Math.max(0, ACTION_MAX_WAIT - elapsed);
    const timeout = window.setTimeout(() => setActionTimedOut(true), remaining);
    return () => window.clearTimeout(timeout);
  }, [actionTimedOut, activeAction]);

  useEffect(() => {
    const execution = executionQuery.data;
    if (!execution || !['succeeded', 'failed'].includes(execution.status)) {
      return;
    }
    if (handledExecutionID.current === execution.id) return;
    handledExecutionID.current = execution.id;

    void Promise.all([
      queryClient.invalidateQueries({ queryKey: sourceQueryKey(projectId) }),
      queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(projectId),
      }),
      queryClient.invalidateQueries({
        queryKey: ['openflare', 'pages', 'deployment-files', projectId],
      }),
      queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
    ]);

    if (execution.status === 'succeeded') {
      toast.success('部署源同步并发布完成');
    } else {
      toast.error(execution.error_message || '部署源同步失败');
    }
    setActiveAction(null);
    setActionTimedOut(false);
  }, [executionQuery.data, projectId, queryClient]);

  const syncMutation = useMutation({
    mutationFn: () => PagesService.syncSource(projectId, {}),
    onSuccess: async (receipt) => {
      handledExecutionID.current = null;
      setActiveAction({ receipt, startedAt: Date.now() });
      setActionTimedOut(false);
      await queryClient.invalidateQueries({
        queryKey: sourceQueryKey(projectId),
      });
      toast.success('同步任务已提交');
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '同步任务提交失败');
    },
  });

  const source = sourceQuery.data;
  const executionBusy =
    activeAction !== null &&
    (executionQuery.data?.status === undefined ||
      executionQuery.data.status === 'pending' ||
      executionQuery.data.status === 'running');
  const sourceBusy =
    source?.source_type !== 'manual' &&
    (source?.sync_status === 'checking' || source?.sync_status === 'syncing');
  const actionsDisabled = syncMutation.isPending || executionBusy || sourceBusy;

  const openSourceDialog = (mode: 'manual' | 'remote_url') => {
    setDialogMode(mode);
    setDialogOpen(true);
  };

  if (sourceQuery.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>部署源</CardTitle>
          <CardDescription>加载来源配置...</CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3'>
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-20 w-full' />
        </CardContent>
      </Card>
    );
  }

  if (sourceQuery.isError || !source) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>部署源</CardTitle>
          <CardDescription>来源配置与部署历史相互独立。</CardDescription>
        </CardHeader>
        <CardContent>
          <ErrorInline
            message={
              sourceQuery.error instanceof Error
                ? sourceQuery.error.message
                : '部署源加载失败'
            }
            onRetry={() => void sourceQuery.refetch()}
          />
        </CardContent>
      </Card>
    );
  }

  const status =
    source.source_type === 'manual'
      ? null
      : SOURCE_STATUS[source.sync_status ?? 'idle'];

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>部署源</CardTitle>
          <CardDescription>
            来源配置负责发现内容，发布结果记录在独立的部署历史中。
          </CardDescription>
          <CardAction>
            {status ? (
              <Badge variant={status.variant}>{status.label}</Badge>
            ) : (
              <Badge variant='outline'>手动部署</Badge>
            )}
          </CardAction>
        </CardHeader>

        <CardContent className='flex flex-col gap-4'>
          {source.source_type === 'manual' ? (
            <div className='rounded-lg border bg-muted/20 p-4'>
              <p className='text-sm font-medium'>本地部署包</p>
              <p className='mt-1 text-sm text-muted-foreground'>
                当前没有持久化远端来源。上传部署包后，再从部署历史显式激活。
              </p>
            </div>
          ) : source.source_type === 'remote_url' ? (
            <div className='grid gap-4 md:grid-cols-2'>
              <div className='flex min-w-0 flex-col gap-1 rounded-lg border p-4 md:col-span-2'>
                <span className='text-xs text-muted-foreground'>脱敏地址</span>
                <code className='truncate text-sm'>{source.display_url}</code>
              </div>
              <div className='flex flex-col gap-1 rounded-lg border p-4'>
                <span className='text-xs text-muted-foreground'>网络策略</span>
                <span className='text-sm font-medium'>
                  {source.remote_network_policy === 'trusted_internal'
                    ? '受信内网模式'
                    : '公网安全模式'}
                </span>
              </div>
              <div className='flex flex-col gap-1 rounded-lg border p-4'>
                <span className='text-xs text-muted-foreground'>最近同步</span>
                <span className='text-sm font-medium'>
                  {source.last_synced_at
                    ? formatDateTime(source.last_synced_at)
                    : '尚未同步'}
                </span>
              </div>
              <div className='flex flex-col gap-1 rounded-lg border p-4 md:col-span-2'>
                <span className='text-xs text-muted-foreground'>
                  已应用 revision
                </span>
                <span className='font-mono text-sm'>
                  {revisionSummary(source)}
                </span>
              </div>
              {source.last_error ? (
                <div className='md:col-span-2'>
                  <ErrorInline message={source.last_error} />
                </div>
              ) : null}
            </div>
          ) : (
            <div className='rounded-lg border p-4 text-sm text-muted-foreground'>
              当前版本暂不提供该来源类型的编辑界面。
            </div>
          )}

          {executionQuery.isError ? (
            <ErrorInline
              message={
                executionQuery.error instanceof Error
                  ? executionQuery.error.message
                  : '任务状态读取失败'
              }
              onRetry={() => void executionQuery.refetch()}
            />
          ) : null}
          {actionTimedOut ? (
            <div className='flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'>
              <span className='text-xs text-muted-foreground'>
                自动等待已停止，任务可能仍在后台运行。
              </span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => {
                  if (!activeAction) return;
                  setActiveAction({ ...activeAction, startedAt: Date.now() });
                  setActionTimedOut(false);
                  void executionQuery.refetch();
                  void sourceQuery.refetch();
                }}
              >
                <RefreshCw data-icon='inline-start' />
                刷新任务状态
              </Button>
            </div>
          ) : null}
        </CardContent>

        <CardFooter className='flex flex-wrap gap-2 border-t'>
          {source.source_type === 'manual' ? (
            <Button
              type='button'
              onClick={() => openSourceDialog('remote_url')}
            >
              <Download data-icon='inline-start' />
              配置 Remote URL
            </Button>
          ) : source.source_type === 'remote_url' ? (
            <>
              <Button
                type='button'
                variant='outline'
                disabled={actionsDisabled}
                onClick={() => openSourceDialog('remote_url')}
              >
                <Pencil data-icon='inline-start' />
                编辑来源
              </Button>
              <Button
                type='button'
                disabled={actionsDisabled}
                onClick={() => syncMutation.mutate()}
              >
                {actionsDisabled ? (
                  <Spinner data-icon='inline-start' />
                ) : (
                  <Download data-icon='inline-start' />
                )}
                同步并发布
              </Button>
              <Button
                type='button'
                variant='ghost'
                disabled={actionsDisabled}
                onClick={() => openSourceDialog('manual')}
              >
                <RotateCcw data-icon='inline-start' />
                切换回手动
              </Button>
            </>
          ) : null}
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='ml-auto'
            onClick={() => void sourceQuery.refetch()}
          >
            <RefreshCw data-icon='inline-start' />
            刷新
          </Button>
        </CardFooter>
      </Card>

      <PagesSourceDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        projectId={projectId}
        source={source}
        initialMode={dialogMode}
      />
    </>
  );
}
