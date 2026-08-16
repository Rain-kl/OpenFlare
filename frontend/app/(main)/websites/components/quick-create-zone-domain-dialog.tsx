'use client';

import { useEffect, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Loader2 } from 'lucide-react';
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
import {
  TlsCertificateService,
  ZoneDomainService,
  ZoneService,
  zoneQueryKey,
  type ZoneDomainItem,
  type ZoneItem,
} from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import {
  previewZoneDomainInput,
  resolveZoneDomainInput,
  type ZoneDomainInputError,
} from './resolve-zone-domain-input';

type Values = {
  zone_id: string;
  domain_input: string;
  cert_id: string;
};

export function QuickCreateZoneDomainDialog({
  open,
  onOpenChange,
  fixedZoneId,
  fixedZoneRoot,
  zones: zonesProp,
  onCreated,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
  /** When set, Zone 选择器隐藏 */
  fixedZoneId?: number;
  fixedZoneRoot?: string;
  zones?: ZoneItem[];
  onCreated(domain: ZoneDomainItem): void | Promise<void>;
}) {
  const t = useTranslations('websites');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const domainErrorMessage = (
    error: ZoneDomainInputError | undefined,
    root?: string,
  ) => {
    if (!error) return t('invalidFormat');
    if (error === 'mustBelongToZone') {
      return t('mustBelongToZone', { root: root ?? '' });
    }
    return t(error);
  };
  const zonesQuery = useQuery({
    queryKey: zoneQueryKey,
    queryFn: () => ZoneService.list(),
    enabled: open && !fixedZoneId && !zonesProp,
  });
  const certificatesQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates'],
    queryFn: () => TlsCertificateService.list(),
    enabled: open,
  });

  const zones = useMemo(
    () => zonesProp ?? zonesQuery.data ?? [],
    [zonesProp, zonesQuery.data],
  );
  const fixedZone = useMemo(() => {
    if (!fixedZoneId) {
      return undefined;
    }
    return (
      zones.find((zone) => zone.id === fixedZoneId) ??
      (fixedZoneRoot
        ? ({
            id: fixedZoneId,
            domain: fixedZoneRoot,
            created_at: '',
            updated_at: '',
          } as ZoneItem)
        : undefined)
    );
  }, [fixedZoneId, fixedZoneRoot, zones]);

  const schema = z.object({
    zone_id: z.string().min(1, t('selectZone')),
    domain_input: z.string().trim().min(1, t('enterDomain')),
    cert_id: z.string(),
  });
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      zone_id: fixedZoneId ? String(fixedZoneId) : '',
      domain_input: '',
      cert_id: '',
    },
  });

  useEffect(() => {
    if (!open) {
      return;
    }
    form.reset({
      zone_id: fixedZoneId ? String(fixedZoneId) : '',
      domain_input: '',
      cert_id: '',
    });
  }, [fixedZoneId, form, open]);

  const watchedZoneId = form.watch('zone_id');
  const watchedInput = form.watch('domain_input');
  const selectedZone = useMemo(() => {
    if (fixedZone) {
      return fixedZone;
    }
    const id = Number(watchedZoneId);
    return zones.find((zone) => zone.id === id);
  }, [fixedZone, watchedZoneId, zones]);

  const preview = selectedZone
    ? previewZoneDomainInput(watchedInput, selectedZone.domain)
    : '';

  const mutation = useMutation({
    mutationFn: async (values: Values) => {
      const zoneId = Number(values.zone_id);
      const zone = fixedZone ?? zones.find((item) => item.id === zoneId);
      if (!zone) {
        throw new Error(t('selectZone'));
      }
      const resolved = resolveZoneDomainInput(values.domain_input, zone.domain);
      if (resolved.error || !resolved.domain) {
        throw new Error(domainErrorMessage(resolved.error, zone.domain));
      }
      return ZoneDomainService.create(zone.id, {
        domain: resolved.domain,
        cert_id: values.cert_id ? Number(values.cert_id) : null,
      });
    },
    onSuccess: async (domain) => {
      toast.success(t('domainAdded'), { description: domain.domain });
      await Promise.all([
        onCreated(domain),
        queryClient.invalidateQueries({ queryKey: zoneQueryKey }),
        queryClient.invalidateQueries({
          queryKey: [...zoneQueryKey, 'all-domains'],
        }),
      ]);
      onOpenChange(false);
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : t('addFailed')),
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('quickCreateTitle')}</DialogTitle>
          <DialogDescription>{t('quickCreateDesc')}</DialogDescription>
        </DialogHeader>

        <form
          id='quick-create-zone-domain'
          className='space-y-4'
          onSubmit={form.handleSubmit((values) => {
            if (!selectedZone) {
              form.setError('zone_id', { message: t('selectZone') });
              return;
            }
            const resolved = resolveZoneDomainInput(
              values.domain_input,
              selectedZone.domain,
            );
            if (resolved.error) {
              form.setError('domain_input', {
                message: domainErrorMessage(
                  resolved.error,
                  selectedZone.domain,
                ),
              });
              return;
            }
            mutation.mutate(values);
          })}
        >
          {!fixedZoneId ? (
            <div className='space-y-1.5'>
              <Label>Zone</Label>
              <Select
                value={form.watch('zone_id') || undefined}
                onValueChange={(value) => form.setValue('zone_id', value)}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('selectRegisteredRoot')} />
                </SelectTrigger>
                <SelectContent>
                  {zones.map((zone) => (
                    <SelectItem key={zone.id} value={String(zone.id)}>
                      {zone.domain}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {form.formState.errors.zone_id ? (
                <p className='text-xs text-destructive'>
                  {form.formState.errors.zone_id.message}
                </p>
              ) : null}
            </div>
          ) : (
            <div className='rounded-md border bg-muted/30 px-3 py-2 text-sm'>
              {t('zoneLabel')}
              <span className='ml-1 font-medium'>
                {fixedZoneRoot || fixedZone?.domain || `#${fixedZoneId}`}
              </span>
            </div>
          )}

          <div className='space-y-1.5'>
            <Label htmlFor='domain-input'>{t('domain')}</Label>
            <Input
              id='domain-input'
              placeholder={
                selectedZone
                  ? t('domainPlaceholderWithZone', {
                      domain: selectedZone.domain,
                    })
                  : t('domainPlaceholder')
              }
              {...form.register('domain_input')}
            />
            {preview ? (
              <p className='text-xs text-muted-foreground'>
                {t('willCreate')}
                <code className='ml-1 rounded bg-muted px-1 py-0.5 font-mono text-[11px]'>
                  {preview}
                </code>
              </p>
            ) : (
              <p className='text-xs text-muted-foreground'>
                {t.rich('domainExample', {
                  api: (chunks) => <code className='font-mono'>{chunks}</code>,
                  fqdn: (chunks) => <code className='font-mono'>{chunks}</code>,
                  apex: (chunks) => <code className='font-mono'>{chunks}</code>,
                })}
              </p>
            )}
            {form.formState.errors.domain_input ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.domain_input.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-1.5'>
            <Label>{t('certOptional')}</Label>
            <Select
              value={form.watch('cert_id') || '__none'}
              onValueChange={(value) =>
                form.setValue('cert_id', value === '__none' ? '' : value)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t('noCert')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='__none'>{t('noCert')}</SelectItem>
                {(certificatesQuery.data ?? []).map((certificate) => (
                  <SelectItem
                    key={certificate.id}
                    value={String(certificate.id)}
                  >
                    {certificate.name} · {certificate.primary_domain}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </form>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {tc('cancel')}
          </Button>
          <Button
            type='submit'
            form='quick-create-zone-domain'
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <Loader2 className='mr-1 size-4 animate-spin' />
            ) : null}
            {t('addDomain')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
