'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { KeyRound, Plus, Settings, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import Link from 'next/link';

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
import { Badge } from '@/components/ui/badge';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { DnsActionMenu } from '../dns/components/dns-action-menu';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import type { DnsAccountItem } from '@/lib/services/openflare';
import {
  CloudflareService,
  cloudflareQueryKey,
  DnsAccountService,
} from '@/lib/services/openflare';
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

  const connectionQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'connection'],
    queryFn: () => CloudflareService.getConnection(),
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

  const accounts = dnsAccountsQuery.data ?? [];
  const linkedID = connectionQuery.data?.dns_account_id;

  return (
    <div className='flex w-full flex-col gap-5 py-6 px-1'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <KeyRound className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Button variant='outline' size='sm' asChild>
            <Link href='/dns/settings'>
              <Settings data-icon='inline-start' />
              {t('connectionSettings')}
            </Link>
          </Button>
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            {t('addAccount')}
          </Button>
        </div>
      </div>
      <p className='text-sm text-muted-foreground'>{t('listDesc')}</p>

      {connectionQuery.isError && (
        <ErrorInline
          message={t('connectionStatusError')}
          onRetry={() => void connectionQuery.refetch()}
        />
      )}
      {dnsAccountsQuery.isPending ? (
        <LoadingStateWithBorder
          icon={KeyRound}
          description={t('loadingList')}
        />
      ) : dnsAccountsQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(dnsAccountsQuery.error, t('requestFailed'))}
          onRetry={() => void dnsAccountsQuery.refetch()}
        />
      ) : accounts.length === 0 ? (
        <EmptyStateWithBorder
          icon={KeyRound}
          description={t('emptyList')}
          actionText={t('addAccount')}
          onAction={() => setCreateOpen(true)}
        />
      ) : (
        <div className='rounded-lg border border-dashed'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='pl-4'>{t('accountName')}</TableHead>
                <TableHead>{t('provider')}</TableHead>
                <TableHead>{t('createdColumn')}</TableHead>
                <TableHead className='pr-4 text-right'>
                  {t('actionsColumn')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {accounts.map((account) => (
                <TableRow key={account.id}>
                  <TableCell className='py-3 pl-4'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <span className='font-medium'>{account.name}</span>
                      {linkedID === account.id && (
                        <Link href='/dns/settings'>
                          <Badge variant='secondary'>{t('linked')}</Badge>
                        </Link>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    {account.type === 'cloudflare'
                      ? 'Cloudflare'
                      : account.type}
                  </TableCell>
                  <TableCell className='text-muted-foreground tabular-nums'>
                    {formatDateTime(account.created_at)}
                  </TableCell>
                  <TableCell className='pr-4 text-right'>
                    <DnsActionMenu name={account.name}>
                      {linkedID === account.id ? (
                        <DropdownMenuItem asChild>
                          <Link href='/dns/settings'>
                            <Settings />
                            {t('connectionSettings')}
                          </Link>
                        </DropdownMenuItem>
                      ) : (
                        <DropdownMenuItem
                          variant='destructive'
                          disabled={!connectionQuery.isSuccess}
                          onSelect={() => setDeleteTarget(account)}
                        >
                          <Trash2 />
                          {tc('delete')}
                        </DropdownMenuItem>
                      )}
                    </DnsActionMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <DnsAccountCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => toast.success(t('created'))}
      />

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
              disabled={
                deleteMutation.isPending || linkedID === deleteTarget?.id
              }
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {tc('delete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
