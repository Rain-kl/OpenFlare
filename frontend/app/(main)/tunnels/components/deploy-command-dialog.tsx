'use client';

import { useState } from 'react';
import { Copy, Eye, EyeOff } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
  buildConnectorDockerCommand,
  buildConnectorInstallScriptCommand,
  buildRelayDockerCommand,
  buildRelayInstallScriptCommand,
} from './tunnel-utils';

export function DeployCommandDialog({
  open,
  mode,
  title,
  accessToken,
  bootstrapToken,
  serverUrl,
  deployCommand,
  onClose,
}: {
  open: boolean;
  mode: 'relay' | 'tunnel';
  title: string;
  accessToken: string;
  bootstrapToken: string;
  serverUrl: string;
  deployCommand: string;
  onClose: () => void;
}) {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const [showSecret, setShowSecret] = useState(false);

  const resolvedServerUrl =
    serverUrl || (typeof window !== 'undefined' ? window.location.origin : '');
  const effectiveToken = bootstrapToken || accessToken;
  const scriptCmd =
    mode === 'relay'
      ? buildRelayInstallScriptCommand(resolvedServerUrl, effectiveToken)
      : buildConnectorInstallScriptCommand(resolvedServerUrl, effectiveToken);
  const dockerCmd =
    mode === 'relay'
      ? buildRelayDockerCommand(resolvedServerUrl, effectiveToken)
      : buildConnectorDockerCommand(resolvedServerUrl, effectiveToken);

  const handleCopy = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(t('deploy.copied', { target: label }));
    } catch {
      toast.error(t('deploy.copyFailed'));
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{t('deploy.description')}</DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='rounded-lg border border-dashed p-3 space-y-2'>
            <div className='flex items-center justify-between gap-2'>
              <p className='text-xs font-medium text-muted-foreground'>
                {t('deploy.bootstrapToken')}
              </p>
              <div className='flex items-center gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='h-7 px-2 text-xs'
                  onClick={() => setShowSecret((prev) => !prev)}
                >
                  {showSecret ? (
                    <EyeOff className='size-3.5 mr-1' />
                  ) : (
                    <Eye className='size-3.5 mr-1' />
                  )}
                  {showSecret ? t('deploy.hide') : t('deploy.reveal')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='h-7 px-2 text-xs'
                  onClick={() =>
                    void handleCopy(effectiveToken, t('deploy.bootstrapToken'))
                  }
                >
                  <Copy className='size-3.5 mr-1' />
                  {t('deploy.copy')}
                </Button>
              </div>
            </div>
            <pre
              className={`overflow-x-auto text-xs font-mono break-all whitespace-pre-wrap transition-[filter] ${showSecret ? '' : 'blur-sm select-none'}`}
            >
              {effectiveToken}
            </pre>
          </div>

          <div className='space-y-1.5'>
            <div className='flex items-center justify-between'>
              <p className='text-xs font-medium'>{t('deploy.cliCommand')}</p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-7 px-2 text-xs'
                onClick={() =>
                  void handleCopy(deployCommand, t('deploy.cliCommand'))
                }
              >
                <Copy className='size-3.5 mr-1' />
                {t('deploy.copy')}
              </Button>
            </div>
            <pre
              className={`overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs font-mono whitespace-pre-wrap break-all transition-[filter] ${showSecret ? '' : 'blur-sm select-none'}`}
            >
              {deployCommand}
            </pre>
          </div>

          <div className='space-y-1.5'>
            <div className='flex items-center justify-between'>
              <p className='text-xs font-medium'>{t('deploy.scriptCommand')}</p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-7 px-2 text-xs'
                onClick={() =>
                  void handleCopy(scriptCmd, t('deploy.scriptCommand'))
                }
              >
                <Copy className='size-3.5 mr-1' />
                {t('deploy.copy')}
              </Button>
            </div>
            <pre
              className={`overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs font-mono whitespace-pre-wrap break-all transition-[filter] ${showSecret ? '' : 'blur-sm select-none'}`}
            >
              {scriptCmd}
            </pre>
          </div>

          <div className='space-y-1.5'>
            <div className='flex items-center justify-between'>
              <p className='text-xs font-medium'>{t('deploy.dockerCommand')}</p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-7 px-2 text-xs'
                onClick={() =>
                  void handleCopy(dockerCmd, t('deploy.dockerCommand'))
                }
              >
                <Copy className='size-3.5 mr-1' />
                {t('deploy.copy')}
              </Button>
            </div>
            <pre
              className={`overflow-x-auto rounded-lg border bg-muted/40 p-3 text-xs font-mono whitespace-pre-wrap break-all transition-[filter] ${showSecret ? '' : 'blur-sm select-none'}`}
            >
              {dockerCmd}
            </pre>
          </div>
        </div>

        <DialogFooter>
          <Button type='button' variant='outline' onClick={onClose}>
            {tc('close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
