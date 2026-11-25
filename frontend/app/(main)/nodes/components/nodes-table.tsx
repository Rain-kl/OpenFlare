'use client';

import Link from 'next/link';
import { Eye, Pencil, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { NodeItem } from '@/lib/services/openflare';

import {
  MissingMetric,
  NodeLatency,
  NodeNetworkRate,
  NodeUsageBar,
} from './node-monitoring-metrics';
import { NodeStatusBadge } from './node-status-badge';
import {
  formatUsageRatio,
  getNodeStatusLabel,
  getNodeStatusTone,
  getOpenrestyStatusLabel,
  getOpenrestyStatusTone,
} from './node-utils';

export type NodeListView = 'list' | 'grid';

type NodeActionsProps = {
  deletingId: number | null;
  actionsDisabled?: boolean;
  onEdit: (node: NodeItem) => void;
  onDelete: (node: NodeItem) => void;
};

function NodeActions({
  node,
  deletingId,
  actionsDisabled,
  onEdit,
  onDelete,
}: NodeActionsProps & { node: NodeItem }) {
  const t = useTranslations('nodes');
  return (
    <div className='flex flex-col items-center'>
      {actionsDisabled ? (
        <Button
          variant='ghost'
          size='icon-sm'
          className='size-7'
          disabled
          aria-label={t('detail')}
        >
          <Eye />
        </Button>
      ) : (
        <Button variant='ghost' size='icon-sm' className='size-7' asChild>
          <Link
            href={`/nodes/detail?id=${node.id}`}
            aria-label={t('detail')}
            title={t('detail')}
          >
            <Eye />
          </Link>
        </Button>
      )}
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        className='size-7'
        aria-label={t('edit')}
        title={t('edit')}
        disabled={actionsDisabled}
        onClick={() => onEdit(node)}
      >
        <Pencil />
      </Button>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        className='size-7 text-destructive hover:text-destructive'
        aria-label={t('delete')}
        title={t('delete')}
        disabled={actionsDisabled || deletingId === node.id}
        onClick={() => onDelete(node)}
      >
        <Trash2 />
      </Button>
    </div>
  );
}

function NodeIdentity({ node }: { node: NodeItem }) {
  const t = useTranslations('nodes');
  const ip = node.ip
    ? t('ipLine', { ip: node.ip }) +
      (node.ip_manual_override ? t('lockedSuffix') : '')
    : '';
  const description = [node.geo_name, ip].filter(Boolean).join(' · ');
  const system = node.monitoring?.system;

  return (
    <div className='flex min-w-0 flex-col gap-0.5'>
      <p className='truncate font-semibold' title={node.name}>
        {node.name}
      </p>
      {description && (
        <p
          className='truncate text-xs text-muted-foreground'
          title={description}
        >
          {description}
        </p>
      )}
      {system && (
        <p className='truncate text-xs text-muted-foreground' title={system}>
          {system}
        </p>
      )}
    </div>
  );
}

function NodeHealth({ node }: { node: NodeItem }) {
  const t = useTranslations('nodes');
  const status = getNodeStatusLabel(node.status, t);
  const online = node.status === 'online';
  return (
    <NodeStatusBadge
      className='max-w-full justify-start gap-1.5 border-0 bg-transparent p-0 text-xs whitespace-normal before:size-1.5 before:shrink-0 before:rounded-full before:bg-current'
      label={
        online
          ? `${status} · ${getOpenrestyStatusLabel(node.openresty_status, t)}`
          : status
      }
      tone={
        online
          ? getOpenrestyStatusTone(node.openresty_status)
          : getNodeStatusTone(node.status)
      }
    />
  );
}

function getMetricValues(node: NodeItem) {
  const metric = node.monitoring?.metric;
  return {
    cpu: metric?.cpu_usage_percent,
    memory: formatUsageRatio(
      metric?.memory_used_bytes,
      metric?.memory_total_bytes,
    ),
    disk: formatUsageRatio(
      metric?.storage_used_bytes,
      metric?.storage_total_bytes,
    ),
  };
}

const usageKeys = ['cpu', 'memory', 'disk'] as const;

function NodeCards({
  nodes,
  ...actions
}: { nodes: NodeItem[] } & NodeActionsProps) {
  const t = useTranslations('nodes');
  return (
    <div className='@container' aria-busy={actions.actionsDisabled}>
      <div className='grid gap-3 @min-[34rem]:grid-cols-2 @min-[64rem]:grid-cols-3'>
        {nodes.map((node) => {
          const metrics = getMetricValues(node);
          return (
            <Card key={node.id} className='gap-4 py-4'>
              <CardHeader className='grid-cols-[minmax(0,1fr)_auto] grid-rows-[auto] gap-2 px-4'>
                <div className='flex min-w-0 flex-col gap-2'>
                  <div className='flex flex-wrap items-start justify-between gap-2'>
                    <CardTitle className='min-w-0'>
                      <NodeIdentity node={node} />
                    </CardTitle>
                    <NodeHealth node={node} />
                  </div>
                  <dl className='grid grid-cols-2 gap-x-3 gap-y-1 text-xs'>
                    {(
                      [
                        [t('table.version'), node.version],
                        [
                          t('table.currentVersion'),
                          node.current_version || t('notApplied'),
                        ],
                      ] as const
                    ).map(([label, value]) => (
                      <div
                        key={label}
                        className='flex min-w-0 flex-col gap-0.5'
                      >
                        <dt className='text-muted-foreground'>{label}</dt>
                        <dd className='truncate' title={value}>
                          {value || <MissingMetric />}
                        </dd>
                      </div>
                    ))}
                  </dl>
                </div>
                <NodeActions node={node} {...actions} />
              </CardHeader>
              <CardContent className='flex flex-1 flex-col gap-4 px-4'>
                <div className='grid grid-cols-3 gap-3'>
                  {usageKeys.map((key) => (
                    <div key={key} className='flex flex-col gap-1.5'>
                      <p className='text-xs text-muted-foreground'>
                        {t(`monitoring.${key}`)}
                      </p>
                      <NodeUsageBar
                        label={t(`monitoring.${key}`)}
                        value={metrics[key]}
                      />
                    </div>
                  ))}
                </div>
                <div className='grid grid-cols-[minmax(5rem,1fr)_minmax(0,1.6fr)] gap-3'>
                  <div className='flex flex-col gap-1.5'>
                    <p className='text-xs text-muted-foreground'>
                      {t('monitoring.rate')}
                    </p>
                    <NodeNetworkRate
                      upload={node.monitoring?.metric?.net_out}
                      download={node.monitoring?.metric?.net_in}
                    />
                  </div>
                  <div className='flex min-w-0 flex-col gap-1'>
                    <p className='text-xs text-muted-foreground'>
                      {t('monitoring.latency')}
                    </p>
                    <NodeLatency tasks={node.monitoring?.ping} />
                  </div>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>
    </div>
  );
}

export function NodesTable({
  nodes,
  view,
  ...actions
}: { nodes: NodeItem[]; view: NodeListView } & NodeActionsProps) {
  const t = useTranslations('nodes');
  if (view === 'grid') return <NodeCards nodes={nodes} {...actions} />;

  return (
    <div aria-busy={actions.actionsDisabled}>
      <Table className='min-w-[59rem] table-fixed border-separate border-spacing-y-2'>
        <colgroup>
          <col />
          <col className='w-28' />
          <col className='w-20' />
          <col className='w-21' />
          <col className='w-20' />
          <col className='w-20' />
          <col className='w-20' />
          <col className='w-24' />
          <col className='w-43' />
          <col className='w-15' />
        </colgroup>
        <TableHeader>
          <TableRow className='hover:bg-transparent [&>th]:h-8 [&>th]:text-xs [&>th]:whitespace-normal [&>th]:text-muted-foreground [&>th:first-child]:pl-3'>
            <TableHead scope='col'>{t('table.node')}</TableHead>
            <TableHead scope='col'>{t('table.status')}</TableHead>
            <TableHead scope='col'>{t('table.version')}</TableHead>
            <TableHead scope='col'>{t('table.currentVersion')}</TableHead>
            {usageKeys.map((key) => (
              <TableHead scope='col' key={key}>
                {t(`monitoring.${key}`)}
              </TableHead>
            ))}
            <TableHead scope='col'>{t('monitoring.rate')}</TableHead>
            <TableHead scope='col'>{t('monitoring.latency')}</TableHead>
            <TableHead scope='col' className='text-center'>
              {t('table.actions')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {nodes.map((node) => {
            const metrics = getMetricValues(node);
            const currentVersion = node.current_version || t('notApplied');
            return (
              <TableRow
                key={node.id}
                className='bg-card [&>td]:border-y [&>td]:border-dashed [&>td]:py-2 [&>td:first-child]:rounded-l-lg [&>td:first-child]:border-l [&>td:first-child]:pl-3 [&>td:last-child]:rounded-r-lg [&>td:last-child]:border-r [&>td:last-child]:py-1'
              >
                <TableCell>
                  <NodeIdentity node={node} />
                </TableCell>
                <TableCell>
                  <NodeHealth node={node} />
                </TableCell>
                <TableCell
                  className='truncate text-xs tabular-nums'
                  title={node.version}
                >
                  {node.version || <MissingMetric />}
                </TableCell>
                <TableCell
                  className='truncate text-xs text-muted-foreground tabular-nums'
                  title={currentVersion}
                >
                  {currentVersion}
                </TableCell>
                {usageKeys.map((key) => (
                  <TableCell key={key}>
                    <NodeUsageBar
                      label={t(`monitoring.${key}`)}
                      value={metrics[key]}
                    />
                  </TableCell>
                ))}
                <TableCell>
                  <NodeNetworkRate
                    upload={node.monitoring?.metric?.net_out}
                    download={node.monitoring?.metric?.net_in}
                  />
                </TableCell>
                <TableCell>
                  <NodeLatency tasks={node.monitoring?.ping} />
                </TableCell>
                <TableCell>
                  <NodeActions node={node} {...actions} />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
