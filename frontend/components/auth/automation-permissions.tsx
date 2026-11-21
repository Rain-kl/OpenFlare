'use client';

import { useId } from 'react';
import { ChevronRight } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import {
  Field,
  FieldGroup,
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
      <FieldGroup className='gap-3'>
        {permissionGroups.map((group) => {
          const groupScopes = scopes.filter((scope) =>
            scope.endsWith(`:${group}`),
          );
          if (!groupScopes.length) return null;
          const labelId = `${id}-${group}`;

          return (
            <Collapsible
              key={group}
              className='min-w-0 rounded-md border border-dashed p-2'
            >
              <Field orientation='horizontal' className='pe-2'>
                <CollapsibleTrigger asChild>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='group/permission min-w-0 flex-1 justify-start'
                  >
                    <ChevronRight
                      data-icon='inline-start'
                      aria-hidden='true'
                      className='transition-transform group-data-[state=open]/permission:rotate-90 motion-reduce:transition-none'
                    />
                    <span id={labelId}>{t(`permissionGroups.${group}`)}</span>
                  </Button>
                </CollapsibleTrigger>
                <Switch
                  aria-labelledby={labelId}
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
              <CollapsibleContent role='region' aria-labelledby={labelId}>
                <ul
                  role='list'
                  className='flex list-disc flex-col gap-1.5 pt-2 pb-1 ps-8 pe-2 text-sm wrap-anywhere text-muted-foreground'
                >
                  {groupScopes.map((scope) => (
                    <li key={scope}>
                      {t.has(`scopes.${scope}`) ? t(`scopes.${scope}`) : scope}
                    </li>
                  ))}
                </ul>
              </CollapsibleContent>
            </Collapsible>
          );
        })}
      </FieldGroup>
    </FieldSet>
  );
}
