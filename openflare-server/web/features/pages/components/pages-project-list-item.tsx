'use client';

import Link from 'next/link';

import {StatusBadge} from '@/components/ui/status-badge';
import type {PagesProject} from '@/features/pages/types';
import {formatDateTime} from '@/lib/utils/date';

export function PagesProjectListItem({ project }: { project: PagesProject }) {
  return (
    <Link
      href={`/pages/detail?id=${project.id}`}
      className="group block rounded-[28px] border border-[var(--border-default)] bg-[var(--surface-panel)] p-5 shadow-[var(--shadow-card)] transition hover:-translate-y-0.5 hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-soft)]"
    >
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-semibold text-[var(--foreground-primary)]">
              {project.name}
            </h2>
            <StatusBadge
              label={project.enabled ? '已启用' : '已停用'}
              variant={project.enabled ? 'success' : 'warning'}
            />
            <StatusBadge
              label={project.spa_fallback_enabled ? 'SPA fallback' : '严格 404'}
              variant={project.spa_fallback_enabled ? 'info' : 'warning'}
            />
          </div>
          <p className="text-sm text-[var(--foreground-secondary)]">
            {project.slug}
          </p>
          {project.description ? (
            <p className="line-clamp-2 text-sm leading-6 text-[var(--foreground-secondary)]">
              {project.description}
            </p>
          ) : null}
          {project.spa_fallback_enabled ? (
            <p className="text-xs text-[var(--foreground-secondary)]">
              回退路径：{project.spa_fallback_path || '/index.html'}
            </p>
          ) : null}
        </div>

        <div className="grid shrink-0 grid-cols-1 gap-3 text-sm md:min-w-40">
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] px-4 py-3">
            <p className="text-xs text-[var(--foreground-secondary)]">
              当前激活
            </p>
            <p className="mt-1 font-semibold text-[var(--foreground-primary)]">
              {project.active_deployment
                ? `#${project.active_deployment.deployment_number}`
                : '暂无'}
            </p>
          </div>
        </div>
      </div>
      <div className="mt-4 flex items-center justify-between border-t border-[var(--border-default)] pt-4">
        <p className="text-xs text-[var(--foreground-secondary)]">
          激活时间：
          {project.active_deployment?.activated_at
            ? formatDateTime(project.active_deployment.activated_at)
            : '未激活'}
        </p>
        <span className="text-sm font-medium text-[var(--brand-primary)] transition group-hover:translate-x-1">
          查看详情 →
        </span>
      </div>
    </Link>
  );
}
