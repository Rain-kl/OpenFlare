'use client';

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Gauge, Pencil, Plus, Server, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';
import {
  AlertDialog,
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
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import {
  PerformanceService,
  type PerformancePolicy,
} from '@/lib/services/openflare';
import { NodeAssignmentDialog } from './components/node-assignment-dialog';
import { PolicyEditor } from './components/policy-editor';

const queryKey = ['openflare', 'performance-policies'];

type Action =
  | { type: 'edit'; policy?: PerformancePolicy }
  | { type: 'assign' | 'delete'; policy: PerformancePolicy };

export default function PerformancePage() {
  const t = useTranslations('performancePolicies');
  const client = useQueryClient();
  const query = useQuery({
    queryKey,
    queryFn: () => PerformanceService.list(),
  });
  const [action, setAction] = useState<Action>();
  const [deleting, setDeleting] = useState(false);
  const policies = query.data?.policies ?? [];
  const nodes = query.data?.nodes ?? [];
  const counts = new Map<number, number>();
  for (const { policy_id } of nodes) {
    if (policy_id !== null) {
      counts.set(policy_id, (counts.get(policy_id) ?? 0) + 1);
    }
  }

  function complete() {
    setAction(undefined);
    void client.invalidateQueries({ queryKey });
  }

  async function remove(policy: PerformancePolicy) {
    setDeleting(true);
    try {
      await PerformanceService.remove(policy.id);
      toast.success(t('deleted'));
      complete();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('operationFailed'),
      );
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className='flex w-full flex-col gap-6 py-6'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <Gauge className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <Button
          disabled={!query.data}
          onClick={() => setAction({ type: 'edit' })}
        >
          <Plus data-icon='inline-start' />
          {t('create')}
        </Button>
      </div>

      <p className='text-sm text-muted-foreground'>{t('description')}</p>

      {query.isPending && <Skeleton className='h-28 w-full' />}
      {query.isError && (
        <div className='flex items-center gap-3'>
          <p className='text-sm'>{t('loadFailed')}</p>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void query.refetch()}
          >
            {t('retry')}
          </Button>
        </div>
      )}
      {query.isSuccess && policies.length === 0 && (
        <Card>
          <CardHeader>
            <CardDescription>{t('empty')}</CardDescription>
          </CardHeader>
        </Card>
      )}
      <div className='grid gap-3 xl:grid-cols-2'>
        {policies.map((policy) => (
          <Card key={policy.id}>
            <CardHeader>
              <CardTitle className='break-all'>{policy.name}</CardTitle>
              <CardDescription>
                {t('assignedCount', { count: counts.get(policy.id) ?? 0 })}
              </CardDescription>
              <CardAction className='flex gap-1'>
                <Button
                  size='icon-sm'
                  variant='ghost'
                  aria-label={t('edit')}
                  onClick={() => setAction({ type: 'edit', policy })}
                >
                  <Pencil />
                </Button>
                <Button
                  size='icon-sm'
                  variant='ghost'
                  aria-label={t('delete')}
                  onClick={() => setAction({ type: 'delete', policy })}
                >
                  <Trash2 />
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent className='flex flex-wrap items-center justify-between gap-3'>
              <p className='text-sm text-muted-foreground'>
                {policy.settings.threads_auto
                  ? t('threadAuto')
                  : t('threadCount', {
                      count: Number(policy.settings.threads),
                    })}
                {' · '}
                {t('poolSize', {
                  count: Number(policy.settings.upstream_keepalive_pool_size),
                })}
              </p>
              <Button
                variant='outline'
                size='sm'
                onClick={() => setAction({ type: 'assign', policy })}
              >
                <Server data-icon='inline-start' />
                {t('assign')}
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      {action?.type === 'edit' && query.data && (
        <PolicyEditor
          policy={action.policy}
          parameters={query.data.parameters}
          onClose={() => setAction(undefined)}
          onSaved={complete}
        />
      )}
      {action?.type === 'assign' && (
        <NodeAssignmentDialog
          policy={action.policy}
          policies={policies}
          nodes={nodes}
          onClose={() => setAction(undefined)}
          onSaved={complete}
        />
      )}
      {action?.type === 'delete' && (
        <AlertDialog
          open
          onOpenChange={(open) => !open && !deleting && setAction(undefined)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t('deleteTitle', { name: action.policy.name })}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t('deleteDescription')}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={deleting}>
                {t('cancel')}
              </AlertDialogCancel>
              <Button
                variant='destructive'
                disabled={deleting}
                onClick={() => void remove(action.policy)}
              >
                {t('delete')}
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      )}
    </div>
  );
}
