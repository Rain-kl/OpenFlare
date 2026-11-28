'use client';

import Link from 'next/link';
import { Eye, Pencil, Terminal, Trash2 } from 'lucide-react';
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
import type { TunnelView } from '@/lib/services/openflare';
import { ShieldAlert, Network } from 'lucide-react';

import { TunnelStatusBadge } from './tunnel-status-badge';
import { getTunnelStatusTone } from './tunnel-utils';

export function TunnelsTab({
  tunnels,
  onDeploy,
  onEdit,
  onDelete,
}: {
  tunnels: TunnelView[];
  onDeploy: (tunnel: TunnelView) => void;
  onEdit: (tunnel: TunnelView) => void;
  onDelete: (tunnel: TunnelView) => void;
}) {
  const t = useTranslations('tunnels');

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='pb-3'>
        <CardTitle className='text-base font-semibold'>
          {t('tunnels.listTitle')}
        </CardTitle>
        <CardDescription>{t('tunnels.listDesc')}</CardDescription>
      </CardHeader>
      <CardContent>
        {tunnels.length === 0 ? (
          <EmptyStateWithBorder
            icon={Network}
            description={t('tunnels.empty')}
          />
        ) : (
          <div className='border border-dashed rounded-lg overflow-hidden'>
            <Table>
              <TableHeader>
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 py-2'>
                    {t('tunnels.name')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('tunnels.status')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('tunnels.pool')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('tunnels.haAndTransport')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('tunnels.topologyMetrics')}
                  </TableHead>
                  <TableHead className='h-8 py-2 text-center'>
                    {t('common.actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tunnels.map((item) => (
                  <TableRow key={item.id} className='border-dashed align-top'>
                    <TableCell className='py-3'>
                      <div className='space-y-1'>
                        <div className='flex items-center gap-2'>
                          <Link
                            href={`/tunnels/detail?id=${item.id}`}
                            className='font-medium hover:underline'
                          >
                            {item.name}
                          </Link>
                          {!item.enabled ? (
                            <TunnelStatusBadge
                              label={t('status.disabled')}
                              tone='warning'
                            />
                          ) : null}
                          {item.quarantined_at ? (
                            <TunnelStatusBadge
                              label={t('status.quarantined')}
                              tone='danger'
                            />
                          ) : null}
                        </div>
                        <p className='text-xs font-mono text-muted-foreground'>
                          {item.tunnel_id} · rev {item.config_revision}
                        </p>
                      </div>
                    </TableCell>
                    <TableCell className='py-3'>
                      <TunnelStatusBadge
                        label={t(`status.${item.status}`)}
                        tone={getTunnelStatusTone(item.status)}
                      />
                    </TableCell>
                    <TableCell className='py-3 text-sm'>
                      {item.pool_key || t('pools.defaultPool')}
                    </TableCell>
                    <TableCell className='py-3 text-xs text-muted-foreground'>
                      <div>
                        K={item.ha_relay_count} · {item.preferred_transport}
                      </div>
                      {item.client_proxy_url ? (
                        <div className='font-mono mt-0.5'>
                          {item.client_proxy_url}
                        </div>
                      ) : null}
                    </TableCell>
                    <TableCell className='py-3 text-xs text-muted-foreground'>
                      <div>
                        {t('tunnels.metricsLine', {
                          connectors: item.online_connectors,
                          attachments: item.active_attachments,
                          routes: item.bound_routes_count,
                        })}
                      </div>
                    </TableCell>
                    <TableCell className='py-2 text-center'>
                      <ListActions>
                        <ListActionButton
                          icon={Eye}
                          label={t('common.details')}
                          href={`/tunnels/detail?id=${item.id}`}
                        />
                        <ListActionButton
                          icon={Terminal}
                          label={t('common.deploy')}
                          onClick={() => onDeploy(item)}
                        />
                        <ListActionButton
                          icon={Pencil}
                          label={t('common.edit')}
                          onClick={() => onEdit(item)}
                        />
                        <ListActionButton
                          icon={Trash2}
                          label={t('common.delete')}
                          tone='destructive'
                          onClick={() => onDelete(item)}
                        />
                      </ListActions>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
        <div className='mt-3 flex items-center gap-2 text-xs text-muted-foreground'>
          <ShieldAlert className='size-3.5 text-primary' />
          <span>{t('tunnels.securityNote')}</span>
        </div>
      </CardContent>
    </Card>
  );
}
