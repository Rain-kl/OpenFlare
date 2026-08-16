'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

function buildCleanupSchema(t: (key: string) => string) {
  return z.object({
    keepCount: z.number().int().min(3, t('keepMin')),
  });
}

type CleanupFormValues = z.infer<ReturnType<typeof buildCleanupSchema>>;

interface CleanupDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (keepCount: number) => void;
  loading: boolean;
}

export function CleanupDialog({
  open,
  onOpenChange,
  onConfirm,
  loading,
}: CleanupDialogProps) {
  const t = useTranslations('configVersions');
  const tc = useTranslations('common');
  const form = useForm<CleanupFormValues>({
    resolver: zodResolver(buildCleanupSchema(t)),
    defaultValues: { keepCount: 10 },
  });

  useEffect(() => {
    if (open) {
      form.reset({ keepCount: 10 });
    }
  }, [form, open]);

  const handleConfirm = form.handleSubmit((values) => {
    onConfirm(values.keepCount);
  });

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('cleanupTitle')}</AlertDialogTitle>
          <AlertDialogDescription>{t('cleanupDesc')}</AlertDialogDescription>
        </AlertDialogHeader>

        <div className='space-y-2'>
          <Label htmlFor='keepCount'>{t('keepCount')}</Label>
          <Input
            id='keepCount'
            type='number'
            min={3}
            disabled={loading}
            {...form.register('keepCount', { valueAsNumber: true })}
          />
          <p className='text-xs text-muted-foreground'>{t('keepHint')}</p>
          {form.formState.errors.keepCount ? (
            <p className='text-xs text-destructive'>
              {form.formState.errors.keepCount.message}
            </p>
          ) : null}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>
            {tc('cancel')}
          </AlertDialogCancel>
          <Button
            variant='destructive'
            onClick={() => void handleConfirm()}
            disabled={loading}
          >
            {loading ? (
              <>
                <Loader2 className='size-4 animate-spin mr-1' />
                {t('cleaning')}
              </>
            ) : (
              t('confirmCleanup')
            )}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
