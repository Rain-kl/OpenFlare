'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { Cloud, Plus, Trash2 } from 'lucide-react';
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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import type { DnsAccountItem } from '@/lib/services/openflare';
import { DnsAccountService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { useTranslations } from 'next-intl';

import { DnsAccountCreateDialog } from '../websites/components/dns-account-create-dialog';
import { getErrorMessage } from '../websites/components/website-utils';

const dnsAccountsQueryKey = ['openflare', 'dns-accounts'];

export default function DnsAccountsPage() {
  const t = useTranslations('dnsAccounts');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DnsAccountItem | null>(null);

  const dnsAccountsQuery = useQuery({
    queryKey: dnsAccountsQueryKey,
    queryFn: () => DnsAccountService.list(),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => DnsAccountService.deleteById(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: dnsAccountsQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const accounts = useMemo(
    () => dnsAccountsQuery.data ?? [],
    [dnsAccountsQuery.data],
  );

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex items-center gap-2'>
          <Button
            size='sm'
            className='h-7 text-xs'
            onClick={() => setCreateOpen(true)}
          >
            <Plus className='size-3.5 mr-1' />
            {t('addAccount')}
          </Button>
        </div>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('listTitle')}
          </CardTitle>
          <CardDescription>{t('listDesc')}</CardDescription>
        </CardHeader>
        <CardContent>
          {dnsAccountsQuery.isLoading ? (
            <LoadingStateWithBorder
              icon={Cloud}
              description={t('loadingList')}
            />
          ) : dnsAccountsQuery.isError ? (
            <div className='p-8 border border-dashed rounded-lg'>
              <ErrorInline
                message={getErrorMessage(
                  dnsAccountsQuery.error,
                  t('requestFailed'),
                )}
                onRetry={() => void dnsAccountsQuery.refetch()}
                className='justify-center'
              />
            </div>
          ) : accounts.length === 0 ? (
            <EmptyStateWithBorder icon={Cloud} description={t('emptyList')} />
          ) : (
            <div className='space-y-3'>
              {accounts.map((account) => (
                <div
                  key={account.id}
                  className='flex items-start justify-between gap-3 rounded-lg border bg-card px-4 py-3'
                >
                  <div className='space-y-1'>
                    <p className='text-sm font-semibold'>
                      {account.name}{' '}
                      <span className='text-xs font-normal text-muted-foreground'>
                        ({account.type})
                      </span>
                    </p>
                    <p className='text-xs text-muted-foreground'>
                      {t('createdAt', {
                        date: formatDateTime(account.created_at),
                      })}
                    </p>
                  </div>
                  <Button
                    variant='outline'
                    size='sm'
                    className='h-7 text-xs text-destructive'
                    onClick={() => setDeleteTarget(account)}
                  >
                    <Trash2 className='size-3' />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <DnsAccountCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => toast.success(t('created'))}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {tc('delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
