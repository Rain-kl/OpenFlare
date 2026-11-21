'use client';

import { useId } from 'react';
import { useTranslations } from 'next-intl';

import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';
import { Switch } from '@/components/ui/switch';

const permissionGroups = ['read', 'write'] as const;

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
    <FieldSet className='min-w-0'>
      <FieldLegend>{t('permissionsTitle')}</FieldLegend>
      <FieldGroup className='grid gap-3 sm:grid-cols-2'>
        {permissionGroups.map((group) => {
          const groupScopes = scopes.filter((scope) =>
            scope.endsWith(`:${group}`),
          );
          if (!groupScopes.length) return null;
          const switchId = `${id}-${group}`;
          const labelId = `${switchId}-label`;

          return (
            <div
              key={group}
              className='flex min-w-0 flex-col gap-3 rounded-md border border-dashed p-3 only:col-span-full'
            >
              <Field orientation='horizontal' data-disabled={disabled}>
                <FieldLabel id={labelId} htmlFor={switchId}>
                  {t(`permissionGroups.${group}`)}
                </FieldLabel>
                <Switch
                  id={switchId}
                  checked={groupScopes.every((scope) =>
                    selected.includes(scope),
                  )}
                  disabled={disabled}
                  onCheckedChange={(checked) => {
                    const selection = new Set(selected);
                    for (const scope of groupScopes) {
                      if (checked) selection.add(scope);
                      else selection.delete(scope);
                    }
                    onChange(scopes.filter((scope) => selection.has(scope)));
                  }}
                />
              </Field>
              <div
                role='region'
                aria-labelledby={labelId}
                tabIndex={0}
                className='h-24 overflow-y-auto overscroll-y-contain rounded-sm focus-visible:outline-2 focus-visible:outline-ring sm:h-32'
              >
                <ul
                  role='list'
                  className='flex list-disc flex-col gap-1.5 ps-4 pe-2 text-sm wrap-anywhere text-muted-foreground'
                >
                  {groupScopes.map((scope) => (
                    <li key={scope}>
                      {t.has(`scopes.${scope}`) ? t(`scopes.${scope}`) : scope}
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          );
        })}
      </FieldGroup>
    </FieldSet>
  );
}
