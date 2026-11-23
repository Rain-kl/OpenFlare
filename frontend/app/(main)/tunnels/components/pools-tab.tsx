'use client';

import { Layers, Pencil, Plus, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import type { TunnelPoolView } from '@/lib/services/openflare';

export function PoolsTab({
  pools,
  onCreate,
  onEdit,
  onDelete,
}: {
  pools: TunnelPoolView[];
  onCreate: () => void;
  onEdit: (pool: TunnelPoolView) => void;
  onDelete: (pool: TunnelPoolView) => void;
}) {
  const t = useTranslations('tunnels');

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='pb-3'>
        <div className='flex items-center justify-between gap-2'>
          <div>
            <CardTitle className='text-base font-semibold'>
              {t('pools.listTitle')}
            </CardTitle>
            <CardDescription>{t('pools.listDesc')}</CardDescription>
          </div>
          <Button
            type='button'
            variant='secondary'
            size='sm'
            className='h-7 text-xs'
            onClick={onCreate}
          >
            <Plus className='size-3.5 mr-1' />
            {t('pools.create')}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {pools.length === 0 ? (
          <EmptyStateWithBorder icon={Layers} description={t('pools.empty')} />
        ) : (
          <div className='border border-dashed rounded-lg overflow-hidden'>
            <Table>
              <TableHeader>
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 py-2'>{t('pools.name')}</TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('pools.poolKey')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('pools.region')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('pools.counts')}
                  </TableHead>
                  <TableHead className='h-8 py-2 text-right'>
                    {t('common.actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pools.map((pool) => (
                  <TableRow key={pool.id} className='border-dashed'>
                    <TableCell className='py-3'>
                      <div className='flex items-center gap-2'>
                        <span className='font-medium'>{pool.name}</span>
                        {pool.is_default ? (
                          <Badge variant='secondary'>
                            {t('pools.defaultBadge')}
                          </Badge>
                        ) : null}
                      </div>
                      {pool.description ? (
                        <p className='text-xs text-muted-foreground'>
                          {pool.description}
                        </p>
                      ) : null}
                    </TableCell>
                    <TableCell className='py-3 font-mono text-xs'>
                      {pool.pool_key}
                    </TableCell>
                    <TableCell className='py-3 text-sm'>
                      {pool.region || '—'}
                    </TableCell>
                    <TableCell className='py-3 text-xs text-muted-foreground'>
                      {t('pools.countsLine', {
                        relays: pool.relay_count,
                        tunnels: pool.tunnel_count,
                      })}
                    </TableCell>
                    <TableCell className='py-3'>
                      <div className='flex justify-end gap-1'>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() => onEdit(pool)}
                        >
                          <Pencil className='size-3 mr-1' />
                          {t('common.edit')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs text-destructive hover:text-destructive'
                          disabled={pool.is_default}
                          onClick={() => onDelete(pool)}
                        >
                          <Trash2 className='size-3 mr-1' />
                          {t('common.delete')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
