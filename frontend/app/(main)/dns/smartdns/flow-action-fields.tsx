import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import type { SmartDNSFlowNodeData } from '@/lib/services/openflare';
import { PolicyFilterFields } from './policy-filter-fields';

const defaultWeights = { latency: 0.55, load: 0.3, geo: 0.15 };

export function FlowActionFields({
  data,
  onChange,
}: {
  data: SmartDNSFlowNodeData;
  onChange: (patch: Partial<SmartDNSFlowNodeData>) => void;
}) {
  const t = useTranslations('smartdns.flow');
  const tp = useTranslations('smartdns.policies');
  const filters = {
    node_ids: data.filters?.node_ids ?? [],
    countries: data.filters?.countries ?? [],
    tags: data.filters?.tags ?? [],
    network_groups: data.filters?.network_groups ?? [],
  };
  const weights = data.weights ?? defaultWeights;
  const [weightText, setWeightText] = useState({
    latency: String(weights.latency),
    load: String(weights.load),
    geo: String(weights.geo),
  });
  const weightFields = [
    { key: 'latency', label: t('wLatency') },
    { key: 'load', label: t('wLoad') },
    { key: 'geo', label: t('wGeo') },
  ] as const;
  return (
    <FieldGroup className='gap-4'>
      <Field>
        <FieldLabel htmlFor='sdns-flow-action-mode'>
          {t('actionMode')}
        </FieldLabel>
        <Select
          value={data.mode || 'smart'}
          onValueChange={(mode) => onChange({ mode })}
        >
          <SelectTrigger id='sdns-flow-action-mode' className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value='smart'>{tp('strategySmart')}</SelectItem>
              <SelectItem value='load_balance'>{tp('strategyLB')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
      <div className='grid grid-cols-3 gap-2'>
        {weightFields.map(({ key, label }) => (
          <Field key={key}>
            <FieldLabel htmlFor={`sdns-flow-${key}`}>{label}</FieldLabel>
            <Input
              id={`sdns-flow-${key}`}
              type='number'
              step='any'
              value={weightText[key]}
              onChange={(event) => {
                const value = event.target.value;
                setWeightText((previous) => ({ ...previous, [key]: value }));
                if (value.trim() && Number.isFinite(Number(value)))
                  onChange({ weights: { ...weights, [key]: Number(value) } });
              }}
              onBlur={() =>
                setWeightText((previous) => ({
                  ...previous,
                  [key]: String(weights[key]),
                }))
              }
            />
          </Field>
        ))}
      </div>
      <Field orientation='horizontal'>
        <FieldLabel htmlFor='sdns-flow-tag-weights'>
          {t('useTagWeights')}
        </FieldLabel>
        <Switch
          id='sdns-flow-tag-weights'
          checked={data.use_tag_weights ?? true}
          onCheckedChange={(use_tag_weights) => onChange({ use_tag_weights })}
        />
      </Field>
      <PolicyFilterFields
        id='sdns-flow-filter'
        value={filters}
        onChange={(patch) => onChange({ filters: { ...filters, ...patch } })}
      />
    </FieldGroup>
  );
}
