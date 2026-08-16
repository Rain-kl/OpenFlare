'use client';

import { useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
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
  ZoneService,
  zoneQueryKey,
  type ZoneItem,
} from '@/lib/services/openflare';
import { useTranslations } from 'next-intl';

type Values = { domain: string };

export function ZoneEditorDialog({
  open,
  onOpenChange,
  zone,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
  zone?: ZoneItem | null;
}) {
  const t = useTranslations('websites');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const schema = z.object({
    domain: z
      .string()
      .trim()
      .min(1, t('rootRequired'))
      .refine((value) => !/[*/?#@]|:\/\//.test(value), t('rootInvalid')),
  });
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { domain: '' },
  });
  useEffect(() => {
    if (open) form.reset({ domain: zone?.domain ?? '' });
  }, [form, open, zone]);
  const mutation = useMutation({
    mutationFn: (values: Values) =>
      zone
        ? ZoneService.update(zone.id, { domain: values.domain.toLowerCase() })
        : ZoneService.create({ domain: values.domain.toLowerCase() }),
    onSuccess: async () => {
      toast.success(zone ? t('zoneUpdated') : t('zoneCreated'));
      await queryClient.invalidateQueries({ queryKey: zoneQueryKey });
      onOpenChange(false);
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : t('saveFailed')),
  });
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{zone ? t('editZone') : t('createZone')}</DialogTitle>
          <DialogDescription>{t('editorDesc')}</DialogDescription>
        </DialogHeader>
        <form
          id='zone-editor'
          className='space-y-4'
          onSubmit={form.handleSubmit((values) =>
            mutation.mutate({ domain: values.domain.toLowerCase() }),
          )}
        >
          <div className='space-y-1.5'>
            <Label htmlFor='zone-domain'>{t('rootDomain')}</Label>
            <Input
              id='zone-domain'
              placeholder='example.com'
              {...form.register('domain')}
            />
            {form.formState.errors.domain && (
              <p className='text-xs text-destructive'>
                {form.formState.errors.domain.message}
              </p>
            )}
          </div>
        </form>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {tc('cancel')}
          </Button>
          <Button
            form='zone-editor'
            type='submit'
            disabled={mutation.isPending}
          >
            {mutation.isPending && (
              <Loader2 className='mr-1 size-4 animate-spin' />
            )}
            {zone ? t('saveChanges') : t('createZone')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
