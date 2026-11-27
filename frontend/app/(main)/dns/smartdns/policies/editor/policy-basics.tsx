import { useTranslations } from 'next-intl';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
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
import type { PolicyDraft, PolicyEditorMode } from './policy-draft';

export function PolicyBasics({
  draft,
  onChange,
  onModeChange,
}: {
  draft: PolicyDraft;
  onChange: (patch: Partial<PolicyDraft>) => void;
  onModeChange: (mode: PolicyEditorMode) => void;
}) {
  const t = useTranslations('smartdns.policies');
  return (
    <div className='grid items-start gap-4 xl:grid-cols-[minmax(0,1.25fr)_minmax(0,1fr)]'>
      <Card>
        <CardHeader>
          <CardTitle>{t('tabBasics')}</CardTitle>
        </CardHeader>
        <CardContent>
          <FieldGroup className='gap-4'>
            <Field>
              <FieldLabel htmlFor='sdns-pol-name'>{t('name')}</FieldLabel>
              <Input
                id='sdns-pol-name'
                required
                maxLength={128}
                value={draft.name}
                onChange={(event) => onChange({ name: event.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor='sdns-pol-strategy'>
                {t('strategyType')}
              </FieldLabel>
              <Select
                value={draft.mode}
                onValueChange={(mode) => onModeChange(mode as PolicyEditorMode)}
              >
                <SelectTrigger id='sdns-pol-strategy' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='smart'>{t('strategySmart')}</SelectItem>
                    <SelectItem value='load_balance'>
                      {t('strategyLB')}
                    </SelectItem>
                    <SelectItem value='flow'>{t('tabFlow')}</SelectItem>
                    <SelectItem value='jdm'>{t('tabJdmJson')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel htmlFor='sdns-pol-desc'>{t('descLabel')}</FieldLabel>
              <Input
                id='sdns-pol-desc'
                value={draft.description}
                onChange={(event) =>
                  onChange({ description: event.target.value })
                }
              />
            </Field>
            <Field>
              <FieldLabel htmlFor='sdns-pol-scope'>{t('scopeType')}</FieldLabel>
              <Select
                value={draft.scope}
                onValueChange={(scope) =>
                  onChange({ scope: scope as PolicyDraft['scope'] })
                }
              >
                <SelectTrigger id='sdns-pol-scope' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='all'>{t('scopeAll')}</SelectItem>
                    <SelectItem value='domains'>{t('scopeDomains')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            {draft.scope === 'domains' && (
              <Field>
                <FieldLabel htmlFor='sdns-pol-domains'>
                  {t('scopeDomainsList')}
                </FieldLabel>
                <Input
                  id='sdns-pol-domains'
                  value={draft.domains}
                  placeholder='*.example.com, app.example.com'
                  onChange={(event) =>
                    onChange({ domains: event.target.value })
                  }
                />
              </Field>
            )}
            <Field orientation='horizontal'>
              <FieldLabel htmlFor='sdns-policy-enabled'>
                {t('enablePolicy')}
              </FieldLabel>
              <Switch
                id='sdns-policy-enabled'
                checked={draft.enabled}
                onCheckedChange={(enabled) => onChange({ enabled })}
              />
            </Field>
          </FieldGroup>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('answerSettings')}</CardTitle>
        </CardHeader>
        <CardContent>
          <FieldGroup className='gap-4'>
            <div className='grid grid-cols-2 gap-3 2xl:grid-cols-3'>
              {[
                {
                  key: 'priority',
                  label: t('priority'),
                  min: undefined,
                  max: undefined,
                },
                { key: 'ttl', label: t('ttlLabel'), min: 10, max: 86400 },
                { key: 'maxAnswers', label: t('maxAnswers'), min: 1, max: 16 },
              ].map(({ key, label, min, max }) => (
                <Field key={key}>
                  <FieldLabel htmlFor={`sdns-pol-${key}`}>{label}</FieldLabel>
                  <Input
                    id={`sdns-pol-${key}`}
                    type='number'
                    step={1}
                    min={min}
                    max={max}
                    value={draft[key as 'priority' | 'ttl' | 'maxAnswers']}
                    onChange={(event) =>
                      onChange({ [key]: event.target.value })
                    }
                  />
                </Field>
              ))}
            </div>
            <Field>
              <FieldLabel htmlFor='sdns-pol-fb'>
                {t('fallbackStrategy')}
              </FieldLabel>
              <Select
                value={draft.fallback}
                onValueChange={(fallback) => onChange({ fallback })}
              >
                <SelectTrigger id='sdns-pol-fb' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='smart'>{t('fbSmart')}</SelectItem>
                    <SelectItem value='load_balance'>
                      {t('fbLoadBalance')}
                    </SelectItem>
                    <SelectItem value='static_nodes'>
                      {t('fbStaticNodes')}
                    </SelectItem>
                    <SelectItem value='all_healthy'>
                      {t('fbAllHealthy')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            {draft.fallback === 'static_nodes' && (
              <Field>
                <FieldLabel htmlFor='sdns-pol-fb-nodes'>
                  {t('fbNodeIds')}
                </FieldLabel>
                <Input
                  id='sdns-pol-fb-nodes'
                  value={draft.fallbackNodes}
                  placeholder='node-1, node-2'
                  onChange={(event) =>
                    onChange({ fallbackNodes: event.target.value })
                  }
                />
              </Field>
            )}
          </FieldGroup>
        </CardContent>
      </Card>
    </div>
  );
}
