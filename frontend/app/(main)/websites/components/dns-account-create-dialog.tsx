'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { Loader2 } from 'lucide-react';
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
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import type { DnsAccountMutationPayload } from '@/lib/services/openflare';
import { DnsAccountService } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import { getErrorMessage } from './website-utils';

const dnsAccountsQueryKey = ['openflare', 'dns-accounts'];

type DnsAccountFormValues = {
  name: string;
  authorization: string;
};

interface DnsAccountCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated?: () => void;
}

export function DnsAccountCreateDialog({
  open,
  onOpenChange,
  onCreated,
}: DnsAccountCreateDialogProps) {
  const t = useTranslations('dnsAccounts');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const dnsAccountSchema = z.object({
    name: z.string().trim().min(1, t('nameRequired')).max(255),
    authorization: z.string().trim().min(1, t('tokenRequired')),
  });
  const form = useForm<DnsAccountFormValues>({
    resolver: zodResolver(dnsAccountSchema),
    defaultValues: { name: '', authorization: '' },
  });

  const createMutation = useMutation({
    mutationFn: (payload: DnsAccountMutationPayload) =>
      DnsAccountService.create(payload),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: dnsAccountsQueryKey });
      form.reset();
      setError('');
      onCreated?.();
      onOpenChange(false);
    },
    onError: (err) => setError(getErrorMessage(err, t('requestFailed'))),
  });

  const onSubmit = form.handleSubmit((values) => {
    setError('');
    let auth = values.authorization.trim();
    if (!auth.startsWith('{')) {
      auth = JSON.stringify({ api_token: values.authorization });
    }
    createMutation.mutate({
      ...values,
      type: 'cloudflare',
      authorization: auth,
    });
  });

  const handleClose = () => {
    if (createMutation.isPending) return;
    form.reset();
    setError('');
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && handleClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('createTitle')}</DialogTitle>
          <DialogDescription>{t('createDesc')}</DialogDescription>
        </DialogHeader>

        <form className='flex flex-col gap-5' onSubmit={onSubmit}>
          {error && (
            <p role='alert' className='text-sm text-destructive'>
              {error}
            </p>
          )}
          <FieldGroup>
            <Field data-invalid={!!form.formState.errors.name}>
              <FieldLabel htmlFor='dns-account-name'>
                {t('accountName')}
              </FieldLabel>
              <Input
                id='dns-account-name'
                placeholder={t('accountNamePlaceholder')}
                aria-invalid={!!form.formState.errors.name}
                {...form.register('name')}
              />
              <FieldError errors={[form.formState.errors.name]} />
            </Field>
            <Field data-invalid={!!form.formState.errors.authorization}>
              <FieldLabel htmlFor='dns-account-token'>
                Cloudflare API Token
              </FieldLabel>
              <Input
                id='dns-account-token'
                type='password'
                autoComplete='new-password'
                aria-invalid={!!form.formState.errors.authorization}
                {...form.register('authorization')}
                placeholder={t('tokenPlaceholder')}
              />
              <FieldError errors={[form.formState.errors.authorization]} />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={createMutation.isPending}
              onClick={handleClose}
            >
              {tc('cancel')}
            </Button>
            <Button type='submit' disabled={createMutation.isPending}>
              {createMutation.isPending ? (
                <>
                  <Loader2 className='mr-1 size-3.5 animate-spin' />
                  {t('submitting')}
                </>
              ) : (
                t('submit')
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
