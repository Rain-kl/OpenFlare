import { useMutation, useQueryClient } from '@tanstack/react-query';
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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSInstance,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

import { SmartDNSDiscoveryEnrollment } from './discovery-enrollment';

export function SmartDNSInstanceForm({
  instance,
  onClose,
  onCreated,
}: {
  instance?: SmartDNSInstance;
  onClose: () => void;
  onCreated: (instance: SmartDNSInstance) => void;
}) {
  const t = useTranslations('smartdns.instances');
  const common = useTranslations('common');
  const queryClient = useQueryClient();
  const [name, setName] = useState(instance?.name ?? '');
  const [publicAddr, setPublicAddr] = useState(instance?.public_addr ?? '');
  const [listenAddr, setListenAddr] = useState(
    instance?.listen_addr || '0.0.0.0:53',
  );
  const [geoName, setGeoName] = useState(instance?.geo_name ?? '');
  const [tab, setTab] = useState('pre-register');

  const mutation = useMutation({
    mutationFn: () => {
      const payload = {
        name: name.trim(),
        public_addr: publicAddr.trim(),
        listen_addr: listenAddr.trim() || '0.0.0.0:53',
        geo_name: geoName.trim(),
      };
      return instance
        ? SmartDNSService.updateInstance(instance.id, payload)
        : SmartDNSService.createInstance(payload);
    },
    onSuccess: async (result) => {
      toast.success(t(instance ? 'updated' : 'created'));
      onClose();
      if (!instance && result.access_token) onCreated(result);
      await queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const form = (
    <form
      className='flex flex-col gap-4'
      onSubmit={(event) => {
        event.preventDefault();
        if (!mutation.isPending && name.trim()) mutation.mutate();
      }}
    >
      <div className='space-y-3'>
        <div className='space-y-1.5'>
          <Label htmlFor='sdns-inst-name'>{t('nameLabel')}</Label>
          <Input
            id='sdns-inst-name'
            value={name}
            placeholder='hk-dns-01'
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='sdns-inst-pub'>{t('publicAddrLabel')}</Label>
          <Input
            id='sdns-inst-pub'
            value={publicAddr}
            placeholder='203.0.113.53'
            onChange={(e) => setPublicAddr(e.target.value)}
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='sdns-inst-listen'>{t('listenAddrLabel')}</Label>
          <Input
            id='sdns-inst-listen'
            value={listenAddr}
            placeholder='0.0.0.0:53'
            onChange={(e) => setListenAddr(e.target.value)}
          />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor='sdns-inst-geo'>{t('geoNameLabel')}</Label>
          <Input
            id='sdns-inst-geo'
            value={geoName}
            placeholder='Hong Kong'
            onChange={(e) => setGeoName(e.target.value)}
          />
        </div>
      </div>
      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          disabled={mutation.isPending}
          onClick={onClose}
        >
          {common('cancel')}
        </Button>
        <Button type='submit' disabled={mutation.isPending || !name.trim()}>
          {t('save')}
        </Button>
      </DialogFooter>
    </form>
  );

  return (
    <Dialog
      open
      onOpenChange={(open) => !open && !mutation.isPending && onClose()}
    >
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t(instance ? 'editTitle' : 'createTitle')}</DialogTitle>
          <DialogDescription>{t('dialogDesc')}</DialogDescription>
        </DialogHeader>
        {instance ? (
          form
        ) : (
          <Tabs value={tab} onValueChange={setTab}>
            <TabsList>
              <TabsTrigger value='pre-register'>{t('preRegister')}</TabsTrigger>
              <TabsTrigger value='discovery' disabled={mutation.isPending}>
                {t('discoveryTitle')}
              </TabsTrigger>
            </TabsList>
            <TabsContent value='pre-register' className='pt-3'>
              {form}
            </TabsContent>
            <TabsContent value='discovery' className='pt-3'>
              <SmartDNSDiscoveryEnrollment />
            </TabsContent>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  );
}
