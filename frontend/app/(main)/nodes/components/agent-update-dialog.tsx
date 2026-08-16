'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ExternalLink, Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { formatDateTime } from '@/lib/utils';
import type {
  NodeAgentReleaseInfo,
  NodeItem,
  ReleaseChannel,
} from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';

import { NodeStatusBadge } from './node-status-badge';
import { formatRelativeTime, getErrorMessage } from './node-utils';

export function AgentUpdateDialog({
  open,
  node,
  submitting,
  onClose,
  onConfirm,
}: {
  open: boolean;
  node: NodeItem;
  submitting: boolean;
  onClose: () => void;
  onConfirm: (
    release: NodeAgentReleaseInfo | null,
    channel: ReleaseChannel,
  ) => Promise<void>;
}) {
  const t = useTranslations('nodes');
  const [channel, setChannel] = useState<ReleaseChannel>('stable');

  const releaseQuery = useQuery({
    queryKey: ['openflare', 'node-agent-release', node.id, channel],
    queryFn: () => NodeService.getAgentRelease(node.id, channel),
    enabled: open,
  });

  const release = releaseQuery.data ?? null;

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('update.title')}</DialogTitle>
          <DialogDescription>{t('update.description')}</DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='grid gap-3 sm:grid-cols-3'>
            <div className='rounded-lg border p-3'>
              <p className='text-xs text-muted-foreground'>
                {t('update.currentVersion')}
              </p>
              <p className='mt-1 text-sm font-medium'>
                {node.version || 'unknown'}
              </p>
            </div>
            <div className='rounded-lg border p-3'>
              <p className='text-xs text-muted-foreground'>
                {t('update.channel')}
              </p>
              <p className='mt-1 text-sm font-medium'>
                {channel === 'preview'
                  ? t('channel.preview')
                  : t('channel.stable')}
              </p>
            </div>
            <div className='rounded-lg border p-3'>
              <p className='text-xs text-muted-foreground'>
                {t('update.updateStatus')}
              </p>
              <div className='mt-1'>
                <NodeStatusBadge
                  label={
                    node.update_requested
                      ? node.update_channel === 'preview'
                        ? t('update.waitingPreview')
                        : t('update.waitingUpdate')
                      : t('update.notRequested')
                  }
                  tone={node.update_requested ? 'warning' : 'info'}
                />
              </div>
            </div>
          </div>

          {releaseQuery.isFetching ? (
            <div className='flex items-center justify-center py-8 text-sm text-muted-foreground'>
              <Loader2 className='size-4 mr-2 animate-spin' />
              {t('update.checking')}
            </div>
          ) : releaseQuery.isError ? (
            <p className='text-sm text-destructive'>
              {getErrorMessage(releaseQuery.error, t('requestFailed'))}
            </p>
          ) : release ? (
            <div className='space-y-3 rounded-lg border p-4'>
              <div className='flex flex-wrap items-center gap-2'>
                <NodeStatusBadge
                  label={
                    release.has_update
                      ? t('update.hasUpdate')
                      : t('update.upToDate')
                  }
                  tone={release.has_update ? 'warning' : 'success'}
                />
                {release.prerelease ? (
                  <NodeStatusBadge
                    label={t('update.previewRelease')}
                    tone='warning'
                  />
                ) : (
                  <NodeStatusBadge
                    label={t('update.stableRelease')}
                    tone='info'
                  />
                )}
              </div>
              <div className='grid gap-3 sm:grid-cols-2 text-sm'>
                <div>
                  <p className='text-xs text-muted-foreground'>
                    {t('update.targetVersion')}
                  </p>
                  <p className='mt-1 font-medium'>
                    {release.tag_name || t('update.notFound')}
                  </p>
                </div>
                <div>
                  <p className='text-xs text-muted-foreground'>
                    {t('update.publishedAt')}
                  </p>
                  <p className='mt-1'>
                    {release.published_at
                      ? `${formatRelativeTime(release.published_at, t)} · ${formatDateTime(release.published_at)}`
                      : '—'}
                  </p>
                </div>
              </div>
              <p className='text-sm text-muted-foreground whitespace-pre-wrap'>
                {release.body || t('update.noNotes')}
              </p>
              {release.html_url ? (
                <a
                  href={release.html_url}
                  target='_blank'
                  rel='noreferrer'
                  className='inline-flex items-center text-sm text-primary hover:underline'
                >
                  {t('update.viewRelease')}
                  <ExternalLink className='size-3 ml-1' />
                </a>
              ) : null}
            </div>
          ) : null}
        </div>

        <DialogFooter className='gap-2 sm:gap-0'>
          <Button
            type='button'
            variant='outline'
            disabled={submitting || releaseQuery.isFetching}
            onClick={() => setChannel('stable')}
          >
            {t('update.checkStable')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={submitting || releaseQuery.isFetching}
            onClick={() => setChannel('preview')}
          >
            {t('update.checkPreview')}
          </Button>
          <Button
            type='button'
            disabled={
              submitting ||
              releaseQuery.isFetching ||
              !release?.has_update ||
              node.update_requested
            }
            onClick={() => void onConfirm(release, channel)}
          >
            {submitting
              ? t('dispatching')
              : channel === 'preview'
                ? t('update.upgradePreview')
                : t('update.upgradeStable')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
