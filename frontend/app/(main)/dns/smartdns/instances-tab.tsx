'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Copy,
  KeyRound,
  Plus,
  RefreshCw,
  Server,
  ShieldAlert,
  ShieldCheck,
  Trash2,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
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
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSInstance,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { getErrorMessage } from '../../websites/components/website-utils';
import {
  buildSmartDNSDockerCommand,
  buildSmartDNSInstallCommand,
  type SmartDNSReleaseChannel,
} from './install-command';

export function SmartDNSInstancesTab() {
  const t = useTranslations('smartdns.instances');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [editOpen, setEditOpen] = useState(false);
  const [editing, setEditing] = useState<SmartDNSInstance | null>(null);
  const [name, setName] = useState('');
  const [publicAddr, setPublicAddr] = useState('');
  const [listenAddr, setListenAddr] = useState('0.0.0.0:53');
  const [geoName, setGeoName] = useState('');

  const [secretView, setSecretView] = useState<SmartDNSInstance | null>(null);
  const [deployChannel, setDeployChannel] =
    useState<SmartDNSReleaseChannel>('stable');
  const [quarantineTarget, setQuarantineTarget] =
    useState<SmartDNSInstance | null>(null);
  const [quarantineReason, setQuarantineReason] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<SmartDNSInstance | null>(
    null,
  );

  const discoveryQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'discovery-token'],
    queryFn: () => SmartDNSService.getDiscoveryToken(),
  });

  const instancesQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'instances'],
    queryFn: () => SmartDNSService.listInstances(),
    refetchInterval: 10000,
  });

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
  };

  const rotateDiscoveryMutation = useMutation({
    mutationFn: () => SmartDNSService.rotateDiscoveryToken(),
    onSuccess: async () => {
      toast.success(t('discoveryRotated'));
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = {
        name: name.trim(),
        public_addr: publicAddr.trim(),
        listen_addr: listenAddr.trim() || '0.0.0.0:53',
        geo_name: geoName.trim(),
      };
      if (editing) {
        return SmartDNSService.updateInstance(editing.id, payload);
      }
      return SmartDNSService.createInstance(payload);
    },
    onSuccess: async (inst) => {
      toast.success(editing ? t('updated') : t('created'));
      setEditOpen(false);
      if (!editing && inst.access_token) {
        setSecretView(inst);
      }
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const rotateTokenMutation = useMutation({
    mutationFn: (id: number) => SmartDNSService.rotateInstanceToken(id),
    onSuccess: async (inst) => {
      toast.success(t('tokenRotated'));
      setSecretView(inst);
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const quarantineMutation = useMutation({
    mutationFn: ({ id, reason }: { id: number; reason: string }) =>
      SmartDNSService.quarantineInstance(id, { reason }),
    onSuccess: async () => {
      toast.success(t('quarantinedToast'));
      setQuarantineTarget(null);
      setQuarantineReason('');
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const unquarantineMutation = useMutation({
    mutationFn: (id: number) => SmartDNSService.unquarantineInstance(id),
    onSuccess: async () => {
      toast.success(t('unquarantinedToast'));
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => SmartDNSService.deleteInstance(id),
    onSuccess: async () => {
      toast.success(t('deletedToast'));
      setDeleteTarget(null);
      await refresh();
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const openCreate = () => {
    setEditing(null);
    setName('');
    setPublicAddr('');
    setListenAddr('0.0.0.0:53');
    setGeoName('');
    setEditOpen(true);
  };

  const openEdit = (inst: SmartDNSInstance) => {
    setEditing(inst);
    setName(inst.name);
    setPublicAddr(inst.public_addr);
    setListenAddr(inst.listen_addr || '0.0.0.0:53');
    setGeoName(inst.geo_name);
    setEditOpen(true);
  };

  const copyText = async (value: string, successMessage: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(successMessage);
    } catch {
      toast.error(t('copyFailed'));
    }
  };

  const discoveryToken = discoveryQuery.data?.discovery_token ?? '';
  const originUrl =
    typeof window !== 'undefined'
      ? window.location.origin
      : 'https://controller.example.com';
  const discoveryScriptCommand = discoveryToken
    ? buildSmartDNSInstallCommand({
        controllerUrl: originUrl,
        token: discoveryToken,
        tokenType: 'discovery',
        channel: deployChannel,
      })
    : '';
  const discoveryDockerCommand = discoveryToken
    ? buildSmartDNSDockerCommand({
        controllerUrl: originUrl,
        token: discoveryToken,
        tokenType: 'discovery',
        channel: deployChannel,
      })
    : '';

  const instanceScriptCommand = secretView?.access_token
    ? buildSmartDNSInstallCommand({
        controllerUrl: originUrl,
        token: secretView.access_token,
        tokenType: 'instance',
        instanceId: secretView.instance_id,
        listenAddr: secretView.listen_addr,
        publicAddr: secretView.public_addr,
        channel: deployChannel,
      })
    : '';
  const instanceDockerCommand = secretView?.access_token
    ? buildSmartDNSDockerCommand({
        controllerUrl: originUrl,
        token: secretView.access_token,
        tokenType: 'instance',
        instanceId: secretView.instance_id,
        publicAddr: secretView.public_addr,
        channel: deployChannel,
      })
    : '';

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
          <div>
            <CardTitle className='text-base'>{t('discoveryTitle')}</CardTitle>
            <CardDescription>{t('discoveryDesc')}</CardDescription>
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <div className='flex items-center gap-1'>
              <Button
                variant={deployChannel === 'stable' ? 'default' : 'outline'}
                size='sm'
                onClick={() => setDeployChannel('stable')}
              >
                {t('channelStable')}
              </Button>
              <Button
                variant={deployChannel === 'canary' ? 'default' : 'outline'}
                size='sm'
                onClick={() => setDeployChannel('canary')}
              >
                {t('channelCanary')}
              </Button>
            </div>
            <Button
              variant='outline'
              size='sm'
              disabled={rotateDiscoveryMutation.isPending}
              onClick={() => rotateDiscoveryMutation.mutate()}
            >
              <RefreshCw data-icon='inline-start' />
              {t('rotateDiscovery')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-3'>
          <div className='flex flex-wrap items-center gap-2'>
            <Input
              readOnly
              className='max-w-md font-mono text-xs'
              value={discoveryToken}
            />
            <Button
              variant='outline'
              size='sm'
              disabled={!discoveryToken}
              onClick={() => copyText(discoveryToken, t('copied'))}
            >
              <Copy data-icon='inline-start' />
              {t('copyToken')}
            </Button>
          </div>
          {discoveryScriptCommand ? (
            <Tabs defaultValue='script' className='space-y-2'>
              <TabsList>
                <TabsTrigger value='script'>{t('deployScriptTab')}</TabsTrigger>
                <TabsTrigger value='docker'>{t('deployDockerTab')}</TabsTrigger>
              </TabsList>
              <TabsContent value='script'>
                <div className='flex flex-wrap items-start justify-between gap-2 rounded-lg border border-dashed bg-muted/40 p-3'>
                  <pre className='overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs'>
                    {discoveryScriptCommand}
                  </pre>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      copyText(discoveryScriptCommand, t('copied'))
                    }
                  >
                    <Copy data-icon='inline-start' />
                    {t('copyCommand')}
                  </Button>
                </div>
              </TabsContent>
              <TabsContent value='docker'>
                <div className='flex flex-wrap items-start justify-between gap-2 rounded-lg border border-dashed bg-muted/40 p-3'>
                  <pre className='overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs'>
                    {discoveryDockerCommand}
                  </pre>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      copyText(discoveryDockerCommand, t('copied'))
                    }
                  >
                    <Copy data-icon='inline-start' />
                    {t('copyCommand')}
                  </Button>
                </div>
              </TabsContent>
            </Tabs>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
          <div>
            <CardTitle className='text-base'>{t('listTitle')}</CardTitle>
            <CardDescription>{t('listDesc')}</CardDescription>
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => void instancesQuery.refetch()}
              disabled={instancesQuery.isFetching}
            >
              <RefreshCw
                data-icon='inline-start'
                className={instancesQuery.isFetching ? 'animate-spin' : ''}
              />
              {t('refresh')}
            </Button>
            <Button size='sm' onClick={openCreate}>
              <Plus data-icon='inline-start' />
              {t('create')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {instancesQuery.isLoading ? (
            <LoadingStateWithBorder icon={Server} description={t('loading')} />
          ) : instancesQuery.isError ? (
            <ErrorInline
              message={getErrorMessage(instancesQuery.error)}
              onRetry={() => void instancesQuery.refetch()}
            />
          ) : (instancesQuery.data ?? []).length === 0 ? (
            <p className='py-8 text-center text-sm text-muted-foreground'>
              {t('empty')}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('colName')}</TableHead>
                  <TableHead>{t('colMode')}</TableHead>
                  <TableHead>{t('colStatus')}</TableHead>
                  <TableHead>{t('colAddress')}</TableHead>
                  <TableHead>{t('colSnapshot')}</TableHead>
                  <TableHead>{t('colPerf')}</TableHead>
                  <TableHead className='text-right'>
                    {t('colActions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(instancesQuery.data ?? []).map((inst) => (
                  <TableRow key={inst.id}>
                    <TableCell>
                      <div className='font-medium'>{inst.name}</div>
                      <div className='font-mono text-xs text-muted-foreground'>
                        {inst.instance_id}
                        {inst.geo_name ? ` · ${inst.geo_name}` : ''}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>{inst.mode}</Badge>
                    </TableCell>
                    <TableCell>
                      <div className='flex flex-wrap gap-1'>
                        <Badge
                          variant={
                            inst.quarantined_at
                              ? 'destructive'
                              : inst.status === 'online'
                                ? 'default'
                                : 'secondary'
                          }
                        >
                          {inst.quarantined_at
                            ? t('statusQuarantined')
                            : inst.status === 'online'
                              ? t('statusOnline')
                              : t('statusOffline')}
                        </Badge>
                        {inst.connected ? (
                          <Badge variant='outline'>WS</Badge>
                        ) : null}
                      </div>
                      {inst.last_seen_at ? (
                        <div className='mt-0.5 text-xs text-muted-foreground'>
                          {formatDateTime(inst.last_seen_at)}
                        </div>
                      ) : null}
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      <div>{inst.public_addr || '-'}</div>
                      <div className='text-muted-foreground'>
                        {inst.listen_addr}
                      </div>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      <div>v{inst.snapshot_version}</div>
                      <div className='text-muted-foreground'>
                        {inst.version || '-'}
                      </div>
                    </TableCell>
                    <TableCell className='text-xs tabular-nums'>
                      <div>{inst.qps.toFixed(1)} QPS</div>
                      <div className='text-muted-foreground'>
                        P50 {inst.p50_ms.toFixed(1)} ms · Hit{' '}
                        {(inst.cache_hit_ratio * 100).toFixed(0)}%
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-1.5'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => openEdit(inst)}
                        >
                          {t('edit')}
                        </Button>
                        <Button
                          variant='outline'
                          size='icon-sm'
                          title={t('rotateToken')}
                          aria-label={t('rotateToken')}
                          onClick={() => rotateTokenMutation.mutate(inst.id)}
                        >
                          <KeyRound className='size-3.5' />
                        </Button>
                        {inst.quarantined_at ? (
                          <Button
                            variant='outline'
                            size='icon-sm'
                            title={t('unquarantine')}
                            aria-label={t('unquarantine')}
                            onClick={() => unquarantineMutation.mutate(inst.id)}
                          >
                            <ShieldCheck className='size-3.5' />
                          </Button>
                        ) : (
                          <Button
                            variant='outline'
                            size='icon-sm'
                            title={t('quarantine')}
                            aria-label={t('quarantine')}
                            onClick={() => {
                              setQuarantineTarget(inst);
                              setQuarantineReason('');
                            }}
                          >
                            <ShieldAlert className='size-3.5' />
                          </Button>
                        )}
                        <Button
                          variant='destructive'
                          size='icon-sm'
                          title={t('delete')}
                          aria-label={t('delete')}
                          onClick={() => setDeleteTarget(inst)}
                        >
                          <Trash2 className='size-3.5' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editing ? t('editTitle') : t('createTitle')}
            </DialogTitle>
            <DialogDescription>{t('dialogDesc')}</DialogDescription>
          </DialogHeader>
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
            <Button variant='outline' onClick={() => setEditOpen(false)}>
              {tc('cancel')}
            </Button>
            <Button
              disabled={saveMutation.isPending || !name.trim()}
              onClick={() => saveMutation.mutate()}
            >
              {t('save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={secretView !== null}
        onOpenChange={(open) => !open && setSecretView(null)}
      >
        <DialogContent className='sm:max-w-xl'>
          <DialogHeader>
            <DialogTitle>{t('credentialsTitle')}</DialogTitle>
            <DialogDescription>{t('credentialsDesc')}</DialogDescription>
          </DialogHeader>
          {secretView ? (
            <div className='space-y-3'>
              <div className='space-y-1.5'>
                <Label>{t('accessTokenLabel')}</Label>
                <div className='flex gap-2'>
                  <Input
                    readOnly
                    className='font-mono text-xs'
                    value={secretView.access_token ?? ''}
                  />
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      copyText(secretView.access_token ?? '', t('copied'))
                    }
                  >
                    <Copy />
                  </Button>
                </div>
              </div>
              {instanceScriptCommand || secretView.install_command ? (
                <div className='space-y-2'>
                  <div className='flex flex-wrap items-center justify-between gap-2'>
                    <Label>{t('installCommandLabel')}</Label>
                    <div className='flex items-center gap-1'>
                      <Button
                        variant={
                          deployChannel === 'stable' ? 'default' : 'outline'
                        }
                        size='sm'
                        onClick={() => setDeployChannel('stable')}
                      >
                        {t('channelStable')}
                      </Button>
                      <Button
                        variant={
                          deployChannel === 'canary' ? 'default' : 'outline'
                        }
                        size='sm'
                        onClick={() => setDeployChannel('canary')}
                      >
                        {t('channelCanary')}
                      </Button>
                    </div>
                  </div>
                  <Tabs defaultValue='script' className='space-y-2'>
                    <TabsList>
                      <TabsTrigger value='script'>
                        {t('deployScriptTab')}
                      </TabsTrigger>
                      <TabsTrigger value='docker'>
                        {t('deployDockerTab')}
                      </TabsTrigger>
                    </TabsList>
                    <TabsContent value='script'>
                      <div className='flex items-start justify-between gap-2 rounded-lg border border-dashed bg-muted/40 p-3'>
                        <pre className='overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs'>
                          {instanceScriptCommand || secretView.install_command}
                        </pre>
                        <Button
                          variant='outline'
                          size='icon-sm'
                          onClick={() =>
                            copyText(
                              instanceScriptCommand ||
                                secretView.install_command ||
                                '',
                              t('copied'),
                            )
                          }
                        >
                          <Copy />
                        </Button>
                      </div>
                    </TabsContent>
                    <TabsContent value='docker'>
                      <div className='flex items-start justify-between gap-2 rounded-lg border border-dashed bg-muted/40 p-3'>
                        <pre className='overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs'>
                          {instanceDockerCommand}
                        </pre>
                        <Button
                          variant='outline'
                          size='icon-sm'
                          onClick={() =>
                            copyText(instanceDockerCommand, t('copied'))
                          }
                        >
                          <Copy />
                        </Button>
                      </div>
                    </TabsContent>
                  </Tabs>
                </div>
              ) : null}
            </div>
          ) : null}
          <DialogFooter>
            <Button onClick={() => setSecretView(null)}>{tc('close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={quarantineTarget !== null}
        onOpenChange={(open) => !open && setQuarantineTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('quarantineTitle')}</DialogTitle>
            <DialogDescription>
              {t('quarantineDesc', { name: quarantineTarget?.name ?? '' })}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-1.5'>
            <Label htmlFor='sdns-quarantine-reason'>
              {t('quarantineReason')}
            </Label>
            <Input
              id='sdns-quarantine-reason'
              value={quarantineReason}
              placeholder={t('quarantineReasonPlaceholder')}
              onChange={(e) => setQuarantineReason(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setQuarantineTarget(null)}>
              {tc('cancel')}
            </Button>
            <Button
              variant='destructive'
              disabled={quarantineMutation.isPending}
              onClick={() =>
                quarantineTarget &&
                quarantineMutation.mutate({
                  id: quarantineTarget.id,
                  reason: quarantineReason.trim(),
                })
              }
            >
              {t('confirmQuarantine')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
