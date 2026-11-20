import { useQuery } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { NodeService } from '@/lib/services/openflare';
import type { AttackLogFilter } from '@/lib/services/openflare/attack-types';

export const attackLogPageSize = 25;

export function useAttackLogFilters(initialFilter: AttackLogFilter) {
  const [filter, setFilter] = useState({
    ...initialFilter,
    page: 0,
    page_size: attackLogPageSize,
  });
  const [interval, setInterval] = useState(0);
  return {
    filter,
    interval,
    setInterval,
    apply: (values: AttackLogFilter) =>
      setFilter({ ...values, page: 0, page_size: attackLogPageSize }),
    goToPage: (page: number) => setFilter({ ...filter, page }),
  };
}

function dateInput(unix?: number) {
  if (unix === undefined) return '';
  const date = new Date(unix * 1000);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
    .toISOString()
    .slice(0, 19);
}

function unixDateInput(value: FormDataEntryValue | null) {
  if (typeof value !== 'string' || value === '') return undefined;
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : undefined;
}

export function useAttackLogNodes() {
  return useQuery({
    queryKey: ['openflare', 'waf', 'log-nodes'],
    queryFn: () => NodeService.listNodes(),
    staleTime: 60_000,
  });
}

export function AttackLogFilters({
  filter,
  apply,
  onRefresh,
  interval,
  setInterval,
  fetching,
}: {
  filter: AttackLogFilter;
  apply: (filter: AttackLogFilter) => void;
  onRefresh: () => void;
  interval: number;
  setInterval: (value: number) => void;
  fetching: boolean;
}) {
  const t = useTranslations('attackLogs');
  const nodes = useAttackLogNodes();
  const [action, setAction] = useState<NonNullable<AttackLogFilter['action']>>(
    filter.action ?? 'all',
  );
  const [node, setNode] = useState(filter.node_id ?? 'all');
  const [dateError, setDateError] = useState(false);
  return (
    <form
      className='space-y-3'
      onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const since = unixDateInput(data.get('since'));
        const until = unixDateInput(data.get('until'));
        const invalid =
          since !== undefined && until !== undefined && since >= until;
        setDateError(invalid);
        if (invalid) return;
        const host = String(data.get('host') ?? '').trim();
        const remoteAddr = String(data.get('remote_addr') ?? '').trim();
        apply({
          host: host || undefined,
          remote_addr: remoteAddr || undefined,
          node_id: node === 'all' ? undefined : node,
          action,
          since,
          until,
          exact:
            filter.exact &&
            host === filter.host &&
            remoteAddr === filter.remote_addr,
        });
      }}
    >
      <div className='grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-4 2xl:grid-cols-6'>
        <label className='grid gap-2 text-sm font-medium'>
          {t('sourceIP')}
          <Input
            name='remote_addr'
            defaultValue={filter.remote_addr}
            maxLength={128}
          />
        </label>
        <label className='grid gap-2 text-sm font-medium'>
          {t('host')}
          <Input name='host' defaultValue={filter.host} maxLength={255} />
        </label>
        <div className='grid gap-2'>
          <Label htmlFor='attack-log-node'>{t('edgeNode')}</Label>
          <Select value={node} onValueChange={setNode}>
            <SelectTrigger id='attack-log-node' className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>{t('all')}</SelectItem>
              {nodes.data?.map(({ node_id, name }) => (
                <SelectItem key={node_id} value={node_id}>
                  {name || node_id}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='attack-log-action'>{t('action')}</Label>
          <Select
            value={action}
            onValueChange={(value) => {
              if (
                value === 'all' ||
                value === 'blocked' ||
                value === 'observed'
              )
                setAction(value);
            }}
          >
            <SelectTrigger id='attack-log-action' className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>{t('all')}</SelectItem>
              <SelectItem value='blocked'>{t('blocked')}</SelectItem>
              <SelectItem value='observed'>{t('observed')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <label className='grid gap-2 text-sm font-medium'>
          {t('since')}
          <Input
            name='since'
            type='datetime-local'
            step='1'
            defaultValue={dateInput(filter.since)}
          />
        </label>
        <label className='grid gap-2 text-sm font-medium'>
          {t('until')}
          <Input
            name='until'
            type='datetime-local'
            step='1'
            defaultValue={dateInput(filter.until)}
          />
        </label>
      </div>
      {dateError && (
        <p role='alert' className='text-sm text-destructive'>
          {t('invalidDates')}
        </p>
      )}
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex flex-wrap items-center gap-2'>
          <Button type='submit' disabled={fetching}>
            {t('apply')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={onRefresh}
            disabled={fetching}
          >
            <RefreshCw data-icon='inline-start' />
            {t('refresh')}
          </Button>
        </div>
        <div className='flex items-center gap-2'>
          <Label htmlFor='attack-log-interval'>{t('autoRefresh')}</Label>
          <Select
            value={String(interval)}
            onValueChange={(value) => setInterval(Number(value))}
          >
            <SelectTrigger id='attack-log-interval'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='0'>{t('refreshOff')}</SelectItem>
              <SelectItem value='15000'>
                {t('seconds', { count: 15 })}
              </SelectItem>
              <SelectItem value='60000'>
                {t('seconds', { count: 60 })}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </form>
  );
}
