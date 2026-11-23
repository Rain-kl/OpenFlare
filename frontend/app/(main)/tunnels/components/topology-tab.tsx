'use client';

import { Activity } from 'lucide-react';
import { useTranslations } from 'next-intl';

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
import type { TunnelTopologyView } from '@/lib/services/openflare';

import { TunnelStatusBadge } from './tunnel-status-badge';
import { getTunnelStatusTone } from './tunnel-utils';

export function TopologyTab({ topology }: { topology?: TunnelTopologyView }) {
  const t = useTranslations('tunnels');
  const attachments = topology?.attachments ?? [];
  const connectors = topology?.connectors ?? [];
  const relays = topology?.relays ?? [];
  const tunnels = topology?.tunnels ?? [];
  const pools = topology?.pools ?? [];

  const stats = [
    { label: t('topology.pools'), value: pools.length },
    { label: t('topology.relays'), value: relays.length },
    { label: t('topology.tunnels'), value: tunnels.length },
    { label: t('topology.connectors'), value: connectors.length },
    { label: t('topology.activeLeases'), value: attachments.length },
  ];

  return (
    <div className='space-y-6'>
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-5'>
        {stats.map((item) => (
          <Card key={item.label} className='border-dashed shadow-none'>
            <CardHeader className='pb-2'>
              <CardDescription>{item.label}</CardDescription>
              <CardTitle className='text-2xl font-semibold tabular-nums'>
                {item.value}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('topology.attachmentsTitle')}
          </CardTitle>
          <CardDescription>{t('topology.attachmentsDesc')}</CardDescription>
        </CardHeader>
        <CardContent>
          {attachments.length === 0 ? (
            <EmptyStateWithBorder
              icon={Activity}
              description={t('topology.emptyAttachments')}
            />
          ) : (
            <div className='border border-dashed rounded-lg overflow-hidden'>
              <Table>
                <TableHeader>
                  <TableRow className='border-dashed hover:bg-transparent'>
                    <TableHead className='h-8 py-2'>
                      {t('topology.connector')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('topology.relay')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('topology.service')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('topology.transport')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('topology.state')}
                    </TableHead>
                    <TableHead className='h-8 py-2'>
                      {t('topology.rttStreams')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {attachments.map((att) => (
                    <TableRow key={att.id} className='border-dashed'>
                      <TableCell className='py-2.5 font-mono text-xs'>
                        {att.connector_id}
                      </TableCell>
                      <TableCell className='py-2.5 text-sm'>
                        {att.relay_name ||
                          att.relay_public_id ||
                          `#${att.relay_id}`}
                      </TableCell>
                      <TableCell className='py-2.5 font-mono text-xs text-muted-foreground'>
                        {att.service_name}
                      </TableCell>
                      <TableCell className='py-2.5 text-xs uppercase'>
                        {att.transport}
                      </TableCell>
                      <TableCell className='py-2.5'>
                        <TunnelStatusBadge
                          label={t(`status.${att.state}`)}
                          tone={getTunnelStatusTone(att.state)}
                        />
                      </TableCell>
                      <TableCell className='py-2.5 text-xs text-muted-foreground tabular-nums'>
                        {att.rtt_ms} ms · {att.active_streams}
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
