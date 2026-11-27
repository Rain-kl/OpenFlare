'use client';

import { Globe2, Play } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useDnsTab } from '../components/use-dns-tab';
import { SmartDNSSettingsTab } from './settings-tab';
import { SmartDNSInstancesTab } from './instances-tab';
import { SmartDNSOverviewTab } from './overview-tab';
import { SmartDNSPoliciesTab } from './policies-tab';
import { SmartDNSSimulatorDrawer } from './simulator-drawer';

const tabs = ['overview', 'instances', 'policies', 'delegation'] as const;

export default function SmartDNSPage() {
  const t = useTranslations('smartdns');
  const { activeTab, setActiveTab } = useDnsTab(tabs, 'overview');
  const [simulatorOpen, setSimulatorOpen] = useState(false);
  const [simulatorPolicyId, setSimulatorPolicyId] = useState<
    number | undefined
  >(undefined);

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Globe2 className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <Button
          variant='outline'
          size='sm'
          onClick={() => {
            setSimulatorPolicyId(undefined);
            setSimulatorOpen(true);
          }}
        >
          <Play data-icon='inline-start' />
          {t('openSimulator')}
        </Button>
      </div>

      <Tabs
        value={activeTab}
        onValueChange={setActiveTab}
        activationMode='manual'
      >
        <div className='overflow-x-auto border-b border-dashed pb-1'>
          <TabsList variant='line'>
            {tabs.map((tab) => (
              <TabsTrigger key={tab} value={tab}>
                {t(`tabs.${tab}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>
        <TabsContent value='overview' className='pt-4'>
          <SmartDNSOverviewTab
            onOpenInstances={() => setActiveTab('instances')}
          />
        </TabsContent>
        <TabsContent value='delegation' className='pt-4'>
          <SmartDNSSettingsTab />
        </TabsContent>
        <TabsContent value='instances' className='pt-4'>
          <SmartDNSInstancesTab />
        </TabsContent>
        <TabsContent value='policies' className='pt-4'>
          <SmartDNSPoliciesTab
            onSimulatePolicy={(policyId) => {
              setSimulatorPolicyId(policyId);
              setSimulatorOpen(true);
            }}
          />
        </TabsContent>
      </Tabs>

      <SmartDNSSimulatorDrawer
        open={simulatorOpen}
        onOpenChange={setSimulatorOpen}
        initialPolicyId={simulatorPolicyId}
      />
    </div>
  );
}
