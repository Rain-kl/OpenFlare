import { useTranslations } from 'next-intl';

import { Progress } from '@/components/ui/progress';
import type {
  NodeMetricSnapshot,
  NodeSystemProfile,
} from '@/lib/services/openflare/types';
import { formatDateTime } from '@/lib/utils';

import {
  formatBytes,
  formatMetricCount,
  formatPercent,
  formatRelativeTime,
  formatUptime,
  formatUsageRatio,
  isMeaningfulTime,
} from './node-utils';

export function NodeSystemDetails({
  profile,
  nodeName,
}: {
  profile: NodeSystemProfile;
  nodeName: string;
}) {
  const t = useTranslations('nodes');
  const unknown = t('unknown');

  return (
    <dl className='grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-x-4 gap-y-3 text-sm wrap-anywhere [&>dt]:text-muted-foreground'>
      <dt>{t('obs.hostname')}</dt>
      <dd>{profile.hostname || nodeName}</dd>
      <dt>{t('obs.os')}</dt>
      <dd>
        {profile.os_name || unknown}
        {profile.os_version ? ` ${profile.os_version}` : ''}
      </dd>
      <dt>{t('obs.kernelArch')}</dt>
      <dd>
        {profile.kernel_version || unknown} · {profile.architecture || unknown}
      </dd>
      <dt>{t('obs.uptime')}</dt>
      <dd>{formatUptime(profile.uptime_seconds, t)}</dd>
      <dt>{t('obs.cpu')}</dt>
      <dd>
        {profile.cpu_model || unknown}
        <span className='block text-xs text-muted-foreground'>
          {t('obs.cpuCores', { count: profile.cpu_cores || 0 })}
        </span>
      </dd>
      <dt>{t('obs.reportedAt')}</dt>
      <dd>
        {isMeaningfulTime(profile.reported_at)
          ? formatDateTime(profile.reported_at)
          : '—'}
      </dd>
    </dl>
  );
}

function ResourceMetric({
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
    <div className='grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-x-3 gap-y-1.5 text-sm'>
      <dt className='wrap-anywhere text-muted-foreground'>{label}</dt>
      <dd className='text-right font-medium wrap-anywhere tabular-nums'>
        {value}
      </dd>
      {hint ? (
        <dd className='col-span-full text-xs text-muted-foreground'>{hint}</dd>
      ) : null}
      {progress != null && Number.isFinite(progress) ? (
        <dd className='col-span-full'>
          <Progress
            aria-label={label}
            className='h-1.5'
            value={Math.max(0, Math.min(100, progress))}
          />
        </dd>
      ) : null}
    </div>
  );
}

export function NodeResourceSnapshot({
  metric,
  connectionHint,
}: {
  metric: NodeMetricSnapshot;
  connectionHint: string;
}) {
  const t = useTranslations('nodes');

  return (
    <dl className='flex flex-col gap-4'>
      <ResourceMetric
        label={t('obs.cpu')}
        value={formatPercent(metric.cpu_usage_percent)}
        progress={metric.cpu_usage_percent}
        hint={
          isMeaningfulTime(metric.captured_at)
            ? t('obs.snapshotAgo', {
                time: formatRelativeTime(metric.captured_at, t),
              })
            : undefined
        }
      />
      <ResourceMetric
        label={t('obs.memory')}
        value={`${formatBytes(metric.memory_used_bytes)} / ${formatBytes(metric.memory_total_bytes)}`}
        progress={formatUsageRatio(
          metric.memory_used_bytes,
          metric.memory_total_bytes,
        )}
      />
      <ResourceMetric
        label={t('obs.storage')}
        value={`${formatBytes(metric.storage_used_bytes)} / ${formatBytes(metric.storage_total_bytes)}`}
        progress={formatUsageRatio(
          metric.storage_used_bytes,
          metric.storage_total_bytes,
        )}
      />
      <ResourceMetric
        label={t('obs.connections')}
        value={formatMetricCount(metric.openresty_connections)}
        hint={connectionHint}
      />
    </dl>
  );
}
