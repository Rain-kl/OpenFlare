'use client';

import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { Loader2, Plus, RefreshCw, Server } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
import type { NodeItem, NodeMutationPayload } from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';

import { NodeEditorDialog } from './components/node-editor-dialog';
import {
  filterNodesByType,
  getFilterDescription,
  getNodeFilter,
  NodeTypeFilter,
} from './components/node-type-filter';
import { NodesTable } from './components/nodes-table';
import { getErrorMessage } from './components/node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function NodesPageClient() {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const [editingNode, setEditingNode] = useState<NodeItem | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<NodeItem | null>(null);

  const nodeFilter = useMemo(
    () => getNodeFilter(new URLSearchParams(searchParams.toString())),
    [searchParams],
  );

  const nodesQuery = useQuery({
    queryKey: nodesQueryKey,
    queryFn: () => NodeService.listNodes(),
    refetchInterval: 5000,
  });

  const nodes = useMemo(() => nodesQuery.data ?? [], [nodesQuery.data]);
  const filteredNodes = useMemo(
    () => filterNodesByType(nodes, nodeFilter),
    [nodeFilter, nodes],
  );

  const saveMutation = useMutation({
    mutationFn: async (payload: NodeMutationPayload) => {
      if (editingNode) {
        return NodeService.updateNode(editingNode.id, payload);
      }
      return NodeService.createNode(payload);
    },
    onSuccess: async () => {
      toast.success(editingNode ? t('updated') : t('created'));
      setEditingNode(null);
      setEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('requestFailed')));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => NodeService.deleteNode(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      toast.error(getErrorMessage(error, t('requestFailed')));
    },
  });

  const handleCreate = () => {
    setEditingNode(null);
    setEditorOpen(true);
  };

  const handleEdit = (node: NodeItem) => {
    setEditingNode(node);
    setEditorOpen(true);
  };

  const handleRefresh = () => {
    void queryClient.invalidateQueries({ queryKey: nodesQueryKey });
  };

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Server className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex items-center gap-2'>
          <Button variant='outline' size='sm' className='h-7 text-xs' asChild>
            <Link href='/apply-logs'>{t('applyLogs')}</Link>
          </Button>
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
              <CardDescription>
                {getFilterDescription(nodeFilter, t)}
              </CardDescription>
            </div>
            <Button
              variant='outline'
              size='sm'
              className='h-7 text-xs'
              onClick={handleRefresh}
              disabled={nodesQuery.isFetching}
            >
              {nodesQuery.isFetching ? (
                <Loader2 className='size-3.5 mr-1 animate-spin' />
              ) : (
                <RefreshCw className='size-3.5 mr-1' />
              )}
              {t('refreshNow')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          <NodeTypeFilter />

          {nodesQuery.isLoading ? (
            <LoadingStateWithBorder
              icon={Server}
              description={t('loadingList')}
            />
          ) : nodesQuery.isError ? (
            <div className='p-8 border border-dashed rounded-lg'>
              <ErrorInline
                message={getErrorMessage(nodesQuery.error, t('requestFailed'))}
                onRetry={handleRefresh}
                className='justify-center'
              />
            </div>
          ) : filteredNodes.length === 0 ? (
            <EmptyStateWithBorder
              icon={Server}
              description={
                nodes.length === 0 ? t('emptyAll') : t('emptyFilter')
              }
            />
          ) : (
            <NodesTable
              nodes={filteredNodes}
              deletingId={
                deleteMutation.isPending ? (deleteTarget?.id ?? null) : null
              }
              onEdit={handleEdit}
              onDelete={setDeleteTarget}
            />
          )}
        </CardContent>
      </Card>

      <NodeEditorDialog
        open={editorOpen}
        node={editingNode}
        submitting={saveMutation.isPending}
        onClose={() => {
          setEditorOpen(false);
          setEditingNode(null);
        }}
        onSubmit={async (payload) => {
          await saveMutation.mutateAsync(payload);
        }}
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
              {tc('cancel')}
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
