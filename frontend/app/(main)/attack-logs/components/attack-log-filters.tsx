import { useQuery } from '@tanstack/react-query';
import { RefreshCw, SlidersHorizontal } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
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
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [dateError, setDateError] = useState(false);
  const advancedCount =
    Number(Boolean(filter.node_id)) +
    Number(filter.since !== undefined || filter.until !== undefined);

  return (
    <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen} asChild>
      <form
        className='flex flex-col gap-3'
        onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          const since = unixDateInput(data.get('since'));
          const until = unixDateInput(data.get('until'));
          const invalid =
            since !== undefined && until !== undefined && since >= until;
          setDateError(invalid);
          if (invalid) {
            setAdvancedOpen(true);
            return;
          }
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
        <div className='flex flex-wrap items-center gap-2'>
          <FieldGroup className='grid min-w-0 grid-cols-2 gap-2 md:w-auto md:flex-1 md:basis-64'>
            <Field className='min-w-0'>
              <Input
                aria-label={t('sourceIP')}
                name='remote_addr'
                placeholder={t('sourceIP')}
                defaultValue={filter.remote_addr}
                maxLength={128}
              />
            </Field>
            <Field className='min-w-0'>
              <Input
                aria-label={t('host')}
                name='host'
                placeholder={t('host')}
                defaultValue={filter.host}
                maxLength={255}
              />
            </Field>
          </FieldGroup>
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
            <SelectTrigger aria-label={t('action')} size='sm'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value='all'>{t('allActions')}</SelectItem>
                <SelectItem value='blocked'>{t('blocked')}</SelectItem>
                <SelectItem value='observed'>{t('observed')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <CollapsibleTrigger asChild>
            <Button type='button' variant='outline' size='sm'>
              <SlidersHorizontal data-icon='inline-start' />
              <span>{t('moreFilters')}</span>
              {advancedCount > 0 && (
                <Badge variant='secondary'>{advancedCount}</Badge>
              )}
            </Button>
          </CollapsibleTrigger>
          <div className='ml-auto flex items-center gap-2'>
            <Button type='submit' size='sm' disabled={fetching}>
              {t('apply')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              aria-label={t('refresh')}
              title={t('refresh')}
              onClick={onRefresh}
              disabled={fetching}
            >
              <RefreshCw />
            </Button>
          </div>
        </div>

        {/* Date inputs stay in FormData when the advanced fields are collapsed. */}
        <CollapsibleContent forceMount hidden={!advancedOpen}>
          <FieldGroup className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
            <Field className='gap-1.5'>
              <FieldLabel htmlFor='attack-log-node'>{t('edgeNode')}</FieldLabel>
              <Select value={node} onValueChange={setNode}>
                <SelectTrigger id='attack-log-node' size='sm'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='all'>{t('all')}</SelectItem>
                    {nodes.data?.map(({ node_id, name }) => (
                      <SelectItem key={node_id} value={node_id}>
                        {name || node_id}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field className='min-w-0 gap-1.5' data-invalid={dateError}>
              <FieldLabel htmlFor='attack-log-since'>{t('since')}</FieldLabel>
              <Input
                id='attack-log-since'
                name='since'
                type='datetime-local'
                step='1'
                defaultValue={dateInput(filter.since)}
                aria-invalid={dateError}
                aria-describedby={
                  dateError ? 'attack-log-date-error' : undefined
                }
              />
            </Field>
            <Field className='min-w-0 gap-1.5' data-invalid={dateError}>
              <FieldLabel htmlFor='attack-log-until'>{t('until')}</FieldLabel>
              <Input
                id='attack-log-until'
                name='until'
                type='datetime-local'
                step='1'
                defaultValue={dateInput(filter.until)}
                aria-invalid={dateError}
                aria-describedby={
                  dateError ? 'attack-log-date-error' : undefined
                }
              />
            </Field>
            <Field className='gap-1.5'>
              <FieldLabel htmlFor='attack-log-interval'>
                {t('autoRefresh')}
              </FieldLabel>
              <Select
                value={String(interval)}
                onValueChange={(value) => setInterval(Number(value))}
              >
                <SelectTrigger id='attack-log-interval' size='sm'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='0'>{t('refreshOff')}</SelectItem>
                    <SelectItem value='15000'>
                      {t('seconds', { count: 15 })}
                    </SelectItem>
                    <SelectItem value='60000'>
                      {t('seconds', { count: 60 })}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            {dateError && (
              <p
                id='attack-log-date-error'
                role='alert'
                className='text-sm text-destructive sm:col-span-2 xl:col-span-4'
              >
                {t('invalidDates')}
              </p>
            )}
          </FieldGroup>
        </CollapsibleContent>
      </form>
    </Collapsible>
  );
}
