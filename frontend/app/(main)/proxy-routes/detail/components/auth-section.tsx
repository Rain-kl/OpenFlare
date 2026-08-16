'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { toast } from 'sonner';
import { z } from 'zod';

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { ProxyRouteItem } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import { proxyRouteFormIds } from '../helpers';
import { useRouteSectionSave } from '../hooks/use-route-section-save';
import { SectionShell } from './section-shell';

type AuthValues = {
  auth_mode: 'none' | 'basic';
  basic_auth_username: string;
  basic_auth_password: string;
};

function resolveAuthMode(route: ProxyRouteItem): AuthValues['auth_mode'] {
  return route.basic_auth_enabled ? 'basic' : 'none';
}

interface AuthSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function AuthSection({
  route,
  onRouteUpdate,
  onSavingChange,
}: AuthSectionProps) {
  const t = useTranslations('proxyRoutes');
  const authSchema = z
    .object({
      auth_mode: z.enum(['none', 'basic']),
      basic_auth_username: z.string(),
      basic_auth_password: z.string(),
    })
    .superRefine((value, context) => {
      if (value.auth_mode !== 'basic') {
        return;
      }
      if (!value.basic_auth_username.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['basic_auth_username'],
          message: t('validation.enterUsername'),
        });
      }
      if (!value.basic_auth_password.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['basic_auth_password'],
          message: t('validation.enterPassword'),
        });
      }
    });
  const { saving, save } = useRouteSectionSave(
    route,
    onRouteUpdate,
    onSavingChange,
  );

  const form = useForm<AuthValues>({
    resolver: zodResolver(authSchema),
    defaultValues: {
      auth_mode: resolveAuthMode(route),
      basic_auth_username: route.basic_auth_username || '',
      basic_auth_password: route.basic_auth_password || '',
    },
  });

  useEffect(() => {
    form.reset({
      auth_mode: resolveAuthMode(route),
      basic_auth_username: route.basic_auth_username || '',
      basic_auth_password: route.basic_auth_password || '',
    });
  }, [form, route]);

  const authMode = form.watch('auth_mode');

  return (
    <SectionShell
      title={t('auth')}
      description={t('authDesc')}
      formId={proxyRouteFormIds.auth}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.auth}
          className='space-y-5'
          onSubmit={form.handleSubmit(
            async (values) => {
              await save(
                {
                  basic_auth_enabled: values.auth_mode === 'basic',
                  basic_auth_username:
                    values.auth_mode === 'basic'
                      ? values.basic_auth_username.trim()
                      : '',
                  basic_auth_password:
                    values.auth_mode === 'basic'
                      ? values.basic_auth_password.trim()
                      : '',
                },
                t('authSaved'),
              );
            },
            () => {
              toast.error(t('authFormInvalid'));
            },
          )}
        >
          <FormField
            control={form.control}
            name='auth_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('authMode')}</FormLabel>
                <Select
                  value={field.value}
                  onValueChange={(mode) => {
                    field.onChange(mode);
                    if (mode !== 'basic') {
                      form.setValue('basic_auth_username', '');
                      form.setValue('basic_auth_password', '');
                    }
                  }}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='none'>{t('authNone')}</SelectItem>
                    <SelectItem value='basic'>Basic Auth</SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>{t('powHint')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {authMode === 'basic' ? (
            <>
              <FormField
                control={form.control}
                name='basic_auth_username'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('username')}</FormLabel>
                    <FormControl>
                      <Input placeholder='admin' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='basic_auth_password'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('password')}</FormLabel>
                    <FormControl>
                      <Input type='text' placeholder='secret123' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          ) : null}
        </form>
      </Form>
    </SectionShell>
  );
}
