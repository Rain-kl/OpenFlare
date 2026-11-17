'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';
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
import { Switch } from '@/components/ui/switch';
import {
  PoWService,
  type PoWPolicy,
  type PoWPolicyConfig,
} from '@/lib/services/openflare';
import { PathRulesEditor } from './path-rules-editor';
import { proxyRouteFormIds } from '../../helpers';
import { SectionShell } from '../section-shell';

export function PolicyEditor({
  policy,
  configured = true,
}: {
  policy: PoWPolicy;
  configured?: boolean;
}) {
  const t = useTranslations('pow');
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(policy);
  const update = (change: Partial<PoWPolicyConfig>) =>
    setDraft((prev) => ({ ...prev, config: { ...prev.config, ...change } }));
  const save = useMutation({
    mutationFn: () => PoWService.save(draft),
    onSuccess: async () => {
      toast.success(t('saved'));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['openflare', 'pow'] }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'config-versions'],
        }),
      ]);
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : t('saveFailed')),
  });
  const limits = {
    difficulty: [1, 16],
    session_ttl: [60, 604800],
    challenge_ttl: [30, 3600],
  } as const;
  return (
    <SectionShell
      title={t('sitePolicy')}
      description={t(configured ? 'independentHelp' : 'unconfiguredHelp')}
      formId={proxyRouteFormIds.pow}
      saving={save.isPending}
    >
      <form
        id={proxyRouteFormIds.pow}
        className='flex flex-col gap-6'
        onSubmit={(event) => {
          event.preventDefault();
          save.mutate();
        }}
      >
        <FieldGroup>
          <Field orientation='horizontal'>
            <FieldLabel htmlFor='pow-enabled'>{t('enabled')}</FieldLabel>
            <Switch
              id='pow-enabled'
              checked={draft.enabled}
              onCheckedChange={(enabled) =>
                setDraft((prev) => ({ ...prev, enabled }))
              }
            />
          </Field>
          <Field>
            <FieldLabel htmlFor='pow-algorithm'>{t('algorithm')}</FieldLabel>
            <Select
              value={draft.config.algorithm}
              onValueChange={(algorithm: 'fast' | 'slow') =>
                update({ algorithm })
              }
            >
              <SelectTrigger id='pow-algorithm'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value='fast'>{t('fast')}</SelectItem>
                  <SelectItem value='slow'>{t('slow')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <div className='grid gap-4 md:grid-cols-3'>
            {(['difficulty', 'session_ttl', 'challenge_ttl'] as const).map(
              (key) => (
                <Field key={key}>
                  <FieldLabel htmlFor={`pow-${key}`}>{t(key)}</FieldLabel>
                  <Input
                    id={`pow-${key}`}
                    type='number'
                    required
                    min={limits[key][0]}
                    max={limits[key][1]}
                    value={draft.config[key]}
                    onChange={(event) =>
                      update({ [key]: Number(event.target.value) })
                    }
                  />
                </Field>
              ),
            )}
          </div>
          <FieldDescription>{t('difficultyHelp')}</FieldDescription>
        </FieldGroup>
        <PathRulesEditor
          value={{
            mode: draft.config.path_rules?.mode ?? 'all',
            whitelist: draft.config.path_rules?.whitelist ?? [],
            blacklist: draft.config.path_rules?.blacklist ?? [],
          }}
          onChange={(path_rules) => update({ path_rules })}
        />
        <Button className='w-fit' variant='link' asChild>
          <Link href='/responses?tab=pow'>{t('pageSettings')}</Link>
        </Button>
      </form>
    </SectionShell>
  );
}
