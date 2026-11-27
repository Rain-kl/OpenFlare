'use client';

import { Copy } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  buildSmartDNSDockerCommand,
  buildSmartDNSInstallCommand,
  type SmartDNSCommandOptions,
  type SmartDNSReleaseChannel,
} from './install-command';

type DeploymentOptions = Omit<
  SmartDNSCommandOptions,
  'controllerUrl' | 'channel'
> & {
  tokenLabel: string;
};

export function SmartDNSDeploymentCommands({
  tokenLabel,
  ...credentials
}: DeploymentOptions) {
  const t = useTranslations('smartdns.instances');
  const [channel, setChannel] = useState<SmartDNSReleaseChannel>('stable');
  const [showToken, setShowToken] = useState(false);
  const origin = typeof window === 'undefined' ? '' : window.location.origin;
  const options = { ...credentials, controllerUrl: origin, channel };
  const methods = [
    {
      id: 'script',
      label: t('deployScriptTab'),
      command: buildSmartDNSInstallCommand(options),
    },
    {
      id: 'docker',
      label: t('deployDockerTab'),
      command: buildSmartDNSDockerCommand(options),
    },
  ];
  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(t('copied'));
    } catch {
      toast.error(t('copyFailed'));
    }
  };

  return (
    <div className='flex min-w-0 flex-col gap-5'>
      <Field>
        <FieldLabel htmlFor='sdns-deploy-token'>{tokenLabel}</FieldLabel>
        <Input
          id='sdns-deploy-token'
          readOnly
          type={showToken ? 'text' : 'password'}
          value={credentials.token}
          className='font-mono'
        />
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void copy(credentials.token)}
          >
            <Copy data-icon='inline-start' />
            {t('copyToken')}
          </Button>
          <Button
            variant='ghost'
            size='sm'
            onClick={() => setShowToken(!showToken)}
          >
            {t(showToken ? 'hideToken' : 'showToken')}
          </Button>
        </div>
      </Field>
      <div className='flex flex-col items-start gap-2'>
        <p id='dns-release-channel' className='text-sm font-medium'>
          {t('releaseChannel')}
        </p>
        <ToggleGroup
          variant='outline'
          size='sm'
          type='single'
          value={channel}
          aria-labelledby='dns-release-channel'
          onValueChange={(value) => {
            if (value === 'stable' || value === 'canary') setChannel(value);
          }}
        >
          <ToggleGroupItem value='stable'>{t('channelStable')}</ToggleGroupItem>
          <ToggleGroupItem value='canary'>{t('channelCanary')}</ToggleGroupItem>
        </ToggleGroup>
      </div>
      <Tabs defaultValue='script'>
        <TabsList className='max-w-full' aria-label={t('deploymentMethod')}>
          {methods.map((method) => (
            <TabsTrigger key={method.id} value={method.id}>
              {method.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {methods.map((method) => (
          <TabsContent
            key={method.id}
            value={method.id}
            className='flex min-w-0 flex-col items-start gap-3'
          >
            <pre className='w-full rounded-lg border border-dashed p-3 font-mono text-xs whitespace-pre-wrap wrap-anywhere'>
              {showToken
                ? method.command
                : method.command.replace(credentials.token, '<TOKEN>')}
            </pre>
            <Button size='sm' onClick={() => void copy(method.command)}>
              <Copy data-icon='inline-start' />
              {t('copyCommand')}
            </Button>
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}
