'use client';

import { useId } from 'react';
import { useTranslations } from 'next-intl';

import { Checkbox } from '@/components/ui/checkbox';
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';

interface AutomationPermissionsProps {
  scopes: string[];
  selected: string[];
  onChange: (scopes: string[]) => void;
  disabled?: boolean;
}

export function AutomationPermissions({
  scopes,
  selected,
  onChange,
  disabled,
}: AutomationPermissionsProps) {
  const id = useId();
  const t = useTranslations('automation');
  return (
    <FieldSet>
      <FieldLegend>{t('permissionsTitle')}</FieldLegend>
      <FieldGroup className='gap-3'>
        {scopes.map((scope, index) => (
          <Field key={scope} orientation='horizontal' data-disabled={disabled}>
            <Checkbox
              id={`${id}-${index}`}
              checked={selected.includes(scope)}
              disabled={disabled}
              onCheckedChange={(checked) =>
                onChange(
                  checked === true
                    ? [...selected, scope]
                    : selected.filter((value) => value !== scope),
                )
              }
            />
            <FieldLabel htmlFor={`${id}-${index}`}>
              {t.has(`scopes.${scope}`) ? t(`scopes.${scope}`) : scope}
            </FieldLabel>
          </Field>
        ))}
      </FieldGroup>
    </FieldSet>
  );
}
