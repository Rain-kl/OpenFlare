'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Network, Plus, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

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
import type {
  WAFIPGroup,
  WAFIPGroupAutoTestResult,
  WAFIPGroupPayload,
} from '@/lib/services/openflare';
import { WafService } from '@/lib/services/openflare';

import {
  buildIPGroupPayloadFromGroup,
  getErrorMessage,
  parseAutomaticConfig,
} from '../waf/components/helpers';
import { IPGroupDialog } from '../waf/components/ip-group-dialog';
import { IPGroupTestDialog } from '../waf/components/ip-group-test-dialog';
import { IPGroupViewDialog } from '../waf/components/ip-group-view-dialog';
import { IPGroupsTable } from '../waf/components/ip-groups-table';

const ipGroupsQueryKey = ['openflare', 'waf', 'ip-groups'];

export default function WafIPGroupsPage() {
  const t = useTranslations('ipGroups');
  const tCommon = useTranslations('common');
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<WAFIPGroup | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<WAFIPGroup | null>(null);
  const [testOpen, setTestOpen] = useState(false);
  const [testResult, setTestResult] = useState<WAFIPGroupAutoTestResult | null>(
    null,
  );
  const [syncingId, setSyncingId] = useState<number | null>(null);
  const [viewOpen, setViewOpen] = useState(false);
  const [viewingGroup, setViewingGroup] = useState<WAFIPGroup | null>(null);
  const [removingIp, setRemovingIp] = useState<string | null>(null);

  const groupsQuery = useQuery({
    queryKey: ipGroupsQueryKey,
    queryFn: () => WafService.listIPGroups(),
  });

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ipGroupsQueryKey }),
      queryClient.invalidateQueries({
        queryKey: ['openflare', 'waf', 'rule-groups'],
      }),
      queryClient.invalidateQueries({
        queryKey: ['openflare', 'config-versions', 'diff'],
      }),
    ]);
  };

  const saveMutation = useMutation({
    mutationFn: async ({
      group,
      payload,
    }: {
      group: WAFIPGroup | null;
      payload: WAFIPGroupPayload;
    }) => {
      if (group) {
        return WafService.updateIPGroup(group.id, payload);
      }
      return WafService.createIPGroup(payload);
    },
    onSuccess: async () => {
      toast.success(editingGroup ? t('updated') : t('created'));
      setEditingGroup(null);
      setEditorOpen(false);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('operationFailed')));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => WafService.deleteIPGroup(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('operationFailed')));
    },
  });

  const syncMutation = useMutation({
    mutationFn: (id: number) => WafService.syncIPGroup(id),
    onMutate: (id) => {
      setSyncingId(id);
    },
    onSuccess: async (result) => {
      toast.success(result.message || t('synced'));
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('operationFailed')));
    },
    onSettled: () => {
      setSyncingId(null);
    },
  });

  const viewGroupQuery = useQuery({
    queryKey: ['openflare', 'waf', 'ip-groups', viewingGroup?.id],
    queryFn: () => WafService.getIPGroup(viewingGroup!.id),
    enabled: viewOpen && viewingGroup !== null,
  });

  const removeIpMutation = useMutation({
    mutationFn: async ({ group, ip }: { group: WAFIPGroup; ip: string }) => {
      const nextIpList = group.ip_list.filter((item) => item !== ip);
      return WafService.updateIPGroup(
        group.id,
        buildIPGroupPayloadFromGroup(group, nextIpList),
      );
    },
    onMutate: ({ ip }) => {
      setRemovingIp(ip);
    },
    onSuccess: async (updatedGroup) => {
      toast.success(t('ipRemoved'));
      setViewingGroup(updatedGroup);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('operationFailed')));
    },
    onSettled: () => {
      setRemovingIp(null);
    },
  });

  const testMutation = useMutation({
    mutationFn: (group: WAFIPGroup) =>
      WafService.testIPGroup({
        auto_config: parseAutomaticConfig(
          JSON.stringify(group.auto_config ?? {}, null, 2),
          t('autoConfigMustBeObject'),
        ),
      }),
    onSuccess: (result) => {
      setTestResult(result);
      toast.success(
        result.matched_count > 0
          ? t('testHit', { count: result.matched_count })
          : t('testMiss'),
      );
    },
    onError: (error) => {
      setTestResult(null);
      toast.error(getErrorMessage(error, t('operationFailed')));
    },
  });

  const handleRefresh = () => {
    void queryClient.invalidateQueries({ queryKey: ipGroupsQueryKey });
  };

  const handleCreate = () => {
    setEditingGroup(null);
    setEditorOpen(true);
  };

  const handleEdit = (group: WAFIPGroup) => {
    setEditingGroup(group);
    setEditorOpen(true);
  };

  const handleTest = (group: WAFIPGroup) => {
    setTestResult(null);
    setTestOpen(true);
    testMutation.mutate(group);
  };

  const handleView = (group: WAFIPGroup) => {
    setViewingGroup(group);
    setViewOpen(true);
  };

  const handleRemoveIp = async (ip: string) => {
    const group = viewGroupQuery.data ?? viewingGroup;
    if (!group) return;
    await removeIpMutation.mutateAsync({ group, ip });
  };

  const groups = groupsQuery.data ?? [];
  const viewGroup = viewGroupQuery.data ?? viewingGroup;

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Network className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex items-center gap-2'>
          <Button
            variant='secondary'
            size='sm'
            className='h-7 text-xs'
            onClick={handleCreate}
          >
            <Plus className='size-3.5 mr-1' />
            {t('create')}
          </Button>
        </div>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <div className='flex items-center justify-between gap-3'>
            <div>
              <CardTitle className='text-base font-semibold'>
                {t('listTitle')}
              </CardTitle>
              <CardDescription>{t('listDesc')}</CardDescription>
            </div>
            <Button
              variant='outline'
              size='sm'
              className='h-7 text-xs'
              onClick={handleRefresh}
              disabled={groupsQuery.isFetching}
            >
              {groupsQuery.isFetching ? (
                <Loader2 className='size-3.5 mr-1 animate-spin' />
              ) : (
                <RefreshCw className='size-3.5 mr-1' />
              )}
              {t('refresh')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {groupsQuery.isLoading ? (
            <LoadingStateWithBorder icon={Network} description={t('loading')} />
          ) : groupsQuery.isError ? (
            <div className='p-8 border border-dashed rounded-lg'>
              <ErrorInline
                message={getErrorMessage(
                  groupsQuery.error,
                  t('operationFailed'),
                )}
                onRetry={handleRefresh}
                className='justify-center'
              />
            </div>
          ) : groups.length === 0 ? (
            <EmptyStateWithBorder icon={Network} description={t('empty')} />
          ) : (
            <IPGroupsTable
              groups={groups}
              syncingId={syncingId}
              onView={handleView}
              onEdit={handleEdit}
              onDelete={setDeleteTarget}
              onSync={(group) => syncMutation.mutate(group.id)}
              onTest={handleTest}
            />
          )}
        </CardContent>
      </Card>

      <IPGroupDialog
        open={editorOpen}
        group={editingGroup}
        submitting={saveMutation.isPending}
        onOpenChange={(open) => {
          setEditorOpen(open);
          if (!open) setEditingGroup(null);
        }}
        onSubmit={async (payload) => {
          await saveMutation.mutateAsync({ group: editingGroup, payload });
        }}
      />

      <IPGroupTestDialog
        open={testOpen}
        loading={testMutation.isPending}
        result={testResult}
        onOpenChange={setTestOpen}
      />

      <IPGroupViewDialog
        open={viewOpen}
        group={viewGroup}
        loading={viewGroupQuery.isFetching && !viewGroupQuery.data}
        removingIp={removingIp}
        onOpenChange={(open) => {
          setViewOpen(open);
          if (!open) {
            setViewingGroup(null);
          }
        }}
        onRemoveIp={handleRemoveIp}
      />

      <AlertDialog
        open={Boolean(deleteTarget)}
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
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {deleteMutation.isPending ? t('deleting') : t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
