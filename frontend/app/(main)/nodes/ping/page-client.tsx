'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Activity, Loader2, Plus, RefreshCw } from 'lucide-react';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type {
  PingTask,
  PingTaskBatchUpdateItem,
  PingTaskPayload,
} from '@/lib/services/openflare';
import { NodeService, PingService } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { getErrorMessage } from '../components/node-utils';
import { PingServerView } from './components/ping-server-view';
import { PingTaskDialog } from './components/ping-task-dialog';
import { PingTaskView } from './components/ping-task-view';

const pingTasksQueryKey = ['openflare', 'ping-tasks'];

export function NodePingPageClient() {
  const t = useTranslations('nodePing');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [activeTab, setActiveTab] = useState<'tasks' | 'servers'>('tasks');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<PingTask | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PingTask | null>(null);

  const tasksQuery = useQuery({
    queryKey: pingTasksQueryKey,
    queryFn: () => PingService.listTasks(),
  });

  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes', 'all'],
    queryFn: () => NodeService.listNodes(),
  });

  const createMutation = useMutation({
    mutationFn: (payload: PingTaskPayload) => PingService.createTask(payload),
    onSuccess: async () => {
      toast.success(t('toast.created'));
      setDialogOpen(false);
      await queryClient.invalidateQueries({ queryKey: pingTasksQueryKey });
    },
    onError: (err) => toast.error(getErrorMessage(err, t('toast.failed'))),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: PingTaskPayload }) =>
      PingService.updateTask(id, payload),
    onSuccess: async () => {
      toast.success(t('toast.updated'));
      setDialogOpen(false);
      setEditingTask(null);
      await queryClient.invalidateQueries({ queryKey: pingTasksQueryKey });
    },
    onError: (err) => toast.error(getErrorMessage(err, t('toast.failed'))),
  });

  const batchUpdateMutation = useMutation({
    mutationFn: (items: PingTaskBatchUpdateItem[]) =>
      PingService.batchUpdateTasks(items),
    onSuccess: async () => {
      toast.success(t('toast.updated'));
      await queryClient.invalidateQueries({ queryKey: pingTasksQueryKey });
    },
    onError: (err) => toast.error(getErrorMessage(err, t('toast.failed'))),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => PingService.deleteTask(id),
    onSuccess: async () => {
      toast.success(t('toast.deleted'));
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: pingTasksQueryKey });
    },
    onError: (err) => toast.error(getErrorMessage(err, t('toast.failed'))),
  });

  const reorderMutation = useMutation({
    mutationFn: (weights: Record<string, number>) =>
      PingService.reorderTasks(weights),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: pingTasksQueryKey });
    },
    onError: (err) => toast.error(getErrorMessage(err, t('toast.failed'))),
  });

  const tasks = tasksQuery.data ?? [];
  const nodes = nodesQuery.data ?? [];
  const loading = tasksQuery.isLoading || nodesQuery.isLoading;

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Activity className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>

        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => {
              void tasksQuery.refetch();
              void nodesQuery.refetch();
            }}
          >
            <RefreshCw className='mr-1.5 size-3.5' />
            {t('refresh')}
          </Button>
          <Button
            type='button'
            size='sm'
            onClick={() => {
              setEditingTask(null);
              setDialogOpen(true);
            }}
          >
            <Plus className='mr-1.5 size-4' />
            {t('createTask')}
          </Button>
        </div>
      </div>

      <p className='text-sm text-muted-foreground'>{t('description')}</p>

      <Tabs
        value={activeTab}
        onValueChange={(value) => setActiveTab(value as 'tasks' | 'servers')}
        className='w-full gap-0'
      >
        <TabsList
          variant='line'
          className='h-auto w-full justify-start gap-6 bg-transparent p-0'
        >
          <TabsTrigger
            value='tasks'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            {t('tabs.tasks')}
          </TabsTrigger>
          <TabsTrigger
            value='servers'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            {t('tabs.servers')}
          </TabsTrigger>
        </TabsList>

        {loading ? (
          <div className='mt-6 flex items-center justify-center rounded-xl border border-dashed py-12 text-sm text-muted-foreground'>
            <Loader2 className='mr-2 size-4 animate-spin' />
            {t('loading')}
          </div>
        ) : (
          <>
            <TabsContent value='tasks' className='mt-6 outline-none'>
              <PingTaskView
                tasks={tasks}
                nodes={nodes}
                onEdit={(task) => {
                  setEditingTask(task);
                  setDialogOpen(true);
                }}
                onDelete={setDeleteTarget}
                onReorder={async (weights) => {
                  await reorderMutation.mutateAsync(weights);
                }}
                onUpdateTask={async (id, payload) => {
                  await updateMutation.mutateAsync({ id, payload });
                }}
              />
            </TabsContent>

            <TabsContent value='servers' className='mt-6 outline-none'>
              <PingServerView
                tasks={tasks}
                nodes={nodes}
                onBatchUpdate={async (items) => {
                  await batchUpdateMutation.mutateAsync(items);
                }}
              />
            </TabsContent>
          </>
        )}
      </Tabs>

      <PingTaskDialog
        open={dialogOpen}
        task={editingTask}
        nodes={nodes}
        submitting={createMutation.isPending || updateMutation.isPending}
        onClose={() => {
          setDialogOpen(false);
          setEditingTask(null);
        }}
        onSubmit={async (payload) => {
          if (editingTask) {
            await updateMutation.mutateAsync({ id: editingTask.id, payload });
          } else {
            await createMutation.mutateAsync(payload);
          }
        }}
      />

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => (!open ? setDeleteTarget(null) : undefined)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteDialog.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDialog.description', {
                name: deleteTarget?.name ?? '',
              })}
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
                  deleteMutation.mutate(deleteTarget.id);
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
