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
  TunnelRelayView,
  TunnelServiceInput,
  TunnelServiceView,
} from '@/lib/services/openflare';

function buildSchema(t: (key: string) => string) {
  return z.object({
    name: z.string().trim().min(1, t('services.errNameRequired')).max(128),
    service_type: z.enum(['tcp', 'udp']),
    relay_id: z.string(),
    listen_port: z.string().trim(),
    target_addr: z.string().trim().min(1, t('services.errTargetRequired')),
    enabled: z.boolean(),
  });
}

type ServiceFormValues = z.infer<ReturnType<typeof buildSchema>>;

export function ServiceEditorDialog({
  open,
  service,
  relays,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  service: TunnelServiceView | null;
  relays: TunnelRelayView[];
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: TunnelServiceInput) => Promise<void>;
}) {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const form = useForm<ServiceFormValues>({
    resolver: zodResolver(buildSchema(t)),
    defaultValues: {
      name: '',
      service_type: 'tcp',
      relay_id: '0',
      listen_port: '9000',
      target_addr: '127.0.0.1:22',
      enabled: true,
    },
  });

  useEffect(() => {
    if (!open) return;
    form.reset(
      service
        ? {
            name: service.name,
            service_type: service.service_type,
            relay_id: String(service.relay_id || 0),
            listen_port: String(service.listen_port),
            target_addr: service.target_addr,
            enabled: service.enabled,
          }
        : {
            name: '',
            service_type: 'tcp',
            relay_id: '0',
            listen_port: '9000',
            target_addr: '127.0.0.1:22',
            enabled: true,
          },
    );
  }, [open, service, form]);

  const handleSubmit = form.handleSubmit(async (values) => {
    await onSubmit({
      name: values.name,
      service_type: values.service_type,
      relay_id: Number(values.relay_id) || 0,
      listen_port: Number(values.listen_port) || 0,
      target_addr: values.target_addr,
      enabled: values.enabled,
    });
  });

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>
            {service ? t('services.editTitle') : t('services.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('services.editorDesc')}</DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(event) => void handleSubmit(event)}
          className='space-y-4'
        >
          <div className='space-y-2'>
            <Label htmlFor='svc-name'>{t('services.name')}</Label>
            <Input
              id='svc-name'
              placeholder='internal-ssh'
              {...form.register('name')}
            />
            {form.formState.errors.name ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.name.message}
              </p>
            ) : null}
          </div>

          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('services.protocol')}</Label>
              <Select
                value={form.watch('service_type')}
                onValueChange={(value) =>
                  form.setValue('service_type', value as 'tcp' | 'udp')
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='tcp'>TCP</SelectItem>
                  <SelectItem value='udp'>UDP</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='svc-lport'>{t('services.listenPort')}</Label>
              <Input
                id='svc-lport'
                type='number'
                min={1}
                max={65535}
                {...form.register('listen_port')}
              />
            </div>
          </div>

          <div className='space-y-2'>
            <Label>{t('services.relayScope')}</Label>
            <Select
              value={form.watch('relay_id')}
              onValueChange={(value) => form.setValue('relay_id', value)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='0'>{t('services.floatingRelay')}</SelectItem>
                {relays.map((relay) => (
                  <SelectItem key={relay.id} value={String(relay.id)}>
                    {relay.name} ({relay.public_addr})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='svc-target'>{t('services.targetAddr')}</Label>
            <Input
              id='svc-target'
              placeholder='127.0.0.1:22'
              {...form.register('target_addr')}
            />
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <p className='text-sm font-medium'>{t('tunnels.enabled')}</p>
            <Switch
              checked={form.watch('enabled')}
              onCheckedChange={(checked) => form.setValue('enabled', checked)}
            />
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
