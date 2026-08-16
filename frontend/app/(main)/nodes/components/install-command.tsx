'use client';

import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Copy } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { type NodeItem, StatusService } from '@/lib/services/openflare';

import {
  buildEdgeDockerInstallCommand,
  buildRelayDockerInstallCommand,
  buildRelayInstallCommand,
  buildTunnelDockerInstallCommand,
  buildTunnelInstallCommand,
  getServerUrl,
} from './node-utils';

type InstallVariant = 'edge' | 'relay' | 'tunnel';

export function InstallCommand({
  node,
  variant,
}: {
  node: NodeItem;
  variant: InstallVariant;
}) {
  const t = useTranslations('nodes.install');
  const tn = useTranslations('nodes');
  const [serverUrl, setServerUrl] = useState('');
  const variantMeta: Record<
    InstallVariant,
    {
      title: string;
      description: string;
      tokenLabel: string;
      scriptLabel?: string;
      dockerLabel: string;
    }
  > = {
    edge: {
      title: t('edgeTitle'),
      description: t('edgeDesc'),
      tokenLabel: t('agentToken'),
      dockerLabel: t('dockerLabel'),
    },
    relay: {
      title: t('relayTitle'),
      description: t('relayDesc'),
      tokenLabel: t('discoveryToken'),
      scriptLabel: t('scriptLabel'),
      dockerLabel: t('dockerLabel'),
    },
    tunnel: {
      title: t('tunnelTitle'),
      description: t('tunnelDesc'),
      tokenLabel: t('tunnelToken'),
      scriptLabel: t('scriptLabel'),
      dockerLabel: t('dockerLabel'),
    },
  };
  const meta = variantMeta[variant];

  const statusQuery = useQuery({
    queryKey: ['openflare', 'public-status'],
    queryFn: () => StatusService.getPublicStatus(),
    staleTime: 5 * 60 * 1000,
  });

  const serverVersion = statusQuery.data?.version;

  useEffect(() => {
    if (typeof window !== 'undefined' && !serverUrl) {
      setServerUrl(window.location.origin);
    }
  }, [serverUrl]);

  const normalizedServerUrl = getServerUrl(serverUrl);
  const scriptCommand = useMemo(() => {
    if (!normalizedServerUrl || !node.access_token || variant === 'edge') {
      return '';
    }
    return variant === 'relay'
      ? buildRelayInstallCommand(normalizedServerUrl, node.access_token)
      : buildTunnelInstallCommand(normalizedServerUrl, node.access_token);
  }, [normalizedServerUrl, node.access_token, variant]);

  const dockerCommand = useMemo(() => {
    if (!normalizedServerUrl || !node.access_token) {
      return '';
    }
    if (variant === 'edge') {
      return buildEdgeDockerInstallCommand(
        normalizedServerUrl,
        node.access_token,
        serverVersion,
      );
    }
    return variant === 'relay'
      ? buildRelayDockerInstallCommand(
          normalizedServerUrl,
          node.access_token,
          serverVersion,
        )
      : buildTunnelDockerInstallCommand(
          normalizedServerUrl,
          node.access_token,
          serverVersion,
        );
  }, [normalizedServerUrl, node.access_token, variant, serverVersion]);

  const handleCopy = async (value: string, message: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(message);
    } catch {
      toast.error(t('copyFailed'));
    }
  };

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='flex-row items-start justify-between space-y-0 gap-4'>
        <div>
          <CardTitle className='text-base font-semibold'>
            {meta.title}
          </CardTitle>
          <CardDescription>{meta.description}</CardDescription>
        </div>
        <div className='flex flex-wrap gap-2'>
          {scriptCommand ? (
            <Button
              variant='secondary'
              size='sm'
              className='h-7 text-xs'
              onClick={() => void handleCopy(scriptCommand, t('copiedScript'))}
            >
              <Copy className='size-3.5 mr-1' />
              {t('copyScript')}
            </Button>
          ) : null}
          {dockerCommand ? (
            <Button
              variant='secondary'
              size='sm'
              className='h-7 text-xs'
              onClick={() => void handleCopy(dockerCommand, t('copiedDocker'))}
            >
              <Copy className='size-3.5 mr-1' />
              {t('copyDocker')}
            </Button>
          ) : null}
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-4 md:grid-cols-2'>
          <div className='rounded-lg border px-3 py-3'>
            <p className='text-xs text-muted-foreground'>{t('nodeId')}</p>
            <p className='mt-1 text-sm break-all font-medium'>{node.node_id}</p>
          </div>
          <div className='rounded-lg border px-3 py-3'>
            <p className='text-xs text-muted-foreground'>{meta.tokenLabel}</p>
            <p className='mt-1 text-sm break-all font-medium'>
              {node.access_token || tn('na')}
            </p>
          </div>
        </div>

        <div className='space-y-2'>
          <Label htmlFor={`server-url-${variant}`}>{t('serverUrl')}</Label>
          <Input
            id={`server-url-${variant}`}
            value={serverUrl}
            onChange={(event) => setServerUrl(event.target.value)}
            placeholder='https://openflare.example.com'
          />
          <p className='text-xs text-muted-foreground'>{t('serverUrlHint')}</p>
        </div>

        {!node.access_token ? (
          <p className='text-sm text-muted-foreground'>{t('noToken')}</p>
        ) : !normalizedServerUrl ? (
          <p className='text-sm text-muted-foreground'>{t('needServerUrl')}</p>
        ) : (
          <>
            {scriptCommand && meta.scriptLabel ? (
              <div className='space-y-2'>
                <p className='text-sm font-medium'>{meta.scriptLabel}</p>
                <pre className='overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs whitespace-pre-wrap'>
                  {scriptCommand}
                </pre>
              </div>
            ) : null}
            <div className='space-y-2'>
              <p className='text-sm font-medium'>{meta.dockerLabel}</p>
              <pre className='overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs whitespace-pre-wrap'>
                {dockerCommand}
              </pre>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
