'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Fragment, useState } from 'react';
import {
  KeyRound,
  ListTree,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import { toast } from 'sonner';

import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import type { DnsAccountItem } from '@/lib/services/openflare';
import {
  cloudflareQueryKey,
  dnsAccountsQueryKey,
  DnsAccountService,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { useTranslations } from 'next-intl';

import { DnsAccountCreateDialog } from '../websites/components/dns-account-create-dialog';
import { getErrorMessage } from '../websites/components/website-utils';

export default function DnsAccountsPage() {
  const t = useTranslations('dnsAccounts');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<DnsAccountItem>();
  const [expandedId, setExpandedId] = useState<number>();
  const [deleteTarget, setDeleteTarget] = useState<DnsAccountItem | null>(null);

  const dnsAccountsQuery = useQuery({
    queryKey: dnsAccountsQueryKey,
    queryFn: () => DnsAccountService.list(),
  });

  const refreshMutation = useMutation({
    mutationFn: (id: number) => DnsAccountService.refresh(id),
    onSuccess: async (account) => {
      setExpandedId(account.id);
      if (account.verification_status === 'ready')
        toast.success(t('refreshSuccess'));
      else toast.error(t('refreshFailed'));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: dnsAccountsQueryKey }),
        queryClient.invalidateQueries({ queryKey: cloudflareQueryKey }),
      ]);
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
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
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            {t('addAccount')}
          </Button>
        </div>
      </div>
      <p className='text-sm text-muted-foreground'>{t('listDesc')}</p>

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
                <TableHead>{t('verificationColumn')}</TableHead>
                <TableHead>{t('scopesColumn')}</TableHead>
                <TableHead className='pr-4 text-center'>
                  {t('actionsColumn')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {accounts.map((account) => (
                <Fragment key={account.id}>
                  <TableRow>
                    <TableCell className='py-3 pl-4 font-medium'>
                      {account.name}
                    </TableCell>
                    <TableCell>{t(`providers.${account.type}`)}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          account.verification_status === 'ready'
                            ? 'default'
                            : account.verification_status === 'error'
                              ? 'destructive'
                              : 'secondary'
                        }
                      >
                        {t(
                          account.verification_status === 'ready'
                            ? 'apiReady'
                            : account.verification_status === 'error'
                              ? 'apiError'
                              : 'apiPending',
                        )}
                      </Badge>
                      {account.last_checked_at && (
                        <div className='mt-1 text-xs text-muted-foreground'>
                          {formatDateTime(account.last_checked_at)}
                        </div>
                      )}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant='ghost'
                        size='sm'
                        aria-expanded={expandedId === account.id}
                        onClick={() =>
                          setExpandedId(
                            expandedId === account.id ? undefined : account.id,
                          )
                        }
                      >
                        {t('scopeCount', {
                          count: account.scopes?.length ?? 0,
                        })}
                      </Button>
                    </TableCell>
                    <TableCell className='pr-4 text-center'>
                      <ListActions>
                        <ListActionButton
                          icon={RefreshCw}
                          label={t('refreshScope')}
                          loading={refreshMutation.isPending}
                          onClick={() => refreshMutation.mutate(account.id)}
                        />
                        <ListActionButton
                          icon={ListTree}
                          label={t('scopesColumn')}
                          onClick={() =>
                            setExpandedId(
                              expandedId === account.id
                                ? undefined
                                : account.id,
                            )
                          }
                        />
                        <ListActionButton
                          icon={Pencil}
                          label={t('editTitle')}
                          onClick={() => {
                            setEditing(account);
                            setCreateOpen(true);
                          }}
                        />
                        <ListActionButton
                          icon={Trash2}
                          label={tc('delete')}
                          tone='destructive'
                          onClick={() => setDeleteTarget(account)}
                        />
                      </ListActions>
                    </TableCell>
                  </TableRow>
                  {expandedId === account.id && (
                    <TableRow>
                      <TableCell
                        colSpan={5}
                        className='whitespace-normal px-4 py-3'
                      >
                        {account.scopes?.length ? (
                          <ul className='flex flex-wrap gap-2'>
                            {account.scopes.map((scope) => (
                              <li key={scope.id}>
                                <Badge variant='outline'>{scope.name}</Badge>
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <p className='text-sm text-muted-foreground'>
                            {t('emptyScopes')}
                          </p>
                        )}
                      </TableCell>
                    </TableRow>
                  )}
                </Fragment>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <DnsAccountCreateDialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open);
          if (!open) setEditing(undefined);
        }}
        account={editing}
        onCreated={(account) => {
          toast.success(t(editing ? 'saved' : 'created'));
          if (account.verification_status === 'error')
            toast.error(t('refreshFailed'));
        }}
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
              disabled={deleteMutation.isPending}
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
