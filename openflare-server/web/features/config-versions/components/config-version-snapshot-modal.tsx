'use client';

import { useQuery } from '@tanstack/react-query';

import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppModal } from '@/components/ui/app-modal';
import { getConfigVersion } from '@/features/config-versions/api/config-versions';
import type { ConfigVersionSummary } from '@/features/config-versions/types';
import {
  CodeBlock,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

export function ConfigVersionSnapshotModal({
  version,
  onClose,
}: {
  version: ConfigVersionSummary | null;
  onClose: () => void;
}) {
  const versionDetailQuery = useQuery({
    queryKey: ['config-versions', 'detail', version?.id ?? 0],
    queryFn: () => {
      if (!version) {
        throw new Error('missing config version');
      }
      return getConfigVersion(version.id);
    },
    enabled: Boolean(version?.id),
  });
  const versionDetail = versionDetailQuery.data ?? null;

  return (
    <AppModal
      isOpen={Boolean(version)}
      onClose={onClose}
      title={version ? `版本 ${version.version}` : '查看快照'}
      size="xl"
      footer={
        <div className="flex justify-end">
          <SecondaryButton type="button" onClick={onClose}>
            关闭
          </SecondaryButton>
        </div>
      }
    >
      {!version ? null : versionDetailQuery.isLoading && !versionDetail ? (
        <LoadingState />
      ) : versionDetailQuery.isError ? (
        <ErrorState
          title="配置版本详情加载失败"
          description={getErrorMessage(versionDetailQuery.error)}
        />
      ) : versionDetail ? (
        <div className="space-y-5">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                Checksum
              </p>
              <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                {versionDetail.checksum}
              </p>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                创建人
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {versionDetail.created_by || '系统'}
              </p>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                创建时间
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {formatDateTime(versionDetail.created_at)}
              </p>
            </div>
          </div>

          <div>
            <p className="mb-2 text-sm font-semibold text-[var(--foreground-primary)]">
              快照 JSON
            </p>
            <CodeBlock className="max-h-96 whitespace-pre-wrap">
              {versionDetail.snapshot_json}
            </CodeBlock>
          </div>

          <div>
            <p className="mb-2 text-sm font-semibold text-[var(--foreground-primary)]">
              主配置
            </p>
            <CodeBlock className="max-h-96 whitespace-pre-wrap">
              {versionDetail.main_config}
            </CodeBlock>
          </div>

          <div>
            <p className="mb-2 text-sm font-semibold text-[var(--foreground-primary)]">
              路由配置
            </p>
            <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
              {versionDetail.rendered_config}
            </CodeBlock>
          </div>
        </div>
      ) : null}
    </AppModal>
  );
}
