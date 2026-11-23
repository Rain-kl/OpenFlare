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
import { Textarea } from '@/components/ui/textarea';
import type {
  TunnelInput,
  TunnelPoolView,
  TunnelView,
} from '@/lib/services/openflare';

import { splitCommaOrLineList } from './tunnel-utils';

function buildSchema(t: (key: string) => string) {
  return z.object({
    name: z.string().trim().min(1, t('tunnels.errNameRequired')).max(255),
    description: z.string().trim(),
    pool_id: z.string(),
    ha_relay_count: z.string().trim(),
    preferred_transport: z.string(),
    fallback_transports: z.string().trim(),
    client_proxy_url: z.string().trim(),
    max_connectors: z.string().trim(),
    allowed_target_cidrs: z.string().trim(),
    blocked_target_cidrs: z.string().trim(),
    enabled: z.boolean(),
    auto_update_enabled: z.boolean(),
  });
}

type TunnelFormValues = z.infer<ReturnType<typeof buildSchema>>;

export function TunnelEditorDialog({
  open,
  tunnel,
  pools,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  tunnel: TunnelView | null;
  pools: TunnelPoolView[];
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: TunnelInput) => Promise<void>;
}) {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const form = useForm<TunnelFormValues>({
    resolver: zodResolver(buildSchema(t)),
    defaultValues: {
      name: '',
      description: '',
      pool_id: '',
      ha_relay_count: '2',
      preferred_transport: 'auto',
      fallback_transports: 'noise, websocket, tcp',
      client_proxy_url: '',
      max_connectors: '4',
      allowed_target_cidrs: '',
      blocked_target_cidrs: '169.254.169.254/32',
      enabled: true,
      auto_update_enabled: false,
    },
  });

  useEffect(() => {
    if (!open) return;
    if (tunnel) {
      form.reset({
        name: tunnel.name,
        description: tunnel.description,
        pool_id: tunnel.pool_id ? String(tunnel.pool_id) : '',
        ha_relay_count: String(tunnel.ha_relay_count || 2),
        preferred_transport: tunnel.preferred_transport || 'auto',
        fallback_transports: (tunnel.fallback_transports ?? []).join(', '),
        client_proxy_url: tunnel.client_proxy_url,
        max_connectors: String(tunnel.max_connectors || 4),
        allowed_target_cidrs: (tunnel.allowed_target_cidrs ?? []).join('\n'),
        blocked_target_cidrs: (tunnel.blocked_target_cidrs ?? []).join('\n'),
        enabled: tunnel.enabled,
        auto_update_enabled: tunnel.auto_update_enabled,
      });
    } else {
      const defPool = pools.find((p) => p.is_default) ?? pools[0];
      form.reset({
        name: '',
        description: '',
        pool_id: defPool ? String(defPool.id) : '',
        ha_relay_count: '2',
        preferred_transport: 'auto',
        fallback_transports: 'noise, websocket, tcp',
        client_proxy_url: '',
        max_connectors: '4',
        allowed_target_cidrs: '',
        blocked_target_cidrs: '169.254.169.254/32',
        enabled: true,
        auto_update_enabled: false,
      });
    }
  }, [open, tunnel, pools, form]);

  const handleSubmit = form.handleSubmit(async (values) => {
    await onSubmit({
      name: values.name,
      description: values.description,
      pool_id: values.pool_id ? Number(values.pool_id) : undefined,
      ha_relay_count: Math.max(
        1,
        Math.min(5, Number(values.ha_relay_count) || 2),
      ),
      preferred_transport: values.preferred_transport,
      fallback_transports: splitCommaOrLineList(values.fallback_transports),
      client_proxy_url: values.client_proxy_url,
      max_connectors: Math.max(0, Number(values.max_connectors) || 0),
      allowed_target_cidrs: splitCommaOrLineList(values.allowed_target_cidrs),
      blocked_target_cidrs: splitCommaOrLineList(values.blocked_target_cidrs),
      enabled: values.enabled,
      auto_update_enabled: values.auto_update_enabled,
    });
  });

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {tunnel ? t('tunnels.editTitle') : t('tunnels.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('tunnels.editorDesc')}</DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(event) => void handleSubmit(event)}
          className='space-y-4'
        >
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='tunnel-name'>{t('tunnels.name')}</Label>
              <Input
                id='tunnel-name'
                placeholder='prod-office-vpc'
                {...form.register('name')}
              />
              {form.formState.errors.name ? (
                <p className='text-xs text-destructive'>
                  {form.formState.errors.name.message}
                </p>
              ) : null}
            </div>

            <div className='space-y-2'>
              <Label>{t('tunnels.pool')}</Label>
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

          <div className='space-y-2'>
            <Label htmlFor='tunnel-desc'>{t('tunnels.description')}</Label>
            <Input id='tunnel-desc' {...form.register('description')} />
          </div>

          <div className='grid gap-3 sm:grid-cols-3'>
            <div className='space-y-2'>
              <Label htmlFor='tunnel-ha'>{t('tunnels.haRelayCount')}</Label>
              <Input
                id='tunnel-ha'
                type='number'
                min={1}
                max={5}
                {...form.register('ha_relay_count')}
              />
            </div>

            <div className='space-y-2'>
              <Label>{t('tunnels.preferredTransport')}</Label>
              <Select
                value={form.watch('preferred_transport')}
                onValueChange={(value) =>
                  form.setValue('preferred_transport', value)
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='auto'>Auto</SelectItem>
                  <SelectItem value='noise'>Noise</SelectItem>
                  <SelectItem value='websocket'>WebSocket</SelectItem>
                  <SelectItem value='tcp'>TCP</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='tunnel-max-conn'>
                {t('tunnels.maxConnectors')}
              </Label>
              <Input
                id='tunnel-max-conn'
                type='number'
                min={0}
                {...form.register('max_connectors')}
              />
            </div>
          </div>

          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='tunnel-fallbacks'>
                {t('tunnels.fallbackTransports')}
              </Label>
              <Input
                id='tunnel-fallbacks'
                placeholder='noise, websocket, tcp'
                {...form.register('fallback_transports')}
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='tunnel-proxy'>
                {t('tunnels.clientProxyUrl')}
              </Label>
              <Input
                id='tunnel-proxy'
                placeholder='socks5://127.0.0.1:1080'
                {...form.register('client_proxy_url')}
              />
            </div>
          </div>

          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='tunnel-allowed'>
                {t('tunnels.allowedTargetCidrs')}
              </Label>
              <Textarea
                id='tunnel-allowed'
                className='min-h-20 font-mono text-xs'
                placeholder={'10.0.0.0/8\n192.168.1.0/24'}
                {...form.register('allowed_target_cidrs')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='tunnel-blocked'>
                {t('tunnels.blockedTargetCidrs')}
              </Label>
              <Textarea
                id='tunnel-blocked'
                className='min-h-20 font-mono text-xs'
                placeholder='169.254.169.254/32'
                {...form.register('blocked_target_cidrs')}
              />
            </div>
          </div>

          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
              <p className='text-sm font-medium'>{t('tunnels.enabled')}</p>
              <Switch
                checked={form.watch('enabled')}
                onCheckedChange={(checked) => form.setValue('enabled', checked)}
              />
            </div>
            <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
              <p className='text-sm font-medium'>{t('tunnels.autoUpdate')}</p>
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
