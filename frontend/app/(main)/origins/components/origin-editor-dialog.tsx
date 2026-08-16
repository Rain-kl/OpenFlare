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
import { Textarea } from '@/components/ui/textarea';
import {
  type OriginItem,
  type OriginMutationPayload,
  OriginService,
} from '@/lib/services/openflare';
import { useTranslations } from 'next-intl';

type OriginFormValues = {
  name: string;
  address: string;
  remark: string;
};

const originsQueryKey = ['openflare', 'origins'] as const;

function toFormValues(origin?: OriginItem | null): OriginFormValues {
  if (!origin) return { name: '', address: '', remark: '' };
  return {
    name: origin.name,
    address: origin.address,
    remark: origin.remark || '',
  };
}

function toPayload(values: OriginFormValues): OriginMutationPayload {
  return {
    name: values.name.trim(),
    address: values.address.trim(),
    remark: values.remark.trim(),
  };
}

interface OriginEditorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  origin?: OriginItem | null;
  onSaved?: () => void;
}

export function OriginEditorDialog({
  open,
  onOpenChange,
  origin,
  onSaved,
}: OriginEditorDialogProps) {
  const t = useTranslations('origins');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const originSchema = z.object({
    name: z.string().max(255),
    address: z
      .string()
      .trim()
      .min(1, t('addressRequired'))
      .refine(
        (value) => !/[/?#]/.test(value) && !value.includes('://'),
        t('addressInvalid'),
      ),
    remark: z.string().max(255),
  });
  const form = useForm<OriginFormValues>({
    resolver: zodResolver(originSchema),
    defaultValues: toFormValues(origin),
  });

  useEffect(() => {
    if (open) form.reset(toFormValues(origin));
  }, [form, origin, open]);

  const mutation = useMutation({
    mutationFn: async (values: OriginFormValues) => {
      const payload = toPayload(values);
      return origin
        ? OriginService.update(origin.id, payload)
        : OriginService.create(payload);
    },
    onSuccess: async () => {
      toast.success(origin ? t('updated') : t('created'));
      await queryClient.invalidateQueries({ queryKey: originsQueryKey });
      if (origin) {
        await queryClient.invalidateQueries({
          queryKey: ['openflare', 'origins', String(origin.id)],
        });
      }
      onSaved?.();
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('saveFailed'));
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{origin ? t('editOrigin') : t('create')}</DialogTitle>
          <DialogDescription>{t('editorDesc')}</DialogDescription>
        </DialogHeader>

        <form
          id='origin-editor-form'
          className='space-y-4'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <div className='space-y-1.5'>
            <Label htmlFor='address'>{t('address')}</Label>
            <Input
              id='address'
              placeholder='origin.internal'
              {...form.register('address')}
            />
            {form.formState.errors.address ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.address.message}
              </p>
            ) : null}
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='name'>{t('name')}</Label>
            <Input
              id='name'
              placeholder={t('namePlaceholder')}
              {...form.register('name')}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='remark'>{t('remark')}</Label>
            <Textarea id='remark' rows={2} {...form.register('remark')} />
          </div>
        </form>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {tc('cancel')}
          </Button>
          <Button
            type='submit'
            form='origin-editor-form'
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <>
                <Loader2 className='size-4 animate-spin mr-1' />
                {t('saving')}
              </>
            ) : origin ? (
              t('saveChanges')
            ) : (
              t('create')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
