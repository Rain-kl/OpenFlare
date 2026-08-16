import { CircleHelp, Settings2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
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
  const t = useTranslations('waf.editor');
  return (
    <aside className='w-80 shrink-0 border-l bg-card'>
      <ScrollArea className='h-full'>
        <div className='flex flex-col gap-5 p-5'>
          <div className='flex items-center gap-2'>
            <Settings2 className='size-5 text-primary' />
            <div>
              <h2 className='text-sm font-semibold'>{t('propsTitle')}</h2>
              <p className='text-xs text-muted-foreground'>{t('propsDesc')}</p>
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
  const t = useTranslations('waf.editor');
  if (node.type === 'start' || node.type === 'allow')
    return (
      <p className='text-sm text-muted-foreground'>{t('systemNodeNoConfig')}</p>
    );
  if (node.type === 'ip_match')
    return (
      <FieldGroup>
        <DisplayNameField node={node} onChange={onChange} />
        <CsvField
          id={`${node.id}-ips`}
          label={t('ips')}
          value={node.config.ips}
          onChange={(ips) =>
            onChange({ ...node, config: { ...node.config, ips } })
          }
        />
        <CsvField
          id={`${node.id}-cidrs`}
          label={t('cidrs')}
          value={node.config.cidrs}
          onChange={(cidrs) =>
            onChange({ ...node, config: { ...node.config, cidrs } })
          }
        />
        <MultiSelect
          id={`${node.id}-groups`}
          label={t('ipGroups')}
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
          label={t('countries')}
          description={t('countriesDesc', { count: countryOptions.length })}
          options={countryOptions}
          value={node.config.countries}
          creatablePattern={/^[A-Z]{2}$/}
          onChange={(countries) =>
            onChange({ ...node, config: { ...node.config, countries } })
          }
        />
        <MultiSelect
          id={`${node.id}-regions`}
          label={t('regions')}
          description={t('regionsDesc', { count: regionOptions.length })}
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
          <p className='text-xs font-medium text-muted-foreground'>
            {t('uaCheck')}
          </p>
          <Field
            orientation='horizontal'
            className='items-center justify-between'
          >
            <FieldLabel
              htmlFor={`${node.id}-require-ua`}
              className='flex items-center gap-1.5'
            >
              {t('enableUaCheck')}
              <FieldHelp tip={t('enableUaCheckTip')} />
            </FieldLabel>
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
              <p className='flex items-center gap-1.5 text-xs font-medium text-muted-foreground'>
                {t('block')}
                <FieldHelp tip={t('blockTip')} />
              </p>
              <Field
                orientation='horizontal'
                className='items-center justify-between gap-3'
              >
                <FieldLabel
                  htmlFor={`${node.id}-block-bots`}
                  className='flex items-center gap-1.5'
                >
                  {t('blockBots')}
                  <FieldHelp tip={t('blockBotsTip')} />
                </FieldLabel>
                <Switch
                  id={`${node.id}-block-bots`}
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
                className='items-center justify-between gap-3'
              >
                <FieldLabel
                  htmlFor={`${node.id}-block-abnormal`}
                  className='flex items-center gap-1.5'
                >
                  {t('blockAbnormal')}
                  <FieldHelp tip={t('blockAbnormalTip')} />
                </FieldLabel>
                <Switch
                  id={`${node.id}-block-abnormal`}
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
                className='items-center justify-between gap-3'
              >
                <FieldLabel
                  htmlFor={`${node.id}-block-custom`}
                  className='flex items-center gap-1.5'
                >
                  {t('blockCustom')}
                  <FieldHelp tip={t('blockCustomTip')} />
                </FieldLabel>
                <Switch
                  id={`${node.id}-block-custom`}
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
                  <FieldLabel
                    htmlFor={`${node.id}-custom-patterns`}
                    className='flex items-center gap-1.5'
                  >
                    {t('customUaPatterns')}
                    <FieldHelp tip={t('customUaPatternsTip')} />
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
                </Field>
              )}
            </div>
            <Separator />
            <div className='space-y-3'>
              <p className='text-xs font-medium text-muted-foreground'>
                {t('uaMatch')}
              </p>
              <Field>
                <FieldLabel
                  htmlFor={`${node.id}-match-mode`}
                  className='flex items-center gap-1.5'
                >
                  {t('matchMode')}
                  <FieldHelp tip={t('matchModeTip')} />
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
                      <SelectItem value='or'>{t('matchOr')}</SelectItem>
                      <SelectItem value='and'>{t('matchAnd')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <MultiSelect
                id={`${node.id}-browsers`}
                label={t('browsers')}
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
                label={t('os')}
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
          </>
        )}
      </FieldGroup>
    );
  if (node.type === 'security_check')
    return (
      <FieldGroup>
        <DisplayNameField node={node} onChange={onChange} />
        <div className='space-y-1'>
          <p className='flex items-center gap-1.5 text-xs font-medium text-muted-foreground'>
            {t('security')}
            <FieldHelp tip={t('securityTip')} />
          </p>
        </div>
        <Separator />
        <div className='space-y-3'>
          <p className='text-xs font-medium text-muted-foreground'>
            {t('basicProtection')}
          </p>
          {(
            [
              'path_traversal',
              'file_inclusion',
              'sql_injection',
              'command_injection',
              'xss',
              'ssrf',
              'malicious_upload',
              'xxe',
              'crlf_injection',
            ] as const
          ).map((key) => (
            <Field
              key={key}
              orientation='horizontal'
              className='items-center justify-between gap-3'
            >
              <FieldLabel
                htmlFor={`${node.id}-${key}`}
                className='flex items-center gap-1.5'
              >
                {t(`securityRules.${key}.label`)}
                <FieldHelp tip={t(`securityRules.${key}.tip`)} />
              </FieldLabel>
              <Switch
                id={`${node.id}-${key}`}
                checked={node.config[key]}
                onCheckedChange={(checked) =>
                  onChange({
                    ...node,
                    config: { ...node.config, [key]: checked },
                  })
                }
              />
            </Field>
          ))}
        </div>
      </FieldGroup>
    );
  if (node.type === 'pow')
    return (
      <FieldGroup>
        <DisplayNameField node={node} onChange={onChange} />
        <Field>
          <FieldLabel htmlFor={`${node.id}-algorithm`}>
            {t('algorithm')}
          </FieldLabel>
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
                <SelectItem value='fast'>{t('algorithmFast')}</SelectItem>
                <SelectItem value='slow'>{t('algorithmSlow')}</SelectItem>
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
                  difficulty: t('difficulty'),
                  session_ttl: t('sessionTtl'),
                  challenge_ttl: t('challengeTtl'),
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
        label={t('statusCode')}
        value={node.config.status_code}
        onChange={(status_code) =>
          onChange({ ...node, config: { ...node.config, status_code } })
        }
      />
      <Field>
        <FieldLabel htmlFor={`${node.id}-body`}>{t('responseBody')}</FieldLabel>
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
          {new TextEncoder().encode(node.config.response_body).length} / 16384{' '}
          {t('bytes')}
        </FieldDescription>
      </Field>
    </FieldGroup>
  );
}

