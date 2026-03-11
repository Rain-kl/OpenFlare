'use client';

import { marked } from 'marked';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppCard } from '@/components/ui/app-card';
import { AppModal } from '@/components/ui/app-modal';
import { StatusBadge } from '@/components/ui/status-badge';
import type { LatestReleaseInfo } from '@/features/update/types';
import {
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';

interface VersionUpgradeModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentVersion: string;
  frontendVersion: string;
  startTime?: number;
  release: LatestReleaseInfo | null | undefined;
  isLoading: boolean;
  errorMessage?: string;
  canUpgrade: boolean;
  isChecking: boolean;
  isUpgrading: boolean;
  onRefresh: () => void;
  onUpgrade: () => void;
}

function getUpgradeBadge(release: LatestReleaseInfo | null | undefined) {
  if (!release) {
    return { label: '未检查', variant: 'info' as const };
  }
  if (release.in_progress) {
    return { label: '升级中', variant: 'warning' as const };
  }
  if (release.has_update) {
    return { label: '可升级', variant: 'warning' as const };
  }
  return { label: '最新', variant: 'success' as const };
}

export function VersionUpgradeModal({
  isOpen,
  onClose,
  currentVersion,
  frontendVersion,
  startTime,
  release,
  isLoading,
  errorMessage,
  canUpgrade,
  isChecking,
  isUpgrading,
  onRefresh,
  onUpgrade,
}: VersionUpgradeModalProps) {
  const upgradeBadge = getUpgradeBadge(release);

  return (
    <AppModal
      isOpen={isOpen}
      onClose={onClose}
      title="版本"
      description="在这里检查 GitHub 最新版本，并直接触发 Server 自升级。升级开始后服务会短暂重启。"
      size="lg"
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton
            type="button"
            onClick={onRefresh}
            disabled={isChecking || isUpgrading}
          >
            {isChecking ? '检查中...' : '检查更新'}
          </SecondaryButton>
          {canUpgrade ? (
            <PrimaryButton
              type="button"
              onClick={onUpgrade}
              disabled={
                !release?.has_update ||
                release.in_progress ||
                isUpgrading ||
                !release.upgrade_supported
              }
            >
              {isUpgrading
                ? '升级中...'
                : release?.in_progress
                  ? '升级中...'
                  : '立即升级'}
            </PrimaryButton>
          ) : null}
        </div>
      }
    >
      <div className="space-y-6">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <AppCard title="前端版本">
            <p className="text-sm font-medium text-[var(--foreground-primary)]">
              {frontendVersion}
            </p>
          </AppCard>
          <AppCard title="服务端版本">
            <div className="flex flex-wrap items-center gap-3">
              <p className="text-sm font-medium text-[var(--foreground-primary)]">
                {currentVersion || 'unknown'}
              </p>
              <StatusBadge
                label={upgradeBadge.label}
                variant={upgradeBadge.variant}
              />
            </div>
          </AppCard>
          <AppCard title="最新版本">
            <p className="text-sm font-medium text-[var(--foreground-primary)]">
              {release?.tag_name || '未检查'}
            </p>
          </AppCard>
          <AppCard title="启动时间">
            <p className="text-sm font-medium text-[var(--foreground-primary)]">
              {startTime ? formatDateTime(new Date(startTime * 1000)) : '未知'}
            </p>
          </AppCard>
        </div>

        {isLoading ? <LoadingState /> : null}
        {!isLoading && errorMessage ? (
          <ErrorState title="版本检查失败" description={errorMessage} />
        ) : null}
        {!isLoading && !errorMessage && !release ? (
          <EmptyState
            title="尚未检查更新"
            description="点击“检查更新”后会在这里展示最新 GitHub Release 信息。"
          />
        ) : null}
        {!isLoading && !errorMessage && release ? (
          <AppCard
            title={`GitHub Release · ${release.tag_name}`}
            description={
              release.published_at
                ? `发布时间：${formatRelativeTime(release.published_at)} · ${formatDateTime(release.published_at)}`
                : '未提供发布时间'
            }
          >
            <div className="space-y-4">
              <div className="flex flex-wrap items-center gap-3">
                <StatusBadge
                  label={release.has_update ? '发现新版本' : '已经是最新版本'}
                  variant={release.has_update ? 'warning' : 'success'}
                />
                {!release.upgrade_supported ? (
                  <StatusBadge
                    label="当前平台不支持自动升级"
                    variant="danger"
                  />
                ) : null}
                {release.in_progress ? (
                  <StatusBadge label="升级任务执行中" variant="warning" />
                ) : null}
              </div>
              <div
                className="prose prose-sm max-w-none text-[var(--foreground-primary)] [&_a]:text-[var(--brand-primary)]"
                dangerouslySetInnerHTML={{
                  __html: marked.parse(
                    release.body || '暂无更新说明',
                  ) as string,
                }}
              />
              <a
                href={release.html_url}
                target="_blank"
                rel="noreferrer"
                className="text-sm font-medium text-[var(--brand-primary)] transition hover:opacity-80"
              >
                查看发布详情
              </a>
            </div>
          </AppCard>
        ) : null}
      </div>
    </AppModal>
  );
}
