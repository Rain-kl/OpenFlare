import { TriangleAlert } from 'lucide-react';
import { useTranslations } from 'next-intl';
import type { ReactNode } from 'react';

import { ErrorInline } from '@/components/layout/error';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import {
  type PagesGitHubReleaseSource,
  type PagesRemoteURLSource,
  type PagesSourceRevision,
} from '@/lib/services/openflare';
import { cn, formatDateTime } from '@/lib/utils';

function revisionSummary(
  revision: PagesSourceRevision | undefined,
  empty: string,
) {
  if (!revision) return empty;
  const label = revision.label?.trim();
  const short = revision.revision.slice(0, 12);
  return label ? `${label} · ${short}` : short;
}

function formatOptionalTime(value?: string | null, empty = '—') {
  return value ? formatDateTime(value) : empty;
}

function SourceMetaRow({
  label,
  children,
  mono,
}: {
  label: string;
  children: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className='grid grid-cols-[4.75rem_minmax(0,1fr)] items-baseline gap-x-3 py-0.5 sm:grid-cols-[5.5rem_minmax(0,1fr)]'>
      <dt className='text-xs text-muted-foreground'>{label}</dt>
      <dd
        className={cn(
          'min-w-0 break-all text-sm leading-relaxed',
          mono && 'font-mono text-[13px]',
        )}
      >
        {children}
      </dd>
    </div>
  );
}

export function RemoteSourceDetails({
  source,
}: {
  source: PagesRemoteURLSource;
}) {
  const t = useTranslations('pages.source');
  return (
    <div className='space-y-4'>
      <div className='rounded-lg border border-dashed bg-muted/15 px-5 py-5'>
        <dl className='space-y-3.5'>
          <SourceMetaRow label={t('address')} mono>
            {source.remote_url || '—'}
          </SourceMetaRow>
          <div className='grid gap-3.5 sm:grid-cols-2'>
            <SourceMetaRow label='TLS'>
              {source.allow_insecure ? t('allowInsecure') : t('verifyCert')}
            </SourceMetaRow>
            <SourceMetaRow label={t('lastSync')}>
              {formatOptionalTime(source.last_synced_at, t('neverSynced'))}
            </SourceMetaRow>
          </div>
          <SourceMetaRow label={t('applied')} mono>
            {revisionSummary(source.last_applied, t('noRecord'))}
          </SourceMetaRow>
        </dl>
      </div>

      {source.last_error ? <ErrorInline message={source.last_error} /> : null}
    </div>
  );
}

export function GitHubSourceDetails({
  source,
}: {
  source: PagesGitHubReleaseSource;
}) {
  const t = useTranslations('pages.source');
  const attentionRevision =
    source.sync_status === 'attention' ? source.last_seen : undefined;
  const releaseLabel =
    source.release_selector === 'latest'
      ? t('latestReleaseLabel')
      : t('pinnedTagLabel', { tag: source.release_tag || t('tagMissing') });

  return (
    <div className='space-y-4'>
      {attentionRevision ? (
        <Alert variant='destructive'>
          <TriangleAlert />
          <AlertTitle>{t('assetChangedTitle')}</AlertTitle>
          <AlertDescription>
            <p>{t('assetChangedDesc')}</p>
            <code className='break-all'>{attentionRevision.revision}</code>
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='rounded-lg border border-dashed bg-muted/15 px-5 py-5'>
        <div className='mb-4 flex flex-wrap items-center gap-2 border-b border-dashed pb-4'>
          <code className='text-sm font-medium'>
            {source.github_repository}
          </code>
          <Badge variant='secondary' className='font-normal'>
            {source.asset_name}
          </Badge>
          <Badge variant='outline' className='font-normal'>
            {releaseLabel}
          </Badge>
        </div>

        <dl className='grid gap-x-8 gap-y-3.5 sm:grid-cols-2'>
          {source.release_selector === 'latest' ? (
            <>
              <SourceMetaRow label={t('autoUpdate')}>
                {source.auto_update_enabled
                  ? t('autoUpdateOn')
                  : t('autoUpdateOff')}
              </SourceMetaRow>
              <SourceMetaRow label={t('checkInterval')}>
                {t('intervalMinutes', {
                  count: source.check_interval_minutes,
                })}
              </SourceMetaRow>
              <SourceMetaRow label={t('nextCheck')}>
                {formatOptionalTime(source.next_check_at, t('waitingSchedule'))}
              </SourceMetaRow>
              <SourceMetaRow label={t('lastCheck')}>
                {formatOptionalTime(source.last_checked_at, t('neverChecked'))}
              </SourceMetaRow>
            </>
          ) : (
            <SourceMetaRow label={t('lastCheck')}>
              {formatOptionalTime(source.last_checked_at, t('neverChecked'))}
            </SourceMetaRow>
          )}
          <SourceMetaRow label={t('remote')} mono>
            {revisionSummary(source.last_seen, t('noRecord'))}
          </SourceMetaRow>
          <SourceMetaRow label={t('applied')} mono>
            {revisionSummary(source.last_applied, t('noRecord'))}
          </SourceMetaRow>
          <SourceMetaRow label={t('lastSync')}>
            {formatOptionalTime(source.last_synced_at, t('neverSynced'))}
          </SourceMetaRow>
        </dl>
      </div>

      {source.last_error ? <ErrorInline message={source.last_error} /> : null}
    </div>
  );
}
