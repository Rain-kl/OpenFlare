'use client';

import {useEffect, useMemo, useState} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {Search} from 'lucide-react';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Button} from '@/components/ui/button';
import {Checkbox} from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Textarea} from '@/components/ui/textarea';
import {cn} from '@/lib/utils';
import type {WAFIPGroup} from '@/lib/services/openflare';

import {type CountryOption, normalizeItems, type RuleDimension, type RuleListType, textToList,} from './helpers';

const ruleEntrySchema = z
  .object({
    listType: z.enum(['whitelist', 'blacklist']),
    dimension: z.enum(['ip', 'ip_group', 'country']),
    ipValue: z.string(),
    ipGroupIDs: z.array(z.number()),
    countryValues: z.array(z.string()),
  })
  .superRefine((value, context) => {
    if (value.dimension === 'ip') {
      if (textToList(value.ipValue).length === 0) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['ipValue'],
          message: '请输入至少一个 IP 或 IP 段',
        });
      }
      return;
    }
    if (value.dimension === 'ip_group') {
      if (value.ipGroupIDs.length === 0) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['ipGroupIDs'],
          message: '请选择至少一个 IP 组',
        });
      }
      return;
    }
    if (normalizeItems(value.countryValues).length === 0) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['countryValues'],
        message: '请选择至少一个国家/地区',
      });
    }
  });

export type RuleEntryFormValues = z.infer<typeof ruleEntrySchema>;

const defaultRuleEntryValues: RuleEntryFormValues = {
  listType: 'blacklist',
  dimension: 'ip',
  ipValue: '',
  ipGroupIDs: [],
  countryValues: [],
};

interface RuleEntryDialogProps {
  open: boolean;
  countryOptions: CountryOption[];
  ipGroups: WAFIPGroup[];
  onOpenChange: (open: boolean) => void;
  onSubmit: (values: RuleEntryFormValues) => void;
}

