'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  Cloud,
  Loader2,
  Plus,
  RefreshCw,
  Settings,
  Trash2,
} from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { useParams } from 'next/navigation';
import { useState } from 'react';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  CloudflareService,
  cloudflareQueryKey,
  NodeService,
  type CloudflareGroupPayload,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../../websites/components/website-utils';
import { GroupDialog } from '../../components/group-dialog';
import { MemberAddDialog } from '../../components/member-add-dialog';

export function CloudflareGroupDetailPageClient() {
  const t = useTranslations('cloudflare');
  const params = useParams<{ id: string }>();
  const groupID = Number(params.id);
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const detailQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'groups', groupID],
    queryFn: () => CloudflareService.getGroup(groupID),
    enabled: Number.isInteger(groupID) && groupID > 0,
    refetchInterval: 5000,
  });
  const domainsQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'domains', 'available'],
    queryFn: () => CloudflareService.listAvailableDomains(),
  });
  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
  });
  const invalidate = async () =>
    queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });

  const updateGroupMutation = useMutation({
    mutationFn: (payload: CloudflareGroupPayload) =>
      CloudflareService.updateGroup(groupID, payload),
    onSuccess: async () => {
      toast.success(t('groupUpdated'));
      setEditOpen(false);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const addMutation = useMutation({
    mutationFn: async ({
      domainIDs,
      proxied,
    }: {
      domainIDs: number[];
      proxied: boolean;
    }) => {
      const results = await Promise.allSettled(
        domainIDs.map((domainID) =>
          CloudflareService.createMember(groupID, {
            zone_domain_id: domainID,
            proxied,
          }),
        ),
      );
      const failed = results.filter((r) => r.status === 'rejected');
      const succeeded = results.length - failed.length;
      return { succeeded, failed: failed.length, total: results.length };
    },
    onSuccess: async ({ succeeded, failed, total }) => {
      if (failed === 0) {
        toast.success(
          total === 1
            ? t('memberAddedOne')
            : t('memberAddedMany', { count: succeeded }),
        );
      } else if (succeeded === 0) {
        toast.error(t('memberAddFailed', { count: failed }));
      } else {
        toast.warning(t('memberPartial', { succeeded, failed }));
      }
      setAddOpen(false);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const proxiedMutation = useMutation({
    mutationFn: ({
      memberID,
      proxied,
    }: {
      memberID: number;
      proxied: boolean;
    }) => CloudflareService.updateMember(groupID, memberID, proxied),
    onSuccess: async () => {
      toast.success(t('proxyUpdated'));
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const syncMutation = useMutation({
    mutationFn: (memberID: number) =>
      CloudflareService.syncMember(groupID, memberID),
    onSuccess: () => toast.success(t('memberSyncQueued')),
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const removeMutation = useMutation({
    mutationFn: (memberID: number) =>
      CloudflareService.removeMember(groupID, memberID),
    onSuccess: async () => {
      toast.success(t('memberDeleted'));
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  if (detailQuery.isLoading)
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder icon={Cloud} description={t('loadingDetail')} />
      </div>
    );
  if (detailQuery.isError || !detailQuery.data)
    return (
      <div className='w-full py-6 px-1'>
        <ErrorInline
          message={getErrorMessage(detailQuery.error)}
          onRetry={() => void detailQuery.refetch()}
        />
      </div>
    );
  const { group, members } = detailQuery.data;

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-col gap-4'>
        <Button variant='ghost' size='sm' className='self-start' asChild>
          <Link href='/cloudflare'>
            <ArrowLeft data-icon='inline-start' />
            {t('back')}
          </Link>
        </Button>
        <div className='flex items-center justify-between gap-3'>
          <div className='flex items-center gap-2'>
            <Cloud className='size-5 text-primary' />
            <h1 className='text-2xl font-semibold tracking-tight'>
              {group.name}
            </h1>
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => void detailQuery.refetch()}
              disabled={detailQuery.isFetching}
            >
              {detailQuery.isFetching ? (
                <Loader2 data-icon='inline-start' className='animate-spin' />
              ) : (
                <RefreshCw data-icon='inline-start' />
              )}
              {t('refresh')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setEditOpen(true)}
            >
              <Settings data-icon='inline-start' />
              {t('edit')}
            </Button>
            <Button size='sm' onClick={() => setAddOpen(true)}>
              <Plus data-icon='inline-start' />
              {t('addDomain')}
            </Button>
          </div>
        </div>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-base'>{t('currentPoint')}</CardTitle>
          <CardDescription>
            {t('activeNode', {
              name: group.active_node.name,
              ip: group.active_node.ip,
            })}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-wrap gap-2'>
          <Badge variant={group.enabled ? 'default' : 'secondary'}>
            {group.enabled ? t('syncEnabled') : t('syncDisabled')}
          </Badge>
          <Badge variant='outline'>
            {t('primaryNode', { name: group.primary_node.name })}
          </Badge>
          <Badge variant='outline'>
            {t('backupNode', {
              name: group.backup_node?.name ?? t('backupUnset'),
            })}
          </Badge>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-base'>{t('members')}</CardTitle>
          <CardDescription>{t('membersDesc')}</CardDescription>
        </CardHeader>
        <CardContent>
          {members.length === 0 ? (
            <p className='py-8 text-center text-sm text-muted-foreground'>
              {t('noMembers')}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('columns.domain')}</TableHead>
                  <TableHead>{t('columns.desiredIp')}</TableHead>
                  <TableHead>{t('columns.status')}</TableHead>
                  <TableHead>{t('columns.proxied')}</TableHead>
                  <TableHead className='text-right'>
                    {t('columns.actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell>
                      <div className='font-medium'>{member.domain}</div>
                      {member.last_error ? (
                        <p className='max-w-md text-xs text-destructive'>
                          {member.last_error}
                        </p>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      {member.desired_ip || t('pendingSync')}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          member.sync_status === 'ok'
                            ? 'default'
                            : member.sync_status === 'error'
                              ? 'destructive'
                              : 'secondary'
                        }
                      >
                        {member.sync_status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={member.proxied}
                        onCheckedChange={(proxied) =>
                          proxiedMutation.mutate({
                            memberID: member.id,
                            proxied,
                          })
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => syncMutation.mutate(member.id)}
                        >
                          <RefreshCw data-icon='inline-start' />
                          {t('sync')}
                        </Button>
                        <Button
                          variant='destructive'
                          size='sm'
                          onClick={() => removeMutation.mutate(member.id)}
                        >
                          <Trash2 data-icon='inline-start' />
                          {t('remove')}
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

      <GroupDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        group={group}
        nodes={nodesQuery.data ?? []}
        pending={updateGroupMutation.isPending}
        onSubmit={(payload) => updateGroupMutation.mutate(payload)}
      />
      <MemberAddDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        domains={domainsQuery.data ?? []}
        defaultProxied={group.default_proxied}
        pending={addMutation.isPending}
        onSubmit={(domainIDs, proxied) =>
          addMutation.mutate({ domainIDs, proxied })
        }
      />
    </div>
  );
}
