'use client';

import { useEffect, useMemo } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus } from 'lucide-react';
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
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import type {
  WAFIPGroup,
  WAFIPGroupPayload,
  WAFIPGroupSubscriptionFormat,
  WAFIPGroupType,
} from '@/lib/services/openflare';

import {
  automaticPresetRules,
  listToText,
  parseAutomaticConfig,
  parseTextareaList,
} from './helpers';

function createIPGroupSchema(
  t: (key: string) => string,
  tWaf: (key: string) => string,
) {
  return z
    .object({
      name: z.string().trim().min(1, t('dialog.nameRequired')).max(255),
      type: z.enum(['manual', 'automatic', 'subscription']),
      enabled: z.boolean(),
      ip_list_text: z.string(),
      auto_config_text: z.string(),
      subscription_url: z.string(),
      subscription_format: z.enum(['text', 'json']),
      subscription_mapping_rule: z.string(),
      sync_interval_minutes: z.number().int().min(1),
    })
    .superRefine((value, context) => {
      if (value.type === 'subscription' && !value.subscription_url.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['subscription_url'],
          message: t('dialog.subscriptionUrlRequired'),
        });
      }
      if (value.type === 'automatic') {
        try {
          parseAutomaticConfig(
            value.auto_config_text,
            tWaf('autoConfigMustBeObject'),
          );
        } catch (error) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['auto_config_text'],
            message:
              error instanceof Error
                ? error.message
                : tWaf('autoConfigInvalid'),
          });
        }
      }
    });
}

type IPGroupFormValues = z.infer<ReturnType<typeof createIPGroupSchema>>;

const defaultValues: IPGroupFormValues = {
  name: '',
  type: 'manual',
  enabled: true,
  ip_list_text: '',
  auto_config_text: JSON.stringify(
    { lookback: '1h', ttl: -1, rules: [] },
    null,
    2,
  ),
  subscription_url: '',
  subscription_format: 'text',
  subscription_mapping_rule: '',
  sync_interval_minutes: 1440,
};

function buildFormValues(group: WAFIPGroup | null): IPGroupFormValues {
  if (!group) return defaultValues;
  return {
    name: group.name,
    type: group.type,
    enabled: group.enabled,
    ip_list_text: listToText(group.ip_list),
    auto_config_text: JSON.stringify(group.auto_config ?? {}, null, 2),
    subscription_url: group.subscription_url ?? '',
    subscription_format: group.subscription_format ?? 'text',
    subscription_mapping_rule: group.subscription_mapping_rule ?? '',
    sync_interval_minutes: group.sync_interval_minutes || 1440,
  };
}

function buildPayload(
  values: IPGroupFormValues,
  invalidMessage: string,
): WAFIPGroupPayload {
  const autoConfig =
    values.type === 'automatic'
      ? parseAutomaticConfig(values.auto_config_text, invalidMessage)
      : {};
  return {
    name: values.name.trim(),
    type: values.type,
    enabled: values.enabled,
    ip_list: parseTextareaList(values.ip_list_text),
    auto_config: autoConfig,
    subscription_url: values.subscription_url.trim(),
    subscription_format: values.subscription_format,
    subscription_mapping_rule: values.subscription_mapping_rule.trim(),
    sync_interval_minutes: values.sync_interval_minutes,
  };
}

function appendAutomaticPresetRule(
  autoConfigText: string,
  rule: (typeof automaticPresetRules)[number],
  invalidMessage: string,
) {
  const config = parseAutomaticConfig(autoConfigText, invalidMessage);
  const rules = Array.isArray(config.rules) ? config.rules : [];
  const exists = rules.some(
    (item) =>
      item &&
      typeof item === 'object' &&
      'expr' in item &&
      (item as { expr?: unknown }).expr === rule.expr,
  );
  const nextRules = exists ? rules : [...rules, rule];
  const lookback =
    typeof config.lookback === 'string' && config.lookback.trim()
      ? config.lookback
      : typeof config.lookback_minutes === 'number'
        ? `${config.lookback_minutes}m`
        : '1h';
  // strip legacy field so saved JSON only keeps lookback duration string
  const {
    lookback_minutes: _legacyLookbackMinutes,
    lookback: _existingLookback,
    ...rest
  } = config;
  return JSON.stringify(
    {
      ...rest,
      lookback,
      ttl: typeof rest.ttl === 'number' ? rest.ttl : -1,
      rules: nextRules,
    },
    null,
    2,
  );
}

interface IPGroupDialogProps {
  open: boolean;
  group: WAFIPGroup | null;
  submitting: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: WAFIPGroupPayload) => Promise<void>;
}

