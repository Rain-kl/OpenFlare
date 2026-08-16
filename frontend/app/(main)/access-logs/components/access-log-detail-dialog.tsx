'use client';

import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { AccessLogItem } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { formatBytes } from '@/lib/utils/metrics';

import { cacheOutcomeLabel, resolveCacheOutcome } from './access-log-utils';

function DetailField({
  label,
  value,
  mono,
  full,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
  full?: boolean;
}) {
  return (
    <div className={full ? 'sm:col-span-2 space-y-1' : 'space-y-1'}>
      <p className='text-[11px] uppercase tracking-wider text-muted-foreground'>
        {label}
      </p>
      <div
        className={`text-sm break-all ${mono ? 'font-mono text-xs' : 'text-foreground'}`}
      >
        {value}
      </div>
    </div>
  );
}

function formatRequestTimeMs(value: number | undefined | null) {
  if (value == null || !Number.isFinite(value) || value < 0) {
    return '—';
  }
  if (value < 1000) {
    return `${Math.round(value)} ms`;
  }
  return `${(value / 1000).toFixed(2)} s`;
}

export function AccessLogDetailDialog({
  open,
  item,
  onOpenChange,
}: {
  open: boolean;
  item: AccessLogItem | null;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations('accessLogs');
  // Keep last selected item while the dialog closes to avoid empty-state flash.
  const [displayItem, setDisplayItem] = useState<AccessLogItem | null>(item);
  if (item && item !== displayItem) {
    setDisplayItem(item);
  }
  const activeItem = item ?? displayItem;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] sm:max-w-2xl overflow-y-auto hide-scrollbar'>
        <DialogHeader>
          <DialogTitle>{t('detail.title')}</DialogTitle>
          <DialogDescription>{t('detail.description')}</DialogDescription>
        </DialogHeader>

        {activeItem ? (
          <div className='grid gap-4 sm:grid-cols-2'>
            <DetailField
              label={t('detail.logId')}
              value={activeItem.id || '—'}
              mono
            />
            <DetailField
              label={t('detail.requestTime')}
              value={formatDateTime(activeItem.logged_at)}
            />
            <DetailField
              label={t('detail.storedAt')}
              value={
                activeItem.created_at
                  ? formatDateTime(activeItem.created_at)
                  : '—'
              }
            />
            <DetailField
              label={t('detail.node')}
              value={activeItem.node_name || activeItem.node_id || '—'}
            />
            <DetailField
              label={t('detail.nodeId')}
              value={activeItem.node_id || '—'}
              mono
            />
            <DetailField
              label={t('detail.clientIp')}
              value={activeItem.remote_addr || '—'}
              mono
            />
            <DetailField
              label={t('detail.region')}
              value={activeItem.region || '—'}
            />
            <DetailField
              label={t('detail.host')}
              value={activeItem.host || '—'}
            />
            <DetailField
              label={t('detail.status')}
              value={
                <Badge variant='outline' className='text-[10px]'>
                  {activeItem.status_code}
                </Badge>
              }
            />
            <DetailField
              label={t('detail.cache')}
              value={
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge variant='outline' className='text-[10px]'>
                    {cacheOutcomeLabel(
                      resolveCacheOutcome(activeItem.cache_status),
                      (key) => t(key),
                    )}
                  </Badge>
                  <span className='font-mono text-xs text-muted-foreground'>
                    {activeItem.cache_status || '—'}
                  </span>
                </div>
              }
            />
            <DetailField
              label={t('detail.bytesSent')}
              value={formatBytes(activeItem.bytes_sent ?? 0)}
            />
            <DetailField
              label={t('detail.requestLength')}
              value={formatBytes(activeItem.request_length ?? 0)}
            />
            <DetailField
              label={t('detail.duration')}
              value={formatRequestTimeMs(activeItem.request_time_ms)}
            />
            <DetailField
              label={t('detail.path')}
              value={activeItem.path || '—'}
              full
              mono
            />
            <DetailField
              label='User-Agent'
              value={activeItem.user_agent || '—'}
              full
              mono
            />
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
