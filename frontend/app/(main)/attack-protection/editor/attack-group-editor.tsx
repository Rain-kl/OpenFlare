import { useMutation } from '@tanstack/react-query';
import { List, Save, Shield } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
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
import {
  attackCategories,
  attackLevels,
  type AttackGroup,
  type AttackGroupInput,
  type AttackLevel,
  type AttackRuleAction,
  type AttackRuleOverrides,
} from '@/lib/services/openflare/attack-types';
import { WafService } from '@/lib/services/openflare';

import { AttackRuleCatalog, ruleCounts } from '../attack-rule-catalog';

export function AttackGroupEditor({
  group,
  onSaved,
}: {
  group?: AttackGroup;
  onSaved: () => Promise<void>;
}) {
  const t = useTranslations('attackProtection');
  const [name, setName] = useState(group?.name ?? '');
  const [levels, setLevels] = useState<AttackGroupInput['levels']>(
    group?.levels ?? {},
  );
  const [ruleOverrides, setRuleOverrides] = useState<AttackRuleOverrides>(
    group?.rule_overrides ?? {},
  );
  const save = useMutation({
    mutationFn: () =>
      WafService.saveAttackGroup(
        { name: name.trim(), levels, rule_overrides: ruleOverrides },
        group?.id,
      ),
    onSuccess: onSaved,
  });
  const editor = {
    levels,
    rule_overrides: ruleOverrides,
    onRuleChange(id: number, action: AttackRuleAction | undefined) {
      setRuleOverrides((current) => {
        const next = { ...current };
        if (action === undefined) delete next[id];
        else next[id] = action;
        return next;
      });
    },
  };

  return (
    <form
      className='flex flex-col gap-4'
      onSubmit={(event) => {
        event.preventDefault();
        save.mutate();
      }}
    >
      <header className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <Shield className='size-5 shrink-0 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {group ? t('editGroup') : t('create')}
          </h1>
        </div>
        <Button type='submit' disabled={!name.trim() || save.isPending}>
          <Save data-icon='inline-start' />
          {t('save')}
        </Button>
      </header>
      {save.isError && (
        <p role='alert' className='text-sm text-destructive'>
          {t('saveFailed')}
        </p>
      )}
      <Card>
        <CardContent>
          <FieldGroup className='gap-4'>
            <Field className='max-w-md'>
              <FieldLabel htmlFor='attack-group-name'>{t('name')}</FieldLabel>
              <Input
                id='attack-group-name'
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={save.isPending}
                required
                maxLength={100}
              />
            </Field>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <p className='max-w-xl text-sm text-muted-foreground'>
                {t('levelHelp')}
              </p>
              <div className='flex flex-wrap items-center gap-2'>
                <AttackRuleCatalog editor={editor}>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={save.isPending}
                  >
                    <List data-icon='inline-start' />
                    {t('catalog.title')}
                  </Button>
                </AttackRuleCatalog>
                <Select
                  value=''
                  disabled={save.isPending}
                  onValueChange={(value: AttackLevel) =>
                    setLevels(
                      Object.fromEntries(
                        attackCategories.map((category) => [category, value]),
                      ),
                    )
                  }
                >
                  <SelectTrigger aria-label={t('applyAll')}>
                    <SelectValue placeholder={t('applyAll')} />
                  </SelectTrigger>
                  <SelectContent>
                    <LevelOptions />
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className='divide-y rounded-md border border-dashed'>
              {attackCategories.map((category) => {
                const label = t(`categories.${category}`);
                const { active, overrides } = ruleCounts(editor, category);
                return (
                  <div
                    key={category}
                    className='grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 px-3 py-2 sm:grid-cols-[minmax(0,1fr)_auto_10rem]'
                  >
                    <FieldLabel
                      htmlFor={`level-${category}`}
                      className='min-w-0'
                    >
                      {label}
                    </FieldLabel>
                    <div className='col-span-2 row-start-2 flex flex-wrap items-center gap-2 sm:col-span-1 sm:col-start-2 sm:row-start-1 sm:justify-end'>
                      {overrides > 0 && (
                        <Badge variant='secondary'>
                          {t('catalog.overrideCount', { count: overrides })}
                        </Badge>
                      )}
                      <AttackRuleCatalog category={category} editor={editor}>
                        <Button
                          type='button'
                          size='sm'
                          variant='ghost'
                          disabled={save.isPending}
                        >
                          {t('catalog.enabledRules', { count: active })}
                        </Button>
                      </AttackRuleCatalog>
                    </div>
                    <Select
                      value={levels[category] ?? 'off'}
                      disabled={save.isPending}
                      onValueChange={(value: AttackLevel) => {
                        setLevels((current) => ({
                          ...current,
                          [category]: value,
                        }));
                      }}
                    >
                      <SelectTrigger
                        id={`level-${category}`}
                        className='col-start-2 row-start-1 w-full sm:col-start-3'
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <LevelOptions />
                      </SelectContent>
                    </Select>
                  </div>
                );
              })}
            </div>
          </FieldGroup>
        </CardContent>
      </Card>
    </form>
  );
}

function LevelOptions() {
  const t = useTranslations('attackProtection');
  return (
    <SelectGroup>
      {attackLevels.map((level) => (
        <SelectItem key={level} value={level}>
          {t(`levels.${level}`)}
        </SelectItem>
      ))}
    </SelectGroup>
  );
}
