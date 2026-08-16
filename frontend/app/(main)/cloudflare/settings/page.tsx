'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { ArrowLeft, Cloud, Save, ShieldCheck, Trash2 } from 'lucide-react';
import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Field,
  FieldDescription,
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
import { ErrorInline } from '@/components/layout/error';
import {
  CloudflareService,
  cloudflareQueryKey,
  DnsAccountService,
  type CloudflareConnectionSource,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

const dnsAccountsQueryKey = ['openflare', 'dns-accounts'] as const;

export default function CloudflareSettingsPage() {
  const t = useTranslations('cloudflare.settings');
  const queryClient = useQueryClient();
  const [source, setSource] =
    useState<CloudflareConnectionSource>('dns_account');
  const [dnsAccountID, setDNSAccountID] = useState('');
  const [apiToken, setAPIToken] = useState('');

  const connectionQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'connection'],
    queryFn: () => CloudflareService.getConnection(),
  });
  const accountsQuery = useQuery({
    queryKey: dnsAccountsQueryKey,
    queryFn: () => DnsAccountService.list(),
  });

  const cloudflareAccounts = useMemo(
    () =>
      (accountsQuery.data ?? []).filter(
        (account) => account.type === 'cloudflare',
      ),
    [accountsQuery.data],
  );

  useEffect(() => {
    const connection = connectionQuery.data;
    if (!connection?.configured) return;
    if (connection.source) setSource(connection.source);
    if (connection.dns_account_id)
      setDNSAccountID(String(connection.dns_account_id));
  }, [connectionQuery.data]);

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });
  };

  const saveMutation = useMutation({
    mutationFn: () =>
      CloudflareService.saveConnection({
        source,
        dns_account_id: source === 'dns_account' ? Number(dnsAccountID) : 0,
        api_token: source === 'standalone' ? apiToken : '',
      }),
    onSuccess: async () => {
      toast.success(t('saved'));
      setAPIToken('');
      await refresh();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const verifyMutation = useMutation({
    mutationFn: () => CloudflareService.verifyConnection(),
    onSuccess: async () => {
      toast.success(t('verified'));
      await refresh();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const clearMutation = useMutation({
    mutationFn: () => CloudflareService.clearConnection(),
    onSuccess: async () => {
      toast.success(t('cleared'));
      setDNSAccountID('');
      setAPIToken('');
      await refresh();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-col gap-4'>
        <Button variant='ghost' size='sm' className='self-start' asChild>
          <Link href='/cloudflare'>
            <ArrowLeft data-icon='inline-start' />
            {t('back')}
          </Link>
        </Button>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
      </div>

      {connectionQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(connectionQuery.error)}
          onRetry={() => void connectionQuery.refetch()}
        />
      ) : null}

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-base'>{t('sourceTitle')}</CardTitle>
          <CardDescription>{t('sourceDesc')}</CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='cf-source'>{t('source')}</FieldLabel>
              <Select
                value={source}
                onValueChange={(value) =>
                  setSource(value as CloudflareConnectionSource)
                }
              >
                <SelectTrigger id='cf-source' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='dns_account'>
                      {t('importDns')}
                    </SelectItem>
                    <SelectItem value='standalone'>
                      {t('standalone')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>

            {source === 'dns_account' ? (
              <Field>
                <FieldLabel htmlFor='cf-dns-account'>
                  {t('dnsAccount')}
                </FieldLabel>
                <Select value={dnsAccountID} onValueChange={setDNSAccountID}>
                  <SelectTrigger id='cf-dns-account' className='w-full'>
                    <SelectValue placeholder={t('selectAccount')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {cloudflareAccounts.map((account) => (
                        <SelectItem key={account.id} value={String(account.id)}>
                          {account.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FieldDescription>
                  {t('dnsHint')}{' '}
                  <Link href='/dns-accounts'>{t('dnsLink')}</Link>
                  {t('dnsHintAfter')}
                </FieldDescription>
              </Field>
            ) : (
              <Field>
                <FieldLabel htmlFor='cf-api-token'>API Token</FieldLabel>
                <Input
                  id='cf-api-token'
                  type='password'
                  autoComplete='new-password'
                  value={apiToken}
                  onChange={(event) => setAPIToken(event.target.value)}
                  placeholder={
                    connectionQuery.data?.configured
                      ? t('tokenKeep')
                      : t('tokenEnter')
                  }
                />
              </Field>
            )}

            <div className='flex flex-wrap items-center gap-2'>
              <Button
                onClick={() => saveMutation.mutate()}
                disabled={
                  saveMutation.isPending ||
                  (source === 'dns_account' ? !dnsAccountID : !apiToken.trim())
                }
              >
                <Save data-icon='inline-start' />
                {t('save')}
              </Button>
              <Button
                variant='outline'
                onClick={() => verifyMutation.mutate()}
                disabled={
                  !connectionQuery.data?.configured || verifyMutation.isPending
                }
              >
                <ShieldCheck data-icon='inline-start' />
                {t('test')}
              </Button>
              <Button
                variant='destructive'
                onClick={() => clearMutation.mutate()}
                disabled={
                  !connectionQuery.data?.configured || clearMutation.isPending
                }
              >
                <Trash2 data-icon='inline-start' />
                {t('clear')}
              </Button>
            </div>
          </FieldGroup>
        </CardContent>
      </Card>

      <Alert>
        <ShieldCheck />
        <AlertTitle>{t('statusTitle')}</AlertTitle>
        <AlertDescription>
          {connectionQuery.data?.ready ? t('ready') : t('notReady')}
        </AlertDescription>
      </Alert>
    </div>
  );
}
