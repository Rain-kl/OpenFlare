'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
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
import { SmartDNSEnrollmentSheet } from './enrollment-sheet';
import { DnsActionMenu } from '../components/dns-action-menu';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
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
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSInstance,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { getErrorMessage } from '../../websites/components/website-utils';
import { SmartDNSDeploymentCommands } from './deployment-commands';

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

  const [enrollmentOpen, setEnrollmentOpen] = useState(false);
  const [rotateTarget, setRotateTarget] = useState<SmartDNSInstance | null>(
    null,
  );
  const [secretView, setSecretView] = useState<SmartDNSInstance | null>(null);
  const [quarantineTarget, setQuarantineTarget] =
    useState<SmartDNSInstance | null>(null);
  const [quarantineReason, setQuarantineReason] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<SmartDNSInstance | null>(
    null,
  );

  const instancesQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'instances'],
    queryFn: () => SmartDNSService.listInstances(),
    refetchInterval: 10000,
  });

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
  };

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
      setRotateTarget(null);
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

  return (
    <div className='flex flex-col gap-5'>
      <Card>
        <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
          <div>
            <CardTitle className='text-base'>{t('listTitle')}</CardTitle>
            <CardDescription>{t('listDesc')}</CardDescription>
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setEnrollmentOpen(true)}
            >
              <KeyRound data-icon='inline-start' />
              {t('openEnrollment')}
            </Button>
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
            <div className='flex flex-col items-start gap-2 rounded-lg border border-dashed p-4'>
              <p className='text-sm text-muted-foreground'>{t('empty')}</p>
            </div>
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
                      <div className='flex justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => openEdit(inst)}
                        >
                          {t('edit')}
                        </Button>
                        <DnsActionMenu name={inst.name}>
                          <DropdownMenuItem
                            onSelect={() => setRotateTarget(inst)}
                          >
                            <KeyRound />
                            {t('rotateToken')}
                          </DropdownMenuItem>
                          {inst.quarantined_at ? (
                            <DropdownMenuItem
                              disabled={unquarantineMutation.isPending}
                              onSelect={() =>
                                unquarantineMutation.mutate(inst.id)
                              }
                            >
                              <ShieldCheck />
                              {t('unquarantine')}
                            </DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem
                              onSelect={() => {
                                setQuarantineTarget(inst);
                                setQuarantineReason('');
                              }}
                            >
                              <ShieldAlert />
                              {t('quarantine')}
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuItem
                            variant='destructive'
                            onSelect={() => setDeleteTarget(inst)}
                          >
                            <Trash2 />
                            {t('delete')}
                          </DropdownMenuItem>
                        </DnsActionMenu>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <SmartDNSEnrollmentSheet
        open={enrollmentOpen}
        onOpenChange={setEnrollmentOpen}
      />
      <AlertDialog
        open={rotateTarget !== null}
        onOpenChange={(open) =>
          !open && !rotateTokenMutation.isPending && setRotateTarget(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('rotateToken')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('rotateAccessDescription', { name: rotateTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={rotateTokenMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              disabled={rotateTokenMutation.isPending}
              onClick={() =>
                rotateTarget && rotateTokenMutation.mutate(rotateTarget.id)
              }
            >
              {t('rotateToken')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

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
        <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl'>
          <DialogHeader>
            <DialogTitle>{t('credentialsTitle')}</DialogTitle>
            <DialogDescription>{t('credentialsDesc')}</DialogDescription>
          </DialogHeader>
          {secretView?.access_token && (
            <SmartDNSDeploymentCommands
              tokenLabel={t('accessTokenLabel')}
              token={secretView.access_token}
              tokenType='instance'
              instanceId={secretView.instance_id}
              listenAddr={secretView.listen_addr}
              publicAddr={secretView.public_addr}
            />
          )}
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
