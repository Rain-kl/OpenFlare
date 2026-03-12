'use client';

import { AppModal } from '@/components/ui/app-modal';
import type { ConfigVersionItem } from '@/features/config-versions/types';
import {
  CodeBlock,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

export function ConfigVersionSnapshotModal({
  version,
  onClose,
}: {
  version: ConfigVersionItem | null;
  onClose: () => void;
}) {
  return (
    <AppModal
      isOpen={Boolean(version)}
      onClose={onClose}
      title={version ? `版本 ${version.version}` : '查看快照'}
      description="在弹窗中查看快照 JSON 与渲染结果，避免把页面主视图撑长。"
      size="xl"
      footer={
        <div className="flex justify-end">
          <SecondaryButton type="button" onClick={onClose}>
            关闭
          </SecondaryButton>
        </div>
      }
    >
      {version ? (
        <div className="space-y-5">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                Checksum
              </p>
              <p className="mt-2 text-sm break-all text-[var(--foreground-primary)]">
                {version.checksum}
              </p>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                创建人
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {version.created_by || '系统'}
              </p>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                创建时间
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {formatDateTime(version.created_at)}
              </p>
            </div>
          </div>

          <div>
            <p className="mb-2 text-sm font-semibold text-[var(--foreground-primary)]">
              快照 JSON
            </p>
            <CodeBlock className="max-h-96 whitespace-pre-wrap">
              {version.snapshot_json}
            </CodeBlock>
          </div>
          <div>
            <p className="mb-2 text-sm font-semibold text-[var(--foreground-primary)]">
              主配置
            </p>
            <CodeBlock className="max-h-96 whitespace-pre-wrap">
              {version.main_config}
            </CodeBlock>
          </div>
          <div>
            <p className="mb-2 text-sm font-semibold text-[var(--foreground-primary)]">
              路由配置
            </p>
            <CodeBlock className="max-h-[32rem] whitespace-pre-wrap">
              {version.rendered_config}
            </CodeBlock>
          </div>
        </div>
      ) : null}
    </AppModal>
  );
}
