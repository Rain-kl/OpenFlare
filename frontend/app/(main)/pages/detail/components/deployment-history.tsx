'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { toast } from 'sonner';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
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
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import { type PagesDeployment, PagesService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import {
  deploymentFilesQueryKey,
  deploymentsQueryKey,
  formatBytes,
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../../components/pages-utils';
import { DeploymentFilesPanel } from './deployment-files-panel';

const SOURCE_LABELS: Record<PagesDeployment['source_type'], string> = {
  manual_upload: '本地上传',
  manual_url: 'URL 导入',
  remote_url: 'Remote URL',
  github_release: 'GitHub Release',
};

const TRIGGER_LABELS: Record<PagesDeployment['trigger_type'], string> = {
  manual_upload: '手动上传',
  manual_url: '手动导入',
  manual_sync: '手动同步',
  scheduled_auto_update: '定时更新',
};

interface DeploymentHistoryProps {
  projectId: number;
  activeDeploymentId?: number | null;
}

type PendingAction = {
  type: 'activate' | 'delete';
  deployment: PagesDeployment;
};

function deploymentSnapshot(deployment: PagesDeployment) {
  return [
    SOURCE_LABELS[deployment.source_type],
    deployment.source_label,
    TRIGGER_LABELS[deployment.trigger_type],
  ]
    .filter(Boolean)
    .join(' · ');
}

export function DeploymentHistory({
  projectId,
  activeDeploymentId,
}: DeploymentHistoryProps) {
  const queryClient = useQueryClient();
  const [expandedDeploymentId, setExpandedDeploymentId] = useState<
    number | null
  >(null);
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(
    null,
  );

  const deploymentsQuery = useQuery({
    queryKey: deploymentsQueryKey(projectId),
    queryFn: () => PagesService.listDeployments(projectId),
  });

  const deployments = useMemo(() => {
    const records = [...(deploymentsQuery.data ?? [])];
    return records.sort((left, right) => {
      const leftActive =
        left.id === activeDeploymentId || left.status === 'active';
      const rightActive =
        right.id === activeDeploymentId || right.status === 'active';
      if (leftActive !== rightActive) return leftActive ? -1 : 1;
      return right.deployment_number - left.deployment_number;
    });
  }, [activeDeploymentId, deploymentsQuery.data]);

  const invalidateDeploymentState = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(projectId),
      }),
      queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
      queryClient.invalidateQueries({ queryKey: sourceQueryKey(projectId) }),
      queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
    ]);
  };

  const activateMutation = useMutation({
    mutationFn: (deploymentId: number) =>
      PagesService.activateDeployment(projectId, deploymentId),
    onSuccess: async () => {
      toast.success('部署已激活');
      await invalidateDeploymentState();
      setPendingAction(null);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '激活失败');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (deploymentId: number) =>
      PagesService.deleteDeployment(projectId, deploymentId),
    onSuccess: async (_, deploymentId) => {
      toast.success('部署已删除');
      queryClient.removeQueries({
        queryKey: deploymentFilesQueryKey(projectId, deploymentId),
      });
      await invalidateDeploymentState();
      setPendingAction(null);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '删除失败');
    },
  });

  const actionPending = activateMutation.isPending || deleteMutation.isPending;

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>部署历史</CardTitle>
          <CardDescription>
            部署记录不可变，来源信息是创建部署时的安全快照。
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3'>
          {deploymentsQuery.isLoading ? (
            <LoadingStateWithBorder description='加载部署历史...' />
          ) : deploymentsQuery.isError ? (
            <div className='rounded-lg border p-4'>
              <ErrorInline
                message={
                  deploymentsQuery.error instanceof Error
                    ? deploymentsQuery.error.message
                    : '部署历史加载失败'
                }
                onRetry={() => void deploymentsQuery.refetch()}
              />
            </div>
          ) : deployments.length === 0 ? (
            <EmptyStateWithBorder
              title='暂无部署'
              description='上传本地部署包，或配置 Remote URL 来源后同步发布。'
            />
          ) : (
            deployments.map((deployment) => {
              const active =
                deployment.id === activeDeploymentId ||
                deployment.status === 'active';
              const expanded = expandedDeploymentId === deployment.id;

              return (
                <div key={deployment.id} className='rounded-lg border'>
                  <div className='flex flex-col gap-4 p-4 md:flex-row md:items-center md:justify-between'>
                    <div className='flex min-w-0 items-start gap-2'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        aria-label={expanded ? '收起文件清单' : '展开文件清单'}
                        onClick={() =>
                          setExpandedDeploymentId(
                            expanded ? null : deployment.id,
                          )
                        }
                      >
                        {expanded ? <ChevronDown /> : <ChevronRight />}
                      </Button>
                      <div className='flex min-w-0 flex-col gap-2'>
                        <div className='flex flex-wrap items-center gap-2'>
                          <span className='text-sm font-medium'>
                            部署 #{deployment.deployment_number}
                          </span>
                          <Badge variant={active ? 'default' : 'outline'}>
                            {active ? '当前生产部署' : '历史部署'}
                          </Badge>
                          <Badge variant='secondary'>
                            {deploymentSnapshot(deployment)}
                          </Badge>
                        </div>
                        <p className='truncate text-xs text-muted-foreground'>
                          {deployment.checksum.slice(0, 16)} ·{' '}
                          {deployment.file_count} 个文件 ·{' '}
                          {formatBytes(deployment.total_size)}
                        </p>
                        <p className='text-xs text-muted-foreground'>
                          创建于 {formatDateTime(deployment.created_at)}
                        </p>
                      </div>
                    </div>
                    <div className='flex gap-2 md:ml-10'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        disabled={active || actionPending}
                        onClick={() =>
                          setPendingAction({ type: 'activate', deployment })
                        }
                      >
                        激活
                      </Button>
                      <Button
                        type='button'
                        variant='destructive'
                        size='sm'
                        disabled={active || actionPending}
                        onClick={() =>
                          setPendingAction({ type: 'delete', deployment })
                        }
                      >
                        删除
                      </Button>
                    </div>
                  </div>
                  {expanded ? (
                    <DeploymentFilesPanel
                      projectId={projectId}
                      deploymentId={deployment.id}
                    />
                  ) : null}
                </div>
              );
            })
          )}
        </CardContent>
      </Card>

      <AlertDialog
        open={pendingAction !== null}
        onOpenChange={(open) => {
          if (!open && !actionPending) setPendingAction(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingAction?.type === 'activate' ? '激活历史部署' : '删除部署'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingAction?.type === 'activate'
                ? '激活其它历史部署会终止当前来源任务；若已开启自动更新，将同时关闭自动更新。'
                : `确认删除部署 #${pendingAction?.deployment.deployment_number} 吗？此操作不可恢复。`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={actionPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={actionPending}
              onClick={(event) => {
                event.preventDefault();
                if (!pendingAction) return;
                if (pendingAction.type === 'activate') {
                  activateMutation.mutate(pendingAction.deployment.id);
                } else {
                  deleteMutation.mutate(pendingAction.deployment.id);
                }
              }}
            >
              {actionPending ? <Spinner data-icon='inline-start' /> : null}
              确认
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
