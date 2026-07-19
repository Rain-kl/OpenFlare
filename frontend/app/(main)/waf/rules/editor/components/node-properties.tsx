import { Settings2 } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import type { WAFIPGroup, WAFRuleNode } from '@/lib/services/openflare';

import { countryOptions, regionOptions, type GeoOption } from './geo-options';
import { NODE_TYPE_LABELS } from './node-factory';
import { UA_BROWSER_OPTIONS, UA_OS_OPTIONS } from './ua-options';

export function NodeProperties({
  node,
  ipGroups,
  onChange,
}: {
  node: WAFRuleNode;
  ipGroups: WAFIPGroup[];
  onChange: (node: WAFRuleNode) => void;
}) {
  return (
    <aside className='w-80 shrink-0 border-l bg-card'>
      <ScrollArea className='h-full'>
        <div className='flex flex-col gap-5 p-5'>
          <div className='flex items-center gap-2'>
            <Settings2 className='size-5 text-primary' />
            <div>
              <h2 className='text-sm font-semibold'>节点属性</h2>
              <p className='text-xs text-muted-foreground'>配置当前处理单元</p>
            </div>
          </div>
          <Separator />
          <PropertyFields node={node} ipGroups={ipGroups} onChange={onChange} />
        </div>
      </ScrollArea>
    </aside>
  );
}

