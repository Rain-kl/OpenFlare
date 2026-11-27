'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  GitBranch,
  Save,
} from 'lucide-react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { useEffect, useState, type MouseEvent } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Spinner } from '@/components/ui/spinner';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSPolicy,
  type SmartDNSPolicyValidateInput,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../../../websites/components/website-utils';
import { PolicyFlowEditor } from '../../policy-flow-editor';
import { PresetRuleFields } from '../../preset-rule-fields';
import { PolicyBasics } from './policy-basics';
import {
  balanceWeights,
  buildPolicyPayload,
  createPolicyDraft,
  type PolicyDraft,
  type PolicyEditorMode,
  smartWeights,
} from './policy-draft';

const policiesHref = '/dns/smartdns#policies';

export function SmartDNSPolicyEditor() {
  const t = useTranslations('smartdns.policies');
  const params = useSearchParams();
  const rawID = params.get('id');
  const id = Number(rawID);
  const isEdit = rawID !== null;
  const validID = Number.isSafeInteger(id) && id > 0;
  const policyQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'policy', id],
    queryFn: () => SmartDNSService.getPolicy(id),
    enabled: isEdit && validID,
  });
  if (!isEdit) return <PolicyForm key='new' />;
  if (!validID || policyQuery.isError) {
    return (
      <div className='flex w-full flex-col items-start gap-4 py-6 px-1'>
        <Button variant='outline' asChild>
          <Link href={policiesHref}>
            <ArrowLeft data-icon='inline-start' />
            {t('backToPolicies')}
          </Link>
        </Button>
        <ErrorInline
          message={
            validID ? getErrorMessage(policyQuery.error) : t('invalidPolicyId')
          }
          onRetry={validID ? () => void policyQuery.refetch() : undefined}
        />
      </div>
    );
  }
  if (!policyQuery.data)
    return (
      <LoadingStateWithBorder icon={GitBranch} description={t('loading')} />
    );
  return <PolicyForm key={id} policy={policyQuery.data} />;
}

