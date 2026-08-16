'use client';

import Link from 'next/link';
import { Pencil, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { formatDateTime } from '@/lib/utils';
import type { NodeItem } from '@/lib/services/openflare';

import { NodeStatusBadge } from './node-status-badge';
import {
  formatRelativeTime,
  getApplyLabel,
  getApplyTone,
  getNodeStatusLabel,
  getNodeStatusTone,
  getNodeTypeLabel,
  getOpenrestyStatusLabel,
  getOpenrestyStatusTone,
  getRelayStatusLabel,
  getRelayStatusTone,
  isMeaningfulTime,
  isWSConnectedLastSeen,
} from './node-utils';

export function NodesTable({
  nodes,
  deletingId,
  onEdit,
  onDelete,
}: {
  nodes: NodeItem[];
  deletingId: number | null;
  onEdit: (node: NodeItem) => void;
  onDelete: (node: NodeItem) => void;
}) {
  const t = useTranslations('nodes');
  return (
    <div className='border border-dashed rounded-lg overflow-hidden'>
      <Table>
        <TableHeader>
          <TableRow className='border-dashed hover:bg-transparent'>
            <TableHead className='py-2 h-8'>{t('table.node')}</TableHead>
            <TableHead className='py-2 h-8'>{t('table.status')}</TableHead>
            <TableHead className='py-2 h-8'>{t('table.version')}</TableHead>
            <TableHead className='py-2 h-8'>{t('table.runHealth')}</TableHead>
            <TableHead className='py-2 h-8'>
              {t('table.currentVersion')}
            </TableHead>
            <TableHead className='py-2 h-8'>{t('table.latestApply')}</TableHead>
            <TableHead className='py-2 h-8'>{t('table.lastSeen')}</TableHead>
            <TableHead className='py-2 h-8 text-right'>
              {t('table.actions')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {nodes.map((node) => (
            <TableRow key={node.id} className='border-dashed align-top'>
              <TableCell className='py-3'>
                <div className='space-y-1'>
                  <div className='flex items-center gap-2'>
                    <p className='font-medium'>{node.name}</p>
                    <NodeStatusBadge
                      label={getNodeTypeLabel(node.node_type)}
                      tone='info'
                    />
                  </div>
                  <p className='text-xs text-muted-foreground'>
                    {t('ipLine', { ip: node.ip || '—' })}
                    {node.ip_manual_override ? t('lockedSuffix') : ''}
                  </p>
                  <p className='text-xs text-muted-foreground'>
                    {t('locationLine', {
                      location: node.geo_name || t('noGeo'),
                    })}
                  </p>
                </div>
              </TableCell>
              <TableCell className='py-3'>
                <NodeStatusBadge
                  label={getNodeStatusLabel(node.status, t)}
                  tone={getNodeStatusTone(node.status)}
                />
              </TableCell>
              <TableCell className='py-3 text-muted-foreground'>
                {node.version || 'unknown'}
              </TableCell>
              <TableCell className='py-3'>
                {node.node_type === 'tunnel_relay' ? (
                  <NodeStatusBadge
                    label={getRelayStatusLabel(node.relay_status, t)}
                    tone={getRelayStatusTone(node.relay_status)}
                  />
                ) : node.node_type === 'tunnel_client' ? (
                  <NodeStatusBadge
                    label={
                      node.status === 'online' ? t('running') : t('unknown')
                    }
                    tone={node.status === 'online' ? 'success' : 'warning'}
                  />
                ) : (
                  <NodeStatusBadge
                    label={getOpenrestyStatusLabel(node.openresty_status, t)}
                    tone={getOpenrestyStatusTone(node.openresty_status)}
                  />
                )}
              </TableCell>
              <TableCell className='py-3 text-muted-foreground'>
                {node.current_version ||
                  (node.node_type === 'tunnel_relay'
                    ? t('liveConfig')
                    : t('notApplied'))}
              </TableCell>
              <TableCell className='py-3'>
                {node.node_type === 'tunnel_relay' ? (
                  <span className='text-sm text-muted-foreground'>—</span>
                ) : (
                  <NodeStatusBadge
                    label={getApplyLabel(node.latest_apply_result, t)}
                    tone={getApplyTone(node.latest_apply_result)}
                  />
                )}
              </TableCell>
              <TableCell className='py-3 text-muted-foreground'>
                {isWSConnectedLastSeen(node.last_seen_at)
                  ? t('wsConnected')
                  : isMeaningfulTime(node.last_seen_at)
                    ? `${formatRelativeTime(node.last_seen_at, t)} · ${formatDateTime(node.last_seen_at)}`
                    : t('na')}
              </TableCell>
              <TableCell className='py-3'>
                <div className='flex flex-wrap justify-end gap-1'>
                  <Button
                    variant='outline'
                    size='sm'
                    className='h-7 text-xs'
                    asChild
                  >
                    <Link href={`/nodes/detail?id=${node.id}`}>
                      {t('detail')}
                    </Link>
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    className='h-7 text-xs'
                    onClick={() => onEdit(node)}
                  >
                    <Pencil className='size-3 mr-1' />
                    {t('edit')}
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    className='h-7 text-xs text-destructive hover:text-destructive'
                    disabled={deletingId === node.id}
                    onClick={() => onDelete(node)}
                  >
                    <Trash2 className='size-3 mr-1' />
                    {t('delete')}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
