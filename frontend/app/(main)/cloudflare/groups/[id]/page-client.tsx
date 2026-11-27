'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  ArrowRightLeft,
  Cloud,
  CloudCog,
  Loader2,
  Plus,
  RefreshCw,
  Settings,
  Search,
  Trash2,
} from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { usePathname } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  CloudflareService,
  cloudflareQueryKey,
  NodeService,
  SmartDNSService,
  smartdnsQueryKey,
  type CloudflareGroup,
  type CloudflareGroupPayload,
  type CloudflareMember,
  type SmartDNSPolicy,
} from '@/lib/services/openflare';

import { getErrorMessage } from '../../../websites/components/website-utils';
import { GroupDialog } from '../../components/group-dialog';
import { MemberAddDialog } from '../../components/member-add-dialog';
import { MemberTable } from '../../components/member-table';
import { MemberMoveDialog } from '../../components/member-move-dialog';

const emptySmartDNSPolicies: SmartDNSPolicy[] = [];

function getGroupIdFromPathname(pathname: string | null): number {
  const match = pathname?.match(/^\/cloudflare\/groups\/([^/]+)$/);
  return Number(match?.[1]);
}

export function CloudflareGroupDetailPageClient() {
  const t = useTranslations('cloudflare');
  const tCommon = useTranslations('common');
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  const pathname = usePathname();
  const groupID = useMemo(() => getGroupIdFromPathname(pathname), [pathname]);
  const queryClient = useQueryClient();
  const [editingGroup, setEditingGroup] = useState<CloudflareGroup | null>(
    null,
  );
  const editOpen = editingGroup !== null;
  const [addOpen, setAddOpen] = useState(false);
  const [selectedMemberIDs, setSelectedMemberIDs] = useState<Set<number>>(
    () => new Set(),
  );
  const [moveDialogOpen, setMoveDialogOpen] = useState(false);
  const [movingMembers, setMovingMembers] = useState<CloudflareMember[]>([]);
  const [batchRemoveOpen, setBatchRemoveOpen] = useState(false);
  const [memberSearch, setMemberSearch] = useState('');
  const [removeTarget, setRemoveTarget] = useState<CloudflareMember | null>(
    null,
  );

  const detailQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'groups', groupID],
    queryFn: () => CloudflareService.getGroup(groupID),
    enabled: Number.isInteger(groupID) && groupID > 0,
    refetchInterval: 5000,
  });
  const domainsQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'domains', 'available'],
    queryFn: () => CloudflareService.listAvailableDomains(),
    enabled: addOpen,
  });
  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
    enabled: editOpen,
  });
  const groupsQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'groups'],
    queryFn: () => CloudflareService.listGroups(),
    enabled: moveDialogOpen,
  });
  const smartdnsPoliciesQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'policies'],
    queryFn: () => SmartDNSService.listPolicies(),
    enabled: editOpen,
  });
  const invalidate = async () =>
    queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });

  const updateGroupMutation = useMutation({
    mutationFn: (payload: CloudflareGroupPayload) =>
      CloudflareService.updateGroup(groupID, payload),
    onSuccess: async () => {
      toast.success(t('groupUpdated'));
      setEditingGroup(null);
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
    onSuccess: async () => {
      toast.success(t('memberSyncQueued'));
      await queryClient.invalidateQueries({
        queryKey: [...cloudflareQueryKey, 'sync-executions'],
      });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const removeMutation = useMutation({
    mutationFn: (memberID: number) =>
      CloudflareService.removeMember(groupID, memberID),
    onSuccess: async (_, memberID) => {
      toast.success(t('memberDeleted'));
      setRemoveTarget(null);
      setSelectedMemberIDs((prev) => {
        const next = new Set(prev);
        next.delete(memberID);
        return next;
      });
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const moveMutation = useMutation({
    mutationFn: async ({
      targetGroupId,
      membersToMove,
    }: {
      targetGroupId: number;
      membersToMove: CloudflareMember[];
    }) => {
      if (membersToMove.length === 1) {
        return CloudflareService.moveMember(
          groupID,
          membersToMove[0].id,
          targetGroupId,
        );
      }
      return CloudflareService.batchMoveMembers(
        groupID,
        membersToMove.map((m) => m.id),
        targetGroupId,
      );
    },
    onSuccess: async (_, variables) => {
      if (variables.membersToMove.length === 1) {
        toast.success(t('memberMoved'));
      } else {
        toast.success(
          t('batchMoved', { count: variables.membersToMove.length }),
        );
      }
      setMoveDialogOpen(false);
      setSelectedMemberIDs((prev) => {
        const next = new Set(prev);
        for (const m of variables.membersToMove) {
          next.delete(m.id);
        }
        return next;
      });
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const batchRemoveMutation = useMutation({
    mutationFn: (memberIds: number[]) =>
      CloudflareService.batchRemoveMembers(groupID, memberIds),
    onSuccess: async (_, memberIds) => {
      toast.success(t('batchRemoved', { count: memberIds.length }));
      setBatchRemoveOpen(false);
      setSelectedMemberIDs(new Set());
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const batchEnableProxyMutation = useMutation({
    mutationFn: (memberIds: number[]) =>
      CloudflareService.batchEnableProxy(groupID, memberIds),
    onSuccess: async (_, memberIds) => {
      toast.success(t('batchProxyEnabled', { count: memberIds.length }));
      setSelectedMemberIDs(new Set());
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const handleSingleMove = (member: CloudflareMember) => {
    setMovingMembers([member]);
    setMoveDialogOpen(true);
  };

  const handleBatchMove = (selectedList: CloudflareMember[]) => {
    if (selectedList.length === 0) return;
    setMovingMembers(selectedList);
    setMoveDialogOpen(true);
  };

  if (!mounted || detailQuery.isLoading)
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

  const selectedMembers = members.filter((m) => selectedMemberIDs.has(m.id));
  const visibleMembers = members.filter((member) =>
    member.domain.toLowerCase().includes(memberSearch.trim().toLowerCase()),
  );
  const selectMembers = (ids: number[], checked: boolean) => {
    setSelectedMemberIDs((previous) => {
      const next = new Set(previous);
      for (const id of ids) {
        if (checked) next.add(id);
        else next.delete(id);
      }
      return next;
    });
  };

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-col gap-3'>
        <Button variant='ghost' size='sm' className='self-start' asChild>
          <Link href='/cloudflare'>
            <ArrowLeft data-icon='inline-start' />
            {t('back')}
          </Link>
        </Button>
        <div className='flex flex-wrap items-center justify-between gap-3'>
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
              onClick={() => setEditingGroup(group)}
            >
              <Settings data-icon='inline-start' />
              {t('edit')}
            </Button>
          </div>
        </div>
      </div>

      <section
        className='flex flex-wrap items-center justify-between gap-3 rounded-lg border border-dashed px-4 py-3'
        aria-label={t('currentPoint')}
      >
        <div className='min-w-0'>
          <p className='text-xs text-muted-foreground'>{t('currentPoint')}</p>
          <p className='break-all text-sm font-medium'>
            {group.target_mode === 'smartdns'
              ? t('smartdnsTargetSummary', {
                  policy:
                    group.smartdns_policy_name ||
                    `#${group.smartdns_policy_id ?? '-'}`,
                  host:
                    group.smartdns_target_host ||
                    group.record_content ||
                    t('smartdnsAutoHost'),
                })
              : group.target_mode === 'custom'
                ? t('customTargetSummary', {
                    type: group.record_type,
                    content: group.record_content,
                  })
                : t('activeNode', {
                    name: group.active_node.name,
                    ip: group.active_node.ip,
                  })}
          </p>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Badge variant={group.enabled ? 'default' : 'secondary'}>
            {group.enabled ? t('syncEnabled') : t('syncDisabled')}
          </Badge>
          {group.target_mode === 'node' ? (
            <>
              <Badge variant='outline'>
                {t('primaryNode', { name: group.primary_node.name })}
              </Badge>
              <Badge variant='outline'>
                {t('backupNode', {
                  name: group.backup_node?.name ?? t('backupUnset'),
                })}
              </Badge>
            </>
          ) : null}
          <Badge variant='outline'>
            {t(
              group.target_mode === 'smartdns'
                ? 'smartdnsTargetMode'
                : group.target_mode === 'custom'
                  ? 'customTargetMode'
                  : 'nodeTargetMode',
            )}
          </Badge>
          <Badge variant='outline'>
            {group.ttl === 1
              ? t('ttlAutoSummary')
              : t('ttlSecondsSummary', { seconds: group.ttl ?? 300 })}
          </Badge>
        </div>
      </section>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-col gap-4'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div>
              <CardTitle className='text-base'>{t('members')}</CardTitle>
              <CardDescription>
                {t('domainCount', { count: members.length })}
              </CardDescription>
            </div>
            <div className='flex w-full flex-wrap items-center gap-2 sm:w-auto'>
              {members.length > 0 && (
                <div className='relative min-w-40 flex-1'>
                  <Search className='pointer-events-none absolute top-2.5 left-3 size-4 text-muted-foreground' />
                  <Input
                    type='search'
                    className='pl-9'
                    aria-label={t('searchMembers')}
                    placeholder={t('searchMembers')}
                    value={memberSearch}
                    onChange={(event) => setMemberSearch(event.target.value)}
                  />
                </div>
              )}
              <Button size='sm' onClick={() => setAddOpen(true)}>
                <Plus data-icon='inline-start' />
                {t('addDomain')}
              </Button>
            </div>
          </div>
          {selectedMembers.length > 0 && (
            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant='secondary' className='text-xs'>
                {t('selectedCount', { count: selectedMembers.length })}
              </Badge>
              {group.target_mode !== 'smartdns' ? (
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    batchEnableProxyMutation.mutate(
                      selectedMembers.map((member) => member.id),
                    )
                  }
                  disabled={batchEnableProxyMutation.isPending}
                >
                  <CloudCog data-icon='inline-start' />
                  {t('batchEnableProxy')}
                </Button>
              ) : null}
              <Button
                variant='outline'
                size='sm'
                onClick={() => handleBatchMove(selectedMembers)}
              >
                <ArrowRightLeft data-icon='inline-start' />
                {t('batchMove')}
              </Button>
              <Button
                variant='destructive'
                size='sm'
                onClick={() => setBatchRemoveOpen(true)}
              >
                <Trash2 data-icon='inline-start' />
                {t('batchRemove')}
              </Button>
              <Button
                variant='ghost'
                size='sm'
                onClick={() => setSelectedMemberIDs(new Set())}
              >
                {t('clearSelection')}
              </Button>
            </div>
          )}
        </CardHeader>
        <CardContent>
          {members.length === 0 ? (
            <p className='py-8 text-center text-sm text-muted-foreground'>
              {t('noMembers')}
            </p>
          ) : visibleMembers.length === 0 ? (
            <p className='py-8 text-center text-sm text-muted-foreground'>
              {t('noMatchingMembers')}
            </p>
          ) : (
            <MemberTable
              members={visibleMembers}
              selectedMemberIDs={selectedMemberIDs}
              dnsOnly={group.target_mode === 'smartdns'}
              updatingProxy={proxiedMutation.isPending}
              syncing={!group.enabled || syncMutation.isPending}
              onSelect={selectMembers}
              onProxy={(memberID, proxied) =>
                proxiedMutation.mutate({ memberID, proxied })
              }
              onMove={handleSingleMove}
              onSync={(memberID) => syncMutation.mutate(memberID)}
              onRemove={setRemoveTarget}
            />
          )}
        </CardContent>
      </Card>

      <GroupDialog
        open={editOpen}
        onOpenChange={(open) => !open && setEditingGroup(null)}
        group={editingGroup}
        nodes={nodesQuery.data ?? []}
        smartdnsPolicies={smartdnsPoliciesQuery.data ?? emptySmartDNSPolicies}
        pending={updateGroupMutation.isPending}
        onSubmit={(payload) => updateGroupMutation.mutate(payload)}
      />
      <MemberAddDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        domains={domainsQuery.data ?? []}
        defaultProxied={
          group.target_mode === 'smartdns' ? false : group.default_proxied
        }
        forceDnsOnly={group.target_mode === 'smartdns'}
        pending={addMutation.isPending}
        onSubmit={(domainIDs, proxied) =>
          addMutation.mutate({ domainIDs, proxied })
        }
      />
      <MemberMoveDialog
        open={moveDialogOpen}
        onOpenChange={setMoveDialogOpen}
        members={movingMembers}
        groups={groupsQuery.data ?? []}
        currentGroupId={groupID}
        pending={moveMutation.isPending}
        onSubmit={(targetGroupId) =>
          moveMutation.mutate({
            targetGroupId,
            membersToMove: movingMembers,
          })
        }
      />
      <AlertDialog
        open={removeTarget !== null}
        onOpenChange={(open) =>
          !open && !removeMutation.isPending && setRemoveTarget(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('removeMemberTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('removeMemberDesc', { name: removeTarget?.domain ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removeMutation.isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              disabled={removeMutation.isPending}
              onClick={() =>
                removeTarget && removeMutation.mutate(removeTarget.id)
              }
            >
              {t('remove')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={batchRemoveOpen} onOpenChange={setBatchRemoveOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('batchRemoveDialog.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('batchRemoveDialog.desc', { count: selectedMembers.length })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              disabled={batchRemoveMutation.isPending}
              onClick={() =>
                batchRemoveMutation.mutate(selectedMembers.map((m) => m.id))
              }
            >
              {batchRemoveMutation.isPending
                ? t('batchRemoveDialog.removing')
                : t('batchRemoveDialog.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
