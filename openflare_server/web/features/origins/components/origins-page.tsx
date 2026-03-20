'use client';

import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { deleteOrigin, getOrigins } from '@/features/origins/api/origins';
import { OriginEditorModal } from '@/features/origins/components/origin-editor-modal';
import type { OriginItem } from '@/features/origins/types';
import {
  DangerButton,
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

type FeedbackState = {
  tone: 'success' | 'danger';
  message: string;
};

export function OriginsPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [editingOrigin, setEditingOrigin] = useState<OriginItem | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);

  const originsQuery = useQuery({
    queryKey: ['origins'],
    queryFn: getOrigins,
  });

  const deleteMutation = useMutation({
    mutationFn: deleteOrigin,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '源站已删除。' });
      await queryClient.invalidateQueries({ queryKey: ['origins'] });
    },
    onError: (error) => {
      setFeedback({
        tone: 'danger',
        message:
          error instanceof Error ? error.message : '请求失败，请稍后重试。',
      });
    },
  });

  const origins = useMemo(() => originsQuery.data ?? [], [originsQuery.data]);

  const handleDelete = (origin: OriginItem) => {
    if (!window.confirm(`确认删除源站 ${origin.name} 吗？`)) {
      return;
    }
    setFeedback(null);
    deleteMutation.mutate(origin.id);
  };

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="源站"
          description="集中维护规则复用的源站地址，减少批量改地址时的重复操作。"
          action={
            <PrimaryButton
              type="button"
              onClick={() => {
                setEditingOrigin(null);
                setFeedback(null);
                setIsEditorOpen(true);
              }}
            >
              新增源站
            </PrimaryButton>
          }
        />

        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <AppCard
          title="源站列表"
          description="编辑源站地址后，所有引用该源站的规则会同步更新。"
        >
          {originsQuery.isLoading ? (
            <LoadingState />
          ) : originsQuery.isError ? (
            <ErrorState
              title="源站列表加载失败"
              description={
                originsQuery.error instanceof Error
                  ? originsQuery.error.message
                  : '请求失败，请稍后重试。'
              }
            />
          ) : origins.length === 0 ? (
            <EmptyState
              title="暂无源站"
              description="点击右上角“新增源站”开始录入。后续规则可直接复用这些地址。"
            />
          ) : (
            <div className="grid gap-4 lg:grid-cols-2">
              {origins.map((origin) => (
                <article
                  key={origin.id}
                  className="rounded-[28px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="space-y-4">
                      <div className="space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <h2 className="text-lg font-semibold text-[var(--foreground-primary)]">
                            {origin.name}
                          </h2>
                          <StatusBadge
                            label={`${origin.route_count} 条规则`}
                            variant={
                              origin.route_count > 0 ? 'success' : 'warning'
                            }
                          />
                        </div>
                        <p className="text-sm text-[var(--foreground-primary)]">
                          {origin.address}
                        </p>
                        <p className="text-sm text-[var(--foreground-secondary)]">
                          {origin.remark || '暂无备注'}
                        </p>
                      </div>

                      <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] px-4 py-3 text-sm text-[var(--foreground-secondary)]">
                        最后更新：{formatDateTime(origin.updated_at)}
                      </div>
                    </div>

                    <div className="flex flex-row gap-2">
                      <Link
                        href={`/origin/detail?id=${origin.id}`}
                        className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
                      >
                        详情
                      </Link>
                      <SecondaryButton
                        type="button"
                        onClick={() => {
                          setEditingOrigin(origin);
                          setFeedback(null);
                          setIsEditorOpen(true);
                        }}
                      >
                        编辑
                      </SecondaryButton>
                      <DangerButton
                        type="button"
                        onClick={() => handleDelete(origin)}
                        disabled={deleteMutation.isPending}
                      >
                        删除
                      </DangerButton>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          )}
        </AppCard>
      </div>

      {isEditorOpen ? (
        <OriginEditorModal
          isOpen={isEditorOpen}
          origin={editingOrigin}
          onClose={() => setIsEditorOpen(false)}
          onSaved={(origin, mode) => {
            setFeedback({
              tone: 'success',
              message: mode === 'create' ? '源站已创建。' : '源站已更新。',
            });
            setEditingOrigin(origin);
          }}
        />
      ) : null}
    </>
  );
}
