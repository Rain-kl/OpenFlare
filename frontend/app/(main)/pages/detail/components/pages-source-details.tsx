import { TriangleAlert } from 'lucide-react';
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

function revisionSummary(revision?: PagesSourceRevision) {
  if (!revision) return '尚无记录';
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
  return (
    <div className='space-y-4'>
      <div className='rounded-lg border border-dashed bg-muted/15 px-5 py-5'>
        <dl className='space-y-3.5'>
          <SourceMetaRow label='地址' mono>
            {source.remote_url || '—'}
          </SourceMetaRow>
          <div className='grid gap-3.5 sm:grid-cols-2'>
            <SourceMetaRow label='TLS'>
              {source.allow_insecure ? '允许不安全连接' : '校验证书'}
            </SourceMetaRow>
            <SourceMetaRow label='最近同步'>
              {formatOptionalTime(source.last_synced_at, '尚未同步')}
            </SourceMetaRow>
          </div>
          <SourceMetaRow label='已应用' mono>
            {revisionSummary(source.last_applied)}
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
  const attentionRevision =
    source.sync_status === 'attention' ? source.last_seen : undefined;
  const releaseLabel =
    source.release_selector === 'latest'
      ? '最新 Release'
      : `固定 Tag · ${source.release_tag || '未提供'}`;

  return (
    <div className='space-y-4'>
      {attentionRevision ? (
        <Alert variant='destructive'>
          <TriangleAlert />
          <AlertTitle>Release Asset 发生变化，需要显式确认</AlertTitle>
          <AlertDescription>
            <p>
              当前远端 revision
              与已发布内容不一致。请核对版本和资源后，再确认发布这一精确
              revision。
            </p>
            <code className='break-all'>{attentionRevision.revision}</code>
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='rounded-lg border border-dashed bg-muted/15 px-5 py-5'>
        <div className='mb-4 flex flex-wrap items-center gap-2 border-b border-dashed pb-4'>
          <code className='text-sm font-medium'>{source.github_repository}</code>
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
              <SourceMetaRow label='自动更新'>
                {source.auto_update_enabled ? '已开启' : '已关闭'}
              </SourceMetaRow>
              <SourceMetaRow label='检查间隔'>
                {source.check_interval_minutes} 分钟
              </SourceMetaRow>
              <SourceMetaRow label='下次检查'>
                {formatOptionalTime(source.next_check_at, '等待调度')}
              </SourceMetaRow>
              <SourceMetaRow label='最近检查'>
                {formatOptionalTime(source.last_checked_at, '尚未检查')}
              </SourceMetaRow>
            </>
          ) : (
            <SourceMetaRow label='最近检查'>
              {formatOptionalTime(source.last_checked_at, '尚未检查')}
            </SourceMetaRow>
          )}
          <SourceMetaRow label='远端' mono>
            {revisionSummary(source.last_seen)}
          </SourceMetaRow>
          <SourceMetaRow label='已应用' mono>
            {revisionSummary(source.last_applied)}
          </SourceMetaRow>
          <SourceMetaRow label='最近同步'>
            {formatOptionalTime(source.last_synced_at, '尚未同步')}
          </SourceMetaRow>
        </dl>
      </div>

      {source.last_error ? <ErrorInline message={source.last_error} /> : null}
    </div>
  );
}
