import { useQuery } from '@tanstack/react-query';
import { useFormatter, useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { AttackLog } from '@/lib/services/openflare/attack-types';
import { WafService } from '@/lib/services/openflare/waf.service';

export function useAttackReason() {
  const t = useTranslations('attackLogs');
  const protection = useTranslations('attackProtection');
  return (log: AttackLog) => {
    const category = log.rules?.[0]?.category;
    if (category && protection.has(`categories.${category}`))
      return protection(`categories.${category}`);
    return t.has(`reasons.${log.reason}`)
      ? t(`reasons.${log.reason}`)
      : t('reasons.block');
  };
}

export function AttackLogDetails({
  log,
  nodeName,
  onClose,
}: {
  log: AttackLog;
  nodeName?: string;
  onClose: () => void;
}) {
  const t = useTranslations('attackLogs');
  const protection = useTranslations('attackProtection');
  const format = useFormatter();
  const reason = useAttackReason();
  const request = useQuery({
    queryKey: [
      'openflare',
      'attack-log-request',
      log.node_id,
      log.logged_at,
      log.id,
    ],
    queryFn: () => WafService.getAttackLogRequest(log),
    gcTime: 0,
    staleTime: Infinity,
    retry: false,
  });
  const preview = request.data?.request;
  const details = [
    { label: t('reason'), value: reason(log) },
    {
      label: t('sourceIP'),
      value: [log.remote_addr, log.region].filter(Boolean).join(' · '),
    },
    {
      label: t('edgeNode'),
      value: nodeName ? `${nodeName} (${log.node_id})` : log.node_id,
    },
    { label: t('flow'), value: log.flow_name || String(log.flow_id) },
    { label: t('flowNode'), value: log.flow_node_id },
    { label: t('group'), value: log.attack_group_name || t('emptyValue') },
    { label: t('status'), value: log.status_code || t('emptyValue') },
    {
      label: t('time'),
      value: format.dateTime(new Date(log.logged_at), {
        dateStyle: 'medium',
        timeStyle: 'medium',
      }),
    },
    { label: t('recordID'), value: log.id },
  ];
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('details')}</DialogTitle>
          <DialogDescription className='break-all font-mono'>
            {log.method} {log.https ? 'https' : 'http'}://{log.host}
            {log.path}
          </DialogDescription>
        </DialogHeader>
        <Badge variant={log.action === 'blocked' ? 'destructive' : 'secondary'}>
          {t(log.action)}
        </Badge>
        <dl className='grid grid-cols-[minmax(7rem,1fr)_minmax(0,2fr)] gap-x-4 gap-y-3 rounded-md border border-dashed p-4 text-sm'>
          {details.map(({ label, value }) => (
            <div key={label} className='contents'>
              <dt className='text-muted-foreground'>{label}</dt>
              <dd className='break-words'>{value}</dd>
            </div>
          ))}
        </dl>
        {!!log.rules?.length && (
          <section className='space-y-3'>
            <h2 className='text-sm font-semibold'>{t('matchedRules')}</h2>
            <ul className='grid gap-2'>
              {log.rules.map((rule) => (
                <li
                  key={rule.id}
                  className='space-y-1 rounded-md border border-dashed p-3 text-sm'
                >
                  <div className='flex flex-wrap items-center gap-2'>
                    <code className='font-medium'>{rule.id}</code>
                    <span>
                      {protection.has(`categories.${rule.category}`)
                        ? protection(`categories.${rule.category}`)
                        : rule.category}
                    </span>
                    {protection.has(`levels.${rule.level}`) && (
                      <Badge variant='outline'>
                        {protection(`levels.${rule.level}`)}
                      </Badge>
                    )}
                  </div>
                  <p className='text-muted-foreground'>
                    {t.has(`rules.${rule.id}`)
                      ? t(`rules.${rule.id}`)
                      : rule.message}
                  </p>
                </li>
              ))}
            </ul>
          </section>
        )}
        <section
          aria-labelledby='attack-log-request'
          className='min-w-0 space-y-3'
        >
          <h2 id='attack-log-request' className='text-sm font-semibold'>
            {t('requestMessage.title')}
          </h2>
          {request.isPending ? (
            <p role='status' className='text-sm text-muted-foreground'>
              {t('requestMessage.loading')}
            </p>
          ) : request.isError ? (
            <div className='flex items-center gap-3 text-sm'>
              <p role='alert'>{t('requestMessage.error')}</p>
              <Button
                variant='outline'
                size='sm'
                onClick={() => void request.refetch()}
              >
                {t('requestMessage.retry')}
              </Button>
            </div>
          ) : preview ? (
            <>
              <pre className='max-h-80 overflow-auto whitespace-pre-wrap break-all rounded-md border border-dashed bg-muted/40 p-3 text-xs leading-relaxed'>
                <code>{preview.message}</code>
              </pre>
              {(preview.truncated || preview.body_omitted) && (
                <p className='text-xs text-muted-foreground'>
                  {preview.truncated && t('requestMessage.truncated')}{' '}
                  {preview.body_omitted && t('requestMessage.bodyOmitted')}
                </p>
              )}
            </>
          ) : (
            <p className='text-sm text-muted-foreground'>
              {t('requestMessage.empty')}
            </p>
          )}
        </section>
      </DialogContent>
    </Dialog>
  );
}
