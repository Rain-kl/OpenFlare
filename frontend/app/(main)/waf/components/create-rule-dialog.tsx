'use client';

import { useTranslations } from 'next-intl';
import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';

interface CreateRuleDialogProps {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (name: string) => Promise<void>;
}

export function CreateRuleDialog({
  open,
  pending,
  onOpenChange,
  onCreate,
}: CreateRuleDialogProps) {
  const t = useTranslations('waf');
  const tCommon = useTranslations('common');
  const [name, setName] = useState('');

  useEffect(() => {
    if (!open) setName('');
  }, [open]);

  const trimmedName = name.trim();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form
          className='flex flex-col gap-4'
          onSubmit={async (event) => {
            event.preventDefault();
            if (!trimmedName || pending) return;
            await onCreate(trimmedName);
          }}
        >
          <DialogHeader>
            <DialogTitle>{t('createDialog.title')}</DialogTitle>
            <DialogDescription>
              {t('createDialog.description')}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='waf-rule-name'>
                {t('createDialog.name')}
              </FieldLabel>
              <Input
                id='waf-rule-name'
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t('createDialog.namePlaceholder')}
                autoComplete='off'
                autoFocus
              />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={pending}
              onClick={() => onOpenChange(false)}
            >
              {tCommon('cancel')}
            </Button>
            <Button type='submit' disabled={!trimmedName || pending}>
              {pending ? <Spinner data-icon='inline-start' /> : null}
              {t('createDialog.submit')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
