'use client';

import { CheckCircle2, Clock, Copy, Loader2, XCircle } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type { ExecTaskResult } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

export function ExecResultCard({ result }: { result: ExecTaskResult }) {
  const t = useTranslations('nodeExec');

  const isRunning = result.finished_at === null && result.exit_code === null;
  const isSuccess = !isRunning && result.exit_code === 0;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(result.result || '');
      toast.success(t('result.copied'));
    } catch {
      toast.error(t('result.copyFailed'));
    }
  };

  return (
    <Card>
      <CardHeader className='flex-row items-start justify-between gap-3 space-y-0 pb-3'>
        <div className='min-w-0 space-y-0.5'>
          <CardTitle className='truncate text-sm font-semibold'>
            {result.node_name || result.client}
          </CardTitle>
          <p className='truncate font-mono text-[11px] text-muted-foreground'>
            {result.client}
          </p>
        </div>

        <div className='flex shrink-0 items-center gap-2'>
          {isRunning ? (
            <Badge variant='secondary' className='gap-1 text-xs'>
              <Loader2 className='size-3 animate-spin' />
              {t('result.running')}
            </Badge>
          ) : isSuccess ? (
            <Badge variant='outline' className='gap-1 text-xs'>
              <CheckCircle2 className='size-3 text-primary' />
              {t('result.success')} (0)
            </Badge>
          ) : (
            <Badge variant='destructive' className='gap-1 text-xs'>
              <XCircle className='size-3' />
              {t('result.failed')} ({result.exit_code ?? -1})
            </Badge>
          )}

          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='size-7'
            aria-label={t('result.copy')}
            disabled={!result.result}
            onClick={handleCopy}
          >
            <Copy className='size-3.5' />
          </Button>
        </div>
      </CardHeader>

      <CardContent className='space-y-2'>
        <pre className='max-h-72 overflow-auto rounded-lg border border-dashed bg-muted/30 p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all text-foreground'>
          {isRunning
            ? t('result.waitingOutput')
            : result.result || t('result.emptyOutput')}
        </pre>

        {result.finished_at ? (
          <div className='flex items-center gap-1.5 text-[11px] text-muted-foreground tabular-nums'>
            <Clock className='size-3' />
            <span>
              {t('result.finishedAt', {
                time: formatDateTime(result.finished_at),
              })}
            </span>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
