import { useMutation } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
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
import { Field, FieldLabel } from '@/components/ui/field';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import {
  SmartDNSService,
  type SmartDNSInstance,
  type SmartDNSUpdateChannel,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

export function canUpdateInstance(instance: SmartDNSInstance) {
  return (
    instance.mode === 'standalone' &&
    instance.status === 'online' &&
    !instance.quarantined_at &&
    instance.remote_update_supported === true &&
    instance.update_status !== 'queued' &&
    instance.update_status !== 'running'
  );
}

export function SmartDNSUpdateDialog({
  instances,
  onClose,
  onQueued,
}: {
  instances: SmartDNSInstance[];
  onClose: () => void;
  onQueued: () => void;
}) {
  const t = useTranslations('smartdns.instances');
  const common = useTranslations('common');
  const [channel, setChannel] = useState<SmartDNSUpdateChannel>('stable');
  const mutation = useMutation({
    mutationFn: () =>
      SmartDNSService.requestInstanceUpdates(
        instances.map((instance) => instance.id),
        channel,
      ),
    onSuccess: ({ queued }) => {
      toast.success(t('updateQueued', { count: queued }));
      onQueued();
      onClose();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  return (
    <Dialog
      open
      onOpenChange={(open) => !open && !mutation.isPending && onClose()}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('remoteUpdate')}</DialogTitle>
          <DialogDescription>
            {t('updateDescription', { count: instances.length })}
          </DialogDescription>
        </DialogHeader>
        <Field>
          <FieldLabel htmlFor='smartdns-update-channel'>
            {t('updateChannel')}
          </FieldLabel>
          <Select
            value={channel}
            onValueChange={(value) =>
              setChannel(value === 'canary' ? 'canary' : 'stable')
            }
            disabled={mutation.isPending}
          >
            <SelectTrigger id='smartdns-update-channel' className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='stable'>{t('updateStable')}</SelectItem>
              <SelectItem value='canary'>{t('updateCanary')}</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={mutation.isPending}
            onClick={onClose}
          >
            {common('cancel')}
          </Button>
          <Button
            disabled={mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending && <Spinner />}
            {t('remoteUpdate')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
