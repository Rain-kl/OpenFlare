'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { ChevronDown, Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import type { TlsCertificateItem } from '@/lib/services/openflare';
import {
  DnsAccountService,
  TlsCertificateService,
} from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import {
  type AcmeApplyFormValues,
  createAcmeApplySchema,
  defaultAcmeApplyValues,
} from './schemas';
import { getErrorMessage } from './website-utils';

const certificatesQueryKey = ['openflare', 'tls-certificates'];

type CertificateApplyMode = 'create' | 'edit-acme' | 'convert-upload';

interface CertificateApplyDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onApplied?: (certificate: TlsCertificateItem) => void;
  mode?: CertificateApplyMode;
  certificate?: TlsCertificateItem | null;
}

export function CertificateApplyDialog({
  open,
  onOpenChange,
  onApplied,
  mode = 'create',
  certificate,
}: CertificateApplyDialogProps) {
  const t = useTranslations('certificates');
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);

  const dnsAccountsQuery = useQuery({
    queryKey: ['openflare', 'dns-accounts'],
    queryFn: () => DnsAccountService.list(),
    enabled: open,
  });

  const defaultAcmeAccountQuery = useQuery({
    queryKey: ['openflare', 'acme-accounts', 'default'],
    queryFn: () => TlsCertificateService.getDefaultAcmeAccount(),
    enabled: open,
  });

  const form = useForm<AcmeApplyFormValues>({
    resolver: zodResolver(createAcmeApplySchema(t)),
    defaultValues: defaultAcmeApplyValues,
  });

  useEffect(() => {
    if (!open) return;
    setError('');
    setShowAdvanced(false);

    if (certificate) {
      form.reset({
        name: certificate.name,
        primary_domain:
          mode === 'convert-upload' ? '' : certificate.primary_domain || '',
        other_domains:
          mode === 'convert-upload' ? '' : certificate.other_domains || '',
        remark: certificate.remark || '',
        acme_account_id:
          mode === 'convert-upload' ? 0 : certificate.acme_account_id,
        dns_account_id:
          mode === 'convert-upload' ? 0 : certificate.dns_account_id,
        key_algorithm: certificate.key_algorithm || 'EC256',
        auto_renew: mode === 'convert-upload' ? true : certificate.auto_renew,
        dns1: mode === 'convert-upload' ? '' : certificate.dns1 || '',
        dns2: mode === 'convert-upload' ? '' : certificate.dns2 || '',
        disable_cname:
          mode === 'convert-upload' ? false : certificate.disable_cname,
        skip_dns: mode === 'convert-upload' ? false : certificate.skip_dns,
      });
      if (
        mode !== 'convert-upload' &&
        (certificate.dns1 ||
          certificate.dns2 ||
          certificate.disable_cname ||
          certificate.skip_dns)
      ) {
        setShowAdvanced(true);
      }
    } else {
      form.reset(defaultAcmeApplyValues);
    }
  }, [certificate, form, mode, open]);

  useEffect(() => {
    if (
      defaultAcmeAccountQuery.data &&
      form.getValues('acme_account_id') === 0
    ) {
      form.setValue('acme_account_id', defaultAcmeAccountQuery.data.id);
    }
  }, [defaultAcmeAccountQuery.data, form, open]);

  const applyMutation = useMutation({
    mutationFn: (values: AcmeApplyFormValues) => {
      if (mode === 'edit-acme' && certificate) {
        return TlsCertificateService.updateAcme(certificate.id, values);
      }
      if (mode === 'convert-upload' && certificate) {
        return TlsCertificateService.convertToAcme(certificate.id, values);
      }
      return TlsCertificateService.apply(values);
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: certificatesQueryKey });
      onApplied?.(result);
      onOpenChange(false);
    },
    onError: (err) => setError(getErrorMessage(err, t('requestFailed'))),
  });

  const title =
    mode === 'edit-acme'
      ? t('applyEditTitle')
      : mode === 'convert-upload'
        ? t('applyConvertTitle')
        : t('applyTitle');

  const description =
    mode === 'edit-acme'
      ? t('applyEditDesc')
      : mode === 'convert-upload'
        ? t('applyConvertDesc')
        : t('applyDesc');

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-2xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <form
          className='space-y-4'
          onSubmit={form.handleSubmit((values) => {
            setError('');
            applyMutation.mutate(values);
          })}
        >
          {error ? <p className='text-sm text-destructive'>{error}</p> : null}

          <div className='grid gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('name')}</Label>
              <Input
                placeholder={t('namePlaceholder')}
                {...form.register('name')}
              />
            </div>
            <div className='space-y-2'>
              <Label>{t('primaryDomain')}</Label>
              <Input
                placeholder={t('primaryDomainPlaceholder')}
                {...form.register('primary_domain')}
              />
            </div>
          </div>

          <div className='space-y-2'>
            <Label>{t('otherDomains')}</Label>
            <Textarea
              rows={3}
              placeholder='example.net'
              {...form.register('other_domains')}
            />
            <p className='text-xs text-muted-foreground'>
              {t('otherDomainsHint')}
            </p>
          </div>

          <div className='grid gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('dnsAccount')}</Label>
              <Select
                value={String(form.watch('dns_account_id') || 0)}
                onValueChange={(value) =>
                  form.setValue('dns_account_id', Number(value), {
                    shouldValidate: true,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('selectDnsAccount')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='0'>{t('selectDnsAccount')}</SelectItem>
                  {dnsAccountsQuery.data?.map((account) => (
                    <SelectItem key={account.id} value={String(account.id)}>
                      {account.name} ({account.type})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label>{t('keyAlgorithm')}</Label>
              <Select
                value={form.watch('key_algorithm')}
                onValueChange={(value) => form.setValue('key_algorithm', value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='RSA2048'>RSA 2048</SelectItem>
                  <SelectItem value='RSA4096'>RSA 4096</SelectItem>
                  <SelectItem value='EC256'>ECC 256</SelectItem>
                  <SelectItem value='EC384'>ECC 384</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className='space-y-2'>
            <Label>{t('remark')}</Label>
            <Input
              placeholder={t('remarkPlaceholder')}
              {...form.register('remark')}
            />
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('autoRenew')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('autoRenewDesc')}
              </p>
            </div>
            <Switch
              checked={form.watch('auto_renew')}
              onCheckedChange={(checked) =>
                form.setValue('auto_renew', checked)
              }
            />
          </div>

          <div className='overflow-hidden rounded-lg border'>
            <Button
              type='button'
              variant='ghost'
              className='w-full justify-between rounded-none'
              onClick={() => setShowAdvanced((current) => !current)}
            >
              {t('advanced')}
              <ChevronDown
                className={`size-4 transition-transform ${showAdvanced ? 'rotate-180' : ''}`}
              />
            </Button>
            {showAdvanced ? (
              <div className='space-y-4 border-t p-3'>
                <div className='grid gap-4 md:grid-cols-2'>
                  <div className='space-y-2'>
                    <Label>{t('dnsServer1')}</Label>
                    <Input
                      placeholder={t('dnsServerPlaceholder')}
                      {...form.register('dns1')}
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label>{t('dnsServer2')}</Label>
                    <Input
                      placeholder={t('dnsServerPlaceholder')}
                      {...form.register('dns2')}
                    />
                  </div>
                </div>
                <div className='grid gap-4 md:grid-cols-2'>
                  <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
                    <div>
                      <p className='text-sm font-medium'>{t('skipCname')}</p>
                      <p className='text-xs text-muted-foreground'>
                        {t('skipCnameDesc')}
                      </p>
                    </div>
                    <Switch
                      checked={form.watch('disable_cname')}
                      onCheckedChange={(checked) =>
                        form.setValue('disable_cname', checked)
                      }
                    />
                  </div>
                  <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
                    <div>
                      <p className='text-sm font-medium'>{t('skipDns')}</p>
                      <p className='text-xs text-muted-foreground'>
                        {t('skipDnsDesc')}
                      </p>
                    </div>
                    <Switch
                      checked={form.watch('skip_dns')}
                      onCheckedChange={(checked) =>
                        form.setValue('skip_dns', checked)
                      }
                    />
                  </div>
                </div>
              </div>
            ) : null}
          </div>

          <Button type='submit' disabled={applyMutation.isPending}>
            {applyMutation.isPending ? (
              <>
                <Loader2 className='mr-1 size-3.5 animate-spin' />
                {t('submitting')}
              </>
            ) : mode === 'edit-acme' ? (
              t('saveAndApply')
            ) : mode === 'convert-upload' ? (
              t('startConvert')
            ) : (
              t('startApply')
            )}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
