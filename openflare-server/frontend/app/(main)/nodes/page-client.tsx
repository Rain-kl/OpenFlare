'use client';

import Link from 'next/link';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useMemo, useState} from 'react';
import {useSearchParams} from 'next/navigation';
import {Loader2, Plus, RefreshCw, Server} from 'lucide-react';
import {toast} from 'sonner';

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
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardDescription, CardHeader, CardTitle,} from '@/components/ui/card';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import type {NodeItem, NodeMutationPayload} from '@/lib/services/openflare';
import {NodeService} from '@/lib/services/openflare';

import {NodeEditorDialog} from './components/node-editor-dialog';
import {filterNodesByType, getFilterDescription, getNodeFilter, NodeTypeFilter,} from './components/node-type-filter';
import {NodesTable} from './components/nodes-table';
import {getErrorMessage} from './components/node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function NodesPageClient() {
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
      toast.success(editingNode ? '节点已更新' : '节点已创建');
      setEditingNode(null);
      setEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => NodeService.deleteNode(id),
    onSuccess: async () => {
      toast.success('节点已删除');
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
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
    <div className="py-6 px-1 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Server className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">节点管理</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" className="h-7 text-xs" asChild>
            <Link href="/apply-logs">应用记录</Link>
          </Button>
          <Button variant="secondary" size="sm" className="h-7 text-xs" onClick={handleCreate}>
            <Plus className="size-3.5 mr-1" />
            新增节点
          </Button>
        </div>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base font-semibold">节点列表</CardTitle>
              <CardDescription>{getFilterDescription(nodeFilter)}</CardDescription>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={handleRefresh}
              disabled={nodesQuery.isFetching}
            >
              {nodesQuery.isFetching ? (
                <Loader2 className="size-3.5 mr-1 animate-spin" />
              ) : (
                <RefreshCw className="size-3.5 mr-1" />
              )}
              立即刷新
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <NodeTypeFilter />

          {nodesQuery.isLoading ? (
            <LoadingStateWithBorder icon={Server} description="加载节点列表中..." />
          ) : nodesQuery.isError ? (
            <div className="p-8 border border-dashed rounded-lg">
              <ErrorInline
                message={getErrorMessage(nodesQuery.error)}
                onRetry={handleRefresh}
                className="justify-center"
              />
            </div>
          ) : filteredNodes.length === 0 ? (
            <EmptyStateWithBorder
              icon={Server}
              description={nodes.length === 0 ? '暂无节点，请先创建一个节点。' : '当前筛选无结果'}
            />
          ) : (
            <NodesTable
              nodes={filteredNodes}
              deletingId={deleteMutation.isPending ? deleteTarget?.id ?? null : null}
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

      <AlertDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除节点</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除节点「{deleteTarget?.name}」吗？删除后该节点需要重新创建并重新接入。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              disabled={deleteMutation.isPending}
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
            >
              {deleteMutation.isPending ? '删除中...' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
