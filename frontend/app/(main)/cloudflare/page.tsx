'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  Cloud,
  Plus,
  RefreshCw,
  Settings,
  Trash2,
} from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { GroupDialog } from '@/app/(main)/cloudflare/components/group-dialog';
import { SyncTasksPanel } from '@/app/(main)/cloudflare/components/sync-tasks-panel';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
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
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  CloudflareService,
  cloudflareQueryKey,
  NodeService,
  type CloudflareGroup,
  type CloudflareGroupPayload,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../websites/components/website-utils';

export default function CloudflarePage() {
  const t = useTranslations('cloudflare');
  const tCommon = useTranslations('common');
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CloudflareGroup | null>(
    null,
  );
  const overviewQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'overview'],
    queryFn: () => CloudflareService.getOverview(),
  });
  const groupsQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'groups'],
    queryFn: () => CloudflareService.listGroups(),
  });
  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
  });

  const invalidate = async () =>
    queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });
  const createMutation = useMutation({
    mutationFn: (payload: CloudflareGroupPayload) =>
      CloudflareService.createGroup(payload),
    onSuccess: async () => {
      toast.success(t('created'));
      setCreateOpen(false);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const syncMutation = useMutation({
    mutationFn: (id: number) => CloudflareService.syncGroup(id),
    onSuccess: async () => {
      toast.success(t('syncQueued'));
      await queryClient.invalidateQueries({
        queryKey: [...cloudflareQueryKey, 'sync-executions'],
      });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const deleteMutation = useMutation({
    mutationFn: (id: number) => CloudflareService.deleteGroup(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const overview = overviewQuery.data;

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex items-center gap-2'>
          <Button asChild variant='outline' size='sm'>
            <Link href='/cloudflare/settings'>
              <Settings data-icon='inline-start' />
              {t('connectionSettings')}
            </Link>
          </Button>
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            {t('addGroup')}
          </Button>
        </div>
      </div>

      {overviewQuery.isLoading ? (
        <LoadingStateWithBorder
          icon={Cloud}
          description={t('loadingOverview')}
        />
      ) : overviewQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(overviewQuery.error)}
          onRetry={() => void overviewQuery.refetch()}
        />
      ) : !overview?.connection.ready ? (
        <Alert>
          <AlertTriangle />
          <AlertTitle>{t('notReadyTitle')}</AlertTitle>
          <AlertDescription className='flex flex-col items-start gap-3'>
            <span>{t('notReadyDesc')}</span>
            <Button asChild size='sm'>
              <Link href='/cloudflare/settings'>{t('configure')}</Link>
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='flex flex-col gap-4'>
        <div>
          <h2 className='text-lg font-semibold'>{t('groupsTitle')}</h2>
          <p className='text-sm text-muted-foreground'>{t('groupsDesc')}</p>
        </div>

        {groupsQuery.isLoading ? (
          <LoadingStateWithBorder
            icon={Cloud}
            description={t('loadingGroups')}
          />
        ) : groupsQuery.isError ? (
          <ErrorInline
            message={getErrorMessage(groupsQuery.error)}
            onRetry={() => void groupsQuery.refetch()}
          />
        ) : (groupsQuery.data ?? []).length === 0 ? (
          <EmptyStateWithBorder
            icon={Cloud}
            title={t('emptyTitle')}
            description={t('emptyDesc')}
            actionText={t('addGroup')}
            onAction={() => setCreateOpen(true)}
          />
        ) : (
          <div className='grid gap-4 lg:grid-cols-2'>
            {(groupsQuery.data ?? []).map((group) => (
              <Card key={group.id} className='border-dashed shadow-none'>
                <CardHeader>
                  <div className='flex items-start justify-between gap-3'>
                    <div>
                      <CardTitle className='text-base'>{group.name}</CardTitle>
                      <CardDescription>
                        {group.active_node.name} · {group.active_node.ip}
                      </CardDescription>
                    </div>
                    <Badge variant={group.enabled ? 'default' : 'secondary'}>
                      {group.enabled ? t('enabled') : t('disabled')}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent className='flex flex-col gap-4'>
                  <p className='text-sm text-muted-foreground'>
                    {t('memberSummary', {
                      count: group.member_count,
                      proxy: group.default_proxied
                        ? t('proxyOn')
                        : t('proxyOff'),
                    })}
                  </p>
                  <div className='flex flex-wrap gap-2'>
                    <Button asChild size='sm'>
                      <Link href={`/cloudflare/groups/${group.id}`}>
                        <Settings data-icon='inline-start' />
                        {t('manage')}
                      </Link>
                    </Button>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => syncMutation.mutate(group.id)}
                    >
                      <RefreshCw data-icon='inline-start' />
                      {t('sync')}
                    </Button>
                    <Button
                      variant='destructive'
                      size='sm'
                      onClick={() => setDeleteTarget(group)}
                    >
                      <Trash2 data-icon='inline-start' />
                      {t('delete')}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      <SyncTasksPanel />

      <GroupDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        nodes={nodesQuery.data ?? []}
        pending={createMutation.isPending}
        onSubmit={(payload) => createMutation.mutate(payload)}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