export function IPGroupDialog({
  open,
  group,
  submitting,
  onOpenChange,
  onSubmit,
}: IPGroupDialogProps) {
  const t = useTranslations('ipGroups');
  const tWaf = useTranslations('waf');
  const tCommon = useTranslations('common');
  const ipGroupSchema = useMemo(
    () =>
      createIPGroupSchema(
        (key) => t(key),
        (key) => tWaf(key),
      ),
    [t, tWaf],
  );
  const form = useForm<IPGroupFormValues>({
    resolver: zodResolver(ipGroupSchema),
    defaultValues,
  });

  const type = form.watch('type');

  useEffect(() => {
    if (!open) return;
    form.reset(buildFormValues(group));
  }, [form, group, open]);

  const handleSubmit = form.handleSubmit(async (values) => {
    try {
      await onSubmit(buildPayload(values, tWaf('autoConfigMustBeObject')));
      onOpenChange(false);
    } catch (error) {
      form.setError('root', {
        message: error instanceof Error ? error.message : tWaf('saveFailed'),
      });
    }
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-3xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {group
              ? t('dialog.editTitle', { name: group.name })
              : t('dialog.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('dialog.description')}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={handleSubmit} className='space-y-4'>
            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('dialog.name')}</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('dialog.type')}</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={(value) =>
                        field.onChange(value as WAFIPGroupType)
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='manual'>
                          {t('types.manual')}
                        </SelectItem>
                        <SelectItem value='automatic'>
                          {t('types.automatic')}
                        </SelectItem>
                        <SelectItem value='subscription'>
                          {t('types.subscription')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className='flex items-center justify-between rounded-lg border border-dashed p-4 md:col-span-2'>
                    <div className='space-y-0.5'>
                      <FormLabel>{t('dialog.enable')}</FormLabel>
                      <FormDescription>
                        {t('dialog.enableDesc')}
                      </FormDescription>
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
            </div>

            {type === 'subscription' ? (
              <div className='grid gap-4 md:grid-cols-2 rounded-lg border border-dashed p-4'>
                <FormField
                  control={form.control}
                  name='subscription_url'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('dialog.subscriptionUrl')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder='https://example.com/ip-list.txt'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='subscription_format'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('dialog.subscriptionFormat')}</FormLabel>
                      <Select
                        value={field.value}
                        onValueChange={(value) =>
                          field.onChange(value as WAFIPGroupSubscriptionFormat)
                        }
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value='text'>
                            {t('dialog.formatText')}
                          </SelectItem>
                          <SelectItem value='json'>JSON</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='sync_interval_minutes'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('dialog.syncInterval')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={1}
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('dialog.syncIntervalDesc')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='subscription_mapping_rule'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('dialog.mappingRule')}</FormLabel>
                      <FormControl>
                        <Input
                          disabled={
                            form.watch('subscription_format') !== 'json'
                          }
                          placeholder={t('dialog.mappingPlaceholder')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            ) : null}

            {type === 'automatic' ? (
              <div className='space-y-4 rounded-lg border border-dashed p-4'>
                <FormField
                  control={form.control}
                  name='sync_interval_minutes'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('dialog.syncInterval')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={1}
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('dialog.autoIntervalDesc')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <div className='space-y-2'>
                  <FormLabel>{t('dialog.presetRules')}</FormLabel>
                  <div className='flex flex-wrap gap-2'>
                    {automaticPresetRules.map((rule) => (
                      <Button
                        key={rule.expr}
                        type='button'
                        size='sm'
                        variant='outline'
                        onClick={() => {
                          const current = form.getValues('auto_config_text');
                          form.setValue(
                            'auto_config_text',
                            appendAutomaticPresetRule(
                              current,
                              rule,
                              tWaf('autoConfigMustBeObject'),
                            ),
                          );
                        }}
                      >
                        <Plus className='size-3.5 mr-1' />
                        {t(rule.labelKey)}
                      </Button>
                    ))}
                  </div>
                </div>
                <FormField
                  control={form.control}
                  name='auto_config_text'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('dialog.autoConfigJson')}</FormLabel>
                      <FormControl>
                        <Textarea
                          className='min-h-48 font-mono text-xs'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            ) : null}

            {type !== 'automatic' ? (
              <FormField
                control={form.control}
                name='ip_list_text'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('dialog.ipList')}</FormLabel>
                    <FormControl>
                      <Textarea
                        className='min-h-48 font-mono text-xs'
                        placeholder={'203.0.113.10\n198.51.100.0/24'}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {type === 'subscription'
                        ? t('dialog.ipListSubscriptionHint')
                        : t('dialog.ipListManualHint')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ) : null}

            {form.formState.errors.root ? (
              <p className='text-sm text-destructive'>
                {form.formState.errors.root.message}
              </p>
            ) : null}

            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
              >
                {tCommon('cancel')}
              </Button>
              <Button type='submit' disabled={submitting}>
                {submitting ? t('dialog.saving') : t('dialog.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
