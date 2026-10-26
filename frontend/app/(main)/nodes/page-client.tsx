'use client';

import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';
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
import type {
  NodeItem,
  NodeListQuery,
  NodeMutationPayload,
  NodeStatus,
  NodeType,
} from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';

import { NodeEditorDialog } from './components/node-editor-dialog';
import {
  getFilterDescription,
  getNodeFilter,
  NodeTypeFilter,
  type NodeFilter,
} from './components/node-type-filter';
import { NodesTable } from './components/nodes-table';
import { NodeListControls } from './components/node-list-controls';
import { getErrorMessage } from './components/node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function NodesPageClient() {
  const searchParams = useSearchParams();
  const nodeFilter = getNodeFilter(
    new URLSearchParams(searchParams.toString()),
  );
  return <NodesPage key={nodeFilter} nodeFilter={nodeFilter} />;
}

function NodesPage({ nodeFilter }: { nodeFilter: NodeFilter }) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [editingNode, setEditingNode] = useState<NodeItem | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<NodeItem | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [status, setStatus] = useState<NodeStatus | ''>('');
  const searchTooLong = new TextEncoder().encode(search.trim()).length > 256;

  const resetNodeActions = useCallback(() => {
    setEditingNode(null);
    setEditorOpen(false);
    setDeleteTarget(null);
  }, []);

  useEffect(() => {
    if (searchTooLong || search.trim() === debouncedSearch) return;
    const timeout = setTimeout(() => {
      setDebouncedSearch(search.trim());
      setPage(1);
      resetNodeActions();
    }, 300);
    return () => clearTimeout(timeout);
  }, [search, searchTooLong, debouncedSearch, resetNodeActions]);

  const nodeTypes: Record<NodeFilter, NodeType | ''> = {
    all: '',
    edge: 'edge_node',
    relay: 'tunnel_relay',
    tunnel: 'tunnel_client',
  };
  const query = {
    page,
    page_size: pageSize,
    search: debouncedSearch,
    status,
    node_type: nodeTypes[nodeFilter],
  };
  const nodesQuery = useQuery({
    queryKey: [...nodesQueryKey, 'page', query],
    queryFn: () => NodeService.listNodePage(query),
    placeholderData: (previousData, previousQuery) => {
      const previousParams = previousQuery?.queryKey[3] as
        | NodeListQuery
        | undefined;
      return previousParams?.search === query.search &&
        previousParams?.status === query.status &&
        previousParams?.node_type === query.node_type &&
        previousParams?.page_size === query.page_size
        ? previousData
        : undefined;
    },
    refetchInterval: (query) =>
      query.state.status === 'error' ? false : 15000,
    refetchIntervalInBackground: false,
  });

  const nodes = useMemo(
    () => nodesQuery.data?.results ?? [],
    [nodesQuery.data],
  );
  const total = nodesQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  if (
    !nodesQuery.isPlaceholderData &&
    nodesQuery.data?.page === page &&
    page > totalPages
  ) {
    setPage(totalPages);
    resetNodeActions();
  }

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
    if (nodesQuery.isPlaceholderData) return;
    setEditingNode(node);
    setEditorOpen(true);
  };

  const handleRefresh = () => {
    void queryClient.invalidateQueries({ queryKey: nodesQueryKey });
  };

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
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
        <CardContent className='flex flex-col gap-4'>
          <NodeTypeFilter />
          <NodeListControls
            search={search}
            searchTooLong={searchTooLong}
            onSearchChange={setSearch}
            status={status}
            onStatusChange={(value) => {
              setStatus(value);
              setPage(1);
              resetNodeActions();
            }}
            page={nodesQuery.data?.page ?? page}
            pageSize={pageSize}
            onPageSizeChange={(value) => {
              setPageSize(value);
              setPage(1);
              resetNodeActions();
            }}
            total={total}
            totalPages={totalPages}
            loading={nodesQuery.isFetching || nodesQuery.isPlaceholderData}
            hasData={Boolean(nodesQuery.data)}
            onPageChange={(value) => {
              setPage(value);
              resetNodeActions();
            }}
          />

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
          ) : nodes.length === 0 ? (
            <EmptyStateWithBorder
              icon={Server}
              description={
                debouncedSearch || status || nodeFilter !== 'all'
                  ? t('emptyFilter')
                  : t('emptyAll')
              }
            />
          ) : (
            <NodesTable
              nodes={nodes}
              deletingId={
                deleteMutation.isPending ? (deleteTarget?.id ?? null) : null
              }
              onEdit={handleEdit}
              onDelete={(node) => {
                if (!nodesQuery.isPlaceholderData) setDeleteTarget(node);
              }}
              actionsDisabled={nodesQuery.isPlaceholderData}
            />
          )}
        </CardContent>
      </Card>

      <NodeEditorDialog
        open={editorOpen}
        node={editingNode}
        submitting={
          saveMutation.isPending ||
          (Boolean(editingNode) && nodesQuery.isPlaceholderData)
        }
        onClose={() => {
          setEditorOpen(false);
          setEditingNode(null);
        }}
        onSubmit={async (payload) => {
          if (!editingNode || !nodesQuery.isPlaceholderData)
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
            <AlertDialogCancel
              disabled={
                deleteMutation.isPending || nodesQuery.isPlaceholderData
              }
            >
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={
                deleteMutation.isPending || nodesQuery.isPlaceholderData
              }
              onClick={() =>
                deleteTarget &&
                !nodesQuery.isPlaceholderData &&
                deleteMutation.mutate(deleteTarget.id)
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
