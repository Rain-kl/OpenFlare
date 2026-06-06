'use client';

import { useState } from 'react';

import { InlineMessage } from '@/components/feedback/inline-message';
import { AppModal } from '@/components/ui/app-modal';
import {
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';

export function CleanupConfigVersionsModal({
  isOpen,
  onClose,
  onConfirm,
  isPending,
}: {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (keepCount: number) => void;
  isPending: boolean;
}) {
  const [keepCount, setKeepCount] = useState<number>(10);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (keepCount < 3) {
      setError('最少保留 3 个历史快照');
      return;
    }
    setError(null);
    onConfirm(keepCount);
  };

  return (
    <AppModal
      isOpen={isOpen}
      onClose={onClose}
      title="清理历史快照"
      description="删除旧的历史快照配置，系统将始终自动保护当前已激活的版本不被删除。"
      size="md"
      footer={
        <div className="flex justify-end gap-3">
          <SecondaryButton type="button" onClick={onClose} disabled={isPending}>
            取消
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            form="cleanup-form"
            disabled={isPending}
            className="border-[var(--status-danger-foreground)] bg-[var(--status-danger-foreground)] hover:bg-[var(--status-danger-foreground)] hover:opacity-90"
          >
            {isPending ? '清理中...' : '确认清理'}
          </PrimaryButton>
        </div>
      }
    >
      <form id="cleanup-form" onSubmit={handleSubmit} className="space-y-4">
        {error && <InlineMessage tone="danger" message={error} />}
        <div>
          <label
            htmlFor="keepCount"
            className="mb-1 block text-sm font-medium text-[var(--foreground-primary)]"
          >
            保留最近快照个数
          </label>
          <input
            id="keepCount"
            type="number"
            min={3}
            value={keepCount}
            onChange={(e) => setKeepCount(parseInt(e.target.value, 10) || 3)}
            className="w-full rounded-lg border border-[var(--border-default)] bg-[var(--surface-default)] px-3 py-2 text-sm text-[var(--foreground-primary)] focus:border-[var(--brand-primary)] focus:ring-1 focus:ring-[var(--brand-primary)] focus:outline-none"
            disabled={isPending}
          />
          <p className="mt-1 text-xs text-[var(--foreground-secondary)]">
            默认为 10 个，最少需保留 3 个。
          </p>
        </div>
      </form>
    </AppModal>
  );
}