function PropertyFields({
  node,
  ipGroups,
  onChange,
}: {
  node: WAFRuleNode;
  ipGroups: WAFIPGroup[];
  onChange: (node: WAFRuleNode) => void;
}) {
  if (node.type === 'start' || node.type === 'allow')
    return <p className='text-sm text-muted-foreground'>系统节点无需配置。</p>;
  if (node.type === 'ip_match')
    return (
      <FieldGroup>
        <DisplayNameField node={node} onChange={onChange} />
        <CsvField
          id={`${node.id}-ips`}
          label='IP 地址'
          value={node.config.ips}
          onChange={(ips) =>
            onChange({ ...node, config: { ...node.config, ips } })
          }
        />
        <CsvField
          id={`${node.id}-cidrs`}
          label='CIDR 网段'
          value={node.config.cidrs}
          onChange={(cidrs) =>
            onChange({ ...node, config: { ...node.config, cidrs } })
          }
        />
        <MultiSelect
          id={`${node.id}-groups`}
          label='IP 组'
          options={ipGroups.map((group) => ({
            value: String(group.id),
            label: group.name,
            searchText: `${group.name} ${group.id}`,
          }))}
          value={node.config.ip_group_ids.map(String)}
          onChange={(values) =>
            onChange({
              ...node,
              config: { ...node.config, ip_group_ids: values.map(Number) },
            })
          }
        />
      </FieldGroup>
    );
  if (node.type === 'geo_match')
    return (
      <FieldGroup>
        <DisplayNameField node={node} onChange={onChange} />
        <MultiSelect
          id={`${node.id}-countries`}
          label='国家代码'
          description={`共 ${countryOptions.length} 个国家/地区，显示国家名与 ISO 代码`}
          options={countryOptions}
          value={node.config.countries}
          creatablePattern={/^[A-Z]{2}$/}
          onChange={(countries) =>
            onChange({ ...node, config: { ...node.config, countries } })
          }
        />
        <MultiSelect
          id={`${node.id}-regions`}
          label='地区代码'
          description={`共 ${regionOptions.length} 个一级行政区，输入名称或代码搜索`}
          options={regionOptions}
          value={node.config.regions}
          creatablePattern={/^[A-Z]{2}-[A-Z0-9]{1,3}$/}
          searchRequired
          onChange={(regions) =>
            onChange({ ...node, config: { ...node.config, regions } })
          }
        />
      </FieldGroup>
    );
  if (node.type === 'ua_check')
    return (
      <FieldGroup>
        <DisplayNameField node={node} onChange={onChange} />
        <div className='space-y-1'>
          <p className='text-xs font-medium text-muted-foreground'>UA 检查</p>
          <Field
            orientation='horizontal'
            className='items-center justify-between'
          >
            <div className='space-y-1'>
              <FieldLabel htmlFor={`${node.id}-require-ua`}>
                开启 UA 检查
              </FieldLabel>
              <FieldDescription>
                开启后如果请求头不携带 UA 返回 False；并可配置匹配与屏蔽
              </FieldDescription>
            </div>
            <Switch
              id={`${node.id}-require-ua`}
              checked={node.config.require_ua}
              onCheckedChange={(require_ua) =>
                onChange({ ...node, config: { ...node.config, require_ua } })
              }
            />
          </Field>
        </div>
        {node.config.require_ua && (
          <>
            <Separator />
            <div className='space-y-3'>
              <p className='text-xs font-medium text-muted-foreground'>
                UA 匹配
              </p>
              <Field>
                <FieldLabel htmlFor={`${node.id}-match-mode`}>
                  匹配模式
                </FieldLabel>
                <Select
                  value={node.config.match_mode}
                  onValueChange={(match_mode: 'and' | 'or') =>
                    onChange({
                      ...node,
                      config: { ...node.config, match_mode },
                    })
                  }
                >
                  <SelectTrigger
                    id={`${node.id}-match-mode`}
                    className='w-full'
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value='or'>或（OR）</SelectItem>
                      <SelectItem value='and'>且（AND）</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FieldDescription>
                  浏览器与操作系统两侧都有选择时生效
                </FieldDescription>
              </Field>
              <MultiSelect
                id={`${node.id}-browsers`}
                label='浏览器'
                options={UA_BROWSER_OPTIONS.map((option) => ({
                  value: option.value,
                  label: option.label,
                  searchText: `${option.label} ${option.value}`,
                }))}
                value={node.config.browsers}
                onChange={(browsers) =>
                  onChange({ ...node, config: { ...node.config, browsers } })
                }
              />
              <MultiSelect
                id={`${node.id}-os`}
                label='操作系统'
                options={UA_OS_OPTIONS.map((option) => ({
                  value: option.value,
                  label: option.label,
                  searchText: `${option.label} ${option.value}`,
                }))}
                value={node.config.operating_systems}
                onChange={(operating_systems) =>
                  onChange({
                    ...node,
                    config: { ...node.config, operating_systems },
                  })
                }
              />
            </div>
            <Separator />
            <div className='space-y-3'>
              <div className='space-y-1'>
                <p className='text-xs font-medium text-muted-foreground'>
                  屏蔽
                </p>
                <FieldDescription>
                  命中返回 false，优先级高于匹配
                </FieldDescription>
              </div>
              <Field
                orientation='horizontal'
                className='items-start justify-between gap-3'
              >
                <div className='space-y-1'>
                  <FieldLabel htmlFor={`${node.id}-block-bots`}>
                    屏蔽常见爬虫 UA
                  </FieldLabel>
                  <FieldDescription>
                    浏览器或操作系统分类为 Bot（含 bot / spider / crawler /
                    slurp 等特征，如 Googlebot）
                  </FieldDescription>
                </div>
                <Switch
                  id={`${node.id}-block-bots`}
                  className='mt-0.5'
                  checked={node.config.block_common_bots}
                  onCheckedChange={(block_common_bots) =>
                    onChange({
                      ...node,
                      config: { ...node.config, block_common_bots },
                    })
                  }
                />
              </Field>
              <Field
                orientation='horizontal'
                className='items-start justify-between gap-3'
              >
                <div className='space-y-1'>
                  <FieldLabel htmlFor={`${node.id}-block-abnormal`}>
                    屏蔽非正常 UA
                  </FieldLabel>
                  <FieldDescription>
                    浏览器分类为 Other 或 Unknown（不含搜索引擎等爬虫
                    Bot，爬虫请用上方开关）
                  </FieldDescription>
                </div>
                <Switch
                  id={`${node.id}-block-abnormal`}
                  className='mt-0.5'
                  checked={node.config.block_abnormal_ua}
                  onCheckedChange={(block_abnormal_ua) =>
                    onChange({
                      ...node,
                      config: { ...node.config, block_abnormal_ua },
                    })
                  }
                />
              </Field>
              <Field
                orientation='horizontal'
                className='items-start justify-between gap-3'
              >
                <div className='space-y-1'>
                  <FieldLabel htmlFor={`${node.id}-block-custom`}>
                    屏蔽自定义 UA
                  </FieldLabel>
                  <FieldDescription>
                    原始 User-Agent 命中任一条正则时返回 false（Lua 模式语法）
                  </FieldDescription>
                </div>
                <Switch
                  id={`${node.id}-block-custom`}
                  className='mt-0.5'
                  checked={node.config.block_custom_ua}
                  onCheckedChange={(block_custom_ua) =>
                    onChange({
                      ...node,
                      config: { ...node.config, block_custom_ua },
                    })
                  }
                />
              </Field>
              {node.config.block_custom_ua && (
                <Field>
                  <FieldLabel htmlFor={`${node.id}-custom-patterns`}>
                    自定义 UA 正则
                  </FieldLabel>
                  <Textarea
                    id={`${node.id}-custom-patterns`}
                    rows={4}
                    value={node.config.custom_ua_patterns.join('\n')}
                    placeholder={'python%-requests\ncurl/'}
                    onChange={(event) =>
                      onChange({
                        ...node,
                        config: {
                          ...node.config,
                          custom_ua_patterns: event.target.value
                            .split('\n')
                            .map((item) => item.trim())
                            .filter(Boolean),
                        },
                      })
                    }
                  />
                  <FieldDescription>
                    每行一条 Lua 模式正则，命中任一条即 false；最多 32 条
                  </FieldDescription>
                </Field>
              )}
            </div>
          </>
        )}
      </FieldGroup>
    );
  if (node.type === 'pow')
    return (
      <FieldGroup>
        <DisplayNameField node={node} onChange={onChange} />
        <Field>
          <FieldLabel htmlFor={`${node.id}-algorithm`}>算法</FieldLabel>
          <Select
            value={node.config.algorithm}
            onValueChange={(algorithm: 'fast' | 'slow') =>
              onChange({ ...node, config: { ...node.config, algorithm } })
            }
          >
            <SelectTrigger id={`${node.id}-algorithm`} className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value='fast'>快速</SelectItem>
                <SelectItem value='slow'>稳健</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        {(['difficulty', 'session_ttl', 'challenge_ttl'] as const).map(
          (key) => (
            <NumberField
              id={`${node.id}-${key}`}
              key={key}
              min={{ difficulty: 1, session_ttl: 60, challenge_ttl: 30 }[key]}
              max={key === 'difficulty' ? 16 : undefined}
              label={
                {
                  difficulty: '难度',
                  session_ttl: '会话 TTL（秒）',
                  challenge_ttl: '挑战 TTL（秒）',
                }[key]
              }
              value={node.config[key]}
              onChange={(value) =>
                onChange({ ...node, config: { ...node.config, [key]: value } })
              }
            />
          ),
        )}
      </FieldGroup>
    );
  return (
    <FieldGroup>
      <DisplayNameField node={node} onChange={onChange} />
      <NumberField
        id={`${node.id}-status`}
        min={400}
        max={599}
        label='HTTP 状态码'
        value={node.config.status_code}
        onChange={(status_code) =>
          onChange({ ...node, config: { ...node.config, status_code } })
        }
      />
      <Field>
        <FieldLabel htmlFor={`${node.id}-body`}>HTML 响应体</FieldLabel>
        <Textarea
          id={`${node.id}-body`}
          rows={9}
          value={node.config.response_body}
          onChange={(event) =>
            onChange({
              ...node,
              config: { ...node.config, response_body: event.target.value },
            })
          }
        />
        <FieldDescription>
          {new TextEncoder().encode(node.config.response_body).length} / 16384
          字节
        </FieldDescription>
      </Field>
    </FieldGroup>
  );
}

