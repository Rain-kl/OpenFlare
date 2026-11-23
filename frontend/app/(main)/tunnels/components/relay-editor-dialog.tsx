'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslations } from 'next-intl';
import { useForm } from 'react-hook-form';
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
import type {
  TunnelPoolView,
  TunnelRelayInput,
  TunnelRelayView,
} from '@/lib/services/openflare';

import { splitCommaOrLineList } from './tunnel-utils';

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
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  relay: TunnelRelayView | null;
  pools: TunnelPoolView[];
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: TunnelRelayInput) => Promise<void>;
}) {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
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

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {relay ? t('relays.editTitle') : t('relays.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('relays.editorDesc')}</DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(event) => void handleSubmit(event)}
          className='space-y-4'
        >
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
      </DialogContent>
    </Dialog>
  );
}
