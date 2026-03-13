'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

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
import { ConfigVersionSnapshotModal } from '@/features/config-versions/components/config-version-snapshot-modal';
import type {
  ConfigOptionDiffItem,
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

function hasConfigDiff(diff: ConfigDiffResult) {
  return (
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.main_config_changed ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  );
}

function DiffList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-[var(--foreground-primary)]">
          {title}
        </p>
        <StatusBadge
          label={`${items.length} 项`}
          variant={items.length > 0 ? 'info' : 'warning'}
        />
      </div>
      {items.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {items.map((item) => (
            <span
              key={item}
              className="rounded-full border border-[var(--border-default)] bg-[var(--surface-elevated)] px-3 py-1 text-xs text-[var(--foreground-secondary)]"
            >
              {item}
            </span>
          ))}
        </div>
      ) : (
        <p className="text-sm text-[var(--foreground-secondary)]">
          当前无相关变更。
        </p>
      )}
    </div>
  );
}

function SupportFilesList({ files }: { files: SupportFile[] }) {
  if (files.length === 0) {
    return (
      <p className="text-sm text-[var(--foreground-secondary)]">
        当前发布不需要额外支持文件。
      </p>
    );
  }

  return (
    <div className="space-y-3">
      {files.map((file) => (
        <details
          key={file.path}
          className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-3"
        >
          <summary className="cursor-pointer text-sm font-medium text-[var(--foreground-primary)]">
            {file.path}
          </summary>
          <CodeBlock className="mt-3 max-h-72 whitespace-pre-wrap">
            {file.content}
          </CodeBlock>
        </details>
      ))}
    </div>
  );
}

function renderOptionValue(value: string) {
  return value === '' ? '空' : value;
}

