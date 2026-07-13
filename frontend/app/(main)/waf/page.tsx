'use client';

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {Loader2, Plus, RefreshCw, Shield} from 'lucide-react';
import {useRouter} from 'next/navigation';
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
import type {WAFRule} from '@/lib/services/openflare';
import {WafService} from '@/lib/services/openflare';

import {CreateRuleDialog} from './components/create-rule-dialog';
import {getErrorMessage} from './components/helpers';
import {RuleGroupsTable} from './components/rule-groups-table';

const ruleGroupsQueryKey = ['openflare', 'waf', 'rule-groups'];

export default function WafPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<WAFRule | null>(null);

  const groupsQuery = useQuery({
    queryKey: ruleGroupsQueryKey,
    queryFn: () => WafService.listRuleGroups(),
  });

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ruleGroupsQueryKey }),
      queryClient.invalidateQueries({ queryKey: ['openflare', 'config-versions', 'diff'] }),
    ]);
  };

  const createMutation = useMutation({
    mutationFn: (name: string) => WafService.createRule({name}),
    onSuccess: (rule) => {
      toast.success('规则已创建');
      setCreateOpen(false);
      router.push(`/waf/rules/editor?id=${rule.id}`);
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

  const handleRefresh = () => {
    void queryClient.invalidateQueries({ queryKey: ruleGroupsQueryKey });
  };

  const groups = groupsQuery.data ?? [];
  const loading = groupsQuery.isLoading;
  const error = groupsQuery.error ?? null;

  return (
    <div className="w-full py-6 px-1 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Shield className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">WAF</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={() => setCreateOpen(true)}>
            <Plus data-icon="inline-start" />
            新建规则
          </Button>
        </div>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base font-semibold">规则组</CardTitle>
              <CardDescription>
                使用可视化流程编排 WAF 防护规则；全局规则始终应用到所有网站。
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
              onEdit={(rule) => router.push(`/waf/rules/editor?id=${rule.id}`)}
              onDelete={setDeleteTarget}
            />
          )}
        </CardContent>
      </Card>

      <CreateRuleDialog
        open={createOpen}
        pending={createMutation.isPending}
        onOpenChange={setCreateOpen}
        onCreate={async (name) => {
          await createMutation.mutateAsync(name);
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
