'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Cloud, ShieldCheck } from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
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
import { Spinner } from '@/components/ui/spinner';
import {
  CloudflareService,
  cloudflareQueryKey,
  DnsAccountService,
  type CloudflareConnectionSource,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { getErrorMessage } from '../../websites/components/website-utils';

const connectionKey = [...cloudflareQueryKey, 'connection'] as const;

export default function DNSSettingsPage() {
  const t = useTranslations('cloudflare.settings');
  const common = useTranslations('common');
  const queryClient = useQueryClient();
  const [draftSource, setSource] = useState<CloudflareConnectionSource>();
  const [draftAccountID, setAccountID] = useState<string>();
  const [apiToken, setAPIToken] = useState('');
  const [clearOpen, setClearOpen] = useState(false);

  const connectionQuery = useQuery({
    queryKey: connectionKey,
    queryFn: () => CloudflareService.getConnection(),
  });
  const connection = connectionQuery.data;
  const source = draftSource ?? (connection?.source || 'dns_account');
  const savedAccountID = connection?.dns_account_id
    ? String(connection.dns_account_id)
    : '';
  const accountID = draftAccountID ?? savedAccountID;
  const accountsQuery = useQuery({
    queryKey: ['openflare', 'dns-accounts'],
    queryFn: () => DnsAccountService.list(),
    enabled: source === 'dns_account',
  });
  const accounts = (accountsQuery.data ?? []).filter(
    (account) => account.type === 'cloudflare',
  );
  const isAccountSource = source === 'dns_account';
  const dirty =
    source !== connection?.source ||
    (isAccountSource ? accountID !== savedAccountID : !!apiToken.trim());
  const valid = isAccountSource
    ? accountsQuery.isSuccess &&
      accounts.some((account) => String(account.id) === accountID)
    : !!apiToken.trim();

  const resetDraft = () => {
    setSource(undefined);
    setAccountID(undefined);
    setAPIToken('');
  };
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });

  const verifyMutation = useMutation({
    mutationFn: () => CloudflareService.verifyConnection(),
    onSuccess: (data) => {
      queryClient.setQueryData(connectionKey, data);
      toast.success(t('verified'));
    },
    onSettled: refresh,
  });
  const saveMutation = useMutation({
    mutationFn: () =>
      CloudflareService.saveConnection({
        source,
        dns_account_id: isAccountSource ? Number(accountID) : 0,
        api_token: isAccountSource ? '' : apiToken.trim(),
      }),
    onSuccess: (data) => {
      queryClient.setQueryData(connectionKey, data);
      resetDraft();
      toast.success(t('saved'));
      verifyMutation.mutate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const clearMutation = useMutation({
    mutationFn: () => CloudflareService.clearConnection(),
    onSuccess: async () => {
      resetDraft();
      verifyMutation.reset();
      await refresh();
      setClearOpen(false);
      toast.success(t('cleared'));
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const saving = saveMutation.isPending;
  const busy = saving || verifyMutation.isPending || clearMutation.isPending;
  const status = connection?.ready
    ? 'ready'
    : connection?.status === 'error'
      ? 'failed'
      : connection?.configured
        ? 'unverified'
        : 'unconfigured';

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <Button asChild variant='ghost' size='sm'>
          <Link href='/cloudflare'>
            <ArrowLeft data-icon='inline-start' />
            {t('back')}
          </Link>
        </Button>
      </div>

      {connectionQuery.isPending ? (
        <LoadingStateWithBorder icon={Cloud} description={t('loading')} />
      ) : connectionQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(connectionQuery.error)}
          onRetry={() => void connectionQuery.refetch()}
        />
      ) : (
        <div className='grid min-w-0 items-start gap-5 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]'>
          <Card>
            <CardHeader>
              <CardTitle className='text-base'>{t('sourceTitle')}</CardTitle>
              <CardDescription>{t('sourceDesc')}</CardDescription>
            </CardHeader>
            <form
              onSubmit={(event) => {
                event.preventDefault();
                if (dirty && valid && !busy) saveMutation.mutate();
              }}
            >
              <CardContent>
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor='cf-source'>{t('source')}</FieldLabel>
                    <Select
                      value={source}
                      disabled={busy}
                      onValueChange={(value) => {
                        setSource(value as CloudflareConnectionSource);
                        setAPIToken('');
                      }}
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
                  {isAccountSource ? (
                    <Field>
                      <FieldLabel htmlFor='cf-dns-account'>
                        {t('dnsAccount')}
                      </FieldLabel>
                      <Select
                        value={accountID}
                        onValueChange={setAccountID}
                        disabled={
                          busy || !accounts.length || accountsQuery.isError
                        }
                      >
                        <SelectTrigger id='cf-dns-account' className='w-full'>
                          <SelectValue placeholder={t('selectAccount')} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            {accounts.map((account) => (
                              <SelectItem
                                key={account.id}
                                value={String(account.id)}
                              >
                                {account.name}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      {accountsQuery.isError ? (
                        <ErrorInline
                          message={getErrorMessage(accountsQuery.error)}
                          onRetry={() => void accountsQuery.refetch()}
                        />
                      ) : (
                        <FieldDescription>
                          {accountsQuery.isPending
                            ? t('loading')
                            : accounts.length
                              ? null
                              : t('noAccounts')}{' '}
                          <Link href='/dns-accounts'>
                            {t('manageAccounts')}
                          </Link>
                        </FieldDescription>
                      )}
                    </Field>
                  ) : (
                    <Field>
                      <FieldLabel htmlFor='cf-api-token'>API Token</FieldLabel>
                      <Input
                        id='cf-api-token'
                        type='password'
                        autoComplete='new-password'
                        value={apiToken}
                        disabled={busy}
                        onChange={(event) => setAPIToken(event.target.value)}
                        placeholder={t('tokenEnter')}
                      />
                    </Field>
                  )}
                </FieldGroup>
              </CardContent>
              <CardFooter className='mt-5 flex-wrap gap-2'>
                {dirty || !connection?.configured ? (
                  <Button type='submit' disabled={!dirty || !valid || busy}>
                    {saving ? (
                      <Spinner />
                    ) : (
                      <ShieldCheck data-icon='inline-start' />
                    )}
                    {t('saveAndTest')}
                  </Button>
                ) : (
                  <Button
                    type='button'
                    variant={connection.ready ? 'outline' : 'default'}
                    disabled={busy}
                    onClick={() => verifyMutation.mutate()}
                  >
                    {verifyMutation.isPending ? (
                      <Spinner />
                    ) : (
                      <ShieldCheck data-icon='inline-start' />
                    )}
                    {t('test')}
                  </Button>
                )}
                {dirty && connection?.configured && (
                  <Button
                    type='button'
                    variant='ghost'
                    disabled={busy}
                    onClick={resetDraft}
                  >
                    {t('discard')}
                  </Button>
                )}
              </CardFooter>
            </form>
          </Card>

          <Card>
            <CardHeader>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <CardTitle className='text-base'>{t('statusTitle')}</CardTitle>
                <Badge variant={status === 'ready' ? 'default' : 'secondary'}>
                  {t(`${status}Label`)}
                </Badge>
              </div>
              <CardDescription>
                {t(status === 'unverified' ? 'notReady' : status)}
              </CardDescription>
            </CardHeader>
            {(connection?.verified_at || verifyMutation.error) && (
              <CardContent className='flex flex-col gap-2 text-sm text-muted-foreground'>
                {connection?.verified_at && (
                  <p>
                    {t('verifiedAt', {
                      date: formatDateTime(connection.verified_at),
                    })}
                  </p>
                )}
                {verifyMutation.error && (
                  <p role='alert' className='text-destructive'>
                    {getErrorMessage(verifyMutation.error)}
                  </p>
                )}
              </CardContent>
            )}
            {connection?.configured && (
              <CardFooter>
                <Button
                  variant='ghost'
                  size='sm'
                  disabled={busy}
                  onClick={() => setClearOpen(true)}
                >
                  {t('clear')}
                </Button>
              </CardFooter>
            )}
          </Card>
        </div>
      )}

      <AlertDialog
        open={clearOpen}
        onOpenChange={(open) => !busy && setClearOpen(open)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('clear')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('clearDescription')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>
              {common('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              disabled={busy}
              onClick={() => clearMutation.mutate()}
            >
              {clearMutation.isPending && <Spinner />}
              {t('clear')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
