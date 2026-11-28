'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

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
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import {
  cloudflareQueryKey,
  dnsAccountsQueryKey,
  DnsAccountService,
  type DnsAccountItem,
  type DnsAccountMutationPayload,
  type DnsProvider,
} from '@/lib/services/openflare';

import { getErrorMessage } from './website-utils';

const providers: DnsProvider[] = ['cloudflare', 'alidns', 'tencentcloud'];

interface DnsAccountCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated?: (account: DnsAccountItem) => void;
  account?: DnsAccountItem;
}

export function DnsAccountCreateDialog(props: DnsAccountCreateDialogProps) {
  const t = useTranslations('dnsAccounts');
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {props.open && (
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t(props.account ? 'editTitle' : 'createTitle')}
            </DialogTitle>
            <DialogDescription>{t('createDesc')}</DialogDescription>
          </DialogHeader>
          <DnsAccountForm key={props.account?.id ?? 'new'} {...props} />
        </DialogContent>
      )}
    </Dialog>
  );
}

function DnsAccountForm({
  account,
  onCreated,
  onOpenChange,
}: DnsAccountCreateDialogProps) {
  const t = useTranslations('dnsAccounts');
  const common = useTranslations('common');
  const queryClient = useQueryClient();
  const [provider, setProvider] = useState<DnsProvider>(
    account?.type ?? 'cloudflare',
  );
  const [name, setName] = useState(account?.name ?? '');
  const [identifier, setIdentifier] = useState('');
  const [secret, setSecret] = useState('');
  const isCloudflare = provider === 'cloudflare';

  const mutation = useMutation({
    mutationFn: (payload: DnsAccountMutationPayload) =>
      account
        ? DnsAccountService.update(account.id, payload)
        : DnsAccountService.create(payload),
    onSuccess: async (saved) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: dnsAccountsQueryKey }),
        queryClient.invalidateQueries({ queryKey: cloudflareQueryKey }),
      ]);
      onCreated?.(saved);
      onOpenChange(false);
    },
  });

  return (
    <form
      className='flex flex-col gap-5'
      onSubmit={(event) => {
        event.preventDefault();
        if (
          mutation.isPending ||
          !name.trim() ||
          !secret.trim() ||
          (!isCloudflare && !identifier.trim())
        )
          return;
        const id = identifier.trim();
        const key = secret.trim();
        const credentials =
          provider === 'alidns'
            ? { access_key_id: id, access_key_secret: key }
            : provider === 'tencentcloud'
              ? { secret_id: id, secret_key: key }
              : { api_token: key };
        mutation.mutate({
          name: name.trim(),
          type: provider,
          authorization:
            isCloudflare && key.startsWith('{')
              ? key
              : JSON.stringify(credentials),
        });
      }}
    >
      {mutation.isError && (
        <p role='alert' className='text-sm text-destructive'>
          {getErrorMessage(mutation.error, t('requestFailed'))}
        </p>
      )}
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor='dns-account-name'>{t('accountName')}</FieldLabel>
          <Input
            id='dns-account-name'
            placeholder={t('accountNamePlaceholder')}
            required
            maxLength={255}
            value={name}
            onChange={(event) => setName(event.target.value)}
            disabled={mutation.isPending}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='dns-account-provider'>
            {t('provider')}
          </FieldLabel>
          <Select
            value={provider}
            disabled={!!account || mutation.isPending}
            onValueChange={(value) => {
              const selected = providers.find((item) => item === value);
              if (!selected) return;
              setProvider(selected);
              setIdentifier('');
              setSecret('');
            }}
          >
            <SelectTrigger id='dns-account-provider' className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {providers.map((item) => (
                <SelectItem key={item} value={item}>
                  {t(`providers.${item}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {!isCloudflare && (
            <FieldDescription>{t('proxyUnsupported')}</FieldDescription>
          )}
        </Field>
        {!isCloudflare && (
          <Field>
            <FieldLabel htmlFor='dns-account-key-id'>
              {t(provider === 'alidns' ? 'accessKeyId' : 'secretId')}
            </FieldLabel>
            <Input
              id='dns-account-key-id'
              required
              value={identifier}
              onChange={(event) => setIdentifier(event.target.value)}
              disabled={mutation.isPending}
            />
          </Field>
        )}
        <Field>
          <FieldLabel htmlFor='dns-account-token'>
            {t(
              isCloudflare
                ? 'apiToken'
                : provider === 'alidns'
                  ? 'accessKeySecret'
                  : 'secretKey',
            )}
          </FieldLabel>
          <Input
            id='dns-account-token'
            required
            type='password'
            autoComplete='new-password'
            value={secret}
            onChange={(event) => setSecret(event.target.value)}
            disabled={mutation.isPending}
          />
        </Field>
      </FieldGroup>
      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          disabled={mutation.isPending}
          onClick={() => onOpenChange(false)}
        >
          {common('cancel')}
        </Button>
        <Button
          type='submit'
          disabled={
            mutation.isPending ||
            !name.trim() ||
            !secret.trim() ||
            (!isCloudflare && !identifier.trim())
          }
        >
          {mutation.isPending && <Spinner />}
          {t(mutation.isPending ? 'submitting' : 'submit')}
        </Button>
      </DialogFooter>
    </form>
  );
}
