'use client';

import { Suspense, useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import axios from 'axios';
import { ArrowLeft, GitBranch, Save } from 'lucide-react';
import { useRouter, useSearchParams } from 'next/navigation';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import {
  services,
  type WAFRule,
  type WAFRuleGraph,
  type WAFRuleNode,
} from '@/lib/services';

import { getErrorMessage } from '../../components/helpers';
import {
  findGraphErrorTarget,
  type GraphErrorTarget,
} from './components/editor-behavior';
import { layoutRuleGraph } from './components/graph-layout';
import { validateGraph } from './components/graph-validation';
import { NodeProperties } from './components/node-properties';
import { RuleFlowCanvas } from './components/rule-flow-canvas';
import { UnsavedChanges } from './components/unsaved-changes';

export default function WAFRuleEditorPage() {
  return (
    <Suspense fallback={<EditorSkeleton />}>
      <EditorContent />
    </Suspense>
  );
}

function EditorContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const id = Number(searchParams.get('id'));
  const [graph, setGraph] = useState<WAFRuleGraph>();
  const [revision, setRevision] = useState(0);
  const [selectedId, setSelectedId] = useState<string>();
  const [selectedEdgeId, setSelectedEdgeId] = useState<string>();
  const [focusTarget, setFocusTarget] = useState<GraphErrorTarget>();
  const [dirty, setDirty] = useState(false);
  const [conflict, setConflict] = useState(false);
  const ruleQueryKey = ['waf-rule', id] as const;

  const ruleQuery = useQuery({
    queryKey: ruleQueryKey,
    queryFn: () => services.openflareWaf.getRule(id),
    enabled: Number.isFinite(id) && id > 0,
  });
  const ipGroupsQuery = useQuery({
    queryKey: ['waf-ip-groups'],
    queryFn: () => services.openflareWaf.listIPGroups(),
  });
  useEffect(() => {
    if (ruleQuery.data && !dirty) {
      setGraph(ruleQuery.data.graph);
      setRevision(ruleQuery.data.revision);
    }
  }, [dirty, ruleQuery.data]);
  const issues = useMemo(() => (graph ? validateGraph(graph) : []), [graph]);
  const selected = graph?.nodes.find((node) => node.id === selectedId);

  const saveMutation = useMutation({
    mutationFn: () =>
      services.openflareWaf.saveRuleGraph(id, { revision, graph: graph! }),
    onSuccess: (rule) => {
      setGraph(rule.graph);
      setRevision(rule.revision);
      setDirty(false);
      setConflict(false);
      queryClient.setQueryData(['waf-rule', id], rule);
      toast.success('规则图已保存');
    },
    onError: (error) => {
      if (axios.isAxiosError(error) && error.response?.status === 409) {
        setConflict(true);
        toast.error('规则已在其他页面更新，请重新加载');
        return;
      }
      const payload = axios.isAxiosError(error) ? error.response?.data : error;
      const target = graph
        ? findGraphErrorTarget(
            payload,
            graph.nodes.map((node) => node.id),
            graph.edges.map((edge) => edge.id),
          )
        : undefined;
      if (target?.kind === 'node') {
        setSelectedEdgeId(undefined);
        setSelectedId(target.id);
      }
      if (target?.kind === 'edge') {
        setSelectedId(undefined);
        setSelectedEdgeId(target.id);
      }
      setFocusTarget(target ? { ...target } : undefined);
      toast.error('保存失败，请检查标记的节点和连线');
    },
  });

  const metaMutation = useMutation({
    mutationFn: (enabled: boolean) =>
      services.openflareWaf.updateRuleMeta(id, {
        name: ruleQuery.data!.name,
        enabled,
      }),
    onMutate: async (enabled) => {
      await queryClient.cancelQueries({ queryKey: ruleQueryKey });
      const previous = queryClient.getQueryData<WAFRule>(ruleQueryKey);
      queryClient.setQueryData<WAFRule>(ruleQueryKey, (current) =>
        current ? { ...current, enabled } : current,
      );
      return { previous };
    },
    onError: (error, _enabled, context) => {
      if (context?.previous) {
        queryClient.setQueryData(ruleQueryKey, context.previous);
      }
      toast.error(getErrorMessage(error));
    },
    onSuccess: (rule) => {
      queryClient.setQueryData(ruleQueryKey, rule);
      void queryClient.invalidateQueries({
        queryKey: ruleQueryKey,
        refetchType: 'none',
      });
      void queryClient.invalidateQueries({
        queryKey: ['openflare', 'waf', 'rule-groups'],
      });
      void queryClient.invalidateQueries({
        queryKey: ['openflare', 'config-versions', 'diff'],
      });
      toast.success(rule.enabled ? '规则已启用' : '规则已停用');
    },
  });

  const changeGraph = useCallback((next: WAFRuleGraph, persistent = true) => {
    setGraph(next);
    if (persistent) {
      setDirty(true);
      setConflict(false);
    }
  }, []);
  const changeNode = useCallback(
    (next: WAFRuleNode) => {
      if (!graph) return;
      changeGraph({
        ...graph,
        nodes: graph.nodes.map((node) => (node.id === next.id ? next : node)),
      });
    },
    [changeGraph, graph],
  );
  const formatLayout = useCallback(() => {
    if (!graph) return;
    changeGraph(layoutRuleGraph(graph));
  }, [changeGraph, graph]);
  const leave = () => {
    if (!dirty || window.confirm('存在未保存的更改，确定离开吗？'))
      router.push('/waf');
  };

  if (!Number.isFinite(id) || id <= 0)
    return (
      <div className='w-full px-1 py-6'>
        <p className='text-sm text-destructive'>缺少有效的规则 ID。</p>
      </div>
    );
  if (ruleQuery.isError)
    return (
      <div className='flex w-full flex-col items-start gap-3 px-1 py-6'>
        <p className='text-sm text-destructive'>规则加载失败，请重试。</p>
        <Button variant='outline' onClick={() => void ruleQuery.refetch()}>
          重新加载
        </Button>
      </div>
    );
  if (ruleQuery.isLoading || !graph || !ruleQuery.data)
    return <EditorSkeleton />;

  return (
    <div className='flex h-[calc(100dvh-8rem)] w-full flex-col px-1 py-6'>
      <UnsavedChanges dirty={dirty} />
      <header className='mb-4 flex flex-col gap-4'>
        <Button
          variant='ghost'
          size='sm'
          className='h-8 w-fit gap-1.5 px-0 text-xs'
          onClick={leave}
        >
          <ArrowLeft className='size-3.5' />
          返回
        </Button>
        <div className='flex items-center justify-between gap-4'>
          <div className='flex min-w-0 items-center gap-2'>
            <GitBranch className='size-5 text-primary' />
            <h1 className='text-2xl font-semibold tracking-tight'>
              {ruleQuery.data.name}
            </h1>
            <Badge variant={ruleQuery.data.enabled ? 'default' : 'secondary'}>
              {ruleQuery.data.enabled ? '已启用' : '已停用'}
            </Badge>
            <Badge variant={issues.length === 0 ? 'outline' : 'destructive'}>
              {issues.length === 0 ? '图校验通过' : `${issues.length} 个问题`}
            </Badge>
            {dirty && <Badge variant='secondary'>未保存</Badge>}
          </div>
          <div className='flex shrink-0 items-center gap-2'>
            <div className='flex items-center gap-2'>
              <Switch
                id='rule-enabled'
                checked={ruleQuery.data.enabled}
                disabled={
                  dirty ||
                  issues.length > 0 ||
                  saveMutation.isPending ||
                  metaMutation.isPending
                }
                onCheckedChange={(enabled) => metaMutation.mutate(enabled)}
              />
              <Label htmlFor='rule-enabled'>启用规则</Label>
            </div>
            {conflict && (
              <Button
                variant='outline'
                onClick={() => {
                  setDirty(false);
                  setConflict(false);
                  void ruleQuery.refetch();
                }}
              >
                重新加载
              </Button>
            )}
            <Button
              type='button'
              variant='outline'
              title='自动整理节点与连线布局'
              disabled={!graph}
              onClick={formatLayout}
            >
              格式化
            </Button>
            <Button
              disabled={!dirty || issues.length > 0 || saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              <Save data-icon='inline-start' />
              {saveMutation.isPending ? '保存中...' : '保存'}
            </Button>
          </div>
        </div>
      </header>
      <div className='flex min-h-0 flex-1 overflow-hidden rounded-xl border bg-background shadow-sm'>
        <RuleFlowCanvas
          graph={graph}
          issues={issues}
          selectedId={selectedId}
          selectedEdgeId={selectedEdgeId}
          focusTarget={focusTarget}
          onGraphChange={changeGraph}
          onSelect={setSelectedId}
          onSelectEdge={setSelectedEdgeId}
        />
        {selected && (
          <NodeProperties
            node={selected}
            ipGroups={ipGroupsQuery.data ?? []}
            onChange={changeNode}
          />
        )}
      </div>
    </div>
  );
}

function EditorSkeleton() {
  return (
    <div className='flex w-full flex-col gap-4 px-1 py-6'>
      <div className='flex items-center gap-2'>
        <Skeleton className='size-5' />
        <Skeleton className='h-8 w-64' />
      </div>
      <Skeleton className='h-[70dvh] w-full' />
    </div>
  );
}
