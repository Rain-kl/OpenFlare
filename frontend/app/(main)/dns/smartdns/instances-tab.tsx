'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Download,
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
import { SmartDNSInstanceForm } from './instance-form';
import { SmartDNSUpdateDialog, canUpdateInstance } from './update-dialog';
import { Checkbox } from '@/components/ui/checkbox';
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

  const [formTarget, setFormTarget] = useState<SmartDNSInstance | 'create'>();
  const [selected, setSelected] = useState<number[]>([]);
  const [updateTargets, setUpdateTargets] = useState<SmartDNSInstance[]>([]);

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

  const instances = instancesQuery.data ?? [];
  const eligible = instances.filter(canUpdateInstance);
  const selectedInstances = eligible.filter((instance) =>
    selected.includes(instance.id),
  );
  const allSelected =
    eligible.length > 0 && selectedInstances.length === eligible.length;

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

  return (
    <div className='flex flex-col gap-5'>
      <Card>
        <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
          <div>
            <CardTitle className='text-base'>{t('listTitle')}</CardTitle>
            <CardDescription>{t('listDesc')}</CardDescription>
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            {selectedInstances.length > 0 && (
              <Button
                variant='outline'
                size='sm'
                onClick={() => setUpdateTargets(selectedInstances)}
              >
                <Download data-icon='inline-start' />
                {t('batchUpdate', { count: selectedInstances.length })}
              </Button>
            )}
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
            <Button size='sm' onClick={() => setFormTarget('create')}>
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
          ) : instances.length === 0 ? (
            <div className='flex flex-col items-start gap-2 rounded-lg border border-dashed p-4'>
              <p className='text-sm text-muted-foreground'>{t('empty')}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-10'>
                    <Checkbox
                      aria-label={t('selectAll')}
                      disabled={eligible.length === 0}
                      checked={
                        allSelected
                          ? true
                          : selectedInstances.length > 0
                            ? 'indeterminate'
                            : false
                      }
                      onCheckedChange={(checked) =>
                        setSelected(
                          checked === true
                            ? eligible.map((instance) => instance.id)
                            : [],
                        )
                      }
                    />
                  </TableHead>
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
                {instances.map((inst) => (
                  <TableRow key={inst.id}>
                    <TableCell>
                      <Checkbox
                        aria-label={t('selectInstance', { name: inst.name })}
                        disabled={!canUpdateInstance(inst)}
                        checked={
                          canUpdateInstance(inst) && selected.includes(inst.id)
                        }
                        onCheckedChange={(checked) =>
                          setSelected((previous) =>
                            checked === true
                              ? [...previous, inst.id]
                              : previous.filter((id) => id !== inst.id),
                          )
                        }
                      />
                    </TableCell>
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
                      {inst.update_status && (
                        <div className='mt-1 font-sans text-muted-foreground'>
                          {t(`updateStatus.${inst.update_status}`)}
                        </div>
                      )}
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
                          onClick={() => setFormTarget(inst)}
                        >
                          {t('edit')}
                        </Button>
                        <DnsActionMenu name={inst.name}>
                          {inst.mode === 'standalone' && (
                            <DropdownMenuItem
                              disabled={!canUpdateInstance(inst)}
                              onSelect={() => setUpdateTargets([inst])}
                            >
                              <Download />
                              {t(
                                inst.remote_update_supported
                                  ? 'remoteUpdate'
                                  : 'updateUnavailable',
                              )}
                            </DropdownMenuItem>
                          )}
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

      {formTarget && (
        <SmartDNSInstanceForm
          instance={formTarget === 'create' ? undefined : formTarget}
          onClose={() => setFormTarget(undefined)}
          onCreated={setSecretView}
        />
      )}
      {updateTargets.length > 0 && (
        <SmartDNSUpdateDialog
          instances={updateTargets}
          onClose={() => setUpdateTargets([])}
          onQueued={() => {
            setSelected([]);
            void refresh();
          }}
        />
      )}
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
