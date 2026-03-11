'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  activateConfigVersion,
  getConfigVersionDiff,
  getConfigVersionPreview,
  getConfigVersions,
  publishConfigVersion,
} from '@/features/config-versions/api/config-versions';
import type {
  ConfigDiffResult,
  ConfigPreviewResult,
  ConfigVersionItem,
  SupportFile,
} from '@/features/config-versions/types';
import {
  CodeBlock,
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

const versionsQueryKey = ['config-versions'];

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function truncateChecksum(checksum: string) {
  if (!checksum) {
    return '—';
  }

  return checksum.length > 16 ? `${checksum.slice(0, 16)}...` : checksum;
}

function DiffList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <p className='text-sm font-semibold text-[var(--foreground-primary)]'>{title}</p>
        <StatusBadge label={`${items.length} 项`} variant={items.length > 0 ? 'info' : 'warning'} />
      </div>
      {items.length > 0 ? (
        <div className='flex flex-wrap gap-2'>
          {items.map((item) => (
            <span
              key={item}
              className='rounded-full border border-[var(--border-default)] bg-[var(--surface-elevated)] px-3 py-1 text-xs text-[var(--foreground-secondary)]'
            >
              {item}
            </span>
          ))}
        </div>
      ) : (
        <p className='text-sm text-[var(--foreground-secondary)]'>当前无相关变更。</p>
      )}
    </div>
  );
}

function SupportFilesList({ files }: { files: SupportFile[] }) {
  if (files.length === 0) {
    return <p className='text-sm text-[var(--foreground-secondary)]'>当前发布不需要额外支持文件。</p>;
  }

  return (
    <div className='space-y-3'>
      {files.map((file) => (
        <details
          key={file.path}
          className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-3'
        >
          <summary className='cursor-pointer text-sm font-medium text-[var(--foreground-primary)]'>
            {file.path}
          </summary>
          <CodeBlock className='mt-3 max-h-72 whitespace-pre-wrap'>{file.content}</CodeBlock>
        </details>
      ))}
    </div>
  );
}

function VersionDetailCard({
  version,
}: {
  version: ConfigVersionItem;
}) {
  return (
    <AppCard
      title={`版本 ${version.version}`}
      description='查看当前版本保存的快照 JSON 与最终渲染配置，便于比对发布结果。'
      action={
        <StatusBadge label={version.is_active ? '当前激活' : '历史版本'} variant={version.is_active ? 'success' : 'info'} />
      }
    >
      <div className='grid gap-4 md:grid-cols-3'>
        <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4'>
          <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>Checksum</p>
          <p className='mt-2 break-all text-sm text-[var(--foreground-primary)]'>{version.checksum}</p>
        </div>
        <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4'>
          <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>创建人</p>
          <p className='mt-2 text-sm text-[var(--foreground-primary)]'>{version.created_by || '系统'}</p>
        </div>
        <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4'>
          <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>创建时间</p>
          <p className='mt-2 text-sm text-[var(--foreground-primary)]'>{formatDateTime(version.created_at)}</p>
        </div>
      </div>

      <div className='mt-5 space-y-4'>
        <div>
          <p className='mb-2 text-sm font-semibold text-[var(--foreground-primary)]'>快照 JSON</p>
          <CodeBlock className='max-h-96 whitespace-pre-wrap'>{version.snapshot_json}</CodeBlock>
        </div>
        <div>
          <p className='mb-2 text-sm font-semibold text-[var(--foreground-primary)]'>渲染结果</p>
          <CodeBlock className='max-h-[32rem] whitespace-pre-wrap'>{version.rendered_config}</CodeBlock>
        </div>
      </div>
    </AppCard>
  );
}

