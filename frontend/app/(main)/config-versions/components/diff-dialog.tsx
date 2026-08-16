'use client';

import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { ConfigDiffResult } from '@/lib/services/openflare';

interface DiffDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  diff: ConfigDiffResult | null;
  loading: boolean;
  error: string | null;
}

function DiffChipList({ title, items }: { title: string; items: string[] }) {
  const t = useTranslations('configVersions');
  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <p className='text-xs font-semibold'>{title}</p>
        <Badge variant='outline' className='text-[10px] rounded-full'>
          {t('itemCount', { count: items.length })}
        </Badge>
      </div>
      {items.length > 0 ? (
        <div className='flex flex-wrap gap-1.5'>
          {items.map((item) => (
            <Badge
              key={item}
              variant='secondary'
              className='text-[10px] font-normal'
            >
              {item}
            </Badge>
          ))}
        </div>
      ) : (
        <p className='text-xs text-muted-foreground'>{t('noChange')}</p>
      )}
    </div>
  );
}

export function DiffDialog({
  open,
  onOpenChange,
  diff,
  loading,
  error,
}: DiffDialogProps) {
  const t = useTranslations('configVersions');
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-3xl max-h-[85vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{t('diffTitle')}</DialogTitle>
          <DialogDescription>{t('diffDesc')}</DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className='flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground'>
            <Loader2 className='size-4 animate-spin' />
            {t('loadingDiff')}
          </div>
        ) : error ? (
          <p className='text-sm text-destructive'>{error}</p>
        ) : diff ? (
          <div className='space-y-5'>
            <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
              <div className='rounded-lg border border-dashed p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('activeVersion')}
                </p>
                <p className='mt-1 text-sm font-medium'>
                  {diff.active_version || t('none')}
                </p>
              </div>
              <div className='rounded-lg border border-dashed p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('addedDomains')}
                </p>
                <p className='mt-1 text-lg font-semibold text-emerald-600'>
                  {diff.added_domains.length}
                </p>
              </div>
              <div className='rounded-lg border border-dashed p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('removedDomains')}
                </p>
                <p className='mt-1 text-lg font-semibold text-amber-600'>
                  {diff.removed_domains.length}
                </p>
              </div>
              <div className='rounded-lg border border-dashed p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('modifiedDomains')}
                </p>
                <p className='mt-1 text-lg font-semibold text-blue-600'>
                  {diff.modified_domains.length}
                </p>
              </div>
            </div>

            <div className='grid gap-4 md:grid-cols-3'>
              <DiffChipList
                title={t('addedDomains')}
                items={diff.added_domains}
              />
              <DiffChipList
                title={t('removedDomains')}
                items={diff.removed_domains}
              />
              <DiffChipList
                title={t('modifiedDomains')}
                items={diff.modified_domains}
              />
            </div>

            <div className='flex flex-wrap gap-2'>
              <Badge
                variant={diff.main_config_changed ? 'destructive' : 'outline'}
              >
                {diff.main_config_changed
                  ? t('mainChanged')
                  : t('mainUnchanged')}
              </Badge>
              <Badge
                variant={diff.waf_config_changed ? 'destructive' : 'outline'}
              >
                {diff.waf_config_changed ? t('wafChanged') : t('wafUnchanged')}
              </Badge>
              <Badge variant='outline'>
                {t('websiteDelta', {
                  from: diff.active_website_count,
                  to: diff.current_website_count,
                })}
              </Badge>
            </div>

            {diff.changed_option_details.length > 0 ? (
              <div className='border border-dashed rounded-lg overflow-hidden'>
                <Table>
                  <TableHeader className='bg-muted/40'>
                    <TableRow className='border-dashed hover:bg-transparent'>
                      <TableHead className='text-xs font-semibold'>
                        {t('optionKey')}
                      </TableHead>
                      <TableHead className='text-xs font-semibold'>
                        {t('activeValue')}
                      </TableHead>
                      <TableHead className='text-xs font-semibold'>
                        {t('pendingValue')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {diff.changed_option_details.map((item) => (
                      <TableRow key={item.key} className='border-dashed'>
                        <TableCell className='text-xs font-medium'>
                          {item.key}
                        </TableCell>
                        <TableCell className='text-xs font-mono text-muted-foreground'>
                          {item.previous_value === ''
                            ? t('emptyValue')
                            : item.previous_value}
                        </TableCell>
                        <TableCell className='text-xs font-mono text-muted-foreground'>
                          {item.current_value === ''
                            ? t('emptyValue')
                            : item.current_value}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            ) : (
              <p className='text-xs text-muted-foreground'>
                {t('noOptionChanges')}
              </p>
            )}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
