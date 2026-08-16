'use client';

import { Suspense, useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import axios from 'axios';
import { ArrowLeft, GitBranch, Save } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useRouter, useSearchParams } from 'next/navigation';
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
  const t = useTranslations('waf.editor');
  const tWaf = useTranslations('waf');
  const tCommon = useTranslations('common');
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
      toast.success(t('saved'));
    },
    onError: (error) => {
      if (axios.isAxiosError(error) && error.response?.status === 409) {
        setConflict(true);
        toast.error(t('conflict'));
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
      toast.error(t('saveCheckNodes'));
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
      toast.error(getErrorMessage(error, tWaf('operationFailed')));
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
      toast.success(rule.enabled ? t('ruleEnabled') : t('ruleDisabled'));
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
  const [leaveConfirmOpen, setLeaveConfirmOpen] = useState(false);
  const leave = () => {
    if (!dirty) {
      router.push('/waf');
      return;
    }
    setLeaveConfirmOpen(true);
  };

  if (!Number.isFinite(id) || id <= 0)
    return (
      <div className='w-full px-1 py-6'>
        <p className='text-sm text-destructive'>{t('missingId')}</p>
      </div>
    );
  if (ruleQuery.isError)
    return (
      <div className='flex w-full flex-col items-start gap-3 px-1 py-6'>
        <p className='text-sm text-destructive'>{t('loadFailed')}</p>
        <Button variant='outline' onClick={() => void ruleQuery.refetch()}>
          {t('reload')}
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
          {t('back')}
        </Button>
        <div className='flex items-center justify-between gap-4'>
          <div className='flex min-w-0 items-center gap-2'>
            <GitBranch className='size-5 text-primary' />
            <h1 className='text-2xl font-semibold tracking-tight'>
              {ruleQuery.data.name}
            </h1>
            <Badge variant={ruleQuery.data.enabled ? 'default' : 'secondary'}>
              {ruleQuery.data.enabled ? tWaf('enabled') : tWaf('disabled')}
            </Badge>
            <Badge variant={issues.length === 0 ? 'outline' : 'destructive'}>
              {issues.length === 0
                ? t('graphValid')
                : t('issueCount', { count: issues.length })}
            </Badge>
            {dirty && <Badge variant='secondary'>{t('unsaved')}</Badge>}
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
              <Label htmlFor='rule-enabled'>{t('enableRule')}</Label>
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
                {t('reload')}
              </Button>
            )}
            <Button
              type='button'
              variant='outline'
              title={t('formatTitle')}
              disabled={!graph}
              onClick={formatLayout}
            >
              {t('format')}
            </Button>
            <Button
              disabled={!dirty || issues.length > 0 || saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              <Save data-icon='inline-start' />
              {saveMutation.isPending ? t('saving') : t('save')}
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

      <AlertDialog open={leaveConfirmOpen} onOpenChange={setLeaveConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('leaveTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('leaveDesc')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon('cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => router.push('/waf')}>
              {t('leaveConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
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
