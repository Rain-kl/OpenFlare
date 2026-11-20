'use client';

import { ScrollText } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type {
  AttackEvent,
  AttackLogFilter,
} from '@/lib/services/openflare/attack-types';

import { AttackEventsTab } from './components/attack-events-tab';
import { AttackRecordsTab } from './components/attack-records-tab';

type Tab = 'events' | 'records';

export default function AttackLogsPage() {
  const t = useTranslations('attackLogs');
  const [tab, setTab] = useState<Tab>('events');
  const [eventFilter, setEventFilter] = useState<AttackLogFilter>();

  function openEvent(event: AttackEvent, filter: AttackLogFilter) {
    setEventFilter({
      ...filter,
      page: 0,
      exact: true,
      host: event.host,
      remote_addr: event.remote_addr,
      since: Math.max(event.first_unix, filter.since ?? 0),
      until: Math.min(
        event.last_unix + 1,
        filter.until ?? Number.POSITIVE_INFINITY,
      ),
    });
    setTab('records');
  }

  return (
    <div className='w-full space-y-5 px-1 py-6'>
      <div className='flex items-center gap-2'>
        <ScrollText className='size-5 text-primary' />
        <h1 className='text-2xl font-semibold tracking-tight'>{t('title')}</h1>
      </div>
      <Tabs
        value={tab}
        onValueChange={(value) => {
          if (value === 'events' || value === 'records') {
            setEventFilter(undefined);
            setTab(value);
          }
        }}
      >
        <TabsList>
          <TabsTrigger value='events'>{t('events')}</TabsTrigger>
          <TabsTrigger value='records'>{t('records')}</TabsTrigger>
        </TabsList>
        <TabsContent value='events'>
          <AttackEventsTab onOpen={openEvent} />
        </TabsContent>
        <TabsContent value='records'>
          <AttackRecordsTab initialFilter={eventFilter} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
