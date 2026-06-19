'use client';

import {useEffect} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import {Textarea} from '@/components/ui/textarea';
import type {ProxyRouteItem} from '@/lib/services/openflare';

import {linesFromTextarea, validateCacheRules} from '../../components/helpers';
import {proxyRouteFormIds} from '../helpers';
import {useRouteSectionSave} from '../hooks/use-route-section-save';
import {SectionShell} from './section-shell';

const cacheSchema = z
  .object({
    cache_enabled: z.boolean(),
    cache_policy: z.enum(['url', 'suffix', 'path_prefix', 'path_exact']),
    cache_rules_text: z.string(),
  })
  .superRefine((value, context) => {
    if (!value.cache_enabled) {
      return;
    }

    const rules = linesFromTextarea(value.cache_rules_text);
    const error = validateCacheRules(value.cache_policy, rules);
    if (error) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['cache_rules_text'],
        message: error,
      });
    }
  });

type CacheValues = z.infer<typeof cacheSchema>;

interface CacheSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function CacheSection({ route, onRouteUpdate, onSavingChange }: CacheSectionProps) {
  const { saving, save } = useRouteSectionSave(route, onRouteUpdate, onSavingChange);

  const form = useForm<CacheValues>({
    resolver: zodResolver(cacheSchema),
    defaultValues: {
      cache_enabled: route.cache_enabled,
      cache_policy: (route.cache_policy || 'url') as CacheValues['cache_policy'],
      cache_rules_text: route.cache_rule_list.join('\n'),
    },
  });

  useEffect(() => {
    form.reset({
      cache_enabled: route.cache_enabled,
      cache_policy: (route.cache_policy || 'url') as CacheValues['cache_policy'],
      cache_rules_text: route.cache_rule_list.join('\n'),
    });
  }, [form, route]);

  const watchedEnabled = form.watch('cache_enabled');
  const watchedPolicy = form.watch('cache_policy');

  const rulesHint =
    watchedPolicy === 'suffix'
      ? '每行一个后缀，例如 jpg、css、js。'
      : watchedPolicy === 'path_prefix'
        ? '每行一个路径前缀，例如 /assets、/static。'
        : watchedPolicy === 'path_exact'
          ? '每行一个精确路径，例如 /robots.txt。'
          : '按 URL 缓存时无需额外规则。';

  const rulesPlaceholder =
    watchedPolicy === 'suffix'
      ? 'jpg\ncss\njs'
      : watchedPolicy === 'path_prefix'
        ? '/assets\n/static'
        : watchedPolicy === 'path_exact'
          ? '/robots.txt\n/manifest.json'
          : '按 URL 缓存时无需额外规则';

  return (
    <SectionShell
      title="缓存"
      description="保留现有安全绕过逻辑，只对当前站点生效。"
      formId={proxyRouteFormIds.cache}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.cache}
          className="space-y-5"
          onSubmit={form.handleSubmit(async (values) => {
            const rules = linesFromTextarea(values.cache_rules_text);
            await save(
              {
                cache_enabled: values.cache_enabled,
                cache_policy: values.cache_enabled ? values.cache_policy : 'url',
                cache_rules:
                  values.cache_enabled && values.cache_policy !== 'url' ? rules : [],
              },
              '缓存设置已保存',
            );
          })}
        >
          <FormField
            control={form.control}
            name="cache_enabled"
            render={({ field }) => (
              <FormItem className="flex items-center justify-between rounded-lg border p-3">
                <div className="space-y-0.5">
                  <FormLabel>启用站点缓存</FormLabel>
                  <FormDescription>
                    系统仍会自动绕过非 GET、带 Authorization 或常见登录态 Cookie 的请求。
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch checked={field.value} onCheckedChange={field.onChange} />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="cache_policy"
            render={({ field }) => (
              <FormItem>
                <FormLabel>缓存策略</FormLabel>
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
                    <SelectItem value="url">按 URL 缓存</SelectItem>
                    <SelectItem value="suffix">按后缀缓存</SelectItem>
                    <SelectItem value="path_prefix">按路径前缀缓存</SelectItem>
                    <SelectItem value="path_exact">按精确路径缓存</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="cache_rules_text"
            render={({ field }) => (
              <FormItem>
                <FormLabel>缓存规则</FormLabel>
                <FormControl>
                  <Textarea
                    className="min-h-32"
                    disabled={!watchedEnabled || watchedPolicy === 'url'}
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