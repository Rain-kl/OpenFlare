'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { deleteOrigin, getOrigin } from '@/features/origins/api/origins';
import { OriginEditorModal } from '@/features/origins/components/origin-editor-modal';
import {
  DangerButton,
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

export function OriginDetailPage({ originId }: { originId: string }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<{
    tone: 'success' | 'danger';
    message: string;
  } | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);

  const originQuery = useQuery({
    queryKey: ['origins', originId],
    queryFn: () => getOrigin(Number(originId)),
    enabled: originId !== '',
  });

  const deleteMutation = useMutation({
    mutationFn: deleteOrigin,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['origins'] });
      router.push('/origin');
    },
    onError: (error) => {
      setFeedback({
        tone: 'danger',
        message:
          error instanceof Error ? error.message : '请求失败，请稍后重试。',
      });
    },
  });

  if (originQuery.isLoading) {
    return <LoadingState />;
  }

  if (originQuery.isError) {
    return (
      <ErrorState
        title="源站详情加载失败"
        description={
          originQuery.error instanceof Error
            ? originQuery.error.message
            : '请求失败，请稍后重试。'
        }
      />
    );
  }

  const origin = originQuery.data;
  if (!origin) {
    return (
      <EmptyState
        title="源站不存在"
        description="该源站可能已被删除，或当前 ID 无法匹配到源站记录。"
      />
    );
  }

  const handleDelete = () => {
    if (!window.confirm(`确认删除源站 ${origin.name} 吗？`)) {
      return;
    }
    deleteMutation.mutate(origin.id);
  };

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title={origin.name}
          description="源站详情"
          action={
            <>
              <Link
                href="/origin"
                className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
              >
                返回
              </Link>
              <SecondaryButton
                type="button"
                onClick={() => setIsEditorOpen(true)}
              >
                编辑源站
              </SecondaryButton>
              <DangerButton
                type="button"
                onClick={handleDelete}
                disabled={deleteMutation.isPending}
              >
                删除源站
              </DangerButton>
            </>
          }
        />

        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <div className="grid gap-4 xl:grid-cols-4">
          <AppCard title="源站地址">
            <p className="text-sm text-[var(--foreground-primary)]">
              {origin.address}
            </p>
          </AppCard>
          <AppCard title="绑定规则">
            <div className="space-y-3">
              <StatusBadge
                label={`${origin.route_count} 条规则`}
                variant={origin.route_count > 0 ? 'success' : 'warning'}
              />
              <p className="text-sm text-[var(--foreground-secondary)]">
                编辑地址后，绑定规则的主源站地址会一起更新。
              </p>
            </div>
          </AppCard>
          <AppCard title="创建时间">
            <p className="text-sm text-[var(--foreground-secondary)]">
              {formatDateTime(origin.created_at)}
            </p>
          </AppCard>
          <AppCard title="更新时间">
            <p className="text-sm text-[var(--foreground-secondary)]">
              {formatDateTime(origin.updated_at)}
            </p>
          </AppCard>
        </div>

        <AppCard title="备注">
          <p className="text-sm text-[var(--foreground-secondary)]">
            {origin.remark || '暂无备注'}
          </p>
        </AppCard>

        <AppCard
          title="关联规则"
          description="展示当前源站作为主源站绑定的规则。"
        >
          {origin.routes.length === 0 ? (
            <EmptyState
              title="暂无关联规则"
              description="当前源站还没有被任何规则引用。"
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">域名</th>
                    <th className="px-3 py-3 font-medium">源站地址</th>
                    <th className="px-3 py-3 font-medium">状态</th>
                    <th className="px-3 py-3 font-medium">更新时间</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {origin.routes.map((route) => (
                    <tr key={route.id}>
                      <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                        {route.domain}
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        {route.origin_url}
                      </td>
                      <td className="px-3 py-4">
                        <StatusBadge
                          label={route.enabled ? '启用' : '停用'}
                          variant={route.enabled ? 'success' : 'warning'}
                        />
                      </td>
                      <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                        {formatDateTime(route.updated_at)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </AppCard>
      </div>

      {isEditorOpen ? (
        <OriginEditorModal
          isOpen={isEditorOpen}
          origin={origin}
          onClose={() => setIsEditorOpen(false)}
          onSaved={() => {
            setFeedback({ tone: 'success', message: '源站已更新。' });
            void queryClient.invalidateQueries({
              queryKey: ['origins', origin.id],
            });
            void queryClient.invalidateQueries({ queryKey: ['origins'] });
          }}
        />
      ) : null}
    </>
  );
}