export function RuleEntryDialog({
  open,
  countryOptions,
  ipGroups,
  onOpenChange,
  onSubmit,
}: RuleEntryDialogProps) {
  const [keyword, setKeyword] = useState('');
  const form = useForm<RuleEntryFormValues>({
    resolver: zodResolver(ruleEntrySchema),
    defaultValues: defaultRuleEntryValues,
  });

  const listType = form.watch('listType');
  const dimension = form.watch('dimension');
  const ipValue = form.watch('ipValue');
  const ipGroupIDs = form.watch('ipGroupIDs');
  const countryValues = form.watch('countryValues');

  useEffect(() => {
    if (!open) return;
    form.reset(defaultRuleEntryValues);
    setKeyword('');
  }, [form, open]);

  useEffect(() => {
    if (!open) return;
    setKeyword('');
  }, [dimension, open]);

  const selectedCountrySet = useMemo(
    () => new Set(countryValues),
    [countryValues],
  );
  const selectedIPGroupSet = useMemo(
    () => new Set(ipGroupIDs),
    [ipGroupIDs],
  );

  const filteredCountries = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    return countryOptions
      .filter((option) => !normalized || option.searchText.includes(normalized))
      .sort((left, right) => {
        const leftSelected = selectedCountrySet.has(left.code) ? 1 : 0;
        const rightSelected = selectedCountrySet.has(right.code) ? 1 : 0;
        return rightSelected - leftSelected || left.code.localeCompare(right.code);
      });
  }, [countryOptions, keyword, selectedCountrySet]);

  const toggleCountry = (code: string) => {
    const values = selectedCountrySet.has(code)
      ? countryValues.filter((item) => item !== code)
      : normalizeItems([...countryValues, code]);
    form.setValue('countryValues', values, { shouldValidate: true });
  };

  const toggleIPGroup = (id: number) => {
    const values = selectedIPGroupSet.has(id)
      ? ipGroupIDs.filter((item) => item !== id)
      : [...ipGroupIDs, id].sort((left, right) => left - right);
    form.setValue('ipGroupIDs', values, { shouldValidate: true });
  };

  const typeLabel = listType === 'blacklist' ? '黑名单' : '白名单';
  const dimensionLabel =
    dimension === 'ip'
      ? 'IP'
      : dimension === 'ip_group'
        ? 'IP 组'
        : '地域';

  const handleSubmit = form.handleSubmit((values) => {
    onSubmit(values);
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>添加{typeLabel}规则</DialogTitle>
          <DialogDescription>
            当前准备新增 {dimensionLabel} 维度的{typeLabel}项。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>类型</Label>
              <div className="grid grid-cols-2 gap-2">
                {[
                  { value: 'blacklist', label: '黑名单' },
                  { value: 'whitelist', label: '白名单' },
                ].map((option) => (
                  <Button
                    key={option.value}
                    type="button"
                    variant={listType === option.value ? 'default' : 'outline'}
                    onClick={() =>
                      form.setValue('listType', option.value as RuleListType)
                    }
                  >
                    {option.label}
                  </Button>
                ))}
              </div>
            </div>
            <div className="space-y-2">
              <Label>维度</Label>
              <div className="grid grid-cols-3 gap-2">
                {[
                  { value: 'ip', label: 'IP' },
                  { value: 'ip_group', label: 'IP 组' },
                  { value: 'country', label: '地域' },
                ].map((option) => (
                  <Button
                    key={option.value}
                    type="button"
                    size="sm"
                    variant={dimension === option.value ? 'default' : 'outline'}
                    onClick={() =>
                      form.setValue('dimension', option.value as RuleDimension)
                    }
                  >
                    {option.label}
                  </Button>
                ))}
              </div>
            </div>
          </div>

          {dimension === 'ip' ? (
            <div className="space-y-2">
              <Label>IP / IP 段</Label>
              <Textarea
                value={ipValue}
                placeholder="例如 1.1.1.1 或 192.168.0.0/24"
                onChange={(event) =>
                  form.setValue('ipValue', event.target.value, { shouldValidate: true })
                }
              />
              <p className="text-xs text-muted-foreground">
                支持单个 IP、CIDR，或使用换行/逗号一次添加多个。
              </p>
              {form.formState.errors.ipValue ? (
                <p className="text-xs text-destructive">
                  {form.formState.errors.ipValue.message}
                </p>
              ) : null}
            </div>
          ) : null}

          {dimension === 'ip_group' ? (
            <div className="space-y-3 rounded-lg border border-dashed p-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">选择 IP 组</p>
                  <p className="text-xs text-muted-foreground">
                    发布版本只保存引用 ID，IP 组成员由 Agent 按 checksum 差异同步。
                  </p>
                </div>
                <span className="text-xs text-muted-foreground">
                  已选 {ipGroupIDs.length}
                </span>
              </div>
              <div className="max-h-64 space-y-2 overflow-y-auto">
                {ipGroups.length > 0 ? (
                  ipGroups.map((group) => {
                    const selected = selectedIPGroupSet.has(group.id);
                    return (
                      <label
                        key={group.id}
                        className={cn(
                          'flex cursor-pointer items-center gap-3 rounded-md border px-3 py-2',
                          selected && 'border-primary bg-muted/50',
                        )}
                      >
                        <Checkbox
                          checked={selected}
                          onCheckedChange={() => toggleIPGroup(group.id)}
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block text-sm font-medium truncate">
                            {group.name}
                          </span>
                          <span className="block text-xs text-muted-foreground">
                            {group.type} · {group.ip_list.length} 条 ·{' '}
                            {group.enabled ? '启用' : '停用'}
                          </span>
                        </span>
                      </label>
                    );
                  })
                ) : (
                  <p className="text-sm text-muted-foreground">暂无 IP 组，请先创建。</p>
                )}
              </div>
              {form.formState.errors.ipGroupIDs ? (
                <p className="text-xs text-destructive">
                  {form.formState.errors.ipGroupIDs.message}
                </p>
              ) : null}
            </div>
          ) : null}

          {dimension === 'country' ? (
            <div className="space-y-3">
              <div className="flex items-center gap-2 rounded-md border px-3 py-2">
                <Search className="size-4 text-muted-foreground" />
                <Input
                  value={keyword}
                  placeholder="搜索国家代码或中文名"
                  className="border-0 shadow-none focus-visible:ring-0"
                  onChange={(event) => setKeyword(event.target.value)}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    form.setValue(
                      'countryValues',
                      normalizeItems([
                        ...countryValues,
                        ...filteredCountries.map((option) => option.code),
                      ]),
                      { shouldValidate: true },
                    )
                  }
                >
                  全选当前
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    form.setValue('countryValues', [], { shouldValidate: true })
                  }
                >
                  清空
                </Button>
              </div>
              <div className="max-h-64 space-y-2 overflow-y-auto rounded-lg border border-dashed p-3">
                {filteredCountries.map((option) => {
                  const selected = selectedCountrySet.has(option.code);
                  return (
                    <label
                      key={option.code}
                      className={cn(
                        'flex cursor-pointer items-center gap-3 rounded-md border px-3 py-2',
                        selected && 'border-primary bg-muted/50',
                      )}
                    >
                      <Checkbox
                        checked={selected}
                        onCheckedChange={() => toggleCountry(option.code)}
                      />
                      <span className="text-sm">{option.label}</span>
                    </label>
                  );
                })}
              </div>
              {form.formState.errors.countryValues ? (
                <p className="text-xs text-destructive">
                  {form.formState.errors.countryValues.message}
                </p>
              ) : null}
            </div>
          ) : null}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button type="button" onClick={() => void handleSubmit()}>
            添加
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
