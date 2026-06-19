'use client';

import {useEffect, useMemo, useState} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {Plus} from 'lucide-react';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Button} from '@/components/ui/button';
import {Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {Switch} from '@/components/ui/switch';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {Textarea} from '@/components/ui/textarea';
import type {WAFIPGroup, WAFRuleGroup, WAFRuleGroupPayload} from '@/lib/services/openflare';

import {
  buildCountryOptions,
  buildRuleGroupDraft,
  countRuleEntries,
  emptyRuleGroupDraft,
  formatCountryItem,
  getListFieldKey,
  type ListFieldKey,
  normalizeItems,
  textToList,
  updateDraftList,
} from './helpers';
import {PowConfigPanel} from './pow-config-panel';
import {RuleEntryDialog, type RuleEntryFormValues} from './rule-entry-dialog';
import {RuleListSection} from './rule-list-section';

const ruleGroupSchema = z.object({
  name: z.string().trim().min(1, '请输入规则组名称').max(255, '名称不能超过 255 个字符'),
  enabled: z.boolean(),
  block_status_code: z.number().int().min(400).max(599),
  block_response_body: z.string(),
  remark: z.string().max(500, '备注不能超过 500 个字符'),
  pow_enabled: z.boolean(),
});

type RuleGroupFormValues = z.infer<typeof ruleGroupSchema>;

interface RuleGroupSheetProps {
  open: boolean;
  group: WAFRuleGroup | null;
  ipGroups: WAFIPGroup[];
  submitting: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: WAFRuleGroupPayload) => Promise<void>;
}

