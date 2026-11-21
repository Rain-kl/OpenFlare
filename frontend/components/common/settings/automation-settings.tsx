'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import services from '@/lib/services';
import type { SystemConfig } from '@/lib/services/admin';

export function AutomationSettings({ config }: { config: SystemConfig }) {
  const t = useTranslations('settings.operation');
  const queryClient = useQueryClient();
  const [address, setAddress] = useState(config.value);
  const update = useMutation({
    mutationFn: (value: string) =>
      services.adminSystemConfig.updateSystemConfig(config.key, {
        value,
        description: config.description,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      toast.success(t('automationSaved'));
    },
    onError: () => toast.error(t('automationSaveError')),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('automationTitle')}</CardTitle>
        <CardDescription>{t('automationDescription')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            update.mutate(address.trim() || 'disabled');
          }}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='automation-address'>
                {t('automationAddress')}
              </FieldLabel>
              <Input
                id='automation-address'
                type='url'
                placeholder='https://openflare.example.com'
                value={address}
                onChange={(event) => setAddress(event.target.value)}
                disabled={update.isPending}
              />
            </Field>
            <div className='flex flex-wrap gap-2'>
              <Button
                type='submit'
                disabled={update.isPending || address === config.value}
              >
                {t('save')}
              </Button>
            </div>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  );
}
