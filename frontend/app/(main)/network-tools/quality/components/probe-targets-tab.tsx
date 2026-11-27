'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

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
import { Card, CardContent } from '@/components/ui/card';
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type {
  NetToolsProbeTarget,
  NetToolsProbeTargetInput,
} from '@/lib/services/openflare';
import { NetworkToolsService } from '@/lib/services/openflare';

import {
  formatNetToolsMs,
  formatNetToolsTime,
  getNetToolsErrorMessage,
} from '../../shared/utils';

interface ProbeTargetsTabProps {
  refreshNonce: number;
}

interface TargetFormState {
  name: string;
  ip: string;
  probe_proto: 'icmp' | 'tcp' | 'traceroute';
  port: string;
  asn: string;
  country: string;
  region: string;
  target_type: 'public_anchor' | 'customer_isp' | 'manual';
  anycast_status: 'unknown' | 'unicast' | 'confirmed';
  enabled: boolean;
  notes: string;
}

const EMPTY_FORM: TargetFormState = {
  name: '',
  ip: '',
  probe_proto: 'icmp',
  port: '0',
  asn: '',
  country: '',
  region: '',
  target_type: 'customer_isp',
  anycast_status: 'unknown',
  enabled: true,
  notes: '',
};

function toFormState(target?: NetToolsProbeTarget | null): TargetFormState {
  if (!target) return EMPTY_FORM;
  const proto =
    target.probe_proto === 'tcp' || target.probe_proto === 'traceroute'
      ? target.probe_proto
      : 'icmp';
  const targetType =
    target.target_type === 'public_anchor' || target.target_type === 'manual'
      ? target.target_type
      : 'customer_isp';
  const anycast =
    target.anycast_status === 'confirmed' || target.anycast_status === 'unicast'
      ? target.anycast_status
      : 'unknown';
  return {
    name: target.name || '',
    ip: target.ip || '',
    probe_proto: proto,
    port: String(target.port ?? 0),
    asn: target.asn > 0 ? String(target.asn) : '',
    country: target.country || '',
    region: target.region || '',
    target_type: targetType,
    anycast_status: anycast,
    enabled: target.enabled,
    notes: target.notes || '',
  };
}

