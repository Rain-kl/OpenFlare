'use client';

import { useEffect, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Loader2, TriangleAlert } from 'lucide-react';
import { toast } from 'sonner';
import { z } from 'zod';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
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
import { findMatchingCertificate, getCertificateMatch } from './website-utils';

type Values = {
  zone_id: string;
  domain_input: string;
  manual_cert_id: string | null;
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
  const uiText = useTranslations('runtime');

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
    domain_input: z.string().min(1, t('enterDomain')),
    manual_cert_id: z.string().nullable(),
  });
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      zone_id: fixedZoneId ? String(fixedZoneId) : '',
      domain_input: '',
      manual_cert_id: null,
    },
  });

  useEffect(() => {
    if (!open) {
      return;
    }
    form.reset({
      zone_id: fixedZoneId ? String(fixedZoneId) : '',
      domain_input: '',
      manual_cert_id: null,
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

  const certificates = certificatesQuery.data ?? [];
  const manualCertificateId = form.watch('manual_cert_id');
  const suggestedCertificate =
    manualCertificateId === null
      ? findMatchingCertificate(certificates, preview)
      : undefined;
  const certificateId =
    manualCertificateId ?? String(suggestedCertificate?.id ?? '');
  const selectedCertificate = certificates.find(
    (certificate) => String(certificate.id) === certificateId,
  );
  const certificateMismatch =
    !!preview &&
    !!selectedCertificate &&
    !getCertificateMatch(selectedCertificate, preview);
  const { errors } = form.formState;

  const mutation = useMutation({
    mutationFn: ({
      zoneId,
      domain,
      certId,
    }: {
      zoneId: number;
      domain: string;
      certId: number | null;
    }) => ZoneDomainService.create(zoneId, { domain, cert_id: certId }),
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
            mutation.mutate({
              zoneId: selectedZone.id,
              domain: resolved.domain,
              certId: certificateId ? Number(certificateId) : null,
            });
          })}
        >
          <FieldGroup className='gap-4'>
            {!fixedZoneId ? (
              <Field className='gap-1.5' data-invalid={!!errors.zone_id}>
                <FieldLabel htmlFor='quick-create-zone'>
                  {uiText('ui_zone')}
                </FieldLabel>
                <Select
                  value={watchedZoneId}
                  onValueChange={(value) => form.setValue('zone_id', value)}
                >
                  <SelectTrigger
                    id='quick-create-zone'
                    aria-invalid={!!errors.zone_id}
                  >
                    <SelectValue placeholder={t('selectRegisteredRoot')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {zones.map((zone) => (
                        <SelectItem key={zone.id} value={String(zone.id)}>
                          {zone.domain}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FieldError errors={[errors.zone_id]} />
              </Field>
            ) : (
              <div className='rounded-md border bg-muted/30 px-3 py-2 text-sm'>
                {t('zoneLabel')}
                <span className='ml-1 font-medium'>
                  {fixedZoneRoot || fixedZone?.domain || `#${fixedZoneId}`}
                </span>
              </div>
            )}

            <Field className='gap-1.5' data-invalid={!!errors.domain_input}>
              <FieldLabel htmlFor='domain-input'>{t('domain')}</FieldLabel>
              <Input
                id='domain-input'
                aria-invalid={!!errors.domain_input}
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
                <FieldDescription>
                  {t('willCreate')}
                  <code className='ml-1 rounded bg-muted px-1 py-0.5 font-mono text-[11px]'>
                    {preview}
                  </code>
                </FieldDescription>
              ) : (
                <FieldDescription>
                  {t.rich('domainExample', {
                    api: (chunks) => (
                      <code className='font-mono'>{chunks}</code>
                    ),
                    fqdn: (chunks) => (
                      <code className='font-mono'>{chunks}</code>
                    ),
                    apex: (chunks) => (
                      <code className='font-mono'>{chunks}</code>
                    ),
                  })}
                </FieldDescription>
              )}
              <FieldError errors={[errors.domain_input]} />
            </Field>

            <Field className='gap-1.5' data-invalid={certificateMismatch}>
              <FieldLabel htmlFor='quick-create-cert'>
                {t('certOptional')}
              </FieldLabel>
              <Select
                value={certificateId || '__none'}
                onValueChange={(value) => {
                  // The native select can emit an empty value while options load.
                  if (value) {
                    form.setValue(
                      'manual_cert_id',
                      value === '__none' ? '' : value,
                    );
                  }
                }}
              >
                <SelectTrigger
                  id='quick-create-cert'
                  aria-invalid={certificateMismatch}
                  aria-describedby={
                    certificateMismatch
                      ? 'quick-create-cert-warning'
                      : undefined
                  }
                >
                  <SelectValue placeholder={t('noCert')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='__none'>{t('noCert')}</SelectItem>
                    {certificates.map((certificate) => {
                      const labelDomain =
                        certificate.dns_names?.[0] ||
                        certificate.primary_domain;
                      return (
                        <SelectItem
                          key={certificate.id}
                          value={String(certificate.id)}
                        >
                          {certificate.name}
                          {labelDomain ? ` · ${labelDomain}` : ''}
                        </SelectItem>
                      );
                    })}
                  </SelectGroup>
                </SelectContent>
              </Select>
              {certificateMismatch ? (
                <Alert variant='destructive' id='quick-create-cert-warning'>
                  <TriangleAlert />
                  <AlertDescription>
                    {t('certificateMismatch', { domain: preview })}
                  </AlertDescription>
                </Alert>
              ) : null}
            </Field>
          </FieldGroup>
        </form>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {tc('cancel')}
          </Button>
          <Button
            type='submit'
            form='quick-create-zone-domain'
            disabled={
              mutation.isPending ||
              (!!preview &&
                manualCertificateId === null &&
                certificatesQuery.isLoading)
            }
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
