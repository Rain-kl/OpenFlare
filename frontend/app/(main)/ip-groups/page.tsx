'use client';

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {Loader2, Network, Plus, RefreshCw} from 'lucide-react';
import {useState} from 'react';
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
import type {WAFIPGroup, WAFIPGroupAutoTestResult, WAFIPGroupPayload,} from '@/lib/services/openflare';
import {WafService} from '@/lib/services/openflare';

import {buildIPGroupPayloadFromGroup, getErrorMessage, parseAutomaticConfig} from '../components/helpers';
import {IPGroupDialog} from '../components/ip-group-dialog';
import {IPGroupTestDialog} from '../components/ip-group-test-dialog';
import {IPGroupViewDialog} from '../components/ip-group-view-dialog';
import {IPGroupsTable} from '../components/ip-groups-table';

const ipGroupsQueryKey = ['openflare', 'waf', 'ip-groups'];

export default function WafIPGroupsPage() {
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<WAFIPGroup | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<WAFIPGroup | null>(null);
  const [testOpen, setTestOpen] = useState(false);
  const [testResult, setTestResult] = useState<WAFIPGroupAutoTestResult | null>(null);
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
      queryClient.invalidateQueries({ queryKey: ['openflare', 'waf', 'rule-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['openflare', 'config-versions', 'diff'] }),
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
      toast.success(editingGroup ? 'IP 组已更新' : 'IP 组已创建');
      setEditingGroup(null);
      setEditorOpen(false);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => WafService.deleteIPGroup(id),
    onSuccess: async () => {
      toast.success('IP 组已删除');
      setDeleteTarget(null);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
    },
  });

  const syncMutation = useMutation({
    mutationFn: (id: number) => WafService.syncIPGroup(id),
    onMutate: (id) => {
      setSyncingId(id);
    },
    onSuccess: async (result) => {
      toast.success(result.message || '同步完成');
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
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
      return WafService.updateIPGroup(group.id, buildIPGroupPayloadFromGroup(group, nextIpList));
    },
    onMutate: ({ ip }) => {
      setRemovingIp(ip);
    },
    onSuccess: async (updatedGroup) => {
      toast.success('IP 已移除');
      setViewingGroup(updatedGroup);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
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
        ),
      }),
    onSuccess: (result) => {
      setTestResult(result);
      toast.success(
        result.matched_count > 0
          ? `规则测试完成，命中 ${result.matched_count} 个 IP`
          : '规则测试完成，当前未命中任何 IP',
      );
    },
    onError: (error) => {
      setTestResult(null);
      toast.error(getErrorMessage(error));
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
    <div className="py-6 px-1 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Network className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">IP 组</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" className="h-7 text-xs" onClick={handleCreate}>
            <Plus className="size-3.5 mr-1" />
            新建 IP 组
          </Button>
        </div>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base font-semibold">IP 组列表</CardTitle>
              <CardDescription>
                维护可被 WAF IP 黑白名单引用的手动、自动与订阅 IP 集合。
              </CardDescription>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={handleRefresh}
              disabled={groupsQuery.isFetching}
            >
              {groupsQuery.isFetching ? (
                <Loader2 className="size-3.5 mr-1 animate-spin" />
              ) : (
                <RefreshCw className="size-3.5 mr-1" />
              )}
              刷新
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {groupsQuery.isLoading ? (
            <LoadingStateWithBorder icon={Network} description="加载 IP 组中..." />
          ) : groupsQuery.isError ? (
            <div className="p-8 border border-dashed rounded-lg">
              <ErrorInline
                message={getErrorMessage(groupsQuery.error)}
                onRetry={handleRefresh}
                className="justify-center"
              />
            </div>
          ) : groups.length === 0 ? (
            <EmptyStateWithBorder icon={Network} description="暂无 IP 组，请先创建一个。" />
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
            <AlertDialogTitle>确认删除 IP 组</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除 IP 组「{deleteTarget?.name}」吗？删除后无法恢复。
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
