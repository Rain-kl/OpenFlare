'use client';

import { Plus, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { PoWPathMatch, PoWPathRules } from '@/lib/services/openflare';

export function PathRulesEditor({
  value,
  onChange,
}: {
  value: Required<PoWPathRules>;
  onChange: (value: PoWPathRules) => void;
}) {
  const t = useTranslations('pow');
  return (
    <FieldGroup>
      <Field>
        <FieldLabel htmlFor='pow-path-mode'>{t('pathMode')}</FieldLabel>
        <Select
          value={value.mode}
          onValueChange={(mode: PoWPathRules['mode']) =>
            onChange({ ...value, mode })
          }
        >
          <SelectTrigger id='pow-path-mode'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value='all'>{t('allPaths')}</SelectItem>
              <SelectItem value='blacklist'>{t('blacklistOnly')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        <FieldDescription>{t('pathHelp')}</FieldDescription>
      </Field>
      {(['whitelist', 'blacklist'] as const).map((list) => (
        <Field key={list}>
          <FieldLabel>{t(list)}</FieldLabel>
          <FieldDescription>{t(`${list}Help`)}</FieldDescription>
          {value[list].map((rule, index) => (
            <div className='flex flex-wrap items-center gap-2' key={index}>
              <Select
                value={rule.type}
                onValueChange={(type: PoWPathMatch['type']) =>
                  onChange({
                    ...value,
                    [list]: value[list].map((item, i) =>
                      i === index ? { ...item, type } : item,
                    ),
                  })
                }
              >
                <SelectTrigger className='w-36' aria-label={t('matchType')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {(['exact', 'prefix', 'wildcard', 'regex'] as const).map(
                      (type) => (
                        <SelectItem key={type} value={type}>
                          {t(type)}
                        </SelectItem>
                      ),
                    )}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Input
                className='min-w-40 flex-1 font-mono'
                aria-label={t('pathValue')}
                maxLength={1024}
                value={rule.value}
                placeholder={
                  rule.type === 'regex' ? '^/admin(?:/|$)' : '/admin/*'
                }
                onChange={(event) =>
                  onChange({
                    ...value,
                    [list]: value[list].map((item, i) =>
                      i === index
                        ? { ...item, value: event.target.value }
                        : item,
                    ),
                  })
                }
              />
              <Button
                type='button'
                size='icon'
                variant='ghost'
                aria-label={t('removePath')}
                onClick={() =>
                  onChange({
                    ...value,
                    [list]: value[list].filter((_, i) => i !== index),
                  })
                }
              >
                <Trash2 />
              </Button>
            </div>
          ))}
          <Button
            type='button'
            variant='outline'
            className='w-fit'
            disabled={value[list].length >= 100}
            onClick={() =>
              onChange({
                ...value,
                [list]: [...value[list], { type: 'exact', value: '/' }],
              })
            }
          >
            <Plus data-icon='inline-start' />
            {t('addPath')}
          </Button>
        </Field>
      ))}
    </FieldGroup>
  );
}
