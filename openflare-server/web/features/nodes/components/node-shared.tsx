'use client';

import React from 'react';
import type {NodeObservability} from '@/features/nodes/types';

export type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

export type HealthEventFilter = 'all' | 'active' | 'resolved';

export type NodeDetailTab = 'dashboard' | 'info';

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

export async function copyToClipboard(value: string) {
  await navigator.clipboard.writeText(value);
}

export function formatUsageRatio(used?: number | null, total?: number | null) {
  if (!used || !total || total <= 0) {
    return null;
  }
  return Math.max(0, Math.min(100, (used / total) * 100));
}

export function formatUptime(seconds?: number | null) {
  if (!seconds || seconds <= 0) {
    return '—';
  }

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (days > 0) {
    return `${days} 天 ${hours} 小时`;
  }
  if (hours > 0) {
    return `${hours} 小时 ${minutes} 分钟`;
  }
  return `${minutes} 分钟`;
}

export function formatTrendHour(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return `${date.getHours().toString().padStart(2, '0')}:00`;
}

export function parseTrafficMap(value?: string | null) {
  if (!value) {
    return {} as Record<string, number>;
  }
  try {
    const parsed = JSON.parse(value) as Record<string, number>;
    return Object.entries(parsed).reduce<Record<string, number>>(
      (result, [key, count]) => {
        if (typeof count === 'number' && Number.isFinite(count)) {
          result[key] = count;
        }
        return result;
      },
      {},
    );
  } catch {
    return {} as Record<string, number>;
  }
}

export function aggregateTrafficBreakdown(
  reports: NodeObservability['traffic_reports'],
  field: 'status_codes_json' | 'top_domains_json',
) {
  const summary = new Map<string, number>();
  for (const report of reports) {
    const parsed = parseTrafficMap(report[field]);
    for (const [key, value] of Object.entries(parsed)) {
      summary.set(key, (summary.get(key) ?? 0) + value);
    }
  }
  return Array.from(summary.entries())
    .sort((left, right) => {
      if (right[1] === left[1]) {
        return left[0].localeCompare(right[0]);
      }
      return right[1] - left[1];
    })
    .slice(0, 6)
    .map(([label, value]) => ({ label, value }));
}

export function getHealthEventVariant(
  event: NodeObservability['health_events'][number],
): 'success' | 'warning' | 'danger' | 'info' {
  if (event.status === 'resolved') {
    return 'success';
  }
  if (event.severity === 'critical') {
    return 'danger';
  }
  if (event.severity === 'warning') {
    return 'warning';
  }
  return 'info';
}

export function getHealthEventLabel(
  event: NodeObservability['health_events'][number],
) {
  return event.event_type.replaceAll('_', ' ');
}

export function MetricBar({
  label,
  value,
  progress,
  hint,
}: {
  label: string;
  value: string;
  progress?: number | null;
  hint?: string;
}) {
  return (
    <div className="space-y-2 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
            {label}
          </p>
          {hint ? (
            <p className="mt-1 text-xs text-[var(--foreground-muted)]">
              {hint}
            </p>
          ) : null}
        </div>
        <p className="text-sm font-semibold text-[var(--foreground-primary)]">
          {value}
        </p>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-[var(--surface-muted)]">
        <div
          className="h-full rounded-full bg-[var(--status-info-foreground)] transition-[width]"
          style={{ width: `${progress ?? 0}%` }}
        />
      </div>
    </div>
  );
}

export function SummaryStat({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint: string;
}) {
  return (
    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
      <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
        {label}
      </p>
      <p className="mt-3 text-2xl font-semibold text-[var(--foreground-primary)]">
        {value}
      </p>
      <p className="mt-2 text-sm text-[var(--foreground-secondary)]">{hint}</p>
    </div>
  );
}
