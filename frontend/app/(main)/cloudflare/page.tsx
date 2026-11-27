'use client';

import { Cloud, Settings } from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useDnsTab } from '../dns/components/use-dns-tab';
import { SyncTasksPanel } from './components/sync-tasks-panel';
import { CloudflareGroupsTab } from './groups-tab';

const tabs = ['groups', 'sync-history'] as const;

export default function CloudflarePage() {
  const t = useTranslations('cloudflare');
  const { activeTab, setActiveTab } = useDnsTab(tabs, 'groups');

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <Button variant='outline' size='sm' asChild>
          <Link href='/dns/settings'>
            <Settings data-icon='inline-start' />
            {t('connectionSettings')}
          </Link>
        </Button>
      </div>
      <Tabs
        value={activeTab}
        onValueChange={setActiveTab}
        activationMode='manual'
      >
        <div className='overflow-x-auto border-b border-dashed pb-1'>
          <TabsList variant='line'>
            <TabsTrigger value='groups'>{t('groupsTitle')}</TabsTrigger>
            <TabsTrigger value='sync-history'>{t('tasks.title')}</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value='groups' className='pt-4'>
          <CloudflareGroupsTab
            onShowHistory={() => setActiveTab('sync-history')}
          />
        </TabsContent>
        <TabsContent value='sync-history' className='pt-4'>
          <SyncTasksPanel />
        </TabsContent>
      </Tabs>
    </div>
  );
}
