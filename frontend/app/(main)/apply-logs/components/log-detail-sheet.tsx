'use client';

import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import type { ApplyLogItem } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

interface LogDetailSheetProps {
  log: ApplyLogItem | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function ResultBadge({ result }: { result: string }) {
  const t = useTranslations('applyLogs');
  if (result === 'success') {
    return (
      <Badge
        variant='outline'
        className='text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2'
      >
        <span className='size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0' />
        {t('success')}
      </Badge>
    );
  }
  if (result === 'warning') {
    return (
      <Badge
        variant='outline'
        className='text-[10px] bg-amber-500/10 border-amber-500/20 text-amber-600 rounded-full py-0 px-2'
      >
        <span className='size-1 bg-amber-500 rounded-full mr-1.5 shrink-0' />
        {t('warning')}
      </Badge>
    );
  }
  return (
    <Badge
      variant='outline'
      className='text-[10px] bg-destructive/10 border-destructive/20 text-destructive rounded-full py-0 px-2'
    >
      <span className='size-1 bg-destructive rounded-full mr-1.5 shrink-0' />
      {t('failed')}
    </Badge>
  );
}

export function LogDetailSheet({
  log,
  open,
  onOpenChange,
}: LogDetailSheetProps) {
  const t = useTranslations('applyLogs');
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='sm:max-w-md w-full p-0 flex flex-col gap-0'>
        <SheetHeader className='px-5 py-4 border-b'>
          <SheetTitle>{t('sheetTitle')}</SheetTitle>
          <SheetDescription>{t('sheetDesc')}</SheetDescription>
        </SheetHeader>

        {log ? (
          <div className='flex-1 overflow-y-auto px-5 py-4 space-y-4'>
            <div className='flex flex-wrap gap-2'>
              <ResultBadge result={log.result} />
              <Badge variant='outline' className='text-[10px] rounded-full'>
                Node: {log.node_id}
              </Badge>
              <Badge variant='outline' className='text-[10px] rounded-full'>
                {t('versionBadge', { version: log.version })}
              </Badge>
            </div>

            <div className='grid gap-3 text-xs'>
              <div>
                <p className='text-muted-foreground'>{t('createdAt')}</p>
                <p className='font-medium'>{formatDateTime(log.created_at)}</p>
              </div>
              <div>
                <p className='text-muted-foreground'>{t('targetChecksum')}</p>
                <p className='font-mono break-all'>{log.checksum || '—'}</p>
              </div>
              <div>
                <p className='text-muted-foreground'>{t('mainChecksum')}</p>
                <p className='font-mono break-all'>
                  {log.main_config_checksum || '—'}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground'>{t('routeChecksum')}</p>
                <p className='font-mono break-all'>
                  {log.route_config_checksum || '—'}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground'>{t('supportFiles')}</p>
                <p className='font-medium'>{log.support_file_count}</p>
              </div>
            </div>

            <div className='rounded-lg border border-dashed p-3'>
              <p className='text-[10px] uppercase tracking-wider text-muted-foreground mb-2'>
                {t('message')}
              </p>
              <pre className='text-xs whitespace-pre-wrap break-words'>
                {log.message || '—'}
              </pre>
            </div>
          </div>
        ) : null}

        <SheetFooter className='px-5 py-4 border-t'>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('close')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
