'use client';

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ChevronDown,
  Loader2,
  Play,
  RefreshCw,
  Square,
  Terminal,
} from 'lucide-react';
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
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import type { ExecTask, ExecTaskResult } from '@/lib/services/openflare';
import { ExecService, NodeService } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { NodeMultiSelector } from '../components/node-multi-selector';
import { getErrorMessage } from '../components/node-utils';
import { ExecHistoryList } from './components/exec-history-list';
import { ExecResultCard } from './components/exec-result-card';

const execTasksQueryKey = ['openflare', 'exec-tasks'];
const TIMEOUT_OPTIONS = [30, 60, 120, 300, 600];

export function NodeExecPageClient() {
  const t = useTranslations('nodeExec');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [command, setCommand] = useState('');
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const [timeoutSeconds, setTimeoutSeconds] = useState<number>(60);
  const [selectorOpen, setSelectorOpen] = useState(true);

  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [activeCommand, setActiveCommand] = useState<string>('');
  const [results, setResults] = useState<ExecTaskResult[]>([]);
  const [polling, setPolling] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ExecTask | null>(null);

  const pollIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearPolling = useCallback(() => {
    if (pollIntervalRef.current) {
      clearInterval(pollIntervalRef.current);
      pollIntervalRef.current = null;
    }
    if (pollTimeoutRef.current) {
      clearTimeout(pollTimeoutRef.current);
      pollTimeoutRef.current = null;
    }
    setPolling(false);
  }, []);

  useEffect(() => clearPolling, [clearPolling]);

  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes', 'all'],
    queryFn: () => NodeService.listNodes(),
  });

  const historyQuery = useQuery({
    queryKey: execTasksQueryKey,
    queryFn: () => ExecService.listTasks(30),
  });

  const pollTask = useCallback(
    async (taskId: string) => {
      try {
        const task = await ExecService.getTask(taskId);
        setResults(task.results ?? []);
        const allFinished =
          (task.results ?? []).length > 0 &&
          (task.results ?? []).every(
            (item) => item.finished_at !== null || item.exit_code !== null,
          );
        if (allFinished) {
          clearPolling();
          toast.success(t('toast.allCompleted'));
          await queryClient.invalidateQueries({ queryKey: execTasksQueryKey });
        }
      } catch (err) {
        clearPolling();
        toast.error(getErrorMessage(err, t('toast.pollFailed')));
      }
    },
    [clearPolling, queryClient, t],
  );

  const startPolling = useCallback(
    (taskId: string, timeoutSec: number) => {
      clearPolling();
      setPolling(true);
      void pollTask(taskId);

      pollIntervalRef.current = setInterval(() => {
        void pollTask(taskId);
      }, 2000);

      pollTimeoutRef.current = setTimeout(
        () => {
          clearPolling();
          toast.warning(t('toast.pollTimeout'));
          void queryClient.invalidateQueries({ queryKey: execTasksQueryKey });
        },
        (timeoutSec + 15) * 1000,
      );
    },
    [clearPolling, pollTask, queryClient, t],
  );

  const runMutation = useMutation({
    mutationFn: () =>
      ExecService.runCommand({
        // Preserve raw shell whitespace and newlines.
        command,
        clients: selectedNodes,
        timeout_seconds: timeoutSeconds,
      }),
    onSuccess: async (resp) => {
      setActiveTaskId(resp.task_id);
      setActiveCommand(resp.command);
      setResults(resp.results ?? []);
      toast.success(t('toast.dispatched'));
      await queryClient.invalidateQueries({ queryKey: execTasksQueryKey });

      const hasPending = (resp.results ?? []).some(
        (item) => item.finished_at === null && item.exit_code === null,
      );
      if (hasPending) {
        startPolling(resp.task_id, timeoutSeconds);
      }
    },
    onError: (err) => toast.error(getErrorMessage(err, t('toast.failed'))),
  });

  const deleteMutation = useMutation({
    mutationFn: (taskId: string) => ExecService.deleteTask(taskId),
    onSuccess: async (_, taskId) => {
      toast.success(t('toast.deleted'));
      if (activeTaskId === taskId) {
        clearPolling();
        setActiveTaskId(null);
        setActiveCommand('');
        setResults([]);
      }
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: execTasksQueryKey });
    },
    onError: (err) => toast.error(getErrorMessage(err, t('toast.failed'))),
  });

  const handleExecute = () => {
    if (!command.trim()) {
      toast.error(t('errors.emptyCommand'));
      return;
    }
    if (selectedNodes.length === 0) {
      toast.error(t('errors.noNodes'));
      return;
    }
    clearPolling();
    runMutation.mutate();
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (
      (event.ctrlKey || event.metaKey) &&
      event.key === 'Enter' &&
      !event.nativeEvent.isComposing
    ) {
      event.preventDefault();
      if (!runMutation.isPending) {
        handleExecute();
      }
    }
  };

  const handleSelectHistory = (task: ExecTask) => {
    clearPolling();
    setActiveTaskId(task.task_id);
    setActiveCommand(task.command);
    setResults(task.results ?? []);
    const hasPending = (task.results ?? []).some(
      (item) => item.finished_at === null && item.exit_code === null,
    );
    if (hasPending) {
      startPolling(task.task_id, 60);
    }
  };

  const nodes = nodesQuery.data ?? [];
  const history = historyQuery.data ?? [];
  const completedCount = results.filter(
    (item) => item.finished_at !== null || item.exit_code !== null,
  ).length;

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Terminal className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>

        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => {
            void nodesQuery.refetch();
            void historyQuery.refetch();
          }}
        >
          <RefreshCw className='mr-1.5 size-3.5' />
          {t('refresh')}
        </Button>
      </div>

      <p className='text-sm text-muted-foreground'>{t('description')}</p>

      <Card>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('composer.title')}
          </CardTitle>
          <CardDescription>{t('composer.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <Label htmlFor='remote-exec-command'>
                {t('composer.commandLabel')}
              </Label>
              <span className='text-xs text-muted-foreground'>
                {t('composer.shortcutHint')}
              </span>
            </div>
            <Textarea
              id='remote-exec-command'
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t('composer.commandPlaceholder')}
              rows={4}
              className='font-mono text-xs leading-relaxed'
            />
          </div>

          <Collapsible open={selectorOpen} onOpenChange={setSelectorOpen}>
            <div className='rounded-lg border border-dashed p-3'>
              <CollapsibleTrigger asChild>
                <button
                  type='button'
                  className='flex w-full items-center justify-between text-sm font-medium text-foreground'
                >
                  <span>
                    {t('composer.selectNodes', { count: selectedNodes.length })}
                  </span>
                  <ChevronDown
                    className={cn(
                      'size-4 text-muted-foreground transition-transform',
                      selectorOpen && 'rotate-180',
                    )}
                  />
                </button>
              </CollapsibleTrigger>
              <CollapsibleContent className='pt-3'>
                <NodeMultiSelector
                  nodes={nodes}
                  value={selectedNodes}
                  onChange={setSelectedNodes}
                />
              </CollapsibleContent>
            </div>
          </Collapsible>

          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='flex items-center gap-2'>
              <Label htmlFor='remote-exec-timeout' className='text-xs'>
                {t('composer.timeout')}
              </Label>
              <Select
                value={String(timeoutSeconds)}
                onValueChange={(value) => setTimeoutSeconds(Number(value))}
              >
                <SelectTrigger
                  id='remote-exec-timeout'
                  className='h-8 w-28 text-xs'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TIMEOUT_OPTIONS.map((sec) => (
                    <SelectItem key={sec} value={String(sec)}>
                      {sec}s
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Button
              type='button'
              disabled={runMutation.isPending}
              onClick={handleExecute}
            >
              {runMutation.isPending ? (
                <Loader2 className='mr-1.5 size-4 animate-spin' />
              ) : (
                <Play className='mr-1.5 size-4' />
              )}
              {runMutation.isPending
                ? t('composer.executing')
                : t('composer.execute')}
            </Button>
          </div>
        </CardContent>
      </Card>

      {activeTaskId ? (
        <section className='space-y-4'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='min-w-0 space-y-1'>
              <div className='flex flex-wrap items-center gap-2'>
                <h2 className='text-base font-semibold tracking-tight'>
                  {t('result.sectionTitle')}
                </h2>
                <Badge variant='secondary' className='font-mono text-xs'>
                  {completedCount} / {results.length}
                </Badge>
                {polling ? (
                  <Badge variant='outline' className='gap-1 text-xs'>
                    <Loader2 className='size-3 animate-spin' />
                    {t('result.polling')}
                  </Badge>
                ) : null}
              </div>
              <p className='truncate font-mono text-xs text-muted-foreground'>
                {activeCommand}
              </p>
            </div>

            {polling ? (
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={clearPolling}
              >
                <Square className='mr-1.5 size-3' />
                {t('result.stopPolling')}
              </Button>
            ) : null}
          </div>

          <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
            {results.map((item) => (
              <ExecResultCard
                key={`${item.task_id}-${item.client}`}
                result={item}
              />
            ))}
          </div>
        </section>
      ) : null}

      <ExecHistoryList
        tasks={history}
        selectedTaskId={activeTaskId}
        onSelect={handleSelectHistory}
        onDelete={setDeleteTarget}
      />

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => (!open ? setDeleteTarget(null) : undefined)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteDialog.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDialog.description')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleteMutation.isPending}
              onClick={() => {
                if (deleteTarget) {
                  deleteMutation.mutate(deleteTarget.task_id);
                }
              }}
            >
              {t('deleteDialog.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
