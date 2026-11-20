import { useTranslations } from 'next-intl';
import { type FormEvent, useId, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
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
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  PerformanceService,
  type PerformanceParameter,
  type PerformancePolicy,
  type PerformanceValue,
} from '@/lib/services/openflare';

interface Props {
  policy?: PerformancePolicy;
  parameters: PerformanceParameter[];
  onClose: () => void;
  onSaved: () => void;
}

export function PolicyEditor({ policy, parameters, onClose, onSaved }: Props) {
  const t = useTranslations('performancePolicies');
  const id = useId();
  const [name, setName] = useState(policy?.name ?? '');
  const [values, setValues] = useState(() =>
    Object.fromEntries(
      parameters.map(({ key, default: value }) => [
        key,
        policy?.settings[key] ?? value,
      ]),
    ),
  );
  const groups = [...new Set(parameters.map(({ group }) => group))];
  const [group, setGroup] = useState(groups[0]);
  const [saving, setSaving] = useState(false);
  const [invalid, setInvalid] = useState<string>();

  function update(key: string, value: PerformanceValue) {
    setValues((current) => ({ ...current, [key]: value }));
    if (invalid === key) setInvalid(undefined);
  }

  async function save(event: FormEvent) {
    event.preventDefault();
    const invalidParameter = parameters.find(
      ({ key, min, max }) =>
        min !== undefined &&
        max !== undefined &&
        (values[key] === '' ||
          !Number.isInteger(Number(values[key])) ||
          Number(values[key]) < min ||
          Number(values[key]) > max),
    );
    if (invalidParameter) {
      setGroup(invalidParameter.group);
      setInvalid(invalidParameter.key);
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
        name: name.trim(),
        settings,
      });
      toast.success(t('saved'));
      onSaved();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('operationFailed'),
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !saving && onClose()}>
      <DialogContent className='flex max-h-[90dvh] flex-col sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{policy ? t('edit') : t('create')}</DialogTitle>
          <DialogDescription>{t('editDescription')}</DialogDescription>
        </DialogHeader>
        <form
          className='flex min-h-0 flex-col gap-4'
          onSubmit={(event) => void save(event)}
          noValidate
        >
          <Field>
            <FieldLabel htmlFor={`${id}-name`}>{t('name')}</FieldLabel>
            <Input
              id={`${id}-name`}
              value={name}
              maxLength={100}
              onChange={(event) => setName(event.target.value)}
              required
            />
          </Field>
          <Tabs value={group} onValueChange={setGroup} className='min-h-0'>
            <TabsList className='h-auto max-w-full flex-wrap self-start'>
              {groups.map((key) => (
                <TabsTrigger key={key} value={key}>
                  {t(`groups.${key}`)}
                </TabsTrigger>
              ))}
            </TabsList>
            <TabsContent value={group} className='overflow-y-auto p-1'>
              <FieldGroup className='grid gap-5 sm:grid-cols-2'>
                {parameters
                  .filter((parameter) => parameter.group === group)
                  .map((parameter) => {
                    const { key, choices, min, max } = parameter;
                    const inputID = `${id}-${key}`;
                    const descriptionID = `${inputID}-help`;
                    const label = t(`fields.${key}.label`);
                    const disabled =
                      key === 'threads' && values.threads_auto === true;
                    const isInvalid = invalid === key;
                    return (
                      <Field
                        key={key}
                        data-invalid={isInvalid}
                        data-disabled={disabled}
                      >
                        <FieldLabel htmlFor={inputID}>{label}</FieldLabel>
                        {typeof parameter.default === 'boolean' ? (
                          <Switch
                            id={inputID}
                            checked={values[key] === true}
                            onCheckedChange={(value) => update(key, value)}
                            aria-describedby={descriptionID}
                          />
                        ) : choices ? (
                          <Select
                            value={String(values[key])}
                            onValueChange={(value) => update(key, value)}
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
                                  <SelectItem key={choice} value={choice}>
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
                            value={String(values[key])}
                            disabled={disabled}
                            aria-invalid={isInvalid}
                            aria-describedby={descriptionID}
                            onChange={(event) =>
                              update(key, event.target.value)
                            }
                            onBlur={() => {
                              if (values[key] === '')
                                update(key, parameter.default);
                            }}
                          />
                        )}
                        <FieldDescription id={descriptionID}>
                          {t(`fields.${key}.help`)}
                          {isInvalid &&
                            min !== undefined &&
                            max !== undefined &&
                            ` ${t('range', { min, max })}`}
                        </FieldDescription>
                      </Field>
                    );
                  })}
              </FieldGroup>
            </TabsContent>
          </Tabs>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={saving}
              onClick={onClose}
            >
              {t('cancel')}
            </Button>
            <Button type='submit' disabled={saving || !name.trim()}>
              {t('save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
