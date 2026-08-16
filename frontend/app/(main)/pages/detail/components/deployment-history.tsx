'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronDown, ChevronRight, Rocket, Upload } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import { type PagesDeployment, PagesService } from '@/lib/services/openflare';
import { cn, formatDateTime } from '@/lib/utils';

import { DeploymentUploadDialog } from '../../components/deployment-upload-dialog';
import {
  deploymentFilesQueryKey,
  deploymentsQueryKey,
  formatBytes,
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../../components/pages-utils';
import { DeploymentFilesPanel } from './deployment-files-panel';

const SOURCE_LABEL_KEYS: Record<
  PagesDeployment['source_type'],
  'history.source.manual_upload' | 'history.source.manual_url' | null
> = {
  manual_upload: 'history.source.manual_upload',
  manual_url: 'history.source.manual_url',
  remote_url: null,
  github_release: null,
};

const TRIGGER_LABEL_KEYS: Record<
  PagesDeployment['trigger_type'],
  | 'history.trigger.manual_upload'
  | 'history.trigger.manual_url'
  | 'history.trigger.manual_sync'
  | 'history.trigger.scheduled_auto_update'
> = {
  manual_upload: 'history.trigger.manual_upload',
  manual_url: 'history.trigger.manual_url',
  manual_sync: 'history.trigger.manual_sync',
  scheduled_auto_update: 'history.trigger.scheduled_auto_update',
};

interface DeploymentHistoryProps {
  projectId: number;
  activeDeploymentId?: number | null;
  rootDir?: string;
  entryFile?: string;
}

type PendingAction = {
  type: 'activate' | 'delete';
  deployment: PagesDeployment;
};

function isActiveDeployment(
  deployment: PagesDeployment,
  activeDeploymentId?: number | null,
) {
  return deployment.id === activeDeploymentId || deployment.status === 'active';
}

function sourceLabel(
  type: PagesDeployment['source_type'],
  t: (key: string) => string,
) {
  const key = SOURCE_LABEL_KEYS[type];
  if (key) return t(key);
  return type === 'remote_url' ? 'Remote URL' : 'GitHub';
}

function deploymentSnapshot(
  deployment: PagesDeployment,
  t: (key: string) => string,
) {
  return [
    sourceLabel(deployment.source_type, t),
    deployment.source_label,
    t(TRIGGER_LABEL_KEYS[deployment.trigger_type]),
  ]
    .filter(Boolean)
    .join(' · ');
}

function DeploymentMeta({ deployment }: { deployment: PagesDeployment }) {
  const t = useTranslations('pages');
  return (
    <>
      <p className='truncate text-xs text-muted-foreground'>
        {t('history.fileMeta', {
          checksum: deployment.checksum.slice(0, 16),
          count: deployment.file_count,
        })}
        {formatBytes(deployment.total_size)}
      </p>
      <p className='text-xs text-muted-foreground'>
        {t('history.createdAt', {
          time: formatDateTime(deployment.created_at),
        })}
        {deployment.activated_at
          ? t('history.activatedAt', {
              time: formatDateTime(deployment.activated_at),
            })
          : ''}
      </p>
    </>
  );
}

interface DeploymentRowProps {
  deployment: PagesDeployment;
  active: boolean;
  expanded: boolean;
  actionPending: boolean;
  showActions: boolean;
  onToggleExpand: () => void;
  onActivate: () => void;
  onDelete: () => void;
  projectId: number;
}

function DeploymentRow({
  deployment,
  active,
  expanded,
  actionPending,
  showActions,
  onToggleExpand,
  onActivate,
  onDelete,
  projectId,
}: DeploymentRowProps) {
  const t = useTranslations('pages');
  return (
    <div
      className={cn(
        'rounded-lg border border-dashed',
        active && 'border-l-4 border-l-solid border-l-primary',
      )}
    >
      <div className='flex flex-col gap-4 p-4 md:flex-row md:items-center md:justify-between'>
        <div className='flex min-w-0 items-start gap-2'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            aria-label={
              expanded ? t('history.collapseFiles') : t('history.expandFiles')
            }
            onClick={onToggleExpand}
          >
            {expanded ? <ChevronDown /> : <ChevronRight />}
          </Button>
          <div className='flex min-w-0 flex-col gap-2'>
            <div className='flex flex-wrap items-center gap-2'>
              <span className='text-sm font-medium'>
                {t('history.deploymentNumber', {
                  number: deployment.deployment_number,
                })}
              </span>
              <Badge variant='secondary'>
                {deploymentSnapshot(deployment, (key) => t(key))}
              </Badge>
            </div>
            <DeploymentMeta deployment={deployment} />
          </div>
        </div>
        {showActions ? (
          <div className='flex gap-2 md:ml-10'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={active || actionPending}
              onClick={onActivate}
            >
              {t('history.activate')}
            </Button>
            <Button
              type='button'
              variant='destructive'
              size='sm'
              disabled={active || actionPending}
              onClick={onDelete}
            >
              {t('history.delete')}
            </Button>
          </div>
        ) : null}
      </div>
      {expanded ? (
        <DeploymentFilesPanel
          projectId={projectId}
          deploymentId={deployment.id}
        />
      ) : null}
    </div>
  );
}

export function DeploymentHistory({
  projectId,
  activeDeploymentId,
  rootDir = '',
  entryFile = 'index.html',
}: DeploymentHistoryProps) {
  const t = useTranslations('pages');
  const tCommon = useTranslations('common');
  const queryClient = useQueryClient();
  const [uploadOpen, setUploadOpen] = useState(false);
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

  const { productionDeployment, allDeployments } = useMemo(() => {
    const records = [...(deploymentsQuery.data ?? [])];
    records.sort(
      (left, right) => right.deployment_number - left.deployment_number,
    );

    const production =
      records.find((item) => isActiveDeployment(item, activeDeploymentId)) ??
      null;

    return {
      productionDeployment: production,
      allDeployments: records,
    };
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
      toast.success(t('history.activated'));
      await invalidateDeploymentState();
      setPendingAction(null);
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? error.message : t('history.activateFailed'),
      );
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (deploymentId: number) =>
      PagesService.deleteDeployment(projectId, deploymentId),
    onSuccess: async (_, deploymentId) => {
      toast.success(t('history.deleted'));
      queryClient.removeQueries({
        queryKey: deploymentFilesQueryKey(projectId, deploymentId),
      });
      await invalidateDeploymentState();
      setPendingAction(null);
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? error.message : t('history.deleteFailed'),
      );
    },
  });

  const actionPending = activateMutation.isPending || deleteMutation.isPending;

  const toggleExpand = (deploymentId: number) => {
    setExpandedDeploymentId((current) =>
      current === deploymentId ? null : deploymentId,
    );
  };

  const dialogs = (
    <>
      <AlertDialog
        open={pendingAction !== null}
        onOpenChange={(open) => {
          if (!open && !actionPending) setPendingAction(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingAction?.type === 'activate'
                ? t('history.activateTitle')
                : t('history.deleteTitle')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingAction?.type === 'activate'
                ? t('history.activateDesc')
                : t('history.deleteDesc', {
                    number: pendingAction?.deployment.deployment_number ?? 0,
                  })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={actionPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
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
              {t('history.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <DeploymentUploadDialog
        open={uploadOpen}
        onOpenChange={setUploadOpen}
        projectId={projectId}
        rootDir={rootDir}
        entryFile={entryFile}
      />
    </>
  );

  if (deploymentsQuery.isLoading) {
    return (
      <>
        <LoadingStateWithBorder description={t('history.loading')} />
        {dialogs}
      </>
    );
  }

  if (deploymentsQuery.isError) {
    return (
      <>
        <div className='rounded-lg border p-4'>
          <ErrorInline
            message={
              deploymentsQuery.error instanceof Error
                ? deploymentsQuery.error.message
                : t('history.loadFailed')
            }
            onRetry={() => void deploymentsQuery.refetch()}
          />
        </div>
        {dialogs}
      </>
    );
  }

  return (
    <div className='flex flex-col gap-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base'>Production</CardTitle>
          <CardDescription>{t('history.productionHint')}</CardDescription>
          <CardAction>
            <Button
              type='button'
              size='sm'
              className='whitespace-nowrap'
              onClick={() => setUploadOpen(true)}
            >
              <Upload data-icon='inline-start' />
              {t('history.manualUpload')}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className='space-y-3'>
          {productionDeployment ? (
            <DeploymentRow
              deployment={productionDeployment}
              active
              expanded={expandedDeploymentId === productionDeployment.id}
              actionPending={actionPending}
              showActions={false}
              onToggleExpand={() => toggleExpand(productionDeployment.id)}
              onActivate={() => undefined}
              onDelete={() => undefined}
              projectId={projectId}
            />
          ) : (
            <EmptyStateWithBorder
              icon={Rocket}
              title={t('history.emptyProductionTitle')}
              description={t('history.emptyProductionDesc')}
            />
          )}
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base'>All deployments</CardTitle>
          <CardDescription>{t('history.historyHint')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-3'>
          {allDeployments.length === 0 ? (
            <EmptyStateWithBorder
              title={t('history.emptyTitle')}
              description={t('history.emptyDesc')}
            />
          ) : (
            allDeployments.map((deployment) => {
              const active = isActiveDeployment(deployment, activeDeploymentId);
              return (
                <DeploymentRow
                  key={deployment.id}
                  deployment={deployment}
                  active={active}
                  expanded={expandedDeploymentId === deployment.id}
                  actionPending={actionPending}
                  showActions
                  onToggleExpand={() => toggleExpand(deployment.id)}
                  onActivate={() =>
                    setPendingAction({ type: 'activate', deployment })
                  }
                  onDelete={() =>
                    setPendingAction({ type: 'delete', deployment })
                  }
                  projectId={projectId}
                />
              );
            })
          )}
        </CardContent>
      </Card>

      {dialogs}
    </div>
  );
}