function FieldHelp({ tip }: { tip: string }) {
  const t = useTranslations('waf.editor');
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type='button'
          className='inline-flex size-4 shrink-0 items-center justify-center text-muted-foreground transition-colors hover:text-foreground'
          aria-label={t('help')}
          onClick={(event) => event.preventDefault()}
        >
          <CircleHelp className='size-3.5' />
        </button>
      </TooltipTrigger>
      <TooltipContent side='top' className='max-w-56 text-xs'>
        {tip}
      </TooltipContent>
    </Tooltip>
  );
}

function DisplayNameField({
  node,
  onChange,
}: {
  node: WAFRuleNode;
  onChange: (node: WAFRuleNode) => void;
}) {
  const t = useTranslations('waf.editor');
  return (
    <Field>
      <FieldLabel htmlFor={`${node.id}-label`}>{t('displayName')}</FieldLabel>
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
  const t = useTranslations('waf.editor');
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
      <FieldDescription>{t('oneValuePerLine')}</FieldDescription>
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
  const t = useTranslations('waf.editor');
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
            {value.length
              ? t('selectedCount', { count: value.length })
              : t('pleaseSelect')}
          </Button>
        </PopoverTrigger>
        <PopoverContent align='start' className='flex w-80 flex-col gap-2 p-3'>
          {(creatablePattern || options.length > 0) && (
            <div className='flex gap-2'>
              <Input
                aria-label={t('createLabel', { label })}
                placeholder={t('searchNameOrCode')}
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
                  {t('addCode')}
                </Button>
              )}
            </div>
          )}
          <div className='max-h-56 space-y-1 overflow-y-auto pr-1'>
            {visible.length === 0 ? (
              <p className='px-1 py-3 text-sm text-muted-foreground'>
                {searchRequired && !query
                  ? t('searchOptions', { count: options.length })
                  : t('noMatchingOptions')}
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