export function ProbeTargetsTab({ refreshNonce }: ProbeTargetsTabProps) {
  const t = useTranslations('networkTools');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [search, setSearch] = useState('');
  const [targetTypeFilter, setTargetTypeFilter] = useState('all');
  const [ipFamilyFilter, setIpFamilyFilter] = useState('all');
  const [page, setPage] = useState(1);

  const [editorOpen, setEditorOpen] = useState(false);
  const [editingTarget, setEditingTarget] =
    useState<NetToolsProbeTarget | null>(null);
  const [form, setForm] = useState<TargetFormState>(EMPTY_FORM);
  const [deletingTarget, setDeletingTarget] =
    useState<NetToolsProbeTarget | null>(null);

  const targetsQuery = useQuery({
    queryKey: [
      'openflare',
      'network-tools',
      'probe-targets',
      refreshNonce,
      page,
      search,
      targetTypeFilter,
      ipFamilyFilter,
    ],
    queryFn: () =>
      NetworkToolsService.listProbeTargets({
        p: page,
        page_size: 20,
        search: search.trim() || undefined,
        target_type: targetTypeFilter === 'all' ? undefined : targetTypeFilter,
        ip_family: ipFamilyFilter === 'all' ? undefined : ipFamilyFilter,
      }),
  });

  const invalidateTargets = async () => {
    await queryClient.invalidateQueries({
      queryKey: ['openflare', 'network-tools', 'probe-targets'],
    });
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      const parsedPort = Number(form.port.trim() || '0');
      const parsedAsn = Number(form.asn.trim() || '0');
      const port = form.probe_proto === 'icmp' ? 0 : Math.max(0, parsedPort);
      const asn = Number.isInteger(parsedAsn) && parsedAsn > 0 ? parsedAsn : 0;

      if (editingTarget) {
        return NetworkToolsService.updateProbeTarget(editingTarget.id, {
          name: form.name.trim(),
          probe_proto: form.probe_proto,
          port,
          asn,
          country: form.country.trim().toUpperCase(),
          region: form.region.trim().toUpperCase(),
          target_type: form.target_type,
          anycast_status: form.anycast_status,
          enabled: form.enabled,
          notes: form.notes.trim(),
        });
      }

      const payload: NetToolsProbeTargetInput = {
        name: form.name.trim() || undefined,
        ip: form.ip.trim(),
        probe_proto: form.probe_proto,
        port,
        asn: asn || undefined,
        country: form.country.trim().toUpperCase() || undefined,
        region: form.region.trim().toUpperCase() || undefined,
        target_type: form.target_type,
        anycast_status: form.anycast_status,
        enabled: form.enabled,
        notes: form.notes.trim() || undefined,
      };
      return NetworkToolsService.createProbeTarget(payload);
    },
    onSuccess: async () => {
      toast.success(
        editingTarget
          ? t('quality.targets.updateSuccess')
          : t('quality.targets.createSuccess'),
      );
      setEditorOpen(false);
      setEditingTarget(null);
      await invalidateTargets();
    },
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  const toggleMutation = useMutation({
    mutationFn: (item: { id: number; enabled: boolean }) =>
      NetworkToolsService.updateProbeTarget(item.id, { enabled: item.enabled }),
    onSuccess: async () => {
      await invalidateTargets();
    },
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => NetworkToolsService.deleteProbeTarget(id),
    onSuccess: async () => {
      toast.success(t('quality.targets.deleteSuccess'));
      setDeletingTarget(null);
      await invalidateTargets();
    },
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  const openCreate = () => {
    setEditingTarget(null);
    setForm(EMPTY_FORM);
    setEditorOpen(true);
  };

  const openEdit = (target: NetToolsProbeTarget) => {
    setEditingTarget(target);
    setForm(toFormState(target));
    setEditorOpen(true);
  };

  const totalPages = Math.max(
    1,
    Math.ceil((targetsQuery.data?.total ?? 0) / 20),
  );

  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex flex-wrap items-center gap-2'>
          <Input
            className='w-64'
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
            placeholder={t('quality.targets.searchPlaceholder')}
          />
          <Select
            value={targetTypeFilter}
            onValueChange={(v) => {
              setTargetTypeFilter(v);
              setPage(1);
            }}
          >
            <SelectTrigger className='w-44'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>
                {t('quality.targets.filterTypeAll')}
              </SelectItem>
              <SelectItem value='public_anchor'>
                {t('quality.targets.types.public_anchor')}
              </SelectItem>
              <SelectItem value='customer_isp'>
                {t('quality.targets.types.customer_isp')}
              </SelectItem>
              <SelectItem value='manual'>
                {t('quality.targets.types.manual')}
              </SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={ipFamilyFilter}
            onValueChange={(v) => {
              setIpFamilyFilter(v);
              setPage(1);
            }}
          >
            <SelectTrigger className='w-32'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>
                {t('quality.filters.ipFamilyAll')}
              </SelectItem>
              <SelectItem value='ipv4'>IPv4</SelectItem>
              <SelectItem value='ipv6'>IPv6</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button type='button' size='sm' onClick={openCreate}>
          <Plus className='mr-1.5 size-3.5' />
          {t('quality.targets.addButton')}
        </Button>
      </div>

      <Card>
        <CardContent className='pt-6'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('quality.targets.columns.target')}</TableHead>
                <TableHead>{t('quality.targets.columns.endpoint')}</TableHead>
                <TableHead>{t('quality.targets.columns.networkGeo')}</TableHead>
                <TableHead>
                  {t('quality.targets.columns.classification')}
                </TableHead>
                <TableHead>{t('quality.targets.columns.nodeStates')}</TableHead>
                <TableHead>{t('quality.targets.columns.enabled')}</TableHead>
                <TableHead>{t('quality.targets.columns.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(targetsQuery.data?.results ?? []).map((target) => (
                <TableRow key={target.id}>
                  <TableCell className='text-xs'>
                    <div className='font-medium'>
                      {target.name || target.ip}
                    </div>
                    {target.notes ? (
                      <div className='max-w-xs truncate text-muted-foreground'>
                        {target.notes}
                      </div>
                    ) : null}
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    <div>{target.ip}</div>
                    <div className='text-muted-foreground'>
                      {target.ip_family.toUpperCase()} ·{' '}
                      {target.probe_proto.toUpperCase()}
                      {target.port > 0 ? `:${target.port}` : ''}
                    </div>
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    <div>{target.asn > 0 ? `AS${target.asn}` : '—'}</div>
                    <div className='text-muted-foreground'>
                      {[target.country, target.region]
                        .filter(Boolean)
                        .join(' / ') || '—'}
                    </div>
                  </TableCell>
                  <TableCell className='text-xs'>
                    <div className='flex flex-wrap items-center gap-1'>
                      <Badge variant='secondary'>
                        {t(
                          `quality.targets.types.${target.target_type}` as Parameters<
                            typeof t
                          >[0],
                        )}
                      </Badge>
                      <Badge
                        variant={
                          target.anycast_status === 'confirmed'
                            ? 'destructive'
                            : 'outline'
                        }
                      >
                        {t(
                          `quality.targets.anycast.${target.anycast_status || 'unknown'}` as Parameters<
                            typeof t
                          >[0],
                        )}
                      </Badge>
                    </div>
                  </TableCell>
                  <TableCell className='text-xs'>
                    {target.states.length === 0 ? (
                      <span className='text-muted-foreground'>
                        {t('quality.targets.noStates')}
                      </span>
                    ) : (
                      <div className='flex flex-col gap-1'>
                        {target.states.map((st) => (
                          <div
                            key={`${st.node_id}-${st.target_id}`}
                            className='flex flex-wrap items-center gap-1.5 rounded border border-dashed px-2 py-1'
                          >
                            <span className='font-medium'>
                              {st.node_name || st.node_id}
                            </span>
                            {st.consecutive_failures > 0 ? (
                              <Badge
                                variant='destructive'
                                className='text-[10px]'
                              >
                                {t('quality.targets.stateFailures', {
                                  count: st.consecutive_failures,
                                })}
                              </Badge>
                            ) : (
                              <Badge variant='outline' className='text-[10px]'>
                                {t('quality.targets.stateHealthy', {
                                  rtt: formatNetToolsMs(st.last_rtt_ms),
                                })}
                              </Badge>
                            )}
                            {st.next_probe_at ? (
                              <span className='text-[11px] text-muted-foreground'>
                                {t('quality.targets.stateBackoffUntil', {
                                  time: formatNetToolsTime(st.next_probe_at),
                                })}
                              </span>
                            ) : null}
                            {st.last_error ? (
                              <span
                                className='max-w-44 truncate text-[11px] text-destructive'
                                title={st.last_error}
                              >
                                {st.last_error}
                              </span>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    )}
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={target.enabled}
                      disabled={toggleMutation.isPending}
                      onCheckedChange={(checked) =>
                        toggleMutation.mutate({
                          id: target.id,
                          enabled: checked,
                        })
                      }
                    />
                  </TableCell>
                  <TableCell>
                    <div className='flex items-center gap-1'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        className='size-7'
                        onClick={() => openEdit(target)}
                      >
                        <Pencil className='size-3.5' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        className='size-7 text-destructive hover:text-destructive'
                        onClick={() => setDeletingTarget(target)}
                      >
                        <Trash2 className='size-3.5' />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {!targetsQuery.isLoading &&
              (targetsQuery.data?.results?.length ?? 0) === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className='py-8 text-center text-muted-foreground'
                  >
                    {t('common.noData')}
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>

          <div className='mt-4 flex items-center justify-between text-xs text-muted-foreground'>
            <span>
              {t('common.pageSummary', {
                page,
                totalPages,
                total: targetsQuery.data?.total ?? 0,
              })}
            </span>
            <div className='flex gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                {t('common.prevPage')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                {t('common.nextPage')}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {editingTarget
                ? t('quality.targets.editTitle')
                : t('quality.targets.createTitle')}
            </DialogTitle>
            <DialogDescription>
              {t('quality.targets.formDesc')}
            </DialogDescription>
          </DialogHeader>
          <form
            className='space-y-3'
            onSubmit={(e) => {
              e.preventDefault();
              saveMutation.mutate();
            }}
          >
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <Label>{t('quality.targets.form.name')}</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder={t('quality.targets.form.namePlaceholder')}
                />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('quality.targets.form.ip')}</Label>
                <Input
                  value={form.ip}
                  disabled={Boolean(editingTarget)}
                  onChange={(e) => setForm({ ...form, ip: e.target.value })}
                  placeholder={t('quality.targets.form.ipPlaceholder')}
                />
                <p className='text-[11px] text-muted-foreground'>
                  {t('quality.targets.form.ssrfHint')}
                </p>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('quality.targets.form.proto')}</Label>
                <Select
                  value={form.probe_proto}
                  onValueChange={(v) =>
                    setForm({
                      ...form,
                      probe_proto: v as 'icmp' | 'tcp' | 'traceroute',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='icmp'>ICMP</SelectItem>
                    <SelectItem value='tcp'>TCP</SelectItem>
                    <SelectItem value='traceroute'>Traceroute</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('quality.targets.form.port')}</Label>
                <Input
                  type='number'
                  min={0}
                  max={65535}
                  disabled={form.probe_proto === 'icmp'}
                  value={form.probe_proto === 'icmp' ? '0' : form.port}
                  onChange={(e) => setForm({ ...form, port: e.target.value })}
                />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('quality.targets.form.targetType')}</Label>
                <Select
                  value={form.target_type}
                  onValueChange={(v) =>
                    setForm({
                      ...form,
                      target_type: v as
                        | 'public_anchor'
                        | 'customer_isp'
                        | 'manual',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='public_anchor'>
                      {t('quality.targets.types.public_anchor')}
                    </SelectItem>
                    <SelectItem value='customer_isp'>
                      {t('quality.targets.types.customer_isp')}
                    </SelectItem>
                    <SelectItem value='manual'>
                      {t('quality.targets.types.manual')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('quality.targets.form.anycastStatus')}</Label>
                <Select
                  value={form.anycast_status}
                  onValueChange={(v) =>
                    setForm({
                      ...form,
                      anycast_status: v as 'unknown' | 'unicast' | 'confirmed',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='unknown'>
                      {t('quality.targets.anycast.unknown')}
                    </SelectItem>
                    <SelectItem value='unicast'>
                      {t('quality.targets.anycast.unicast')}
                    </SelectItem>
                    <SelectItem value='confirmed'>
                      {t('quality.targets.anycast.confirmed')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('quality.targets.form.asn')}</Label>
                <Input
                  value={form.asn}
                  onChange={(e) => setForm({ ...form, asn: e.target.value })}
                  placeholder='4134'
                />
              </div>
              <div className='grid grid-cols-2 gap-2'>
                <div className='space-y-1.5'>
                  <Label>{t('quality.targets.form.country')}</Label>
                  <Input
                    value={form.country}
                    onChange={(e) =>
                      setForm({ ...form, country: e.target.value })
                    }
                    placeholder='CN'
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label>{t('quality.targets.form.region')}</Label>
                  <Input
                    value={form.region}
                    onChange={(e) =>
                      setForm({ ...form, region: e.target.value })
                    }
                    placeholder='CN-GD'
                  />
                </div>
              </div>
            </div>
            <div className='space-y-1.5'>
              <Label>{t('quality.targets.form.notes')}</Label>
              <Input
                value={form.notes}
                onChange={(e) => setForm({ ...form, notes: e.target.value })}
              />
            </div>
            <div className='flex items-center justify-between rounded-md border border-dashed px-3 py-2'>
              <span className='text-sm font-medium'>
                {t('quality.targets.form.enabled')}
              </span>
              <Switch
                checked={form.enabled}
                onCheckedChange={(checked) =>
                  setForm({ ...form, enabled: checked })
                }
              />
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => setEditorOpen(false)}
                disabled={saveMutation.isPending}
              >
                {tc('cancel')}
              </Button>
              <Button type='submit' disabled={saveMutation.isPending}>
                {tc('confirm')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deletingTarget)}
        onOpenChange={(open) => {
          if (!open) setDeletingTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('quality.targets.deleteConfirmTitle')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t('quality.targets.deleteConfirmDesc', {
                target: deletingTarget?.name || deletingTarget?.ip || '',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deletingTarget) {
                  deleteMutation.mutate(deletingTarget.id);
                }
              }}
              disabled={deleteMutation.isPending}
            >
              {tc('confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