function PublishPreviewCard({
  preview,
  diff,
  isPublishing,
  onConfirm,
  onCancel,
}: {
  preview: ConfigPreviewResult;
  diff: ConfigDiffResult;
  isPublishing: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <AppCard
      title='发布前预览'
      description='先核对增删改域名、渲染结果与支持文件，再决定是否发布为新激活版本。'
      action={<StatusBadge label={`启用规则 ${preview.route_count} 条`} variant='info' />}
    >
      <div className='space-y-5'>
        <div className='grid gap-4 md:grid-cols-4'>
          <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4'>
            <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>当前激活版本</p>
            <p className='mt-2 text-sm text-[var(--foreground-primary)]'>{diff.active_version || '无'}</p>
          </div>
          <div className='rounded-2xl border border-[var(--status-success-border)] bg-[var(--status-success-soft)] px-4 py-4'>
            <p className='text-xs uppercase tracking-[0.2em] text-[var(--status-success-foreground)]'>新增域名</p>
            <p className='mt-2 text-lg font-semibold text-[var(--status-success-foreground)]'>{diff.added_domains.length}</p>
          </div>
          <div className='rounded-2xl border border-[var(--status-warning-border)] bg-[var(--status-warning-soft)] px-4 py-4'>
            <p className='text-xs uppercase tracking-[0.2em] text-[var(--status-warning-foreground)]'>删除域名</p>
            <p className='mt-2 text-lg font-semibold text-[var(--status-warning-foreground)]'>{diff.removed_domains.length}</p>
          </div>
          <div className='rounded-2xl border border-[var(--status-info-border)] bg-[var(--status-info-soft)] px-4 py-4'>
            <p className='text-xs uppercase tracking-[0.2em] text-[var(--status-info-foreground)]'>修改域名</p>
            <p className='mt-2 text-lg font-semibold text-[var(--status-info-foreground)]'>{diff.modified_domains.length}</p>
          </div>
        </div>

        <div className='grid gap-5 xl:grid-cols-3'>
          <DiffList title='新增域名' items={diff.added_domains} />
          <DiffList title='删除域名' items={diff.removed_domains} />
          <DiffList title='修改域名' items={diff.modified_domains} />
        </div>

        <div>
          <div className='mb-2 flex flex-wrap items-center justify-between gap-3'>
            <p className='text-sm font-semibold text-[var(--foreground-primary)]'>渲染结果</p>
            <p className='text-xs text-[var(--foreground-secondary)]'>Checksum：{preview.checksum}</p>
          </div>
          <CodeBlock className='max-h-[32rem] whitespace-pre-wrap'>{preview.rendered_config}</CodeBlock>
        </div>

        <div>
          <p className='mb-2 text-sm font-semibold text-[var(--foreground-primary)]'>支持文件</p>
          <SupportFilesList files={preview.support_files} />
        </div>

        <div className='flex flex-wrap gap-3'>
          <PrimaryButton type='button' onClick={onConfirm} disabled={isPublishing}>
            {isPublishing ? '发布中...' : '确认发布'}
          </PrimaryButton>
          <SecondaryButton type='button' onClick={onCancel} disabled={isPublishing}>
            取消预览
          </SecondaryButton>
        </div>
      </div>
    </AppCard>
  );
}