export function RuleGroupSheet({
  open,
  group,
  ipGroups,
  submitting,
  onOpenChange,
  onSubmit,
}: RuleGroupSheetProps) {
  const [listsDraft, setListsDraft] = useState<WAFRuleGroupPayload>(emptyRuleGroupDraft);
  const [ruleEntryOpen, setRuleEntryOpen] = useState(false);

  const countryOptions = useMemo(() => buildCountryOptions(), []);
  const countryLabelMap = useMemo(
    () => new Map(countryOptions.map((option) => [option.code, option.label])),
    [countryOptions],
  );

  const form = useForm<RuleGroupFormValues>({
    resolver: zodResolver(ruleGroupSchema),
    defaultValues: {
      name: '',
      enabled: true,
      block_status_code: 418,
      block_response_body: '',
      remark: '',
      pow_enabled: false,
    },
  });

  useEffect(() => {
    if (!open) return;
    const draft = buildRuleGroupDraft(group);
    setListsDraft(draft);
    form.reset({
      name: draft.name,
      enabled: draft.enabled,
      block_status_code: draft.block_status_code,
      block_response_body: draft.block_response_body,
      remark: draft.remark,
      pow_enabled: draft.pow_enabled,
    });
    setRuleEntryOpen(false);
  }, [form, group, open]);

  const ipGroupByID = new Map(ipGroups.map((item) => [item.id, item]));
  const whitelistGroupItems = listsDraft.ip_whitelist_group_ids
    .map((id) => ipGroupByID.get(id))
    .filter((item): item is WAFIPGroup => Boolean(item));
  const blacklistGroupItems = listsDraft.ip_blacklist_group_ids
    .map((id) => ipGroupByID.get(id))
    .filter((item): item is WAFIPGroup => Boolean(item));

  const removeRuleItem = (key: ListFieldKey, value: string) => {
    setListsDraft((current) =>
      updateDraftList(current, key, (items) => items.filter((item) => item !== value)),
    );
  };

  const removeRuleGroup = (key: ListFieldKey, id: number) => {
    setListsDraft((current) =>
      updateDraftList(current, key, (items) =>
        items.filter((item) => item !== String(id)),
      ),
    );
  };

  const applyRuleEntry = (entry: RuleEntryFormValues) => {
    const values =
      entry.dimension === 'ip'
        ? textToList(entry.ipValue)
        : entry.dimension === 'ip_group'
          ? entry.ipGroupIDs.map(String)
          : normalizeItems(entry.countryValues);

    const listKey = getListFieldKey(entry.listType, entry.dimension);
    setListsDraft((current) =>
      updateDraftList(current, listKey, (items) =>
        normalizeItems([...items, ...values]),
      ),
    );
    setRuleEntryOpen(false);
  };

  const handleSubmit = form.handleSubmit(async (values) => {
    const payload: WAFRuleGroupPayload = {
      ...listsDraft,
      name: values.name,
      enabled: values.enabled,
      block_status_code: values.block_status_code,
      block_response_body: values.block_response_body,
      remark: values.remark,
      pow_enabled: values.pow_enabled,
    };
    try {
      await onSubmit(payload);
      onOpenChange(false);
    } catch (error) {
      form.setError('root', {
        message: error instanceof Error ? error.message : '保存失败',
      });
    }
  });

  const isGlobal = group?.is_global ?? false;

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent side="right" className="flex h-svh w-full flex-col gap-0 p-0 sm:max-w-4xl">
          <SheetHeader className="border-b px-4 py-4">
            <SheetTitle>{group ? `编辑 ${group.name}` : '新建规则组'}</SheetTitle>
            <SheetDescription>
              白名单命中后直接放行；未命中白名单时继续判断黑名单。当前规则数{' '}
              {countRuleEntries(listsDraft)} 条。
            </SheetDescription>
          </SheetHeader>

          <Form {...form}>
            <form
              id="rule-group-form"
              onSubmit={handleSubmit}
              className="flex min-h-0 flex-1 flex-col"
            >
              <div className="flex-1 space-y-4 overflow-y-auto px-4 py-4">
                <Tabs defaultValue="basic">
                  <TabsList className="grid w-full grid-cols-4">
                    <TabsTrigger value="basic">基本信息</TabsTrigger>
                    <TabsTrigger value="lists">黑白名单</TabsTrigger>
                    <TabsTrigger value="pow">PoW</TabsTrigger>
                    <TabsTrigger value="block">拦截返回</TabsTrigger>
                  </TabsList>

                  <TabsContent value="basic" className="mt-4 space-y-4">
                    <div className="grid gap-4 md:grid-cols-2">
                      <FormField
                        control={form.control}
                        name="name"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>规则组名称</FormLabel>
                            <FormControl>
                              <Input {...field} disabled={isGlobal} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="enabled"
                        render={({ field }) => (
                          <FormItem className="flex items-center justify-between rounded-lg border border-dashed p-4">
                            <div className="space-y-0.5">
                              <FormLabel>启用规则组</FormLabel>
                              <FormDescription>关闭后保留配置，但不会参与匹配。</FormDescription>
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
                  </TabsContent>

                  <TabsContent value="lists" className="mt-4 space-y-4">
                    <div className="flex items-center justify-between">
                      <p className="text-sm text-muted-foreground">
                        通过弹窗添加 IP、IP 组或地域规则。
                      </p>
                      <Button type="button" size="sm" onClick={() => setRuleEntryOpen(true)}>
                        <Plus className="mr-1 size-3.5" />
                        添加规则
                      </Button>
                    </div>
                    <div className="grid gap-4 xl:grid-cols-2">
                      <RuleListSection
                        title="IP 白名单"
                        description="命中后直接放行，不再继续判断黑名单。"
                        items={listsDraft.ip_whitelist}
                        groupItems={whitelistGroupItems}
                        tone="whitelist"
                        emptyText="暂无 IP 白名单规则。"
                        onRemove={(item) => removeRuleItem('ip_whitelist', item)}
                        onRemoveGroup={(id) => removeRuleGroup('ip_whitelist_group_ids', id)}
                      />
                      <RuleListSection
                        title="IP 黑名单"
                        description="未命中白名单时，命中这些 IP / IP 段将被拦截。"
                        items={listsDraft.ip_blacklist}
                        groupItems={blacklistGroupItems}
                        tone="blacklist"
                        emptyText="暂无 IP 黑名单规则。"
                        onRemove={(item) => removeRuleItem('ip_blacklist', item)}
                        onRemoveGroup={(id) => removeRuleGroup('ip_blacklist_group_ids', id)}
                      />
                      <RuleListSection
                        title="地域白名单"
                        description="显示格式为国家代码与中文名，命中后直接放行。"
                        items={listsDraft.country_whitelist.map((code) =>
                          formatCountryItem(code, countryLabelMap),
                        )}
                        tone="whitelist"
                        emptyText="暂无地域白名单规则。"
                        onRemove={(item) => {
                          const code = item.split(' ')[0] ?? item;
                          removeRuleItem('country_whitelist', code);
                        }}
                      />
                      <RuleListSection
                        title="地域黑名单"
                        description="当请求未命中白名单时，命中这些地域将被拦截。"
                        items={listsDraft.country_blacklist.map((code) =>
                          formatCountryItem(code, countryLabelMap),
                        )}
                        tone="blacklist"
                        emptyText="暂无地域黑名单规则。"
                        onRemove={(item) => {
                          const code = item.split(' ')[0] ?? item;
                          removeRuleItem('country_blacklist', code);
                        }}
                      />
                    </div>
                  </TabsContent>

                  <TabsContent value="pow" className="mt-4">
                    <PowConfigPanel
                      enabled={form.watch('pow_enabled')}
                      config={listsDraft.pow_config}
                      onChange={(enabled, config) => {
                        form.setValue('pow_enabled', enabled);
                        setListsDraft((current) => ({
                          ...current,
                          pow_enabled: enabled,
                          pow_config: config,
                        }));
                      }}
                    />
                  </TabsContent>

                  <TabsContent value="block" className="mt-4 space-y-4">
                    <div className="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
                      <FormField
                        control={form.control}
                        name="block_status_code"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>拦截状态码</FormLabel>
                            <FormControl>
                              <Input
                                type="number"
                                min={400}
                                max={599}
                                value={field.value}
                                onChange={(event) =>
                                  field.onChange(Number(event.target.value))
                                }
                              />
                            </FormControl>
                            <div className="flex flex-wrap gap-2 pt-2">
                              {[403, 418, 451, 503].map((code) => (
                                <Button
                                  key={code}
                                  type="button"
                                  size="sm"
                                  variant={field.value === code ? 'default' : 'outline'}
                                  onClick={() => field.onChange(code)}
                                >
                                  {code}
                                </Button>
                              ))}
                            </div>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="block_response_body"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>拦截页面</FormLabel>
                            <FormControl>
                              <Textarea
                                className="min-h-48"
                                placeholder="<html><body><h1>Request blocked</h1></body></html>"
                                {...field}
                              />
                            </FormControl>
                            <FormDescription>留空时只返回状态码。</FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                  </TabsContent>
                </Tabs>

                {form.formState.errors.root ? (
                  <p className="text-sm text-destructive">{form.formState.errors.root.message}</p>
                ) : null}
              </div>

              <SheetFooter className="flex-row justify-end border-t px-4 py-4">
                <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                  取消
                </Button>
                <Button type="submit" disabled={submitting}>
                  {submitting ? '保存中...' : '保存规则组'}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>

      <RuleEntryDialog
        open={ruleEntryOpen}
        countryOptions={countryOptions}
        ipGroups={ipGroups}
        onOpenChange={setRuleEntryOpen}
        onSubmit={applyRuleEntry}
      />
    </>
  );
}