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
  NodeItem,
  NodeMutationPayload,
  NodeType,
} from '@/lib/services/openflare';

function buildNodeSchema(t: (key: string) => string) {
  return z
    .object({
      node_type: z.enum(['edge_node', 'tunnel_relay', 'tunnel_client']),
      name: z.string().trim().min(1, t('editor.errName')).max(255),
      ip: z.string(),
      ip_manual_override: z.boolean(),
      auto_update_enabled: z.boolean(),
      geo_manual_override: z.boolean(),
      geo_name: z.string(),
      geo_latitude: z.string(),
      geo_longitude: z.string(),
      relay_bind_port: z.string(),
      relay_vhost_http_port: z.string(),
    })
    .superRefine((value, context) => {
      if (value.ip_manual_override && !value.ip.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['ip'],
          message: t('editor.errLockIp'),
        });
      }
      if (value.node_type === 'tunnel_relay') {
        const bindPort = Number(value.relay_bind_port);
        const vhostPort = Number(value.relay_vhost_http_port);
        if (!Number.isFinite(bindPort) || bindPort <= 0) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['relay_bind_port'],
            message: t('editor.errBindPort'),
          });
        }
        if (!Number.isFinite(vhostPort) || vhostPort <= 0) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['relay_vhost_http_port'],
            message: t('editor.errVhostPort'),
          });
        }
      }
    });
}

type NodeFormValues = z.infer<ReturnType<typeof buildNodeSchema>>;

const defaultForm: NodeFormValues = {
  node_type: 'edge_node',
  name: '',
  ip: '',
  ip_manual_override: false,
  auto_update_enabled: false,
  geo_manual_override: false,
  geo_name: '',
  geo_latitude: '',
  geo_longitude: '',
  relay_bind_port: '7000',
  relay_vhost_http_port: '8080',
};

function buildFormValues(node?: NodeItem | null): NodeFormValues {
  if (!node) return defaultForm;

  return {
    node_type: node.node_type,
    name: node.name,
    ip: node.ip,
    ip_manual_override: node.ip_manual_override,
    auto_update_enabled: node.auto_update_enabled,
    geo_manual_override: node.geo_manual_override,
    geo_name: node.geo_name,
    geo_latitude:
      node.geo_latitude === undefined || node.geo_latitude === null
        ? ''
        : String(node.geo_latitude),
    geo_longitude:
      node.geo_longitude === undefined || node.geo_longitude === null
        ? ''
        : String(node.geo_longitude),
    relay_bind_port: String(node.relay_bind_port || 7000),
    relay_vhost_http_port: String(node.relay_vhost_http_port || 8080),
  };
}

function toPayload(form: NodeFormValues): NodeMutationPayload {
  const base: NodeMutationPayload = {
    node_type: form.node_type,
    name: form.name.trim(),
    ip: form.ip.trim(),
    ip_manual_override: form.ip_manual_override,
    auto_update_enabled: form.auto_update_enabled,
    geo_manual_override: form.geo_manual_override,
    geo_name: form.geo_manual_override ? form.geo_name.trim() : '',
    geo_latitude:
      form.geo_manual_override && form.geo_latitude
        ? Number(form.geo_latitude)
        : null,
    geo_longitude:
      form.geo_manual_override && form.geo_longitude
        ? Number(form.geo_longitude)
        : null,
  };

  if (form.node_type === 'tunnel_relay') {
    return {
      ...base,
      relay_bind_port: Number(form.relay_bind_port),
      relay_vhost_http_port: Number(form.relay_vhost_http_port),
      relay_web_server_enabled: true,
    };
  }

  return base;
}

