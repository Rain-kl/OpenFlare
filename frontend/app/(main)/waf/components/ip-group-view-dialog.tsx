'use client';

import { Loader2, Search, Trash2, X } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useState } from 'react';

import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { WAFIPGroup } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { getIPGroupViewEntries, type IPGroupViewEntry } from './helpers';

interface IPGroupViewDialogProps {
  open: boolean;
  group: WAFIPGroup | null;
  loading: boolean;
  removingIp: string | null;
  onOpenChange: (open: boolean) => void;
  onRemoveIp: (ip: string) => Promise<void>;
}

export function IPGroupViewDialog({
  open,
  group,
  loading,
  removingIp,
  onOpenChange,
  onRemoveIp,
}: IPGroupViewDialogProps) {
  const t = useTranslations('ipGroups');
  const tCommon = useTranslations('common');
  const [deleteTarget, setDeleteTarget] = useState<IPGroupViewEntry | null>(
    null,
  );
  const [searchKeyword, setSearchKeyword] = useState('');

  useEffect(() => {
    if (!open) {
      setSearchKeyword('');
      setDeleteTarget(null);
    }
  }, [open]);

  const entries = useMemo(
    () =>
      group
        ? getIPGroupViewEntries(group, (key, values) => t(key, values))
        : [],
    [group, t],
  );

  const trimmedKeyword = searchKeyword.trim().toLowerCase();

  const filteredEntries = useMemo(() => {
    if (!trimmedKeyword) return entries;
    return entries.filter((e) => e.ip.toLowerCase().includes(trimmedKeyword));
  }, [entries, trimmedKeyword]);

  const showAutomaticMeta = group?.type === 'automatic';

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          onOpenChange(nextOpen);
          if (!nextOpen) {
            setDeleteTarget(null);
            setSearchKeyword('');
          }
        }}
      >
        <DialogContent className='max-w-3xl max-h-[90vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {group
                ? t('viewDialog.titleNamed', { name: group.name })
                : t('viewDialog.title')}
            </DialogTitle>
            <DialogDescription>
              {group
                ? trimmedKeyword
                  ? t('viewDialog.summaryFiltered', {
                      type: t(`types.${group.type}`),
                      count: entries.length,
                      matched: filteredEntries.length,
                    })
                  : t('viewDialog.summary', {
                      type: t(`types.${group.type}`),
                      count: entries.length,
                    })
                : t('viewDialog.fallbackDesc')}
            </DialogDescription>
          </DialogHeader>

          {loading ? (
            <div className='flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground'>
              <Loader2 className='size-4 animate-spin' />
              {t('viewDialog.loading')}
            </div>
          ) : !group ? (
            <p className='py-8 text-center text-sm text-muted-foreground'>
              {t('viewDialog.noGroup')}
            </p>
          ) : entries.length === 0 ? (
            <EmptyStateWithBorder
              description={
                group.type === 'automatic'
                  ? t('viewDialog.emptyAuto')
                  : t('viewDialog.empty')
              }
            />
          ) : (
            <div className='space-y-3'>
              {group.type === 'subscription' ? (
                <p className='text-xs text-muted-foreground'>
                  {t('viewDialog.subscriptionHint')}
                </p>
              ) : null}
              <div className='relative'>
                <Search className='pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
                <Input
                  value={searchKeyword}
                  onChange={(e) => setSearchKeyword(e.target.value)}
                  placeholder={t('viewDialog.searchPlaceholder')}
                  className='pl-8 pr-8'
                />
                {searchKeyword ? (
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='absolute right-1 top-1/2 size-7 -translate-y-1/2 text-muted-foreground hover:text-foreground'
                    onClick={() => setSearchKeyword('')}
                  >
                    <X className='size-3.5' />
                    <span className='sr-only'>
                      {t('viewDialog.clearSearch')}
                    </span>
                  </Button>
                ) : null}
              </div>
              {filteredEntries.length === 0 ? (
                <EmptyStateWithBorder
                  description={t('viewDialog.noSearchResult')}
                />
              ) : (
                <div className='rounded-lg border border-dashed'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('viewDialog.ipAddress')}</TableHead>
                        {showAutomaticMeta ? (
                          <>
                            <TableHead>{t('viewDialog.capturedAt')}</TableHead>
                            <TableHead>
                              {t('viewDialog.banRemaining')}
                            </TableHead>
                          </>
                        ) : null}
                        <TableHead className='w-[80px] text-center'>
                          {t('viewDialog.actions')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {filteredEntries.map((entry) => (
                        <TableRow key={entry.ip}>
                          <TableCell className='font-mono text-sm'>
                            {entry.ip}
                          </TableCell>
                          {showAutomaticMeta ? (
                            <>
                              <TableCell className='text-sm text-muted-foreground'>
                                {entry.capturedAt
                                  ? formatDateTime(entry.capturedAt)
                                  : '—'}
                              </TableCell>
                              <TableCell className='text-sm'>
                                {entry.banRemaining ?? '—'}
                              </TableCell>
                            </>
                          ) : null}
                          <TableCell className='text-center'>
                            <ListActions>
                              <ListActionButton
                                icon={
                                  removingIp === entry.ip ? Loader2 : Trash2
                                }
                                label={t('viewDialog.deleteIp', {
                                  ip: entry.ip,
                                })}
                                tone='destructive'
                                loading={removingIp === entry.ip}
                                onClick={() => setDeleteTarget(entry)}
                              />
                            </ListActions>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </div>
          )}

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
            >
              {t('viewDialog.close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(nextOpen) => !nextOpen && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('viewDialog.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('viewDialog.deleteDesc', {
                name: group?.name ?? '',
                ip: deleteTarget?.ip ?? '',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={Boolean(removingIp)}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={Boolean(removingIp)}
              onClick={async () => {
                if (!deleteTarget) return;
                await onRemoveIp(deleteTarget.ip);
                setDeleteTarget(null);
              }}
            >
              {removingIp
                ? t('viewDialog.deleting')
                : t('viewDialog.confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
