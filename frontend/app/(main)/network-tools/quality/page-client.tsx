'use client';

import { useState } from 'react';
import { Crosshair, Database, Gauge, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';

import { DatasetsTab } from './components/datasets-tab';
import { ProbeTargetsTab } from './components/probe-targets-tab';
import { QualityMatrixTab } from './components/quality-matrix-tab';

export function NetworkToolsQualityPageClient() {
  const t = useTranslations('networkTools');

  const [tab, setTab] = useState<'matrix' | 'targets' | 'datasets'>('matrix');
  const [refreshNonce, setRefreshNonce] = useState(0);

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Gauge className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('quality.title')}
          </h1>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => setRefreshNonce((n) => n + 1)}
        >
          <RefreshCw className='mr-1.5 size-3.5' />
          {t('common.refresh')}
        </Button>
      </div>

      <p className='text-sm text-muted-foreground'>
        {t('quality.description')}
      </p>

      <Tabs
        value={tab}
        onValueChange={(v) => setTab(v as 'matrix' | 'targets' | 'datasets')}
        className='w-full gap-0'
      >
        <TabsList
          variant='line'
          className='h-auto w-full justify-start gap-6 bg-transparent p-0'
        >
          <TabsTrigger
            value='matrix'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            {t('quality.tabs.matrix')}
          </TabsTrigger>
          <TabsTrigger
            value='targets'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            <Crosshair className='mr-1.5 size-4' />
            {t('quality.tabs.targets')}
          </TabsTrigger>
          <TabsTrigger
            value='datasets'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            <Database className='mr-1.5 size-4' />
            {t('quality.tabs.datasets')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='matrix' className='mt-6 outline-none'>
          <QualityMatrixTab refreshNonce={refreshNonce} />
        </TabsContent>

        <TabsContent value='targets' className='mt-6 outline-none'>
          <ProbeTargetsTab refreshNonce={refreshNonce} />
        </TabsContent>

        <TabsContent value='datasets' className='mt-6 outline-none'>
          <DatasetsTab refreshNonce={refreshNonce} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
