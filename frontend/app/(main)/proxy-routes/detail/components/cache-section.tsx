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

import {
  linesFromTextarea,
  validateCacheRules,
} from '../../components/helpers';
import { proxyRouteFormIds } from '../helpers';
import { useRouteSectionSave } from '../hooks/use-route-section-save';
import { SectionShell } from './section-shell';

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
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          className='size-7 text-muted-foreground'
          aria-label='缓存使用说明'
        >
          <CircleHelp />
        </Button>
      </PopoverTrigger>
      <PopoverContent align='start' className='w-80 flex flex-col gap-3 p-4'>
        <div className='text-sm font-medium'>缓存使用说明</div>
        <div className='flex flex-col gap-2 text-xs text-muted-foreground leading-relaxed'>
          <p>
            扩展名/策略决定是否可缓存，源站头与Set-Cookie
            决定是否入库。须同时开启性能设置中的全局 OpenResty 缓存。
          </p>
          <p>
            <span className='font-medium text-foreground'>推荐策略：</span>
            新建站点建议使用「标准静态资源」（含
            css/js/map/图片/字体/媒体等，不含 HTML/JSON）。
          </p>
          <p>
            <span className='font-medium text-foreground'>通用规则：</span>非
            GET 不缓存；登录 Cookie 不会单独跳过缓存；源站 private/no-store
            或响应 Set-Cookie 不会写入边缘。
          </p>
          <p>
            <span className='font-medium text-foreground'>
              所有可缓存 GET：
            </span>
            高级选项，类似 Cache Everything。个性化页面须由源站声明
            private/no-store，否则 HTML 可能被边缘缓存并串给其他用户。
          </p>
          <p>
            <span className='font-medium text-foreground'>自定义规则：</span>
            后缀填 jpg/css/js；路径前缀填 /assets；精确路径填 /robots.txt。
            保存后须重新发布配置版本才会在节点生效。
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
      ? '每行一个后缀，例如 jpg、css、js。'
      : watchedPolicy === 'path_prefix'
        ? '每行一个路径前缀，例如 /assets、/static。'
        : watchedPolicy === 'path_exact'
          ? '每行一个精确路径，例如 /robots.txt。'
          : watchedPolicy === 'static'
            ? '标准静态资源使用内置扩展名列表，无需填写规则。'
            : '当前策略无需额外路径规则。';

  const rulesPlaceholder =
    watchedPolicy === 'suffix'
      ? 'jpg\ncss\njs'
      : watchedPolicy === 'path_prefix'
        ? '/assets\n/static'
        : watchedPolicy === 'path_exact'
          ? '/robots.txt\n/manifest.json'
          : '当前策略无需额外规则';

  return (
    <SectionShell
      title='缓存'
      description='配置站点边缘缓存策略。'
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
              '缓存设置已保存',
            );
          })}
        >
          <FormField
            control={form.control}
            name='cache_enabled'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between rounded-lg border p-3'>
                <div className='space-y-0.5'>
                  <FormLabel>启用站点缓存</FormLabel>
                  <FormDescription>新建推荐「标准静态资源」。</FormDescription>
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
                    <SelectItem value='static'>标准静态资源（推荐）</SelectItem>
                    <SelectItem value='all'>所有可缓存 GET（高级）</SelectItem>
                    <SelectItem value='suffix'>自定义后缀</SelectItem>
                    <SelectItem value='path_prefix'>路径前缀</SelectItem>
                    <SelectItem value='path_exact'>精确路径</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          {watchedEnabled && watchedPolicy === 'all' ? (
            <Alert>
              <TriangleAlert />
              <AlertTitle>高级策略风险</AlertTitle>
              <AlertDescription>
                个性化 HTML 在源站未声明 private / no-store
                时可能被边缘缓存并串给其他用户。详情见标题旁帮助。
              </AlertDescription>
            </Alert>
          ) : null}

          <FormField
            control={form.control}
            name='cache_rules_text'
            render={({ field }) => (
              <FormItem>
                <FormLabel>缓存规则</FormLabel>
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
