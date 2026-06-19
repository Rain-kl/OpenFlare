'use client';

import {useEffect} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {Plus} from 'lucide-react';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import {Textarea} from '@/components/ui/textarea';
import type {
  WAFIPGroup,
  WAFIPGroupPayload,
  WAFIPGroupSubscriptionFormat,
  WAFIPGroupType,
} from '@/lib/services/openflare';

import {automaticPresetRules, listToText, parseAutomaticConfig, parseTextareaList,} from './helpers';

const ipGroupSchema = z
  .object({
    name: z.string().trim().min(1, '请输入 IP 组名称').max(255),
    type: z.enum(['manual', 'automatic', 'subscription']),
    enabled: z.boolean(),
    ip_list_text: z.string(),
    auto_config_text: z.string(),
    subscription_url: z.string(),
    subscription_format: z.enum(['text', 'json']),
    subscription_mapping_rule: z.string(),
    sync_interval_minutes: z.number().int().min(5),
    remark: z.string().max(500),
  })
  .superRefine((value, context) => {
    if (value.type === 'subscription' && !value.subscription_url.trim()) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['subscription_url'],
        message: '订阅类型需要填写订阅 URL',
      });
    }
    if (value.type === 'automatic') {
      try {
        parseAutomaticConfig(value.auto_config_text);
      } catch (error) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['auto_config_text'],
          message: error instanceof Error ? error.message : '自动配置格式错误',
        });
      }
    }
  });

type IPGroupFormValues = z.infer<typeof ipGroupSchema>;

const defaultValues: IPGroupFormValues = {
  name: '',
  type: 'manual',
  enabled: true,
  ip_list_text: '',
  auto_config_text: '{}',
  subscription_url: '',
  subscription_format: 'text',
  subscription_mapping_rule: '',
  sync_interval_minutes: 1440,
  remark: '',
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
    remark: group.remark ?? '',
  };
}

function buildPayload(values: IPGroupFormValues): WAFIPGroupPayload {
  const autoConfig =
    values.type === 'automatic'
      ? parseAutomaticConfig(values.auto_config_text)
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
    remark: values.remark.trim(),
  };
}

function appendAutomaticPresetRule(
  autoConfigText: string,
  rule: (typeof automaticPresetRules)[number],
) {
  const config = parseAutomaticConfig(autoConfigText);
  const rules = Array.isArray(config.rules) ? config.rules : [];
  const exists = rules.some(
    (item) =>
      item &&
      typeof item === 'object' &&
      'expr' in item &&
      (item as { expr?: unknown }).expr === rule.expr,
  );
  const nextRules = exists ? rules : [...rules, rule];
  return JSON.stringify(
    {
      lookback_minutes:
        typeof config.lookback_minutes === 'number' ? config.lookback_minutes : 60,
      ...config,
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
      await onSubmit(buildPayload(values));
      onOpenChange(false);
    } catch (error) {
      form.setError('root', {
        message: error instanceof Error ? error.message : '保存失败',
      });
    }
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{group ? `编辑 ${group.name}` : '新建 IP 组'}</DialogTitle>
          <DialogDescription>
            维护可被 WAF IP 黑白名单引用的手动、自动与订阅 IP 集合。
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>IP 组名称</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="type"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>类型</FormLabel>
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
                        <SelectItem value="manual">手动</SelectItem>
                        <SelectItem value="automatic">自动</SelectItem>
                        <SelectItem value="subscription">订阅</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="enabled"
                render={({ field }) => (
                  <FormItem className="flex items-center justify-between rounded-lg border border-dashed p-4 md:col-span-2">
                    <div className="space-y-0.5">
                      <FormLabel>启用 IP 组</FormLabel>
                      <FormDescription>
                        关闭后保留配置，但发布时不会展开到 WAF 运行时名单。
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
                name="remark"
                render={({ field }) => (
                  <FormItem className="md:col-span-2">
                    <FormLabel>备注</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            {type === 'subscription' ? (
              <div className="grid gap-4 md:grid-cols-2 rounded-lg border border-dashed p-4">
                <FormField
                  control={form.control}
                  name="subscription_url"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>订阅 URL</FormLabel>
                      <FormControl>
                        <Input placeholder="https://example.com/ip-list.txt" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="subscription_format"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>订阅格式</FormLabel>
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
                          <SelectItem value="text">文本列表</SelectItem>
                          <SelectItem value="json">JSON</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="sync_interval_minutes"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>同步间隔（分钟）</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          min={5}
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormDescription>最小 5 分钟，默认 1440 分钟。</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="subscription_mapping_rule"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>JSON 映射规则</FormLabel>
                      <FormControl>
                        <Input
                          disabled={form.watch('subscription_format') !== 'json'}
                          placeholder="留空表示根数组"
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
              <div className="space-y-4 rounded-lg border border-dashed p-4">
                <FormField
                  control={form.control}
                  name="sync_interval_minutes"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>同步间隔（分钟）</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          min={5}
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        定时从请求日志挖掘恶意 IP 的周期。最小 5 分钟。
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <div className="space-y-2">
                  <FormLabel>预设规则</FormLabel>
                  <div className="flex flex-wrap gap-2">
                    {automaticPresetRules.map((rule) => (
                      <Button
                        key={rule.expr}
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          const current = form.getValues('auto_config_text');
                          form.setValue(
                            'auto_config_text',
                            appendAutomaticPresetRule(current, rule),
                          );
                        }}
                      >
                        <Plus className="size-3.5 mr-1" />
                        {rule.name}
                      </Button>
                    ))}
                  </div>
                </div>
                <FormField
                  control={form.control}
                  name="auto_config_text"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>自动配置 JSON</FormLabel>
                      <FormControl>
                        <Textarea className="min-h-48 font-mono text-xs" {...field} />
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
                name="ip_list_text"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>IP / IP 段</FormLabel>
                    <FormControl>
                      <Textarea
                        className="min-h-48 font-mono text-xs"
                        placeholder={'203.0.113.10\n198.51.100.0/24'}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {type === 'subscription'
                        ? '订阅同步会覆盖此列表；也可以先手动保存当前内容。'
                        : '支持单个 IP 或 CIDR，每行一个。'}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ) : null}

            {form.formState.errors.root ? (
              <p className="text-sm text-destructive">{form.formState.errors.root.message}</p>
            ) : null}

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? '保存中...' : '保存 IP 组'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
