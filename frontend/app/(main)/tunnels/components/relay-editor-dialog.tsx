'use client';

import { useEffect, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { Copy, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useForm } from 'react-hook-form';
import { toast } from 'sonner';
import { z } from 'zod';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type {
  TunnelPoolView,
  TunnelRelayBootstrapView,
  TunnelRelayInput,
  TunnelRelayView,
} from '@/lib/services/openflare';

import {
  buildRelayDockerCommand,
  buildRelayInstallScriptCommand,
  splitCommaOrLineList,
} from './tunnel-utils';

function buildSchema(t: (key: string) => string) {
  return z.object({
    name: z.string().trim().min(1, t('relays.errNameRequired')).max(255),
    pool_id: z.string(),
    public_addr: z.string().trim().min(1, t('relays.errPublicAddrRequired')),
    ip_manual_override: z.boolean(),
    transport_port: z.string().trim(),
    ws_port: z.string().trim(),
    gateway_addr: z.string().trim(),
    gateway_port: z.string().trim(),
    gateway_tls: z.boolean(),
    supported_transports: z.string().trim(),
    zone_tag: z.string().trim(),
    colocated_edge_node_id: z.string().trim(),
    weight: z.string().trim(),
    auto_update_enabled: z.boolean(),
  });
}

type RelayFormValues = z.infer<ReturnType<typeof buildSchema>>;

export function RelayEditorDialog({
  open,
  relay,
  pools,
  bootstrap,
  rotatingBootstrap = false,
  onRotateBootstrap,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  relay: TunnelRelayView | null;
  pools: TunnelPoolView[];
  bootstrap?: TunnelRelayBootstrapView;
  rotatingBootstrap?: boolean;
  onRotateBootstrap?: () => void;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: TunnelRelayInput) => Promise<void>;
}) {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const [createMode, setCreateMode] = useState<'manual' | 'discovery'>(
    'manual',
  );

  const handleCopy = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(t('deploy.copied', { target: label }));
    } catch {
      toast.error(t('deploy.copyFailed'));
    }
  };
  const form = useForm<RelayFormValues>({
    resolver: zodResolver(buildSchema(t)),
    defaultValues: {
      name: '',
      pool_id: pools[0]?.id ? String(pools[0].id) : '',
      public_addr: '',
      ip_manual_override: false,
      transport_port: '7443',
      ws_port: '7444',
      gateway_addr: '',
      gateway_port: '8443',
      gateway_tls: false,
      supported_transports: 'noise, websocket, tcp',
      zone_tag: '',
      colocated_edge_node_id: '',
      weight: '100',
      auto_update_enabled: false,
    },
  });

  useEffect(() => {
    if (!open) return;
    setCreateMode('manual');
    if (relay) {
      form.reset({
        name: relay.name,
        pool_id: relay.pool_id ? String(relay.pool_id) : '',
        public_addr: relay.public_addr,
        ip_manual_override: relay.ip_manual_override,
        transport_port: String(relay.transport_port || 7443),
        ws_port: String(relay.ws_port || 7444),
        gateway_addr: relay.gateway_addr,
        gateway_port: String(relay.gateway_port || 8443),
        gateway_tls: relay.gateway_tls,
        supported_transports: (relay.supported_transports ?? []).join(', '),
        zone_tag: relay.zone_tag,
        colocated_edge_node_id: relay.colocated_edge_node_id,
        weight: String(relay.weight || 100),
        auto_update_enabled: relay.auto_update_enabled,
      });
    } else {
      const defPool = pools.find((p) => p.is_default) ?? pools[0];
      form.reset({
        name: '',
        pool_id: defPool ? String(defPool.id) : '',
        public_addr: '',
        ip_manual_override: false,
        transport_port: '7443',
        ws_port: '7444',
        gateway_addr: '',
        gateway_port: '8443',
        gateway_tls: false,
        supported_transports: 'noise, websocket, tcp',
        zone_tag: '',
        colocated_edge_node_id: '',
        weight: '100',
        auto_update_enabled: false,
      });
    }
  }, [open, relay, pools, form]);

  const handleSubmit = form.handleSubmit(async (values) => {
    await onSubmit({
      name: values.name,
      pool_id: values.pool_id ? Number(values.pool_id) : undefined,
      public_addr: values.public_addr,
      ip_manual_override: values.ip_manual_override,
      transport_port: Number(values.transport_port) || 7443,
      ws_port: Number(values.ws_port) || 0,
      gateway_addr: values.gateway_addr,
      gateway_port: Number(values.gateway_port) || 8443,
      gateway_tls: values.gateway_tls,
      supported_transports: splitCommaOrLineList(values.supported_transports),
      zone_tag: values.zone_tag,
      colocated_edge_node_id: values.colocated_edge_node_id,
      weight: Number(values.weight) || 100,
      auto_update_enabled: values.auto_update_enabled,
    });
  });

  const manualForm = (
    <form onSubmit={(event) => void handleSubmit(event)} className='space-y-4'>
      <div className='grid gap-3 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor='relay-name'>{t('relays.name')}</Label>
          <Input
            id='relay-name'
            placeholder='relay-hk-01'
            {...form.register('name')}
          />
          {form.formState.errors.name ? (
            <p className='text-xs text-destructive'>
              {form.formState.errors.name.message}
            </p>
          ) : null}
        </div>

        <div className='space-y-2'>
          <Label>{t('relays.pool')}</Label>
          <Select
            value={form.watch('pool_id') || 'default'}
            onValueChange={(value) =>
              form.setValue('pool_id', value === 'default' ? '' : value)
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {pools.map((pool) => (
                <SelectItem key={pool.id} value={String(pool.id)}>
                  {pool.name} ({pool.pool_key})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor='relay-pub-addr'>{t('relays.publicAddr')}</Label>
          <Input
            id='relay-pub-addr'
            placeholder='203.0.113.10'
            {...form.register('public_addr')}
          />
          {form.formState.errors.public_addr ? (
            <p className='text-xs text-destructive'>
              {form.formState.errors.public_addr.message}
            </p>
          ) : null}
        </div>

        <div className='space-y-2'>
          <Label htmlFor='relay-gw-addr'>{t('relays.gatewayAddr')}</Label>
          <Input
            id='relay-gw-addr'
            placeholder='10.0.0.2'
            {...form.register('gateway_addr')}
          />
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-3'>
        <div className='space-y-2'>
          <Label htmlFor='relay-tport'>{t('relays.transportPort')}</Label>
          <Input id='relay-tport' {...form.register('transport_port')} />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='relay-wport'>{t('relays.wsPort')}</Label>
          <Input id='relay-wport' {...form.register('ws_port')} />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='relay-gport'>{t('relays.gatewayPort')}</Label>
          <Input id='relay-gport' {...form.register('gateway_port')} />
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-3'>
        <div className='space-y-2 sm:col-span-2'>
          <Label htmlFor='relay-zone'>{t('relays.zoneTag')}</Label>
          <Input
            id='relay-zone'
            placeholder='ap-east-1a'
            {...form.register('zone_tag')}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='relay-weight'>{t('relays.weight')}</Label>
          <Input id='relay-weight' {...form.register('weight')} />
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor='relay-transports'>
            {t('relays.supportedTransports')}
          </Label>
          <Input
            id='relay-transports'
            placeholder='noise, websocket, tcp'
            {...form.register('supported_transports')}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='relay-colocated'>
            {t('relays.colocatedEdgeNodeId')}
          </Label>
          <Input
            id='relay-colocated'
            {...form.register('colocated_edge_node_id')}
          />
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-2'>
        <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
          <p className='text-sm font-medium'>{t('relays.lockIp')}</p>
          <Switch
            checked={form.watch('ip_manual_override')}
            onCheckedChange={(checked) =>
              form.setValue('ip_manual_override', checked)
            }
          />
        </div>
        <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
          <p className='text-sm font-medium'>{t('relays.autoUpdate')}</p>
          <Switch
            checked={form.watch('auto_update_enabled')}
            onCheckedChange={(checked) =>
              form.setValue('auto_update_enabled', checked)
            }
          />
        </div>
      </div>

      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          onClick={onClose}
          disabled={submitting}
        >
          {tc('cancel')}
        </Button>
        <Button type='submit' disabled={submitting}>
          {submitting ? t('common.saving') : t('common.save')}
        </Button>
      </DialogFooter>
    </form>
  );

  const resolvedServerUrl =
    bootstrap?.server_url ||
    (typeof window !== 'undefined' ? window.location.origin : '');
  const effectiveToken =
    bootstrap?.bootstrap_token || bootstrap?.discovery_token || '';
  const scriptCmd = effectiveToken
    ? buildRelayInstallScriptCommand(resolvedServerUrl, effectiveToken)
    : '';
  const dockerCmd = effectiveToken
    ? buildRelayDockerCommand(resolvedServerUrl, effectiveToken)
    : '';

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-2xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {relay ? t('relays.editTitle') : t('relays.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('relays.editorDesc')}</DialogDescription>
        </DialogHeader>

        {relay ? (
          manualForm
        ) : (
          <Tabs
            value={createMode}
            onValueChange={(value) =>
              setCreateMode(value === 'discovery' ? 'discovery' : 'manual')
            }
            className='space-y-4'
          >
            <TabsList className='grid w-full grid-cols-2'>
              <TabsTrigger value='manual'>{t('relays.manualTab')}</TabsTrigger>
              <TabsTrigger value='discovery'>
                {t('relays.discoveryTab')}
              </TabsTrigger>
            </TabsList>

            <TabsContent value='manual'>{manualForm}</TabsContent>

            <TabsContent value='discovery' className='space-y-4'>
              {bootstrap ? (
                <div className='space-y-4'>
                  <div className='flex flex-wrap items-start justify-between gap-2'>
                    <div className='space-y-1'>
                      <p className='text-sm font-medium'>
                        {t('relays.discoveryTitle')}
                      </p>
                      <p className='text-xs text-muted-foreground'>
                        {t('relays.discoveryDesc')}
                      </p>
                    </div>
                    {onRotateBootstrap ? (
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='h-7 text-xs'
                        disabled={rotatingBootstrap}
                        onClick={onRotateBootstrap}
                      >
                        <RefreshCw className='size-3.5 mr-1' />
                        {t('relays.rotateDiscovery')}
                      </Button>
                    ) : null}
                  </div>

                  <div className='rounded-lg border border-dashed p-3 space-y-2'>
                    <div className='flex items-center justify-between gap-2'>
                      <p className='text-xs font-medium text-muted-foreground'>
                        {t('deploy.bootstrapToken')}
                      </p>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='h-7 px-2 text-xs'
                        onClick={() =>
                          void handleCopy(
                            effectiveToken,
                            t('deploy.bootstrapToken'),
                          )
                        }
                      >
                        <Copy className='size-3.5 mr-1' />
                        {t('deploy.copy')}
                      </Button>
                    </div>
                    <pre className='overflow-x-auto text-xs font-mono break-all whitespace-pre-wrap'>
                      {effectiveToken}
                    </pre>
                  </div>

                  <div className='space-y-1.5'>
                    <div className='flex items-center justify-between'>
                      <p className='text-xs font-medium'>
                        {t('deploy.scriptCommand')}
                      </p>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='h-7 px-2 text-xs'
                        onClick={() =>
                          void handleCopy(scriptCmd, t('deploy.scriptCommand'))
                        }
                      >
                        <Copy className='size-3.5 mr-1' />
                        {t('deploy.copy')}
                      </Button>
                    </div>
                    <pre className='overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs font-mono whitespace-pre-wrap break-all'>
                      {scriptCmd}
                    </pre>
                  </div>

                  <div className='space-y-1.5'>
                    <div className='flex items-center justify-between'>
                      <p className='text-xs font-medium'>
                        {t('deploy.dockerCommand')}
                      </p>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='h-7 px-2 text-xs'
                        onClick={() =>
                          void handleCopy(dockerCmd, t('deploy.dockerCommand'))
                        }
                      >
                        <Copy className='size-3.5 mr-1' />
                        {t('deploy.copy')}
                      </Button>
                    </div>
                    <pre className='overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs font-mono whitespace-pre-wrap break-all'>
                      {dockerCmd}
                    </pre>
                  </div>

                  <div className='space-y-1.5'>
                    <div className='flex items-center justify-between'>
                      <p className='text-xs font-medium'>
                        {t('deploy.cliCommand')}
                      </p>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='h-7 px-2 text-xs'
                        onClick={() =>
                          void handleCopy(
                            bootstrap.deploy_command,
                            t('deploy.cliCommand'),
                          )
                        }
                      >
                        <Copy className='size-3.5 mr-1' />
                        {t('deploy.copy')}
                      </Button>
                    </div>
                    <pre className='overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs font-mono whitespace-pre-wrap break-all'>
                      {bootstrap.deploy_command}
                    </pre>
                  </div>
                </div>
              ) : (
                <div className='rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground'>
                  {t('relays.discoveryEmpty')}
                </div>
              )}
              <DialogFooter>
                <Button type='button' variant='outline' onClick={onClose}>
                  {tc('close')}
                </Button>
              </DialogFooter>
            </TabsContent>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  );
}
