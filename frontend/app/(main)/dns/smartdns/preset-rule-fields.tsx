import { useTranslations } from 'next-intl';

import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import {
  PolicyFilterFields,
  type PolicyFilterValues,
} from './policy-filter-fields';

export type PresetRules = PolicyFilterValues & {
  latency: string;
  load: string;
  geo: string;
  useTagWeights: boolean;
};

export function PresetRuleFields({
  rules,
  onChange,
}: {
  rules: PresetRules;
  onChange: (patch: Partial<PresetRules>) => void;
}) {
  const t = useTranslations('smartdns.policies');
  const weights = [
    { key: 'latency', label: t('wLatency') },
    { key: 'load', label: t('wLoad') },
    { key: 'geo', label: t('wGeo') },
  ] as const;
  return (
    <FieldGroup className='gap-4'>
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
        {weights.map(({ key, label }) => (
          <Field key={key}>
            <FieldLabel htmlFor={`sdns-weight-${key}`}>{label}</FieldLabel>
            <Input
              id={`sdns-weight-${key}`}
              type='number'
              step='0.05'
              value={rules[key]}
              onChange={(event) => onChange({ [key]: event.target.value })}
            />
          </Field>
        ))}
      </div>
      <Field
        orientation='horizontal'
        className='justify-between rounded-lg border border-dashed px-3 py-2'
      >
        <FieldLabel htmlFor='sdns-tag-weights'>{t('useTagWeights')}</FieldLabel>
        <Switch
          id='sdns-tag-weights'
          checked={rules.useTagWeights}
          onCheckedChange={(useTagWeights) => onChange({ useTagWeights })}
        />
      </Field>
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
        <PolicyFilterFields
          id='sdns-preset-filter'
          value={rules}
          onChange={onChange}
        />
      </div>
    </FieldGroup>
  );
}
