'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { NetToolsASNeighbor } from '@/lib/services/openflare';

import { formatNetToolsMs } from '../../shared/utils';

const PAGE_SIZE_OPTIONS = [10, 20, 50] as const;

export function NeighborsCard({
  neighbors,
  onSelectAsn,
}: {
  neighbors: NetToolsASNeighbor[];
  onSelectAsn: (asn: number) => void;
}) {
  const t = useTranslations('networkTools');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(PAGE_SIZE_OPTIONS[0]);

  const total = neighbors.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(page, totalPages);
  const startIndex = (currentPage - 1) * pageSize;
  const pagedNeighbors = neighbors.slice(startIndex, startIndex + pageSize);

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between gap-2 pb-3'>
        <CardTitle className='text-base font-medium'>
          {t('asn.neighborsCard')}
        </CardTitle>
        <Select
          value={String(pageSize)}
          onValueChange={(value) => {
            const next = Number(value);
            if (Number.isFinite(next) && next > 0) {
              setPageSize(next);
              setPage(1);
            }
          }}
        >
          <SelectTrigger className='w-32'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PAGE_SIZE_OPTIONS.map((count) => (
              <SelectItem key={count} value={String(count)}>
                {t('common.pageSizeOption', { count })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('asn.columns.peerAsn')}</TableHead>
              <TableHead>{t('asn.columns.peerName')}</TableHead>
              <TableHead>{t('asn.columns.relation')}</TableHead>
              <TableHead>{t('asn.columns.observedRtt')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pagedNeighbors.map((nb) => (
              <TableRow key={`${nb.peer_asn}-${nb.relation}`}>
                <TableCell className='font-mono text-xs'>
                  <button
                    type='button'
                    onClick={() => onSelectAsn(nb.peer_asn)}
                    className='font-mono text-xs text-primary underline-offset-4 hover:underline'
                  >
                    {`AS${nb.peer_asn}`}
                  </button>
                </TableCell>
                <TableCell className='text-xs'>
                  <div className='font-medium'>{nb.peer_name || '—'}</div>
                  <div className='text-muted-foreground'>
                    {[nb.peer_country, nb.peer_class]
                      .filter(Boolean)
                      .join(' · ')}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant='outline'>
                    {nb.relation === 'provider' ||
                    nb.relation === 'customer' ||
                    nb.relation === 'peer' ||
                    nb.relation === 'observed'
                      ? t(`asn.relations.${nb.relation}`)
                      : nb.relation}
                  </Badge>
                </TableCell>
                <TableCell className='font-mono text-xs'>
                  {nb.observed_rtt_ms
                    ? `${formatNetToolsMs(nb.observed_rtt_ms)} ms (${nb.sample_count ?? 0})`
                    : '—'}
                </TableCell>
              </TableRow>
            ))}
            {pagedNeighbors.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={4}
                  className='py-6 text-center text-muted-foreground'
                >
                  {t('common.noData')}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        </Table>

        <div className='mt-4 flex items-center justify-between text-xs text-muted-foreground'>
          <span>
            {t('common.pageSummary', {
              page: currentPage,
              totalPages,
              total,
            })}
          </span>
          <div className='flex gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={currentPage <= 1}
              onClick={() =>
                setPage((p) => Math.max(1, Math.min(p, totalPages) - 1))
              }
            >
              {t('common.prevPage')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={currentPage >= totalPages}
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            >
              {t('common.nextPage')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
