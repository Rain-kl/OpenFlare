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
import { Textarea } from '@/components/ui/textarea';
import type { WAFIPGroup, WAFRuleNode } from '@/lib/services/openflare';

const countries = [
  'CN',
  'US',
  'JP',
  'SG',
  'DE',
  'FR',
  'GB',
  'CA',
  'AU',
  'BR',
  'IN',
  'KR',
].map((value) => ({ value, label: value }));
const regions = [
  'CN-BJ',
  'CN-SH',
  'CN-GD',
  'CN-ZJ',
  'US-CA',
  'US-NY',
  'US-TX',
  'JP-13',
  'DE-BE',
  'GB-ENG',
].map((value) => ({ value, label: value }));

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
        <MultiSelect
          id={`${node.id}-countries`}
          label='国家代码'
          options={countries}
          value={node.config.countries}
          creatablePattern={/^[A-Z]{2}$/}
          onChange={(countries) =>
            onChange({ ...node, config: { ...node.config, countries } })
          }
        />
        <MultiSelect
          id={`${node.id}-regions`}
          label='地区代码'
          options={regions}
          value={node.config.regions}
          creatablePattern={/^[A-Z]{2}-[A-Z0-9]{1,3}$/}
          onChange={(regions) =>
            onChange({ ...node, config: { ...node.config, regions } })
          }
        />
      </FieldGroup>
    );
  if (node.type === 'pow')
    return (
      <FieldGroup>
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
  options,
  value,
  creatablePattern,
  onChange,
}: {
  id: string;
  label: string;
  options: { value: string; label: string }[];
  value: string[];
  creatablePattern?: RegExp;
  onChange: (value: string[]) => void;
}) {
  const [draft, setDraft] = useState('');
  const normalized = draft.trim().toUpperCase();
  const visible = [
    ...options,
    ...value
      .filter(
        (selected) => !options.some((option) => option.value === selected),
      )
      .map((selected) => ({ value: selected, label: selected })),
  ];
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
        <PopoverContent
          align='start'
          className='flex max-h-64 flex-col gap-2 overflow-y-auto'
        >
          {creatablePattern && (
            <div className='flex gap-2'>
              <Input
                aria-label={`新建${label}`}
                placeholder='输入代码并添加'
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
              />
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
            </div>
          )}
          {visible.length === 0 ? (
            <p className='text-sm text-muted-foreground'>暂无可选项</p>
          ) : (
            visible.map((option) => (
              <label
                key={option.value}
                className='flex cursor-pointer items-center gap-2 text-sm'
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
                <span>{option.label}</span>
                <span className='ml-auto font-mono text-xs text-muted-foreground'>
                  {option.value}
                </span>
              </label>
            ))
          )}
        </PopoverContent>
      </Popover>
    </Field>
  );
}
