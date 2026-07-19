import { TriangleAlert } from 'lucide-react';

import { ErrorInline } from '@/components/layout/error';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
  type PagesGitHubReleaseSource,
  type PagesRemoteURLSource,
  type PagesSourceRevision,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

function revisionSummary(revision?: PagesSourceRevision) {
  if (!revision) return '尚无记录';
  return `${revision.label} · ${revision.revision.slice(0, 12)}`;
}

export function RemoteSourceDetails({
  source,
}: {
  source: PagesRemoteURLSource;
}) {
  return (
    <div className='grid gap-4 md:grid-cols-2'>
      <div className='flex min-w-0 flex-col gap-1 rounded-lg border p-4 md:col-span-2'>
        <span className='text-xs text-muted-foreground'>脱敏地址</span>
        <code className='truncate text-sm'>{source.display_url}</code>
      </div>
      <div className='flex flex-col gap-1 rounded-lg border p-4'>
        <span className='text-xs text-muted-foreground'>网络策略</span>
        <span className='text-sm font-medium'>
          {source.remote_network_policy === 'trusted_internal'
            ? '受信内网模式'
            : '公网安全模式'}
        </span>
      </div>
      <div className='flex flex-col gap-1 rounded-lg border p-4'>
        <span className='text-xs text-muted-foreground'>最近同步</span>
        <span className='text-sm font-medium'>
          {source.last_synced_at
            ? formatDateTime(source.last_synced_at)
            : '尚未同步'}
        </span>
      </div>
      <div className='flex flex-col gap-1 rounded-lg border p-4 md:col-span-2'>
        <span className='text-xs text-muted-foreground'>已应用 revision</span>
        <span className='font-mono text-sm'>
          {revisionSummary(source.last_applied)}
        </span>
      </div>
      {source.last_error ? (
        <div className='md:col-span-2'>
          <ErrorInline message={source.last_error} />
        </div>
      ) : null}
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

  return (
    <div className='flex flex-col gap-4'>
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

      <div className='grid gap-4 md:grid-cols-2'>
        <div className='flex min-w-0 flex-col gap-1 rounded-lg border p-4 md:col-span-2'>
          <span className='text-xs text-muted-foreground'>GitHub 仓库</span>
          <code className='truncate text-sm'>{source.github_repository}</code>
        </div>
        <div className='flex flex-col gap-1 rounded-lg border p-4'>
          <span className='text-xs text-muted-foreground'>Release 选择</span>
          <span className='text-sm font-medium'>
            {source.release_selector === 'latest'
              ? '最新 Release'
              : `固定 Tag · ${source.release_tag ?? '未提供'}`}
          </span>
        </div>
        <div className='flex min-w-0 flex-col gap-1 rounded-lg border p-4'>
          <span className='text-xs text-muted-foreground'>Release Asset</span>
          <code className='truncate text-sm'>{source.asset_name}</code>
        </div>
        <div className='flex flex-col gap-1 rounded-lg border p-4'>
          <span className='text-xs text-muted-foreground'>远端已发现</span>
          <span className='font-mono text-sm'>
            {revisionSummary(source.last_seen)}
          </span>
        </div>
        <div className='flex flex-col gap-1 rounded-lg border p-4'>
          <span className='text-xs text-muted-foreground'>当前已应用</span>
          <span className='font-mono text-sm'>
            {revisionSummary(source.last_applied)}
          </span>
        </div>
        <div className='flex flex-col gap-1 rounded-lg border p-4'>
          <span className='text-xs text-muted-foreground'>最近检查</span>
          <span className='text-sm font-medium'>
            {source.last_checked_at
              ? formatDateTime(source.last_checked_at)
              : '尚未检查'}
          </span>
        </div>
        <div className='flex flex-col gap-1 rounded-lg border p-4'>
          <span className='text-xs text-muted-foreground'>最近同步</span>
          <span className='text-sm font-medium'>
            {source.last_synced_at
              ? formatDateTime(source.last_synced_at)
              : '尚未同步'}
          </span>
        </div>
        {source.last_error ? (
          <div className='md:col-span-2'>
            <ErrorInline message={source.last_error} />
          </div>
        ) : null}
      </div>
    </div>
  );
}
