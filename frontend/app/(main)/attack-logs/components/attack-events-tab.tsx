import { useQuery } from '@tanstack/react-query';
import { Eye } from 'lucide-react';
import { useFormatter, useTranslations } from 'next-intl';

import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { WafService } from '@/lib/services/openflare';
import type {
  AttackEvent,
  AttackLogFilter,
} from '@/lib/services/openflare/attack-types';

import { AttackLogFilters, useAttackLogFilters } from './attack-log-filters';
import { AttackLogPanel } from './attack-log-panel';

export function AttackEventsTab({
  onOpen,
}: {
  onOpen: (event: AttackEvent, filter: AttackLogFilter) => void;
}) {
  const t = useTranslations('attackLogs');
  const format = useFormatter();
  const controls = useAttackLogFilters({ action: 'blocked' });
  const { filter, interval, goToPage } = controls;
  const query = useQuery({
    queryKey: ['openflare', 'attack-events', filter],
    queryFn: () => WafService.listAttackEvents(filter),
    refetchInterval: interval || false,
  });
  const refresh = () => void query.refetch();
  return (
    <AttackLogPanel
      filters={
        <AttackLogFilters
          {...controls}
          fetching={query.isFetching}
          onRefresh={refresh}
        />
      }
      loading={query.isPending}
      failed={query.isError}
      refresh={refresh}
      total={query.data?.total ?? 0}
      page={filter.page}
      onPage={goToPage}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='pl-5'>{t('sourceIP')}</TableHead>
            <TableHead>{t('host')}</TableHead>
            <TableHead>{t('count')}</TableHead>
            <TableHead>{t('duration')}</TableHead>
            <TableHead>{t('firstSeen')}</TableHead>
            <TableHead className='pr-5 text-center'>{t('records')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {query.data?.results.map((event) => (
            <TableRow
              key={JSON.stringify([
                event.remote_addr,
                event.host,
                event.bucket_unix,
              ])}
            >
              <TableCell className='pl-5 font-mono'>
                {event.remote_addr}
              </TableCell>
              <TableCell>{event.host}</TableCell>
              <TableCell className='tabular-nums'>
                {format.number(event.count)}
              </TableCell>
              <TableCell>
                {t('seconds', { count: event.last_unix - event.first_unix })}
              </TableCell>
              <TableCell>
                {format.dateTime(new Date(event.first_unix * 1000), {
                  dateStyle: 'medium',
                  timeStyle: 'medium',
                })}
              </TableCell>
              <TableCell className='pr-5 text-center'>
                <ListActions>
                  <ListActionButton
                    icon={Eye}
                    label={t('viewRecords')}
                    onClick={() => onOpen(event, filter)}
                  />
                </ListActions>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </AttackLogPanel>
  );
}
