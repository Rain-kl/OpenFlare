import { useMutation } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
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
} from '@/lib/services/openflare/attack-types';
import { WafService } from '@/lib/services/openflare';

import { AttackRuleCatalog, countEnabledRules } from './attack-rule-catalog';

export function AttackGroupEditor({
  group,
  onCancel,
  onSaved,
}: {
  group?: AttackGroup;
  onCancel: () => void;
  onSaved: () => Promise<void>;
}) {
  const t = useTranslations('attackProtection');
  const [name, setName] = useState(group?.name ?? '');
  const [levels, setLevels] = useState<AttackGroupInput['levels']>(
    group?.levels ?? {},
  );
  const save = useMutation({
    mutationFn: () =>
      WafService.saveAttackGroup({ name: name.trim(), levels }, group?.id),
    onSuccess: onSaved,
  });

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        save.mutate();
      }}
    >
      <Card>
        <CardHeader>
          <CardTitle>{group ? t('editGroup') : t('create')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-5'>
          <div className='grid items-end gap-4 sm:grid-cols-[minmax(0,24rem)_auto] sm:justify-between'>
            <div className='grid gap-2'>
              <Label htmlFor='attack-group-name'>{t('name')}</Label>
              <Input
                id='attack-group-name'
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
                maxLength={100}
              />
            </div>
            <Select
              value=''
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
                {attackLevels.map((level) => (
                  <SelectItem key={level} value={level}>
                    {t(`levels.${level}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <p className='text-sm text-muted-foreground'>{t('levelHelp')}</p>
          <div className='grid divide-y rounded-md border border-dashed'>
            {attackCategories.map((category) => (
              <fieldset
                key={category}
                className='grid items-center gap-3 px-4 py-3 md:grid-cols-[minmax(10rem,1fr)_minmax(0,2fr)]'
              >
                <div className='flex flex-col items-start gap-1'>
                  <span
                    id={`${category}-label`}
                    className='text-sm font-medium'
                  >
                    {t(`categories.${category}`)}
                  </span>
                  <AttackRuleCatalog
                    category={category}
                    level={levels[category] ?? 'off'}
                  >
                    <Button
                      type='button'
                      size='sm'
                      variant='link'
                      className='h-auto p-0'
                    >
                      {t('catalog.enabledRules', {
                        count: countEnabledRules(
                          category,
                          levels[category] ?? 'off',
                        ),
                      })}
                    </Button>
                  </AttackRuleCatalog>
                </div>
                <div
                  role='radiogroup'
                  aria-labelledby={`${category}-label`}
                  className='grid grid-cols-2 gap-2 lg:grid-cols-4'
                >
                  {attackLevels.map((level) => (
                    <label
                      key={level}
                      className='flex cursor-pointer items-center gap-2 rounded-md border border-dashed px-2 py-2 text-sm has-checked:border-primary has-checked:bg-accent'
                    >
                      <input
                        className='size-4 accent-primary'
                        type='radio'
                        name={category}
                        value={level}
                        checked={(levels[category] ?? 'off') === level}
                        onChange={() =>
                          setLevels({ ...levels, [category]: level })
                        }
                      />
                      {t(`levels.${level}`)}
                    </label>
                  ))}
                </div>
              </fieldset>
            ))}
          </div>
          {save.isError && (
            <p role='alert' className='text-sm text-destructive'>
              {t('saveFailed')}
            </p>
          )}
        </CardContent>
        <CardFooter className='justify-end gap-2'>
          <Button
            type='button'
            variant='outline'
            disabled={save.isPending}
            onClick={onCancel}
          >
            {t('cancel')}
          </Button>
          <Button type='submit' disabled={!name.trim() || save.isPending}>
            {t('save')}
          </Button>
        </CardFooter>
      </Card>
    </form>
  );
}
