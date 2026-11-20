'use client';

import { useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Gauge } from 'lucide-react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { type FormEvent, useId, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
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
import { Separator } from '@/components/ui/separator';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  PerformanceService,
  type PerformanceParameter,
  type PerformancePolicy,
  type PerformanceValue,
} from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

export const performancePoliciesQueryKey = [
  'openflare',
  'performance-policies',
] as const;

interface Props {
  policy?: PerformancePolicy;
  parameters: PerformanceParameter[];
}

export function PolicyEditor({ policy, parameters }: Props) {
  const t = useTranslations('performancePolicies');
  const router = useRouter();
  const client = useQueryClient();
  const id = useId();
  const formId = `${id}-form`;
  const [name, setName] = useState(policy?.name ?? '');
  const [nameInvalid, setNameInvalid] = useState(false);
  const [values, setValues] = useState(() =>
    Object.fromEntries(
      parameters.map(({ key, default: value }) => [
        key,
        policy?.settings[key] ?? value,
      ]),
    ),
  );
  const groups = [...new Set(parameters.map(({ group }) => group))];
  const [group, setGroup] = useState(() => groups[0] ?? '');
  const [saving, setSaving] = useState(false);
  const [invalid, setInvalid] = useState<string>();

  function update(key: string, value: PerformanceValue) {
    setValues((current) => ({ ...current, [key]: value }));
    if (invalid === key) setInvalid(undefined);
  }

  function focusElement(elementId: string) {
    requestAnimationFrame(() => {
      document.getElementById(elementId)?.focus();
    });
  }

  async function save(event: FormEvent) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      setNameInvalid(true);
      focusElement(`${id}-name`);
      return;
    }

    const invalidParameter = parameters.find(
      ({ key, min, max }) =>
        min !== undefined &&
        max !== undefined &&
        (String(values[key]).trim() === '' ||
          !Number.isInteger(Number(values[key])) ||
          Number(values[key]) < min ||
          Number(values[key]) > max),
    );
    if (invalidParameter) {
      setGroup(invalidParameter.group);
      setInvalid(invalidParameter.key);
      focusElement(`${id}-${invalidParameter.key}`);
      return;
    }

    const settings = Object.fromEntries(
      parameters.map(({ key, min }) => [
        key,
        min === undefined ? values[key] : Number(values[key]),
      ]),
    );
    setSaving(true);
    try {
      await PerformanceService.save(policy?.id, {
        name: trimmedName,
        settings,
      });
      toast.success(t('saved'));
      await client.invalidateQueries({ queryKey: performancePoliciesQueryKey });
      router.push('/performance');
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('operationFailed'),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className='flex w-full flex-col gap-6 px-1 py-6'>
      <div className='flex flex-col gap-4'>
        <Button
          variant='ghost'
          size='sm'
          className='-ml-2 w-fit text-xs'
          asChild
        >
          <Link href='/performance'>
            <ArrowLeft data-icon='inline-start' />
            {t('back')}
          </Link>
        </Button>

        <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
          <div className='flex flex-col gap-1.5'>
            <div className='flex items-center gap-2'>
              <Gauge className='size-5 text-primary' />
              <h1 className='text-2xl font-semibold tracking-tight'>
                {policy ? t('edit') : t('create')}
              </h1>
            </div>
            <p className='text-sm text-muted-foreground'>
              {t('editDescription')}
            </p>
          </div>
          <div className='flex flex-wrap items-center gap-2 sm:shrink-0'>
            <Button
              type='button'
              variant='outline'
              disabled={saving}
              onClick={() => router.push('/performance')}
            >
              {t('cancel')}
            </Button>
            <Button
              type='submit'
              form={formId}
              disabled={saving || !name.trim()}
            >
              {saving && <Spinner data-icon='inline-start' />}
              {t('save')}
            </Button>
          </div>
        </div>
      </div>

      <form
        id={formId}
        className='flex flex-col gap-6'
        onSubmit={(event) => void save(event)}
        noValidate
      >
        <Card>
          <CardHeader>
            <CardTitle className='text-base'>{t('basicInfo')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Field className='max-w-lg' data-invalid={nameInvalid || undefined}>
              <FieldLabel htmlFor={`${id}-name`}>{t('name')}</FieldLabel>
              <Input
                id={`${id}-name`}
                value={name}
                maxLength={100}
                placeholder={t('namePlaceholder')}
                aria-invalid={nameInvalid || undefined}
                aria-describedby={nameInvalid ? `${id}-name-error` : undefined}
                onChange={(event) => {
                  setName(event.target.value);
                  if (nameInvalid && event.target.value.trim()) {
                    setNameInvalid(false);
                  }
                }}
                required
              />
              {nameInvalid && (
                <FieldError id={`${id}-name-error`}>
                  {t('nameRequired')}
                </FieldError>
              )}
            </Field>
          </CardContent>
        </Card>

        {groups.length > 0 && (
          <Tabs value={group} onValueChange={setGroup} className='w-full'>
            <TabsList
              variant='line'
              className='mb-4 h-auto max-w-full flex-wrap justify-start gap-x-6 gap-y-2'
            >
              {groups.map((key) => (
                <TabsTrigger
                  key={key}
                  value={key}
                  className='flex-none px-0 pb-2 text-xs font-semibold'
                >
                  {t(`groups.${key}`)}
                </TabsTrigger>
              ))}
            </TabsList>

            {groups.map((key) => {
              const groupParameters = parameters.filter(
                (parameter) => parameter.group === key,
              );
              const toggleParameters = groupParameters.filter(
                (parameter) => typeof parameter.default === 'boolean',
              );
              const valueParameters = groupParameters.filter(
                (parameter) => typeof parameter.default !== 'boolean',
              );

              return (
                <TabsContent
                  key={key}
                  value={key}
                  className='focus-visible:outline-none'
                >
                  <Card>
                    <CardHeader>
                      <CardTitle className='text-base'>
                        {t(`groups.${key}`)}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className='flex flex-col gap-6'>
                      {toggleParameters.length > 0 && (
                        <FieldGroup
                          className={cn(
                            'grid gap-3',
                            toggleParameters.length > 1 && 'md:grid-cols-2',
                          )}
                        >
                          {toggleParameters.map((parameter) => {
                            const inputID = `${id}-${parameter.key}`;
                            const descriptionID = `${inputID}-help`;
                            const label = t(`fields.${parameter.key}.label`);
                            const isInvalid = invalid === parameter.key;

                            return (
                              <Field
                                key={parameter.key}
                                orientation='horizontal'
                                data-invalid={isInvalid || undefined}
                                className='justify-between gap-4 rounded-lg border border-dashed p-4'
                              >
                                <FieldContent>
                                  <FieldLabel
                                    htmlFor={inputID}
                                    className='cursor-pointer'
                                  >
                                    {label}
                                  </FieldLabel>
                                  <FieldDescription id={descriptionID}>
                                    {t(`fields.${parameter.key}.help`)}
                                  </FieldDescription>
                                </FieldContent>
                                <Switch
                                  id={inputID}
                                  className='self-center'
                                  checked={values[parameter.key] === true}
                                  onCheckedChange={(value) =>
                                    update(parameter.key, value)
                                  }
                                  aria-describedby={descriptionID}
                                />
                              </Field>
                            );
                          })}
                        </FieldGroup>
                      )}

                      {toggleParameters.length > 0 &&
                        valueParameters.length > 0 && <Separator />}

                      {valueParameters.length > 0 && (
                        <FieldGroup className='grid gap-x-6 gap-y-5 md:grid-cols-2'>
                          {valueParameters.map((parameter) => {
                            const {
                              key: paramKey,
                              choices,
                              min,
                              max,
                            } = parameter;
                            const inputID = `${id}-${paramKey}`;
                            const descriptionID = `${inputID}-help`;
                            const errorID = `${inputID}-error`;
                            const label = t(`fields.${paramKey}.label`);
                            const disabled =
                              paramKey === 'threads' &&
                              values.threads_auto === true;
                            const isInvalid = invalid === paramKey;

                            return (
                              <Field
                                key={paramKey}
                                data-invalid={isInvalid || undefined}
                                data-disabled={disabled || undefined}
                              >
                                <FieldLabel htmlFor={inputID}>
                                  {label}
                                </FieldLabel>
                                {choices ? (
                                  <Select
                                    value={String(values[paramKey])}
                                    onValueChange={(value) =>
                                      update(paramKey, value)
                                    }
                                  >
                                    <SelectTrigger
                                      id={inputID}
                                      aria-describedby={descriptionID}
                                    >
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                      <SelectGroup>
                                        {choices.map((choice) => (
                                          <SelectItem
                                            key={choice}
                                            value={choice}
                                          >
                                            {t(`choices.${choice}`)}
                                          </SelectItem>
                                        ))}
                                      </SelectGroup>
                                    </SelectContent>
                                  </Select>
                                ) : (
                                  <Input
                                    id={inputID}
                                    type='number'
                                    min={min}
                                    max={max}
                                    step={1}
                                    value={String(values[paramKey])}
                                    disabled={disabled}
                                    aria-invalid={isInvalid || undefined}
                                    aria-describedby={
                                      isInvalid
                                        ? `${descriptionID} ${errorID}`
                                        : descriptionID
                                    }
                                    onChange={(event) =>
                                      update(paramKey, event.target.value)
                                    }
                                    onBlur={() => {
                                      if (
                                        String(values[paramKey]).trim() === ''
                                      ) {
                                        update(paramKey, parameter.default);
                                      }
                                    }}
                                  />
                                )}
                                <FieldDescription id={descriptionID}>
                                  {t(`fields.${paramKey}.help`)}
                                </FieldDescription>
                                {isInvalid &&
                                  min !== undefined &&
                                  max !== undefined && (
                                    <FieldError id={errorID}>
                                      {t('range', { min, max })}
                                    </FieldError>
                                  )}
                              </Field>
                            );
                          })}
                        </FieldGroup>
                      )}
                    </CardContent>
                  </Card>
                </TabsContent>
              );
            })}
          </Tabs>
        )}
      </form>
    </div>
  );
}