function OptionDiffTable({ items }: { items: ConfigOptionDiffItem[] }) {
  if (items.length === 0) {
    return (
      <p className="text-sm text-[var(--foreground-secondary)]">
        当前无 OpenResty 性能参数变化。
      </p>
    );
  }

  return (
    <div className="overflow-x-auto rounded-2xl border border-[var(--border-default)]">
      <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
        <thead>
          <tr className="bg-[var(--surface-elevated)] text-[var(--foreground-secondary)]">
            <th className="px-3 py-3 font-medium">参数</th>
            <th className="px-3 py-3 font-medium">当前激活值</th>
            <th className="px-3 py-3 font-medium">待发布值</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[var(--border-default)]">
          {items.map((item) => (
            <tr key={item.key} className="align-top">
              <td className="px-3 py-3 font-medium text-[var(--foreground-primary)]">
                {item.key}
              </td>
              <td className="px-3 py-3 text-[var(--foreground-secondary)]">
                <code>{renderOptionValue(item.previous_value)}</code>
              </td>
              <td className="px-3 py-3 text-[var(--foreground-secondary)]">
                <code>{renderOptionValue(item.current_value)}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PublishPreviewCard({
  preview,
  diff,
  activeVersion,
  isPublishing,
  onConfirm,
  onCancel,
}: {
  preview: ConfigPreviewResult;
  diff: ConfigDiffResult;
  activeVersion: ConfigVersionItem | null;
  isPublishing: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const canPublish = preview.route_count > 0 && hasConfigDiff(diff);

  return (
    <AppCard
      title="发布前预览"
      description="先核对增删改域名、渲染结果与支持文件，再决定是否发布为新激活版本。"
      action={
        <StatusBadge
          label={`启用规则 ${preview.route_count} 条`}
          variant="info"
        />
      }
    >
      <div className="space-y-5">
        <div className="grid gap-4 md:grid-cols-4">
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
              当前激活版本
            </p>
            <p className="mt-2 text-sm text-[var(--foreground-primary)]">
              {diff.active_version || '无'}
            </p>
          </div>
          <div className="rounded-2xl border border-[var(--status-success-border)] bg-[var(--status-success-soft)] px-4 py-4">
            <p className="text-xs tracking-[0.2em] text-[var(--status-success-foreground)] uppercase">
              新增域名
            </p>
            <p className="mt-2 text-lg font-semibold text-[var(--status-success-foreground)]">
              {diff.added_domains.length}
            </p>
          </div>
          <div className="rounded-2xl border border-[var(--status-warning-border)] bg-[var(--status-warning-soft)] px-4 py-4">
            <p className="text-xs tracking-[0.2em] text-[var(--status-warning-foreground)] uppercase">
              删除域名
            </p>
            <p className="mt-2 text-lg font-semibold text-[var(--status-warning-foreground)]">
              {diff.removed_domains.length}
            </p>
          </div>
          <div className="rounded-2xl border border-[var(--status-info-border)] bg-[var(--status-info-soft)] px-4 py-4">
            <p className="text-xs tracking-[0.2em] text-[var(--status-info-foreground)] uppercase">
              修改域名
            </p>
            <p className="mt-2 text-lg font-semibold text-[var(--status-info-foreground)]">
              {diff.modified_domains.length}
            </p>
          </div>
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
              主配置变化
            </p>
            <p className="mt-2 text-lg font-semibold text-[var(--foreground-primary)]">
              {diff.main_config_changed ? '已变化' : '无变化'}
            </p>
          </div>
        </div>

        {!canPublish ? (
          <InlineMessage
            tone="info"
            message="当前规则与已激活版本一致，已阻止重复发布。"
          />
        ) : null}

        <div className="grid gap-5 xl:grid-cols-3">
          <DiffList title="新增域名" items={diff.added_domains} />
          <DiffList title="删除域名" items={diff.removed_domains} />
          <DiffList title="修改域名" items={diff.modified_domains} />
        </div>

        <div className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <p className="text-sm font-semibold text-[var(--foreground-primary)]">
              OpenResty 参数变化
            </p>
            <StatusBadge
              label={`${diff.changed_option_keys.length} 项`}
              variant={diff.changed_option_keys.length > 0 ? 'info' : 'warning'}
            />
          </div>
          <OptionDiffTable items={diff.changed_option_details} />
        </div>

        {diff.main_config_changed && activeVersion ? (
          <div className="grid gap-5 xl:grid-cols-2">
            <div>
              <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                  Current Active Main Config
                </p>
                <StatusBadge label={activeVersion.version} variant="info" />
              </div>
              <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
                {activeVersion.main_config}
              </CodeBlock>
            </div>
            <div>
              <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                  Pending Main Config
                </p>
                <p className="text-xs text-[var(--foreground-secondary)]">
                  {`Checksum: ${preview.checksum}`}
                </p>
              </div>
              <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
                {preview.main_config}
              </CodeBlock>
            </div>
          </div>
        ) : null}

        {/* Legacy duplicated main config preview block kept commented out while
            we replace it with an explicit active-vs-pending comparison above.
        {diff.main_config_changed && activeVersion ? (
          <div className="grid gap-5 xl:grid-cols-2">
            <div>
              <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                  褰撳墠婵€娲讳富閰嶇疆
                </p>
                <StatusBadge label={activeVersion.version} variant="info" />
              </div>
              <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
                {activeVersion.main_config}
              </CodeBlock>
            </div>
            <div>
              <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                  寰呭彂甯冧富閰嶇疆
                </p>
                <p className="text-xs text-[var(--foreground-secondary)]">
                  Checksum锛歿preview.checksum}
                </p>
              </div>
              <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
                {preview.main_config}
              </CodeBlock>
            </div>
          </div>
        ) : null}
        */}

        <div>
          <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm font-semibold text-[var(--foreground-primary)]">
              主配置
            </p>
            <p className="text-xs text-[var(--foreground-secondary)]">
              Checksum：{preview.checksum}
            </p>
          </div>
          <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
            {preview.main_config}
          </CodeBlock>
        </div>

        <div>
          <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm font-semibold text-[var(--foreground-primary)]">
              路由配置
            </p>
          </div>
          <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
            {preview.rendered_config}
          </CodeBlock>
        </div>

        <div>
          <p className="mb-2 text-sm font-semibold text-[var(--foreground-primary)]">
            支持文件
          </p>
          <SupportFilesList files={preview.support_files} />
        </div>

        <div className="flex flex-wrap gap-3">
          <PrimaryButton
            type="button"
            onClick={onConfirm}
            disabled={isPublishing || !canPublish}
          >
            {isPublishing ? '发布中...' : '确认发布'}
          </PrimaryButton>
          <SecondaryButton
            type="button"
            onClick={onCancel}
            disabled={isPublishing}
          >
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
  const [selectedVersionId, setSelectedVersionId] = useState<number | null>(
    null,
  );
  const [publishPreview, setPublishPreview] = useState<{
    preview: ConfigPreviewResult;
    diff: ConfigDiffResult;
  } | null>(null);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);

  const versionsQuery = useQuery({
    queryKey: versionsQueryKey,
    queryFn: getConfigVersions,
  });

  const versions = useMemo(
    () => versionsQuery.data ?? [],
    [versionsQuery.data],
  );
  const activeVersion = useMemo(
    () => versions.find((item) => item.is_active) ?? null,
    [versions],
  );
  const selectedVersion = useMemo(
    () => versions.find((item) => item.id === selectedVersionId) ?? null,
    [selectedVersionId, versions],
  );

  const publishMutation = useMutation({
    mutationFn: publishConfigVersion,
    onSuccess: async (version) => {
      setFeedback({
        tone: 'success',
        message: `发布成功，版本 ${version.version}`,
      });
      setPublishPreview(null);
      setSelectedVersionId(version.id);
      await queryClient.invalidateQueries({ queryKey: versionsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const activateMutation = useMutation({
    mutationFn: activateConfigVersion,
    onSuccess: async (version) => {
      setFeedback({
        tone: 'success',
        message: `已激活版本 ${version.version}`,
      });
      setSelectedVersionId(version.id);
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
      const [preview, diff] = await Promise.all([
        getConfigVersionPreview(),
        getConfigVersionDiff(),
      ]);
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

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="配置版本"
          description="查看历史快照、预览待发布配置差异，并在需要时重新激活旧版本。"
          action={
            <PrimaryButton
              type="button"
              onClick={handleOpenPublishPreview}
              disabled={isPreviewLoading}
            >
              {isPreviewLoading ? '加载预览中...' : '预览并发布'}
            </PrimaryButton>
          }
        />

        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        {publishPreview ? (
          <PublishPreviewCard
            preview={publishPreview.preview}
            diff={publishPreview.diff}
            activeVersion={activeVersion}
            isPublishing={publishMutation.isPending}
            onConfirm={() => publishMutation.mutate()}
            onCancel={() => setPublishPreview(null)}
          />
        ) : null}

        <AppCard
          title="历史版本"
          description="发布成功后会立即刷新列表，不再需要手动刷新页面。"
          action={
            <SecondaryButton
              type="button"
              onClick={() =>
                void queryClient.invalidateQueries({
                  queryKey: versionsQueryKey,
                })
              }
            >
              刷新列表
            </SecondaryButton>
          }
        >
          {versionsQuery.isLoading ? (
            <LoadingState />
          ) : versionsQuery.isError ? (
            <ErrorState
              title="版本列表加载失败"
              description={getErrorMessage(versionsQuery.error)}
            />
          ) : versions.length === 0 ? (
            <EmptyState
              title="暂无历史版本"
              description="当前还没有可查看的发布记录，请先从反代规则页触发一次发布。"
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">版本号</th>
                    <th className="px-3 py-3 font-medium">状态</th>
                    <th className="px-3 py-3 font-medium">创建人</th>
                    <th className="px-3 py-3 font-medium">Checksum</th>
                    <th className="px-3 py-3 font-medium">创建时间</th>
                    <th className="px-3 py-3 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {versions.map((version) => (
                    <tr key={version.id} className="align-top">
                      <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                        {version.version}
                      </td>
                      <td className="px-3 py-4">
                        <StatusBadge
                          label={version.is_active ? '当前激活' : '历史版本'}
                          variant={version.is_active ? 'success' : 'info'}
                        />
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        {version.created_by || '系统'}
                      </td>
                      <td
                        className="px-3 py-4 text-[var(--foreground-secondary)]"
                        title={version.checksum}
                      >
                        {truncateChecksum(version.checksum)}
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        {formatDateTime(version.created_at)}
                      </td>
                      <td className="px-3 py-4">
                        <div className="flex flex-wrap gap-2">
                          <SecondaryButton
                            type="button"
                            onClick={() => setSelectedVersionId(version.id)}
                            className="px-3 py-2 text-xs"
                          >
                            查看快照
                          </SecondaryButton>
                          {!version.is_active ? (
                            <PrimaryButton
                              type="button"
                              onClick={() => handleActivate(version)}
                              disabled={activateMutation.isPending}
                              className="px-3 py-2 text-xs"
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
      </div>

      <ConfigVersionSnapshotModal
        version={selectedVersion}
        onClose={() => setSelectedVersionId(null)}
      />
    </>
  );
}
