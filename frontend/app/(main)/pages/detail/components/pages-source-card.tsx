'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Download,
  Github,
  Pencil,
  RefreshCw,
  RotateCcw,
  Search,
} from 'lucide-react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
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
  type PagesSourceActionPayload,
  type PagesSourceActionReceipt,
  type PagesSourceStatus,
  PagesService,
} from '@/lib/services/openflare';

import {
  deploymentsQueryKey,
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../../components/pages-utils';
import { type PagesSourceMode, PagesSourceDialog } from './pages-source-dialog';
import {
  GitHubSourceDetails,
  RemoteSourceDetails,
} from './pages-source-details';

const ACTION_POLL_INTERVAL = 2_000;
const ACTION_MAX_WAIT = 16 * 60 * 1_000;
const LATEST_POLL_INTERVAL = 5 * 60 * 1_000;
const LATEST_NEAR_DUE_POLL_INTERVAL = 30_000;
const LATEST_OVERDUE_MAX_WAIT = 10 * 60 * 1_000;

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

export interface LatestSourceOverdueWindow {
  nextCheckAt: string;
  startedAt: number;
}

export interface LatestSourcePollingDecision {
  interval: number | false;
  overdueWindow: LatestSourceOverdueWindow | null;
}

export function getLatestSourceIdlePollingDecision(
  source: PagesSource | undefined,
  now: number,
  overdueWindow: LatestSourceOverdueWindow | null,
): LatestSourcePollingDecision {
  if (
    source?.source_type !== 'github_release' ||
    source.release_selector !== 'latest'
  ) {
    return { interval: false, overdueWindow: null };
  }

  const nextCheckAt = source.next_check_at;
  const nextCheckTime = nextCheckAt ? Date.parse(nextCheckAt) : Number.NaN;
  if (!nextCheckAt || !Number.isFinite(nextCheckTime)) {
    return { interval: LATEST_POLL_INTERVAL, overdueWindow: null };
  }

  const timeUntilCheck = nextCheckTime - now;
  if (timeUntilCheck > LATEST_POLL_INTERVAL) {
    return { interval: LATEST_POLL_INTERVAL, overdueWindow: null };
  }
  if (timeUntilCheck > 0) {
    return { interval: LATEST_NEAR_DUE_POLL_INTERVAL, overdueWindow: null };
  }

  const currentWindow =
    overdueWindow?.nextCheckAt === nextCheckAt
      ? overdueWindow
      : { nextCheckAt, startedAt: now };
  if (now - currentWindow.startedAt >= LATEST_OVERDUE_MAX_WAIT) {
    return { interval: false, overdueWindow: currentWindow };
  }
  return {
    interval: LATEST_NEAR_DUE_POLL_INTERVAL,
    overdueWindow: currentWindow,
  };
}

function sourceDeploymentFingerprint(source: PagesSource) {
  if (source.source_type === 'manual') return '|';
  return `${source.last_synced_at ?? ''}|${source.last_applied?.revision ?? ''}`;
}

function sourceActionLabel(action: PagesSourceActionReceipt['action']) {
  return action === 'check' ? '检查' : '同步并发布';
}

export function PagesSourceCard({ projectId }: { projectId: number }) {
  const queryClient = useQueryClient();
  const handledExecutionID = useRef<string | null>(null);
  const sourcePollingStartedAt = useRef<number | null>(null);
  const latestOverdueWindow = useRef<LatestSourceOverdueWindow | null>(null);
  const sourceDeploymentState = useRef<string | undefined>(undefined);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<PagesSourceMode>('manual');
  const [activeAction, setActiveAction] = useState<ActiveAction | null>(null);
  const [actionTimedOut, setActionTimedOut] = useState(false);
  const [attentionDialogOpen, setAttentionDialogOpen] = useState(false);

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
        latestOverdueWindow.current = null;
        return Date.now() - sourcePollingStartedAt.current < ACTION_MAX_WAIT
          ? ACTION_POLL_INTERVAL
          : false;
      }
      sourcePollingStartedAt.current = null;
      const decision = getLatestSourceIdlePollingDecision(
        source,
        Date.now(),
        latestOverdueWindow.current,
      );
      latestOverdueWindow.current = decision.overdueWindow;
      return decision.interval;
    },
  });

  const invalidateSourceState = useCallback(
    () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: sourceQueryKey(projectId) }),
        queryClient.invalidateQueries({
          queryKey: projectQueryKey(projectId),
        }),
        queryClient.invalidateQueries({
          queryKey: deploymentsQueryKey(projectId),
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'pages', 'deployment-files', projectId],
        }),
        queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
      ]),
    [projectId, queryClient],
  );

  const executionQuery = useQuery({
    queryKey: [
      'admin',
      'task-execution',
      activeAction?.receipt.execution_id ?? '',
    ],
    queryFn: () => {
      const executionID = activeAction?.receipt.execution_id;
      if (!executionID) throw new Error('缺少任务执行 ID');
      return AdminTaskService.getTaskExecution(executionID);
    },
    enabled: Boolean(activeAction) && !actionTimedOut,
    refetchInterval: (query) => {
      if (actionTimedOut) return false;
      const status = query.state.data?.status;
      return status === 'pending' || status === 'running'
        ? ACTION_POLL_INTERVAL
        : false;
    },
  });

  const beginActionPolling = useCallback(
    (receipt: PagesSourceActionReceipt) => {
      handledExecutionID.current = null;
      setActiveAction({ receipt, startedAt: Date.now() });
      setActionTimedOut(false);
    },
    [],
  );

  useEffect(() => {
    if (!activeAction || actionTimedOut) return;
    const elapsed = Date.now() - activeAction.startedAt;
    const remaining = Math.max(0, ACTION_MAX_WAIT - elapsed);
    const timeout = window.setTimeout(() => setActionTimedOut(true), remaining);
    return () => window.clearTimeout(timeout);
  }, [actionTimedOut, activeAction]);

  useEffect(() => {
    const source = sourceQuery.data;
    if (!source) return;
    const fingerprint = sourceDeploymentFingerprint(source);
    const previousFingerprint = sourceDeploymentState.current;
    sourceDeploymentState.current = fingerprint;
    if (
      previousFingerprint === undefined ||
      previousFingerprint === fingerprint
    ) {
      return;
    }
    void invalidateSourceState();
  }, [invalidateSourceState, sourceQuery.data]);

  useEffect(() => {
    const execution = executionQuery.data;
    if (
      !activeAction ||
      !execution ||
      !['succeeded', 'failed'].includes(execution.status)
    ) {
      return;
    }
    if (handledExecutionID.current === execution.id) return;
    handledExecutionID.current = execution.id;

    void invalidateSourceState();

    const actionLabel = sourceActionLabel(activeAction.receipt.action);
    if (execution.status === 'succeeded') {
      toast.success(`部署源${actionLabel}完成`);
    } else {
      toast.error(execution.error_message || `部署源${actionLabel}失败`);
    }
    setActiveAction(null);
    setActionTimedOut(false);
  }, [activeAction, executionQuery.data, invalidateSourceState]);

  const checkMutation = useMutation({
    mutationFn: () => PagesService.checkSource(projectId),
    onSuccess: async (receipt) => {
      beginActionPolling(receipt);
      await queryClient.invalidateQueries({
        queryKey: sourceQueryKey(projectId),
      });
      toast.success('检查任务已提交');
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '检查任务提交失败');
    },
  });

  const syncMutation = useMutation({
    mutationFn: (payload: PagesSourceActionPayload) =>
      PagesService.syncSource(projectId, payload),
    onSuccess: async (receipt) => {
      setAttentionDialogOpen(false);
      beginActionPolling(receipt);
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
  const actionsDisabled =
    checkMutation.isPending ||
    syncMutation.isPending ||
    executionBusy ||
    sourceBusy;
  const checkBusy =
    checkMutation.isPending ||
    (executionBusy && activeAction?.receipt.action === 'check') ||
    (source?.source_type !== 'manual' && source?.sync_status === 'checking');
  const syncBusy =
    syncMutation.isPending ||
    (executionBusy && activeAction?.receipt.action === 'sync') ||
    (source?.source_type !== 'manual' && source?.sync_status === 'syncing');
  const dispatchError = checkMutation.error ?? syncMutation.error;

  const openSourceDialog = (mode: PagesSourceMode) => {
    setDialogMode(mode);
    setDialogOpen(true);
  };

  const dispatchSync = () => {
    checkMutation.reset();
    if (
      source?.source_type === 'github_release' &&
      source.sync_status === 'attention'
    ) {
      setAttentionDialogOpen(true);
      return;
    }
    syncMutation.mutate({});
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

  const effectiveSourceStatus = executionBusy
    ? activeAction?.receipt.action === 'check'
      ? 'checking'
      : 'syncing'
    : source.source_type === 'manual'
      ? undefined
      : (source.sync_status ?? 'idle');
  const status =
    source.source_type === 'manual'
      ? null
      : SOURCE_STATUS[effectiveSourceStatus ?? 'idle'];
  const attentionRevision =
    source.source_type === 'github_release' &&
    source.sync_status === 'attention'
      ? source.last_seen
      : undefined;

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
            <RemoteSourceDetails source={source} />
          ) : (
            <GitHubSourceDetails source={source} />
          )}

          {dispatchError ? (
            <ErrorInline
              message={
                dispatchError instanceof Error
                  ? dispatchError.message
                  : '来源任务提交失败'
              }
            />
          ) : null}
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
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => openSourceDialog('remote_url')}
              >
                <Download data-icon='inline-start' />
                配置 Remote URL
              </Button>
              <Button
                type='button'
                onClick={() => openSourceDialog('github_release')}
              >
                <Github data-icon='inline-start' />
                配置 GitHub Release
              </Button>
            </>
          ) : (
            <>
              <Button
                type='button'
                variant='outline'
                disabled={actionsDisabled}
                onClick={() => openSourceDialog(source.source_type)}
              >
                <Pencil data-icon='inline-start' />
                编辑来源
              </Button>
              {source.source_type === 'github_release' ? (
                <Button
                  type='button'
                  variant='outline'
                  disabled={actionsDisabled}
                  onClick={() => {
                    syncMutation.reset();
                    checkMutation.mutate();
                  }}
                >
                  {checkBusy ? (
                    <Spinner data-icon='inline-start' />
                  ) : (
                    <Search data-icon='inline-start' />
                  )}
                  检查更新
                </Button>
              ) : null}
              <Button
                type='button'
                disabled={
                  actionsDisabled ||
                  (source.sync_status === 'attention' && !attentionRevision)
                }
                onClick={dispatchSync}
              >
                {syncBusy ? (
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
          )}
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
        onActionDispatched={beginActionPolling}
      />

      <AlertDialog
        open={attentionDialogOpen}
        onOpenChange={(open) => {
          if (!syncMutation.isPending) setAttentionDialogOpen(open);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认发布当前 GitHub revision</AlertDialogTitle>
            <AlertDialogDescription>
              <span>这将发布卡片当前显示的精确 revision：</span>{' '}
              <span className='break-all font-mono'>
                {attentionRevision?.revision ??
                  '当前 revision 已不可用，请刷新后重试'}
              </span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={syncMutation.isPending}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={syncMutation.isPending || !attentionRevision}
              onClick={(event) => {
                event.preventDefault();
                if (!attentionRevision) return;
                syncMutation.mutate({
                  confirmed_revision: attentionRevision.revision,
                });
              }}
            >
              {syncMutation.isPending ? (
                <Spinner data-icon='inline-start' />
              ) : null}
              确认并发布
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
