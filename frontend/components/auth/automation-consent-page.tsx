'use client';

import { useState, useSyncExternalStore } from 'react';
import { useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { CircleAlert } from 'lucide-react';

import { AuthHeading, AuthShell } from '@/components/auth/auth-shell';
import { AutomationPermissions } from '@/components/auth/automation-permissions';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import type { ConsentDetails } from '@/lib/services/automation';

const authorizePath = '/oauth/automation/authorize';

function subscribeToHash(onChange: () => void) {
  window.addEventListener('hashchange', onChange);
  return () => window.removeEventListener('hashchange', onChange);
}

function getConsentIntent() {
  return new URLSearchParams(window.location.hash.slice(1)).get('intent') || '';
}

async function loadConsent(
  intent: string,
  signal: AbortSignal,
): Promise<ConsentDetails> {
  const response = await fetch('/oauth/automation/consent', {
    method: 'POST',
    body: new URLSearchParams({ intent }),
    credentials: 'same-origin',
    cache: 'no-store',
    signal,
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(
      typeof payload?.error === 'string' ? payload.error : 'invalid_request',
    );
  }
  return response.json();
}

export function AutomationConsentPage() {
  const t = useTranslations('automation.consent');
  const params = useSearchParams();
  const intent = useSyncExternalStore(
    subscribeToHash,
    getConsentIntent,
    () => null,
  );
  const consent = useQuery({
    queryKey: ['automation-consent', intent],
    queryFn: ({ signal }) => loadConsent(intent ?? '', signal),
    enabled: !!intent && !params.has('error'),
    retry: false,
    staleTime: Infinity,
    gcTime: 0,
  });
  const [selected, setSelected] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const loading = intent === null || (consent.isFetching && !consent.data);
  const error = params.get('error') ?? consent.error?.message;

  return (
    <AuthShell>
      <div className='flex w-full flex-col gap-6'>
        <AuthHeading
          title={t('title')}
          description={
            consent.data
              ? t('description', { client: consent.data.client_name })
              : t('review')
          }
        />
        {loading ? (
          <Spinner role='status' aria-label={t('loading')} />
        ) : consent.data && !error ? (
          <form
            method='post'
            action={authorizePath}
            className='flex flex-col gap-6'
            onSubmit={() => setSubmitting(true)}
          >
            <input type='hidden' name='intent' value={intent} />
            <input type='hidden' name='scope' value={selected.join(' ')} />
            <div className='flex flex-col gap-1 text-sm'>
              <p className='font-medium'>{t('callback')}</p>
              <code className='wrap-anywhere text-muted-foreground'>
                {consent.data.redirect_uri}
              </code>
            </div>
            <AutomationPermissions
              scopes={consent.data.scopes}
              selected={selected}
              onChange={setSelected}
              disabled={submitting}
            />
            <div className='flex flex-wrap justify-end gap-2'>
              <Button
                type='submit'
                name='decision'
                value='cancel'
                variant='outline'
              >
                {t('cancel')}
              </Button>
              <Button
                type='submit'
                name='decision'
                value='authorize'
                variant='auth'
                disabled={selected.length === 0}
              >
                {t('confirm')}
              </Button>
            </div>
          </form>
        ) : (
          <Alert variant='destructive'>
            <CircleAlert />
            <AlertDescription>
              {error === 'access_denied'
                ? t('adminRequired')
                : t('invalidRequest')}
            </AlertDescription>
          </Alert>
        )}
      </div>
    </AuthShell>
  );
}
