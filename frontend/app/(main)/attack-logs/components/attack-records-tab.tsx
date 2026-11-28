import { useQuery } from '@tanstack/react-query';
import { Eye } from 'lucide-react';
import { useFormatter, useTranslations } from 'next-intl';
import { useState } from 'react';

import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
import { Badge } from '@/components/ui/badge';
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
  AttackLog,
  AttackLogFilter,
} from '@/lib/services/openflare/attack-types';

import { AttackLogDetails, useAttackReason } from './attack-log-details';
import {
  AttackLogFilters,
  useAttackLogFilters,
  useAttackLogNodes,
} from './attack-log-filters';
import { AttackLogPanel } from './attack-log-panel';

export function AttackRecordsTab({
  initialFilter,
}: {
  initialFilter?: AttackLogFilter;
}) {
  const t = useTranslations('attackLogs');
  const format = useFormatter();
  const reason = useAttackReason();
  const [selected, setSelected] = useState<AttackLog>();
  const nodes = useAttackLogNodes();
  const nodeNames = new Map(
    nodes.data?.map((node) => [node.node_id, node.name]),
  );
  const controls = useAttackLogFilters(initialFilter ?? { action: 'all' });
  const { filter, interval, goToPage } = controls;
  const query = useQuery({
    queryKey: ['openflare', 'attack-logs', filter],
    queryFn: () => WafService.listAttackLogs(filter),
    refetchInterval: interval || false,
  });
  const refresh = () => void query.refetch();
  return (
    <>
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
              <TableHead className='pl-5'>{t('action')}</TableHead>
              <TableHead>{t('request')}</TableHead>
              <TableHead>{t('reason')}</TableHead>
              <TableHead>{t('group')}</TableHead>
              <TableHead>{t('sourceIP')}</TableHead>
              <TableHead>{t('edgeNode')}</TableHead>
              <TableHead>{t('time')}</TableHead>
              <TableHead className='pr-5 text-center'>{t('details')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {query.data?.results.map((log) => (
              <TableRow key={log.id}>
                <TableCell className='pl-5'>
                  <Badge
                    variant={
                      log.action === 'blocked' ? 'destructive' : 'secondary'
                    }
                  >
                    {t(log.action)}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div
                    className='max-w-80 truncate font-mono'
                    title={`${log.host}${log.path}`}
                  >
                    {log.host}
                    {log.path}
                  </div>
                </TableCell>
                <TableCell>{reason(log)}</TableCell>
                <TableCell>{log.attack_group_name || log.flow_name}</TableCell>
                <TableCell>
                  <div className='font-mono'>{log.remote_addr}</div>
                  {log.region && (
                    <div className='text-xs text-muted-foreground'>
                      {log.region}
                    </div>
                  )}
                </TableCell>
                <TableCell>
                  {nodeNames.get(log.node_id) || log.node_id}
                </TableCell>
                <TableCell>
                  {format.dateTime(new Date(log.logged_at), {
                    dateStyle: 'medium',
                    timeStyle: 'medium',
                  })}
                </TableCell>
                <TableCell className='pr-5 text-center'>
                  <ListActions>
                    <ListActionButton
                      icon={Eye}
                      label={t('details')}
                      onClick={() => setSelected(log)}
                    />
                  </ListActions>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </AttackLogPanel>
      {selected && (
        <AttackLogDetails
          log={selected}
          nodeName={nodeNames.get(selected.node_id)}
          onClose={() => setSelected(undefined)}
        />
      )}
    </>
  );
}