export function ConfigVersionsPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [selectedVersion, setSelectedVersion] = useState<ConfigVersionItem | null>(null);
  const [publishPreview, setPublishPreview] = useState<{
    preview: ConfigPreviewResult;
    diff: ConfigDiffResult;
  } | null>(null);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);

  const versionsQuery = useQuery({
    queryKey: versionsQueryKey,
    queryFn: getConfigVersions,
  });

  const publishMutation = useMutation({
    mutationFn: publishConfigVersion,
    onSuccess: async (version) => {
      setFeedback({ tone: 'success', message: `发布成功，版本 ${version.version}` });
      setPublishPreview(null);
      setSelectedVersion(version);
      await queryClient.invalidateQueries({ queryKey: versionsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const activateMutation = useMutation({
    mutationFn: activateConfigVersion,
    onSuccess: async (version) => {
      setFeedback({ tone: 'success', message: `已激活版本 ${version.version}` });
      setSelectedVersion(version);
      await queryClient.invalidateQueries({ queryKey: versionsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const handleOpenPublishPreview = async () => {
    setFeedback(null);
    setIsPreviewLoading(true);

    try {
      const [preview, diff] = await Promise.all([getConfigVersionPreview(), getConfigVersionDiff()]);
      setPublishPreview({ preview, diff });
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    } finally {
      setIsPreviewLoading(false);
    }
  };

  const handleActivate = (version: ConfigVersionItem) => {
    if (version.is_active) {
      return;
    }

    if (!window.confirm(`确认激活版本 ${version.version} 吗？`)) {
      return;
    }

    setFeedback(null);
    activateMutation.mutate(version.id);
  };

  const versions = versionsQuery.data || [];

  return (
    <div className='space-y-6'>
      <PageHeader
        title='配置版本'
        description='查看历史快照、预览待发布配置差异，并在需要时重新激活旧版本。'
        action={
          <PrimaryButton type='button' onClick={handleOpenPublishPreview} disabled={isPreviewLoading}>
            {isPreviewLoading ? '加载预览中...' : '预览并发布'}
          </PrimaryButton>
        }
      />

      {feedback ? <InlineMessage tone={feedback.tone} message={feedback.message} /> : null}

      <div className='grid gap-6'>
        {publishPreview ? (
          <PublishPreviewCard
            preview={publishPreview.preview}
            diff={publishPreview.diff}
            isPublishing={publishMutation.isPending}
            onConfirm={() => publishMutation.mutate()}
            onCancel={() => setPublishPreview(null)}
          />
        ) : (
          <AppCard title='发布建议' description='建议先查看变更摘要，再决定是否发布为新的激活版本。'>
            <div className='space-y-3 text-sm leading-6 text-[var(--foreground-secondary)]'>
              <p>1. 使用“预览并发布”查看新增、删除和修改的域名清单。</p>
              <p>2. 确认渲染出的 Nginx 配置与支持文件内容符合预期。</p>
              <p>3. 若需要回滚，可在下方历史版本列表中重新激活旧版本。</p>
            </div>
          </AppCard>
        )}
      </div>

      <AppCard title='历史版本' description='支持查看快照详情，并在必要时将旧版本重新设为当前激活版本。'>
        {versionsQuery.isLoading ? (
          <LoadingState />
        ) : versionsQuery.isError ? (
          <ErrorState title='版本列表加载失败' description={getErrorMessage(versionsQuery.error)} />
        ) : versions.length === 0 ? (
          <EmptyState title='暂无历史版本' description='当前还没有可查看的发布记录，请先从反代规则页触发一次发布。' />
        ) : (
          <div className='overflow-x-auto'>
            <table className='min-w-full divide-y divide-[var(--border-default)] text-left text-sm'>
              <thead>
                <tr className='text-[var(--foreground-secondary)]'>
                  <th className='px-3 py-3 font-medium'>版本号</th>
                  <th className='px-3 py-3 font-medium'>状态</th>
                  <th className='px-3 py-3 font-medium'>创建人</th>
                  <th className='px-3 py-3 font-medium'>Checksum</th>
                  <th className='px-3 py-3 font-medium'>创建时间</th>
                  <th className='px-3 py-3 font-medium'>操作</th>
                </tr>
              </thead>
              <tbody className='divide-y divide-[var(--border-default)]'>
                {versions.map((version) => (
                  <tr key={version.id} className='align-top'>
                    <td className='px-3 py-4 font-medium text-[var(--foreground-primary)]'>{version.version}</td>
                    <td className='px-3 py-4'>
                      <StatusBadge
                        label={version.is_active ? '当前激活' : '历史版本'}
                        variant={version.is_active ? 'success' : 'info'}
                      />
                    </td>
                    <td className='px-3 py-4 text-[var(--foreground-secondary)]'>{version.created_by || '系统'}</td>
                    <td className='px-3 py-4 text-[var(--foreground-secondary)]' title={version.checksum}>
                      {truncateChecksum(version.checksum)}
                    </td>
                    <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                      {formatDateTime(version.created_at)}
                    </td>
                    <td className='px-3 py-4'>
                      <div className='flex flex-wrap gap-2'>
                        <SecondaryButton
                          type='button'
                          onClick={() => setSelectedVersion(version)}
                          className='px-3 py-2 text-xs'
                        >
                          查看快照
                        </SecondaryButton>
                        {!version.is_active ? (
                          <PrimaryButton
                            type='button'
                            onClick={() => handleActivate(version)}
                            disabled={activateMutation.isPending}
                            className='px-3 py-2 text-xs'
                          >
                            重新激活
                          </PrimaryButton>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </AppCard>

      {selectedVersion ? <VersionDetailCard version={selectedVersion} /> : null}
    </div>
  );
}
