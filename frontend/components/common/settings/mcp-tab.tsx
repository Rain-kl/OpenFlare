'use client';

import Link from 'next/link';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useLocale, useTranslations } from 'next-intl';
import { CircleAlert, Copy } from 'lucide-react';
import { toast } from 'sonner';

import { AutomationSettings } from '@/components/common/settings/automation-settings';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import services from '@/lib/services';
import type { SystemConfig } from '@/lib/services/admin';

export function MCPTab({ config }: { config?: SystemConfig }) {
  const t = useTranslations('automation.settings');
  const locale = useLocale();
  const settings = useQuery({
    queryKey: ['admin', 'automation', 'mcp', config?.value],
    queryFn: () => services.automation.getMCPSettings(),
  });
  const revoke = useMutation({
    mutationFn: () => services.automation.revokeGrants(),
    onSuccess: () => {
      toast.success(t('revoked'));
    },
    onError: () => toast.error(t('revokeError')),
  });

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(t('copied'));
    } catch {
      toast.error(t('copyError'));
    }
  }

  const details = settings.data;
  const enabled =
    details?.enabled &&
    details.mcp_url &&
    details.instance_id &&
    details.client_name;
  return (
    <div className='flex w-full flex-col gap-4'>
      {config && <AutomationSettings key={config.value} config={config} />}
      {settings.isPending ? (
        <Spinner aria-label={t('loading')} />
      ) : settings.isError ? (
        <p role='alert' className='text-sm text-destructive'>
          {t('loadError')}
        </p>
      ) : details?.configuration_error === 'session_secret_too_short' ? (
        <Alert variant='destructive'>
          <CircleAlert />
          <AlertDescription>
            <p>{t('sessionSecretTooShort')}</p>
            <a
              href={`https://openflare.fyrn.link${locale === 'zh-CN' ? '' : '/en'}/guide/session-secret-rotation`}
              target='_blank'
              rel='noreferrer'
              className='underline underline-offset-4'
            >
              {t('rotationGuide')}
            </a>
          </AlertDescription>
        </Alert>
      ) : enabled ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle>{t('connectionTitle')}</CardTitle>
              <CardDescription>{t('connectionDescription')}</CardDescription>
            </CardHeader>
            <CardContent>
              <dl className='grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr]'>
                <dt className='font-medium'>{t('endpoint')}</dt>
                <dd>
                  <code className='wrap-anywhere'>{details.mcp_url}</code>
                </dd>
                <dt className='font-medium'>{t('instanceId')}</dt>
                <dd>
                  <code className='wrap-anywhere'>{details.instance_id}</code>
                </dd>
              </dl>
            </CardContent>
            <CardFooter className='flex-wrap gap-2'>
              <Button
                variant='outline'
                onClick={() => void copy(details.mcp_url!)}
              >
                <Copy data-icon='inline-start' />
                {t('copyURL')}
              </Button>
              <Button
                variant='outline'
                onClick={() =>
                  void copy(
                    t('instructions', {
                      name: details.client_name!,
                      url: details.mcp_url!,
                      instance: details.instance_id!,
                    }),
                  )
                }
              >
                <Copy data-icon='inline-start' />
                {t('copyInstructions')}
              </Button>
            </CardFooter>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>{t('tokenTitle')}</CardTitle>
              <CardDescription>{t('tokenDescription')}</CardDescription>
            </CardHeader>
            <CardContent>
              <code className='wrap-anywhere text-sm'>
                {t('authorizationHeader')}
              </code>
            </CardContent>
            <CardFooter className='flex flex-wrap gap-2'>
              <Button asChild>
                <Link href='/settings/access-token'>
                  {t('manageAccessTokens')}
                </Link>
              </Button>
              <Button
                variant='outline'
                onClick={() =>
                  void copy(
                    t('instructionsAccessToken', { url: details.mcp_url! }),
                  )
                }
              >
                <Copy data-icon='inline-start' />
                {t('copyInstructionsAccessToken')}
              </Button>
            </CardFooter>
          </Card>
        </>
      ) : (
        <p className='text-sm text-muted-foreground'>{t('disabled')}</p>
      )}
      <Card>
        <CardHeader>
          <CardTitle>{t('grantsTitle')}</CardTitle>
          <CardDescription>{t('revokeDescription')}</CardDescription>
        </CardHeader>
        <CardFooter>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant='outline' disabled={revoke.isPending}>
                {t('revoke')}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t('revoke')}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t('revokeDescription')}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t('cancel')}</AlertDialogCancel>
                <AlertDialogAction onClick={() => revoke.mutate()}>
                  {t('confirmRevoke')}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </CardFooter>
      </Card>
    </div>
  );
}
