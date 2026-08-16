'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { CircleHelp, TriangleAlert } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import type { ProxyRouteItem } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import {
  linesFromTextarea,
  validateCacheRules,
} from '../../components/helpers';
import { proxyRouteFormIds } from '../helpers';
import { useRouteSectionSave } from '../hooks/use-route-section-save';
import { SectionShell } from './section-shell';

type CacheValues = {
  cache_enabled: boolean;
  cache_policy: 'static' | 'all' | 'suffix' | 'path_prefix' | 'path_exact';
  cache_rules_text: string;
};

interface CacheSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

/** Map API/DB values for the form. Legacy empty/url → all (compat). */
function normalizeCachePolicyValue(
  policy: string | undefined | null,
  enabled = true,
) {
  if (!enabled) {
    return 'static';
  }
  const value = (policy || '').trim();
  if (!value || value === 'url' || value === 'all') return 'all';
  if (value === 'static') return 'static';
  if (value === 'suffix' || value === 'path_prefix' || value === 'path_exact') {
    return value;
  }
  return 'static';
}

function needsRulesForPolicy(policy: string) {
  return (
    policy === 'suffix' || policy === 'path_prefix' || policy === 'path_exact'
  );
}

function CacheHelpButton() {
  const t = useTranslations('proxyRoutes');
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          className='size-7 text-muted-foreground'
          aria-label={t('cacheHelpTitle')}
        >
          <CircleHelp />
        </Button>
      </PopoverTrigger>
      <PopoverContent align='start' className='w-80 flex flex-col gap-3 p-4'>
        <div className='text-sm font-medium'>{t('cacheHelpTitle')}</div>
        <div className='flex flex-col gap-2 text-xs text-muted-foreground leading-relaxed'>
          <p>{t('cacheHelpIntro')}</p>
          <p>
            <span className='font-medium text-foreground'>
              {t('cacheHelpRecommendLabel')}
            </span>
            {t('cacheHelpRecommend')}
          </p>
          <p>
            <span className='font-medium text-foreground'>
              {t('cacheHelpGeneralLabel')}
            </span>
            {t('cacheHelpGeneral')}
          </p>
          <p>
            <span className='font-medium text-foreground'>
              {t('cacheHelpAllGetLabel')}
            </span>
            {t('cacheHelpAllGet')}
          </p>
          <p>
            <span className='font-medium text-foreground'>
              {t('cacheHelpCustomLabel')}
            </span>
            {t('cacheHelpCustom')}
          </p>
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function CacheSection({
  route,
  onRouteUpdate,
  onSavingChange,
}: CacheSectionProps) {
  const t = useTranslations('proxyRoutes');
  const cacheSchema = z
    .object({
      cache_enabled: z.boolean(),
      cache_policy: z.enum([
        'static',
        'all',
        'suffix',
        'path_prefix',
        'path_exact',
      ]),
      cache_rules_text: z.string(),
    })
    .superRefine((value, context) => {
      if (!value.cache_enabled) {
        return;
      }

      const rules = linesFromTextarea(value.cache_rules_text);
      const error = validateCacheRules(value.cache_policy, rules, t);
      if (error) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['cache_rules_text'],
          message: error,
        });
      }
    });
  const { saving, save } = useRouteSectionSave(
    route,
    onRouteUpdate,
    onSavingChange,
  );

  const form = useForm<CacheValues>({
    resolver: zodResolver(cacheSchema),
    defaultValues: {
      cache_enabled: route.cache_enabled,
      cache_policy: normalizeCachePolicyValue(
        route.cache_policy,
        route.cache_enabled,
      ) as CacheValues['cache_policy'],
      cache_rules_text: route.cache_rule_list.join('\n'),
    },
  });

  useEffect(() => {
    form.reset({
      cache_enabled: route.cache_enabled,
      cache_policy: normalizeCachePolicyValue(
        route.cache_policy,
        route.cache_enabled,
      ) as CacheValues['cache_policy'],
      cache_rules_text: route.cache_rule_list.join('\n'),
    });
  }, [form, route]);

  const watchedEnabled = form.watch('cache_enabled');
  const watchedPolicy = form.watch('cache_policy');
  const needsRules =
    watchedPolicy === 'suffix' ||
    watchedPolicy === 'path_prefix' ||
    watchedPolicy === 'path_exact';

  const rulesHint =
    watchedPolicy === 'suffix'
      ? t('cacheHintSuffix')
      : watchedPolicy === 'path_prefix'
        ? t('cacheHintPrefix')
        : watchedPolicy === 'path_exact'
          ? t('cacheHintExact')
          : watchedPolicy === 'static'
            ? t('cacheHintStatic')
            : t('cacheHintNone');

  const rulesPlaceholder =
    watchedPolicy === 'suffix'
      ? 'jpg\ncss\njs'
      : watchedPolicy === 'path_prefix'
        ? '/assets\n/static'
        : watchedPolicy === 'path_exact'
          ? '/robots.txt\n/manifest.json'
          : t('cachePlaceholderNone');

  return (
    <SectionShell
      title={t('cache')}
      description={t('cacheDesc')}
      titleExtra={<CacheHelpButton />}
      formId={proxyRouteFormIds.cache}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.cache}
          className='space-y-5'
          onSubmit={form.handleSubmit(async (values) => {
            const rules = linesFromTextarea(values.cache_rules_text);
            await save(
              {
                cache_enabled: values.cache_enabled,
                cache_policy: values.cache_enabled ? values.cache_policy : '',
                cache_rules:
                  values.cache_enabled &&
                  needsRulesForPolicy(values.cache_policy)
                    ? rules
                    : [],
              },
              t('cacheSaved'),
            );
          })}
        >
          <FormField
            control={form.control}
            name='cache_enabled'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between rounded-lg border p-3'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('enableCache')}</FormLabel>
                  <FormDescription>{t('enableCacheDesc')}</FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='cache_policy'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('cachePolicy')}</FormLabel>
                <Select
                  disabled={!watchedEnabled}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='static'>{t('policyStatic')}</SelectItem>
                    <SelectItem value='all'>{t('policyAll')}</SelectItem>
                    <SelectItem value='suffix'>{t('policySuffix')}</SelectItem>
                    <SelectItem value='path_prefix'>
                      {t('policyPrefix')}
                    </SelectItem>
                    <SelectItem value='path_exact'>
                      {t('policyExact')}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          {watchedEnabled && watchedPolicy === 'all' ? (
            <Alert>
              <TriangleAlert />
              <AlertTitle>{t('advancedRiskTitle')}</AlertTitle>
              <AlertDescription>{t('advancedRiskDesc')}</AlertDescription>
            </Alert>
          ) : null}

          <FormField
            control={form.control}
            name='cache_rules_text'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('cacheRules')}</FormLabel>
                <FormControl>
                  <Textarea
                    className='min-h-32'
                    disabled={!watchedEnabled || !needsRules}
                    placeholder={rulesPlaceholder}
                    {...field}
                  />
                </FormControl>
                <FormDescription>{rulesHint}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </SectionShell>
  );
}
