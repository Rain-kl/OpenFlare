'use client';

import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const CLEANUP_PRESETS = [3, 7, 30];

interface CleanupDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (retentionDays: number) => void;
  loading: boolean;
}

export function CleanupDialog({
  open,
  onOpenChange,
  onConfirm,
  loading,
}: CleanupDialogProps) {
  const t = useTranslations('accessLogs.cleanupDialog');
  const tCommon = useTranslations('common');
  const [mode, setMode] = useState<string>('7');
  const [customDays, setCustomDays] = useState('14');
  const [error, setError] = useState<string | null>(null);

  const handleConfirm = () => {
    const retentionDays =
      mode === 'custom'
        ? Number.parseInt(customDays, 10)
        : Number.parseInt(mode, 10);

    if (!Number.isFinite(retentionDays) || retentionDays < 1) {
      setError(t('invalidDays'));
      return;
    }

    setError(null);
    onConfirm(retentionDays);
  };

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('title')}</AlertDialogTitle>
          <AlertDialogDescription>{t('description')}</AlertDialogDescription>
        </AlertDialogHeader>

        <div className='space-y-3'>
          <div className='space-y-1.5'>
            <Label>{t('policy')}</Label>
            <Select value={mode} onValueChange={setMode}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CLEANUP_PRESETS.map((days) => (
                  <SelectItem key={days} value={String(days)}>
                    {t('keepDays', { days })}
                  </SelectItem>
                ))}
                <SelectItem value='custom'>{t('custom')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {mode === 'custom' ? (
            <div className='space-y-1.5'>
              <Label htmlFor='customDays'>{t('customDays')}</Label>
              <Input
                id='customDays'
                type='number'
                min={1}
                value={customDays}
                onChange={(e) => setCustomDays(e.target.value)}
                disabled={loading}
              />
            </div>
          ) : null}

          {error ? <p className='text-xs text-destructive'>{error}</p> : null}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>
            {tCommon('cancel')}
          </AlertDialogCancel>
          <Button
            variant='destructive'
            onClick={handleConfirm}
            disabled={loading}
          >
            {loading ? (
              <>
                <Loader2 className='size-4 animate-spin mr-1' />
                {t('cleaning')}
              </>
            ) : (
              t('confirm')
            )}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
