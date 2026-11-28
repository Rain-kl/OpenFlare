'use client';

import Link from 'next/link';
import {
  Eye,
  Pause,
  Pencil,
  Play,
  Server,
  Terminal,
  Trash2,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { TunnelRelayView } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { TunnelStatusBadge } from './tunnel-status-badge';
import { getTunnelStatusTone } from './tunnel-utils';

export function RelaysTab({
  relays,
  onDeploy,
  onToggleDrain,
  onEdit,
  onDelete,
}: {
  relays: TunnelRelayView[];
  onDeploy: (relay: TunnelRelayView) => void;
  onToggleDrain: (relay: TunnelRelayView) => void;
  onEdit: (relay: TunnelRelayView) => void;
  onDelete: (relay: TunnelRelayView) => void;
}) {
  const t = useTranslations('tunnels');

  return (
    <div className='space-y-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('relays.listTitle')}
          </CardTitle>
          <CardDescription>{t('relays.listDesc')}</CardDescription>
        </CardHeader>
        <CardContent>
          {relays.length === 0 ? (
            <EmptyStateWithBorder
              icon={Server}
              description={t('relays.empty')}
            />
          ) : (
            <div className='border border-dashed rounded-lg overflow-hidden'>
              <Table>
                <TableHeader>
                  <TableRow className='border-dashed hover:bg-transparent'>
                    <TableHead className='h-8 py-2'>
                      {t('relays.name')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('relays.status')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('relays.endpoint')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('relays.zoneAndTransports')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('relays.load')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('relays.lastSeen')}
                    </TableHead>
                    <TableHead className='h-8 py-2 text-center'>
                      {t('common.actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {relays.map((relay) => (
                    <TableRow
                      key={relay.id}
                      className='border-dashed align-top'
                    >
                      <TableCell className='py-3'>
                        <div className='space-y-1'>
                          <div className='flex items-center gap-2'>
                            <Link
                              href={`/tunnels/relays/detail?id=${relay.id}`}
                              className='font-medium hover:underline'
                            >
                              {relay.name}
                            </Link>
                            {relay.pool_key ? (
                              <span className='text-xs text-muted-foreground'>
                                [{relay.pool_key}]
                              </span>
                            ) : null}
                          </div>
                          <p className='text-xs font-mono text-muted-foreground'>
                            {relay.relay_id} · {relay.version || 'dev'}
                          </p>
                        </div>
                      </TableCell>
                      <TableCell className='py-3'>
                        <TunnelStatusBadge
                          label={t(`status.${relay.effective_status}`)}
                          tone={getTunnelStatusTone(relay.effective_status)}
                        />
                      </TableCell>
                      <TableCell className='py-3 text-xs font-mono text-muted-foreground'>
                        <div>{relay.public_addr}</div>
                        <div>
                          T:{relay.transport_port} · WS:{relay.ws_port} · GW:
                          {relay.gateway_port}
                        </div>
                      </TableCell>
                      <TableCell className='py-3 text-xs text-muted-foreground'>
                        <div>
                          {(relay.supported_transports ?? []).join(', ')}
                        </div>
                        <div>
                          {relay.zone_tag || '—'} · w={relay.weight}
                        </div>
                      </TableCell>
                      <TableCell className='py-3 text-xs text-muted-foreground'>
                        {t('relays.loadLine', {
                          connectors: relay.active_connectors,
                          streams: relay.active_streams,
                        })}
                      </TableCell>
                      <TableCell className='py-3 text-xs text-muted-foreground'>
                        {relay.last_seen_at
                          ? formatDateTime(relay.last_seen_at)
                          : t('common.never')}
                      </TableCell>
                      <TableCell className='py-2 text-center'>
                        <ListActions>
                          <ListActionButton
                            icon={Eye}
                            label={t('common.details')}
                            href={`/tunnels/relays/detail?id=${relay.id}`}
                          />
                          <ListActionButton
                            icon={Terminal}
                            label={t('common.deploy')}
                            onClick={() => onDeploy(relay)}
                          />
                          <ListActionButton
                            icon={relay.draining ? Play : Pause}
                            label={
                              relay.draining
                                ? t('relays.resume')
                                : t('relays.drain')
                            }
                            onClick={() => onToggleDrain(relay)}
                          />
                          <ListActionButton
                            icon={Pencil}
                            label={t('common.edit')}
                            onClick={() => onEdit(relay)}
                          />
                          <ListActionButton
                            icon={Trash2}
                            label={t('common.delete')}
                            tone='destructive'
                            onClick={() => onDelete(relay)}
                          />
                        </ListActions>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
