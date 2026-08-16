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
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { DnsAccountMutationPayload } from '@/lib/services/openflare';
import { DnsAccountService } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import { getErrorMessage } from './website-utils';

const dnsAccountsQueryKey = ['openflare', 'dns-accounts'];

type DnsAccountFormValues = {
  name: string;
  type: string;
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
    type: z.string().min(1),
    authorization: z.string().trim().min(1, t('tokenRequired')),
  });
  const form = useForm<DnsAccountFormValues>({
    resolver: zodResolver(dnsAccountSchema),
    defaultValues: { name: '', type: 'cloudflare', authorization: '' },
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
    createMutation.mutate({ ...values, authorization: auth });
  });

  const handleClose = () => {
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

        <form className='space-y-4' onSubmit={onSubmit}>
          {error ? <p className='text-sm text-destructive'>{error}</p> : null}

          <div className='space-y-2'>
            <Label>{t('accountName')}</Label>
            <Input
              placeholder={t('accountNamePlaceholder')}
              {...form.register('name')}
            />
            {form.formState.errors.name ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.name.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-2'>
            <Label>{t('provider')}</Label>
            <Select
              value={form.watch('type')}
              onValueChange={(value) => form.setValue('type', value)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='cloudflare'>Cloudflare</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>API Token</Label>
            <Input
              {...form.register('authorization')}
              placeholder={t('tokenPlaceholder')}
            />
            {form.formState.errors.authorization ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.authorization.message}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button type='button' variant='outline' onClick={handleClose}>
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
