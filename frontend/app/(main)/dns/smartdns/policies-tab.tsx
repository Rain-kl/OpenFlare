'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { GitBranch, Plus } from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
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
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSPolicy,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';
import { SmartDNSPolicyList } from './policy-list';
export function SmartDNSPoliciesTab({
  onSimulatePolicy,
}: {
  onSimulatePolicy: (policyId: number) => void;
}) {
  const t = useTranslations('smartdns.policies');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [defaultTarget, setDefaultTarget] = useState<SmartDNSPolicy | null>(
    null,
  );
  const [deleteTarget, setDeleteTarget] = useState<SmartDNSPolicy | null>(null);

  const policiesQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'policies'],
    queryFn: () => SmartDNSService.listPolicies(),
  });

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
  };

  const setDefaultMutation = useMutation({
    mutationFn: (id: number) => SmartDNSService.setDefaultPolicy(id),
    onSuccess: async () => {
      setDefaultTarget(null);
      toast.success(t('defaultUpdated'));
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => SmartDNSService.deletePolicy(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <h2 className='text-lg font-semibold'>{t('listTitle')}</h2>
          <p className='text-sm text-muted-foreground'>{t('listDesc')}</p>
        </div>
        <Button size='sm' asChild>
          <Link href='/dns/smartdns/policies/editor'>
            <Plus data-icon='inline-start' />
            {t('create')}
          </Link>
        </Button>
      </div>

      {policiesQuery.isPending ? (
        <LoadingStateWithBorder icon={GitBranch} description={t('loading')} />
      ) : policiesQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(policiesQuery.error)}
          onRetry={() => void policiesQuery.refetch()}
        />
      ) : (
        <SmartDNSPolicyList
          policies={policiesQuery.data ?? []}
          onSimulate={onSimulatePolicy}
          onSetDefault={setDefaultTarget}
          onDelete={setDeleteTarget}
        />
      )}

      <AlertDialog
        open={defaultTarget !== null}
        onOpenChange={(open) =>
          !open && !setDefaultMutation.isPending && setDefaultTarget(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('setDefault')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('setDefaultDescription', { name: defaultTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={setDefaultMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button
              disabled={setDefaultMutation.isPending}
              onClick={() =>
                defaultTarget && setDefaultMutation.mutate(defaultTarget.id)
              }
            >
              {t('setDefault')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) =>
          !open && !deleteMutation.isPending && setDeleteTarget(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('confirmDelete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
