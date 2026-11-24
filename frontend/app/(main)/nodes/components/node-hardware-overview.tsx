'use client';

import {
  ArrowDown,
  ArrowDownUp,
  ArrowUp,
  Clock,
  Cpu,
  Gauge,
  HardDrive,
  Layers,
  MemoryStick,
  Monitor,
  Network,
  Server,
  Terminal,
  Timer,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import type { ReactNode } from 'react';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type {
  NodeMetricSnapshot,
  NodeSystemProfile,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import {
  formatBytes,
  formatBytesPerSecond,
  formatUptime,
  isMeaningfulTime,
} from './node-utils';

function SpecTile({
  label,
  value,
  icon,
  className,
}: {
  label: string;
  value: ReactNode;
  icon: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`min-w-0 flex flex-col gap-1.5 rounded-lg border border-dashed bg-muted/20 px-3 py-2.5 ${className ?? ''}`}
    >
      <div className='flex items-center gap-1.5 text-xs text-muted-foreground'>
        {icon}
        <span className='truncate'>{label}</span>
      </div>
      <div className='text-xs sm:text-sm font-medium text-foreground break-all tabular-nums'>
        {value}
      </div>
    </div>
  );
}

export function NodeHardwareOverview({
  profile,
  latestMetric,
}: {
  profile: NodeSystemProfile | null;
  latestMetric: NodeMetricSnapshot | null;
}) {
  const t = useTranslations('nodes');
  const unknown = '—';

  const cpuTitle = profile?.cpu_model
    ? `${profile.cpu_model} (×${profile.cpu_cores || 0}${
        profile.cpu_physical_cores &&
        profile.cpu_physical_cores !== profile.cpu_cores
          ? ` / ${profile.cpu_physical_cores}C`
          : ''
      })`
    : unknown;

  const osText = profile?.os_name
    ? `${profile.os_name}${profile.os_version ? ` ${profile.os_version}` : ''}`
    : unknown;

  const uptimeSeconds =
    latestMetric?.uptime_seconds && latestMetric.uptime_seconds > 0
      ? latestMetric.uptime_seconds
      : profile?.uptime_seconds;

  const totalMemory =
    latestMetric?.memory_total_bytes || profile?.total_memory_bytes || 0;
  const totalSwap =
    latestMetric?.swap_total_bytes ?? profile?.swap_total_bytes ?? 0;
  const totalDisk =
    latestMetric?.storage_total_bytes || profile?.total_disk_bytes || 0;

  return (
    <div className='flex flex-col gap-4'>
      <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm font-semibold'>
              {t('overview.hardwareTitle')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
              <SpecTile
                className='sm:col-span-3'
                label={t('obs.cpu')}
                icon={<Cpu className='size-3.5 shrink-0' />}
                value={cpuTitle}
              />
              <SpecTile
                label={t('overview.architecture')}
                icon={<Layers className='size-3.5 shrink-0' />}
                value={profile?.architecture || unknown}
              />
              <SpecTile
                label={t('overview.virtualization')}
                icon={<Server className='size-3.5 shrink-0' />}
                value={profile?.virtualization || unknown}
              />
              <SpecTile
                label={t('overview.gpu')}
                icon={<Monitor className='size-3.5 shrink-0' />}
                value={profile?.gpu_name || unknown}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm font-semibold'>
              {t('obs.systemInfo')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
              <SpecTile
                label={t('obs.os')}
                icon={<Monitor className='size-3.5 shrink-0' />}
                value={osText}
              />
              <SpecTile
                label={t('overview.kernelVersion')}
                icon={<Terminal className='size-3.5 shrink-0' />}
                value={profile?.kernel_version || unknown}
              />
              <SpecTile
                label={t('obs.uptime')}
                icon={<Timer className='size-3.5 shrink-0' />}
                value={formatUptime(uptimeSeconds, t)}
              />
              <SpecTile
                label={t('overview.lastReported')}
                icon={<Clock className='size-3.5 shrink-0' />}
                value={
                  isMeaningfulTime(profile?.reported_at)
                    ? formatDateTime(profile.reported_at)
                    : unknown
                }
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm font-semibold'>
              {t('overview.storageTitle')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
              <SpecTile
                label={t('obs.memory')}
                icon={<MemoryStick className='size-3.5 shrink-0' />}
                value={totalMemory > 0 ? formatBytes(totalMemory) : unknown}
              />
              <SpecTile
                label={t('overview.swap')}
                icon={<ArrowDownUp className='size-3.5 shrink-0' />}
                value={totalSwap > 0 ? formatBytes(totalSwap) : '0 B'}
              />
              <SpecTile
                label={t('overview.disk')}
                icon={<HardDrive className='size-3.5 shrink-0' />}
                value={totalDisk > 0 ? formatBytes(totalDisk) : unknown}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='text-sm font-semibold'>
              {t('overview.networkTitle')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
              <SpecTile
                label={t('overview.totalTraffic')}
                icon={<Network className='size-3.5 shrink-0' />}
                value={
                  latestMetric ? (
                    <span className='inline-flex flex-wrap items-center gap-2'>
                      <span className='inline-flex items-center gap-0.5'>
                        <ArrowUp className='size-3 text-primary' />
                        {formatBytes(latestMetric.net_total_up ?? 0)}
                      </span>
                      <span className='text-muted-foreground'>/</span>
                      <span className='inline-flex items-center gap-0.5'>
                        <ArrowDown className='size-3 text-muted-foreground' />
                        {formatBytes(latestMetric.net_total_down ?? 0)}
                      </span>
                    </span>
                  ) : (
                    unknown
                  )
                }
              />
              <SpecTile
                label={t('overview.networkSpeed')}
                icon={<Gauge className='size-3.5 shrink-0' />}
                value={
                  latestMetric ? (
                    <span className='inline-flex flex-wrap items-center gap-2'>
                      <span className='inline-flex items-center gap-0.5'>
                        <ArrowUp className='size-3 text-primary' />
                        {formatBytesPerSecond(latestMetric.net_out ?? 0)}
                      </span>
                      <span className='text-muted-foreground'>/</span>
                      <span className='inline-flex items-center gap-0.5'>
                        <ArrowDown className='size-3 text-muted-foreground' />
                        {formatBytesPerSecond(latestMetric.net_in ?? 0)}
                      </span>
                    </span>
                  ) : (
                    unknown
                  )
                }
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
