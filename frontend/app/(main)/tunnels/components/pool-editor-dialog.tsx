'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslations } from 'next-intl';
import { useForm } from 'react-hook-form';
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
import { Switch } from '@/components/ui/switch';
import type { TunnelPoolInput, TunnelPoolView } from '@/lib/services/openflare';

function buildSchema(t: (key: string) => string) {
  return z.object({
    pool_key: z
      .string()
      .trim()
      .min(1, t('pools.errKeyRequired'))
      .regex(/^[a-z0-9][a-z0-9_-]{0,63}$/, t('pools.errKeyFormat')),
    name: z.string().trim().min(1, t('pools.errNameRequired')).max(128),
    region: z.string().trim(),
    description: z.string().trim(),
    is_default: z.boolean(),
  });
}

type PoolFormValues = z.infer<ReturnType<typeof buildSchema>>;

const emptyValues: PoolFormValues = {
  pool_key: '',
  name: '',
  region: '',
  description: '',
  is_default: false,
};

export function PoolEditorDialog({
  open,
  pool,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  pool: TunnelPoolView | null;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: TunnelPoolInput) => Promise<void>;
}) {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const form = useForm<PoolFormValues>({
    resolver: zodResolver(buildSchema(t)),
    defaultValues: emptyValues,
  });

  useEffect(() => {
    if (open) {
      form.reset(
        pool
          ? {
              pool_key: pool.pool_key,
              name: pool.name,
              region: pool.region,
              description: pool.description,
              is_default: pool.is_default,
            }
          : emptyValues,
      );
    }
  }, [open, pool, form]);

  const handleSubmit = form.handleSubmit(async (values) => {
    await onSubmit({
      pool_key: values.pool_key,
      name: values.name,
      region: values.region,
      description: values.description,
      is_default: values.is_default,
    });
  });

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>
            {pool ? t('pools.editTitle') : t('pools.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('pools.editorDesc')}</DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(event) => void handleSubmit(event)}
          className='space-y-4'
        >
          <div className='space-y-2'>
            <Label htmlFor='pool-key'>{t('pools.poolKey')}</Label>
            <Input
              id='pool-key'
              disabled={Boolean(pool)}
              placeholder='ap-east'
              {...form.register('pool_key')}
            />
            {form.formState.errors.pool_key ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.pool_key.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-2'>
            <Label htmlFor='pool-name'>{t('pools.name')}</Label>
            <Input
              id='pool-name'
              placeholder='Asia-Pacific Pool'
              {...form.register('name')}
            />
            {form.formState.errors.name ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.name.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-2'>
            <Label htmlFor='pool-region'>{t('pools.region')}</Label>
            <Input
              id='pool-region'
              placeholder='ap-southeast'
              {...form.register('region')}
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='pool-desc'>{t('pools.description')}</Label>
            <Input id='pool-desc' {...form.register('description')} />
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('pools.isDefault')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('pools.isDefaultDesc')}
              </p>
            </div>
            <Switch
              checked={form.watch('is_default')}
              onCheckedChange={(checked) =>
                form.setValue('is_default', checked)
              }
            />
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={onClose}
              disabled={submitting}
            >
              {tc('cancel')}
            </Button>
            <Button type='submit' disabled={submitting}>
              {submitting ? t('common.saving') : t('common.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