function PolicyForm({ policy }: { policy?: SmartDNSPolicy }) {
  const t = useTranslations('smartdns.policies');
  const common = useTranslations('common');
  const router = useRouter();
  const queryClient = useQueryClient();
  const [initial] = useState(() => createPolicyDraft(policy));
  const [draft, setDraft] = useState(initial);
  const [section, setSection] = useState(
    policy && initial.mode === 'flow' ? 'routing' : 'basics',
  );
  const [exitOpen, setExitOpen] = useState(false);
  const [formError, setFormError] = useState('');
  const dirty =
    draft !== initial && JSON.stringify(draft) !== JSON.stringify(initial);
  const edit = (patch: Partial<PolicyDraft>) =>
    setDraft((previous) => ({ ...previous, ...patch }));

  useEffect(() => {
    if (!dirty) return;
    const preventUnload = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener('beforeunload', preventUnload);
    return () => window.removeEventListener('beforeunload', preventUnload);
  }, [dirty]);

  const payload = buildPolicyPayload(draft, policy);
  const validationKey = JSON.stringify([
    payload.strategy_type,
    payload.config,
    payload.jdm_json,
  ]);
  const validateMutation = useMutation({
    mutationFn: ({
      input,
    }: {
      key: string;
      input: SmartDNSPolicyValidateInput;
    }) => SmartDNSService.validatePolicy(input),
  });
  const saveMutation = useMutation({
    mutationFn: () =>
      policy
        ? SmartDNSService.updatePolicy(policy.id, payload)
        : SmartDNSService.createPolicy(payload),
    onSuccess: (saved) => {
      queryClient.setQueryData(
        [...smartdnsQueryKey, 'policy', saved.id],
        saved,
      );
      void queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
      toast.success(t(policy ? 'updated' : 'created'));
      router.replace(policiesHref);
    },
    onError: (error) => setFormError(getErrorMessage(error)),
  });
  const busy = validateMutation.isPending || saveMutation.isPending;
  const relevantValidation = validateMutation.variables?.key === validationKey;
  const validationError =
    relevantValidation && validateMutation.error
      ? getErrorMessage(validateMutation.error)
      : undefined;
  const compiledJdm = relevantValidation
    ? validateMutation.data?.compiled_jdm
    : undefined;
  const validate = () =>
    validateMutation.mutate({
      key: validationKey,
      input: {
        strategy_type: payload.strategy_type,
        config: payload.config,
        jdm_json: payload.jdm_json,
      },
    });
  const validateAndSave = () => {
    if (busy) return;
    setFormError('');
    if (
      !payload.name ||
      (draft.scope === 'domains' && !payload.scope_domains.length) ||
      !draft.priority.trim() ||
      !Number.isInteger(payload.priority) ||
      !Number.isInteger(payload.ttl) ||
      payload.ttl < 10 ||
      payload.ttl > 86400 ||
      !Number.isInteger(payload.max_answers) ||
      payload.max_answers < 1 ||
      payload.max_answers > 16
    ) {
      setSection('basics');
      setFormError(
        t(
          !payload.name
            ? 'missingPolicyName'
            : draft.scope === 'domains' && !payload.scope_domains.length
              ? 'missingScopeDomains'
              : 'invalidAnswerSettings',
        ),
      );
      return;
    }
    if (draft.mode === 'jdm' && !draft.jdm.trim()) {
      setSection('routing');
      setFormError(t('missingJdmJson'));
      return;
    }
    saveMutation.mutate();
  };
  const switchMode = (mode: PolicyEditorMode) => {
    if (mode === draft.mode) return;
    if (mode === 'flow' || mode === 'jdm') {
      edit({ mode, jdm: draft.jdm || compiledJdm || '' });
      setSection('routing');
    } else {
      edit({
        mode,
        preset: {
          ...draft.preset,
          ...(mode === 'load_balance' ? balanceWeights : smartWeights),
        },
      });
    }
  };
  const goBack = (event: MouseEvent<HTMLAnchorElement>) => {
    if (
      !dirty ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    event.preventDefault();
    setExitOpen(true);
  };

  return (
    <div className='flex w-full min-w-0 flex-col gap-4 py-6 px-1'>
      <Button variant='ghost' size='sm' className='self-start' asChild>
        <Link href={policiesHref} onClick={goBack}>
          <ArrowLeft data-icon='inline-start' />
          {t('backToPolicies')}
        </Link>
      </Button>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex min-w-0 items-center gap-2'>
          <GitBranch className='size-5 shrink-0 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight wrap-anywhere'>
            {policy
              ? t('editNamedPolicy', { name: policy.name })
              : t('createTitle')}
          </h1>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          {section === 'routing' &&
            (draft.mode === 'flow' || draft.mode === 'jdm') && (
              <Button
                variant='outline'
                size='sm'
                disabled={busy || (draft.mode === 'jdm' && !draft.jdm.trim())}
                onClick={validate}
              >
                {validateMutation.isPending ? (
                  <Spinner />
                ) : (
                  <CheckCircle2 data-icon='inline-start' />
                )}
                {draft.mode === 'flow' ? t('validateFlow') : t('validateJson')}
              </Button>
            )}
          <Button size='sm' disabled={busy} onClick={validateAndSave}>
            {saveMutation.isPending ? (
              <Spinner />
            ) : (
              <Save data-icon='inline-start' />
            )}
            {t('save')}
          </Button>
        </div>
      </div>
      {formError && (
        <Alert variant='destructive'>
          <AlertDescription>{formError}</AlertDescription>
        </Alert>
      )}
      <Tabs
        value={section}
        onValueChange={setSection}
        inert={saveMutation.isPending}
      >
        <div className='border-b border-dashed pb-1'>
          <TabsList variant='line'>
            <TabsTrigger value='basics'>{t('tabBasics')}</TabsTrigger>
            <TabsTrigger value='routing'>
              {draft.mode === 'flow' ? t('tabFlow') : t('tabRouting')}
            </TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value='basics' className='pt-2'>
          <div className='flex flex-col gap-4'>
            <PolicyBasics
              draft={draft}
              onChange={edit}
              onModeChange={switchMode}
            />
            <Button
              className='self-start'
              variant='outline'
              onClick={() => setSection('routing')}
            >
              {t('tabRouting')}
              <ArrowRight data-icon='inline-end' />
            </Button>
          </div>
        </TabsContent>
        <TabsContent value='routing' className='min-w-0 pt-2'>
          <div className='flex min-w-0 flex-col gap-3'>
            {(validationError || compiledJdm) && (
              <Alert variant={validationError ? 'destructive' : 'default'}>
                {!validationError && <CheckCircle2 />}
                <AlertDescription>
                  {validationError || t('validatedToast')}
                </AlertDescription>
              </Alert>
            )}
            {draft.mode === 'flow' ? (
              <>
                <PolicyFlowEditor
                  graph={draft.graph}
                  onChange={(update) =>
                    setDraft((previous) => ({
                      ...previous,
                      graph: update(previous.graph),
                    }))
                  }
                />
                {compiledJdm && (
                  <details className='rounded-lg border border-dashed p-4'>
                    <summary className='cursor-pointer text-sm font-medium'>
                      {t('compiledPreview')}
                    </summary>
                    <pre className='mt-3 max-h-72 overflow-auto font-mono text-xs'>
                      {JSON.stringify(JSON.parse(compiledJdm), null, 2)}
                    </pre>
                    <Button
                      variant='outline'
                      size='sm'
                      className='mt-3'
                      onClick={() =>
                        edit({
                          mode: 'jdm',
                          jdm: JSON.stringify(JSON.parse(compiledJdm), null, 2),
                        })
                      }
                    >
                      {t('editCompiledJson')}
                    </Button>
                  </details>
                )}
              </>
            ) : draft.mode === 'jdm' ? (
              <Field>
                <FieldLabel htmlFor='sdns-policy-jdm'>
                  {t('jdmJsonLabel')}
                </FieldLabel>
                <Textarea
                  id='sdns-policy-jdm'
                  rows={24}
                  className='font-mono text-sm'
                  value={draft.jdm}
                  spellCheck={false}
                  onChange={(event) => edit({ jdm: event.target.value })}
                />
              </Field>
            ) : (
              <div className='rounded-lg border border-dashed p-4'>
                <PresetRuleFields
                  rules={draft.preset}
                  onChange={(patch) =>
                    setDraft((previous) => ({
                      ...previous,
                      preset: { ...previous.preset, ...patch },
                    }))
                  }
                />
              </div>
            )}
          </div>
        </TabsContent>
      </Tabs>
      <AlertDialog open={exitOpen} onOpenChange={setExitOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('leaveTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('leaveDescription')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{common('cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => router.push(policiesHref)}>
              {t('leaveConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
