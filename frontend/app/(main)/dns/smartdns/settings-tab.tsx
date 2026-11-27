'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Globe2, Save, ShieldCheck, XCircle } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSSettings,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

const settingsKey = [...smartdnsQueryKey, 'settings'] as const;

function toDraft(settings: SmartDNSSettings) {
  return {
    enabled: settings.enabled,
    delegation: settings.delegation_domain ?? '',
    hosts: (settings.ns_hosts ?? []).join('\n'),
    ttl: String(settings.default_ttl ?? 60),
    interval: String(settings.snapshot_interval_seconds ?? 10),
    repo: settings.update_repo ?? '',
  };
}

export function SmartDNSSettingsTab() {
  const t = useTranslations('smartdns');
  const query = useQuery({
    queryKey: settingsKey,
    queryFn: () => SmartDNSService.getSettings(),
  });
  if (query.isPending)
    return (
      <LoadingStateWithBorder
        icon={Globe2}
        description={t('settingsLoading')}
      />
    );
  if (query.isError)
    return (
      <ErrorInline
        message={getErrorMessage(query.error)}
        onRetry={() => void query.refetch()}
      />
    );
  return <SmartDNSSettingsForm settings={query.data} />;
}

function SmartDNSSettingsForm({ settings }: { settings: SmartDNSSettings }) {
  const t = useTranslations('smartdns');
  const queryClient = useQueryClient();
  const [changes, setChanges] = useState<Partial<ReturnType<typeof toDraft>>>(
    {},
  );
  const saved = toDraft(settings);
  const draft = { ...saved, ...changes };
  const update = (patch: Partial<typeof draft>) =>
    setChanges((previous) => ({ ...previous, ...patch }));
  const delegationDirty =
    draft.delegation !== saved.delegation || draft.hosts !== saved.hosts;
  const dirty =
    delegationDirty ||
    draft.enabled !== saved.enabled ||
    draft.ttl !== saved.ttl ||
    draft.interval !== saved.interval ||
    draft.repo !== saved.repo;
  const ttl = Number(draft.ttl);
  const interval = Number(draft.interval);
  const validNumbers =
    Number.isInteger(ttl) &&
    ttl >= 10 &&
    ttl <= 86400 &&
    Number.isInteger(interval) &&
    interval >= 3 &&
    interval <= 300;

  const verifyMutation = useMutation({
    mutationFn: () => SmartDNSService.verifyDelegation(),
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const saveMutation = useMutation({
    mutationFn: () =>
      SmartDNSService.saveSettings({
        enabled: draft.enabled,
        delegation_domain: draft.delegation.trim(),
        ns_hosts: draft.hosts
          .split(/[\n,]+/)
          .map((host) => host.trim())
          .filter(Boolean),
        default_ttl: ttl,
        snapshot_interval_seconds: interval,
        update_repo: draft.repo.trim(),
      }),
    onSuccess: async (data) => {
      queryClient.setQueryData(settingsKey, data);
      setChanges({});
      verifyMutation.reset();
      toast.success(t('settingsSaved'));
      await queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const busy = saveMutation.isPending || verifyMutation.isPending;
  const verifyResult = verifyMutation.data;
  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-base'>{t('settingsTitle')}</CardTitle>
        <CardDescription>{t('settingsDesc')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className='flex flex-col gap-5'
          onSubmit={(event) => {
            event.preventDefault();
            if (dirty && validNumbers && !busy) saveMutation.mutate();
          }}
        >
          <FieldGroup className='gap-5 sm:grid sm:grid-cols-2'>
            <Field
              orientation='horizontal'
              className='justify-between sm:col-span-2'
            >
              <div>
                <FieldLabel htmlFor='sdns-enabled'>
                  {t('enableLabel')}
                </FieldLabel>
                <FieldDescription>{t('enableDesc')}</FieldDescription>
              </div>
              <Switch
                id='sdns-enabled'
                checked={draft.enabled}
                onCheckedChange={(enabled) => update({ enabled })}
                disabled={busy}
              />
            </Field>

            <Field>
              <FieldLabel htmlFor='sdns-delegation-domain'>
                {t('delegationDomain')}
              </FieldLabel>
              <Input
                id='sdns-delegation-domain'
                value={draft.delegation}
                disabled={busy}
                placeholder='sdns.example.com'
                onChange={(event) => update({ delegation: event.target.value })}
              />
              <FieldDescription>{t('delegationDomainDesc')}</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor='sdns-ns-hosts'>{t('nsHosts')}</FieldLabel>
              <Textarea
                id='sdns-ns-hosts'
                rows={3}
                value={draft.hosts}
                disabled={busy}
                placeholder={'ns1.sdns.example.com\nns2.sdns.example.com'}
                onChange={(event) => update({ hosts: event.target.value })}
              />
              <FieldDescription>{t('nsHostsDesc')}</FieldDescription>
            </Field>
          </FieldGroup>
          <details className='rounded-lg border border-dashed px-4 py-3'>
            <summary className='cursor-pointer text-sm font-medium'>
              {t('advancedSettings')}
            </summary>
            <FieldGroup className='mt-4 grid gap-4 sm:grid-cols-2'>
              <Field>
                <FieldLabel htmlFor='sdns-default-ttl'>
                  {t('defaultTTL')}
                </FieldLabel>
                <Input
                  id='sdns-default-ttl'
                  type='number'
                  min={10}
                  max={86400}
                  required
                  value={draft.ttl}
                  disabled={busy}
                  onChange={(event) => update({ ttl: event.target.value })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='sdns-snap-interval'>
                  {t('snapshotInterval')}
                </FieldLabel>
                <Input
                  id='sdns-snap-interval'
                  type='number'
                  min={3}
                  max={300}
                  required
                  value={draft.interval}
                  disabled={busy}
                  onChange={(event) => update({ interval: event.target.value })}
                />
              </Field>
              <Field className='sm:col-span-2'>
                <FieldLabel htmlFor='sdns-update-repo'>
                  {t('updateRepo')}
                </FieldLabel>
                <Input
                  id='sdns-update-repo'
                  value={draft.repo}
                  disabled={busy}
                  placeholder='Rain-kl/OpenFlare'
                  onChange={(event) => update({ repo: event.target.value })}
                />
              </Field>
            </FieldGroup>
          </details>

          <div className='flex flex-wrap items-center gap-2'>
            <Button type='submit' disabled={busy || !dirty || !validNumbers}>
              <Save data-icon='inline-start' />
              {t('saveSettings')}
            </Button>
            <Button
              variant='outline'
              onClick={() => verifyMutation.mutate()}
              type='button'
              disabled={
                busy ||
                delegationDirty ||
                !settings.delegation_domain ||
                !settings.ns_hosts?.length
              }
            >
              <ShieldCheck data-icon='inline-start' />
              {t('verifyDelegation')}
            </Button>
          </div>
        </form>
        {delegationDirty && (
          <p className='mt-3 text-sm text-muted-foreground'>
            {t('saveBeforeVerify')}
          </p>
        )}
        {verifyResult && !delegationDirty ? (
          <Alert className='mt-4'>
            {verifyResult.verified ? (
              <CheckCircle2 className='text-primary' />
            ) : (
              <XCircle className='text-destructive' />
            )}
            <AlertTitle>
              {verifyResult.verified
                ? t('verifyPassTitle')
                : t('verifyFailTitle')}
            </AlertTitle>
            <AlertDescription className='flex flex-col gap-1 text-xs'>
              <p>
                {t('verifyObserved', {
                  records: verifyResult.observed_ns_records.join(', ') || '-',
                })}
              </p>
              {verifyResult.missing_ns_hosts.length > 0 ? (
                <p className='text-destructive'>
                  {t('verifyMissing', {
                    hosts: verifyResult.missing_ns_hosts.join(', '),
                  })}
                </p>
              ) : null}
              {verifyResult.error ? (
                <p className='text-destructive'>{verifyResult.error}</p>
              ) : null}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
    </Card>
  );
}