export function NodeEditorDialog({
  open,
  node,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  node: NodeItem | null;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: NodeMutationPayload) => Promise<void>;
}) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const form = useForm<NodeFormValues>({
    resolver: zodResolver(buildNodeSchema(t)),
    defaultValues: defaultForm,
  });

  useEffect(() => {
    if (open) {
      form.reset(buildFormValues(node));
    }
  }, [form, open, node]);

  const nodeType = form.watch('node_type');
  const ipManualOverride = form.watch('ip_manual_override');
  const geoManualOverride = form.watch('geo_manual_override');

  const handleSubmit = form.handleSubmit(async (values) => {
    await onSubmit(toPayload(values));
  });

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {node ? t('editor.editTitle') : t('editor.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('editor.description')}</DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(event) => void handleSubmit(event)}
          className='space-y-4'
        >
          <div className='space-y-2'>
            <Label>{t('editor.nodeType')}</Label>
            <Select
              value={nodeType}
              disabled={Boolean(node)}
              onValueChange={(value) =>
                form.setValue('node_type', value as NodeType)
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='edge_node'>
                  {t('editor.typeEdge')}
                </SelectItem>
                <SelectItem value='tunnel_relay'>
                  {t('editor.typeRelay')}
                </SelectItem>
                <SelectItem value='tunnel_client'>
                  {t('editor.typeTunnel')}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='node-name'>{t('editor.name')}</Label>
            <Input
              id='node-name'
              placeholder={t('editor.namePlaceholder')}
              {...form.register('name')}
            />
            {form.formState.errors.name ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.name.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-2'>
            <Label htmlFor='node-ip'>{t('editor.ip')}</Label>
            <Input
              id='node-ip'
              placeholder={t('editor.ipPlaceholder')}
              {...form.register('ip')}
            />
            {form.formState.errors.ip ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.ip.message}
              </p>
            ) : null}
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('editor.lockIp')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('editor.lockIpDesc')}
              </p>
            </div>
            <Switch
              checked={ipManualOverride}
              onCheckedChange={(checked) =>
                form.setValue('ip_manual_override', checked)
              }
            />
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('editor.autoUpdate')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('editor.autoUpdateDesc')}
              </p>
            </div>
            <Switch
              checked={form.watch('auto_update_enabled')}
              onCheckedChange={(checked) =>
                form.setValue('auto_update_enabled', checked)
              }
            />
          </div>

          {nodeType === 'tunnel_relay' ? (
            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='space-y-2'>
                <Label htmlFor='relay-bind-port'>{t('editor.bindPort')}</Label>
                <Input
                  id='relay-bind-port'
                  {...form.register('relay_bind_port')}
                />
                {form.formState.errors.relay_bind_port ? (
                  <p className='text-xs text-destructive'>
                    {form.formState.errors.relay_bind_port.message}
                  </p>
                ) : null}
              </div>
              <div className='space-y-2'>
                <Label htmlFor='relay-vhost-port'>
                  {t('editor.vhostPort')}
                </Label>
                <Input
                  id='relay-vhost-port'
                  {...form.register('relay_vhost_http_port')}
                />
                {form.formState.errors.relay_vhost_http_port ? (
                  <p className='text-xs text-destructive'>
                    {form.formState.errors.relay_vhost_http_port.message}
                  </p>
                ) : null}
              </div>
            </div>
          ) : null}

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('editor.manualGeo')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('editor.manualGeoDesc')}
              </p>
            </div>
            <Switch
              checked={geoManualOverride}
              onCheckedChange={(checked) =>
                form.setValue('geo_manual_override', checked)
              }
            />
          </div>

          {geoManualOverride ? (
            <div className='grid gap-3 sm:grid-cols-3'>
              <div className='space-y-2 sm:col-span-3'>
                <Label htmlFor='geo-name'>{t('editor.geoName')}</Label>
                <Input id='geo-name' {...form.register('geo_name')} />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='geo-lat'>{t('editor.latitude')}</Label>
                <Input id='geo-lat' {...form.register('geo_latitude')} />
              </div>
              <div className='space-y-2 sm:col-span-2'>
                <Label htmlFor='geo-lng'>{t('editor.longitude')}</Label>
                <Input id='geo-lng' {...form.register('geo_longitude')} />
              </div>
            </div>
          ) : null}

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
              {submitting
                ? t('editor.saving')
                : node
                  ? t('editor.saveEdit')
                  : t('editor.createAction')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