function DisplayNameField({
  node,
  onChange,
}: {
  node: WAFRuleNode;
  onChange: (node: WAFRuleNode) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor={`${node.id}-label`}>显示名称</FieldLabel>
      <Input
        id={`${node.id}-label`}
        value={node.label ?? ''}
        placeholder={NODE_TYPE_LABELS[node.type]}
        onChange={(event) => onChange({ ...node, label: event.target.value })}
      />
    </Field>
  );
}

function CsvField({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: string[];
  onChange: (value: string[]) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Textarea
        id={id}
        value={value.join('\n')}
        onChange={(event) =>
          onChange(
            event.target.value
              .split(/[\n,]/)
              .map((item) => item.trim())
              .filter(Boolean),
          )
        }
      />
      <FieldDescription>每行一个值</FieldDescription>
    </Field>
  );
}
function NumberField({
  id,
  label,
  value,
  min,
  max,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  min?: number;
  max?: number;
  onChange: (value: number) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input
        id={id}
        min={min}
        max={max}
        type='number'
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </Field>
  );
}

function MultiSelect({
  id,
  label,
  description,
  options,
  value,
  creatablePattern,
  searchRequired = false,
  onChange,
}: {
  id: string;
  label: string;
  description?: string;
  options: GeoOption[];
  value: string[];
  creatablePattern?: RegExp;
  searchRequired?: boolean;
  onChange: (value: string[]) => void;
}) {
  const [draft, setDraft] = useState('');
  const normalized = draft.trim().toUpperCase();
  const query = draft.trim().toLocaleLowerCase();
  const available = [
    ...options,
    ...value
      .filter(
        (selected) => !options.some((option) => option.value === selected),
      )
      .map((selected) => ({
        value: selected,
        label: selected,
        searchText: selected,
      })),
  ];
  const visible = available.filter((option) => {
    if (!query) return !searchRequired || value.includes(option.value);
    return option.searchText.toLocaleLowerCase().includes(query);
  });
  const canCreate = Boolean(
    creatablePattern?.test(normalized) && !value.includes(normalized),
  );
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Popover>
        <PopoverTrigger asChild>
          <Button id={id} variant='outline' className='w-full justify-start'>
            {value.length ? `已选择 ${value.length} 项` : '请选择'}
          </Button>
        </PopoverTrigger>
        <PopoverContent align='start' className='flex w-80 flex-col gap-2 p-3'>
          {(creatablePattern || options.length > 0) && (
            <div className='flex gap-2'>
              <Input
                aria-label={`新建${label}`}
                placeholder='搜索名称或代码'
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
              />
              {creatablePattern && (
                <Button
                  size='sm'
                  disabled={!canCreate}
                  onClick={() => {
                    onChange([...value, normalized]);
                    setDraft('');
                  }}
                >
                  添加代码
                </Button>
              )}
            </div>
          )}
          <div className='max-h-56 space-y-1 overflow-y-auto pr-1'>
            {visible.length === 0 ? (
              <p className='px-1 py-3 text-sm text-muted-foreground'>
                {searchRequired && !query
                  ? `输入名称或代码搜索 ${options.length} 个选项`
                  : '没有匹配的选项'}
              </p>
            ) : (
              visible.map((option) => (
                <label
                  key={option.value}
                  className='flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-1 text-sm hover:bg-accent'
                >
                  <Checkbox
                    checked={value.includes(option.value)}
                    onCheckedChange={(checked) =>
                      onChange(
                        checked
                          ? [...value, option.value]
                          : value.filter((item) => item !== option.value),
                      )
                    }
                  />
                  <span
                    className='min-w-0 flex-1 truncate'
                    title={option.label}
                  >
                    {option.label}
                  </span>
                  <span className='font-mono text-xs text-muted-foreground'>
                    {option.value}
                  </span>
                </label>
              ))
            )}
          </div>
        </PopoverContent>
      </Popover>
      {description && <FieldDescription>{description}</FieldDescription>}
    </Field>
  );
}
