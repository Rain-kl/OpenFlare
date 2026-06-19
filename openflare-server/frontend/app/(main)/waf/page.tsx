'use client';

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {Loader2, Plus, RefreshCw, Shield} from 'lucide-react';
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
import type {WAFRuleGroup, WAFRuleGroupPayload} from '@/lib/services/openflare';
import {ProxyRouteService, WafService} from '@/lib/services/openflare';

import {getErrorMessage} from './components/helpers';
import {RuleGroupSheet} from './components/rule-group-sheet';
import {RuleGroupsTable} from './components/rule-groups-table';
import {SiteBindingSheet} from './components/site-binding-sheet';

const ruleGroupsQueryKey = ['openflare', 'waf', 'rule-groups'];
const ipGroupsQueryKey = ['openflare', 'waf', 'ip-groups'];
const routesQueryKey = ['openflare', 'proxy-routes'];

export default function WafPage() {
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<WAFRuleGroup | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<WAFRuleGroup | null>(null);
  const [bindingGroup, setBindingGroup] = useState<WAFRuleGroup | null>(null);

  const groupsQuery = useQuery({
    queryKey: ruleGroupsQueryKey,
    queryFn: () => WafService.listRuleGroups(),
  });

  const ipGroupsQuery = useQuery({
    queryKey: ipGroupsQueryKey,
    queryFn: () => WafService.listIPGroups(),
  });

  const routesQuery = useQuery({
    queryKey: routesQueryKey,
    queryFn: () => ProxyRouteService.list(),
  });

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ruleGroupsQueryKey }),
      queryClient.invalidateQueries({ queryKey: ipGroupsQueryKey }),
      queryClient.invalidateQueries({ queryKey: ['openflare', 'config-versions', 'diff'] }),
    ]);
  };

  const saveMutation = useMutation({
    mutationFn: async ({
      group,
      payload,
    }: {
      group: WAFRuleGroup | null;
      payload: WAFRuleGroupPayload;
    }) => {
      if (group) {
        return WafService.updateRuleGroup(group.id, payload);
      }
      return WafService.createRuleGroup(payload);
    },
    onSuccess: async () => {
      toast.success(editingGroup ? '规则组已更新' : '规则组已创建');
      setEditingGroup(null);
      setEditorOpen(false);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => WafService.deleteRuleGroup(id),
    onSuccess: async () => {
      toast.success('规则组已删除');
      setDeleteTarget(null);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
    },
  });

  const bindMutation = useMutation({
    mutationFn: ({ id, ids }: { id: number; ids: number[] }) =>
      WafService.updateRuleGroupSites(id, ids),
    onSuccess: async () => {
      toast.success('规则组应用范围已更新');
      setBindingGroup(null);
      await invalidate();
    },
    onError: (error) => {
      toast.error(getErrorMessage(error));
    },
  });

  const handleRefresh = () => {
    void queryClient.invalidateQueries({ queryKey: ruleGroupsQueryKey });
  };

  const handleCreate = () => {
    setEditingGroup(null);
    setEditorOpen(true);
  };

  const handleEdit = (group: WAFRuleGroup) => {
    setEditingGroup(group);
    setEditorOpen(true);
  };

  const groups = groupsQuery.data ?? [];
  const loading = groupsQuery.isLoading || ipGroupsQuery.isLoading || routesQuery.isLoading;
  const error =
    groupsQuery.error ?? ipGroupsQuery.error ?? routesQuery.error ?? null;

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Shield className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">WAF</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" className="h-7 text-xs" onClick={handleCreate}>
            <Plus className="size-3.5 mr-1" />
            新建规则组
          </Button>
        </div>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base font-semibold">规则组</CardTitle>
              <CardDescription>
                按规则组维护 WAF 与 PoW 防护规则，全局规则组始终应用到所有网站。
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
          {loading ? (
            <LoadingStateWithBorder icon={Shield} description="加载 WAF 规则组中..." />
          ) : error ? (
            <div className="p-8 border border-dashed rounded-lg">
              <ErrorInline
                message={getErrorMessage(error)}
                onRetry={handleRefresh}
                className="justify-center"
              />
            </div>
          ) : groups.length === 0 ? (
            <EmptyStateWithBorder
              icon={Shield}
              description="暂无规则组，系统通常会自动创建全局规则组。"
            />
          ) : (
            <RuleGroupsTable
              groups={groups}
              onEdit={handleEdit}
              onDelete={setDeleteTarget}
              onBindSites={setBindingGroup}
            />
          )}
        </CardContent>
      </Card>

      <RuleGroupSheet
        open={editorOpen}
        group={editingGroup}
        ipGroups={ipGroupsQuery.data ?? []}
        submitting={saveMutation.isPending}
        onOpenChange={(open) => {
          setEditorOpen(open);
          if (!open) setEditingGroup(null);
        }}
        onSubmit={async (payload) => {
          await saveMutation.mutateAsync({ group: editingGroup, payload });
        }}
      />

      <SiteBindingSheet
        group={bindingGroup}
        routes={routesQuery.data ?? []}
        open={Boolean(bindingGroup)}
        pending={bindMutation.isPending}
        onOpenChange={(open) => !open && setBindingGroup(null)}
        onSave={(ids) => {
          if (bindingGroup) {
            bindMutation.mutate({ id: bindingGroup.id, ids });
          }
        }}
      />

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除规则组</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除规则组「{deleteTarget?.name}」吗？删除后无法恢复。
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
