'use client';

import { useTranslations } from 'next-intl';

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import type { WAFIPGroupAutoTestResult } from '@/lib/services/openflare';

interface IPGroupTestDialogProps {
  open: boolean;
  loading: boolean;
  result: WAFIPGroupAutoTestResult | null;
  onOpenChange: (open: boolean) => void;
}

export function IPGroupTestDialog({
  open,
  loading,
  result,
  onOpenChange,
}: IPGroupTestDialogProps) {
  const t = useTranslations('ipGroups.testDialog');
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('title')}</DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>

        {loading ? (
          <p className='text-sm text-muted-foreground py-6 text-center'>
            {t('testing')}
          </p>
        ) : result ? (
          <div className='space-y-4'>
            <div className='rounded-lg border border-dashed p-4 text-sm'>
              <p>
                {t('summary', {
                  lookback: result.lookback,
                  rules: result.rule_count,
                  matched: result.matched_count,
                })}
              </p>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('testedAt', {
                  time: new Date(result.tested_at).toLocaleString(),
                })}
              </p>
            </div>
            {result.matched_count > 0 ? (
              <pre className='max-h-64 overflow-auto rounded-lg border bg-muted/40 p-3 text-xs whitespace-pre-wrap break-all'>
                {result.matched_ips.join('\n')}
              </pre>
            ) : (
              <p className='text-sm text-muted-foreground'>{t('noMatch')}</p>
            )}
          </div>
        ) : (
          <p className='text-sm text-muted-foreground py-6 text-center'>
            {t('empty')}
          </p>
        )}

        <DialogFooter>
          <Button type='button' onClick={() => onOpenChange(false)}>
            {t('close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
