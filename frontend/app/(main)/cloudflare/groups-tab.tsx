'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  Cloud,
  Plus,
  RefreshCw,
  Search,
  Settings,
  Trash2,
} from 'lucide-react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { GroupDialog } from '@/app/(main)/cloudflare/components/group-dialog';
import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { EmptyStateWithBorder } from '@/components/layout/empty';
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
  type SmartDNSPolicy,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../websites/components/website-utils';

const emptySmartDNSPolicies: SmartDNSPolicy[] = [];

export function CloudflareGroupsTab({
  onShowHistory,
}: {
  onShowHistory: () => void;
}) {
  const t = useTranslations('cloudflare');
  const tCommon = useTranslations('common');
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CloudflareGroup | null>(
    null,
  );
  const overviewQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'overview'],
    queryFn: () => CloudflareService.getOverview(),
  });
  const groupsQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'groups'],
    queryFn: () => CloudflareService.listGroups(),
  });
  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
    enabled: createOpen,
  });
  const smartdnsPoliciesQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'policies'],
    queryFn: () => SmartDNSService.listPolicies(),
    enabled: createOpen,
  });

  const invalidate = async () =>
    queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });
  const createMutation = useMutation({
    mutationFn: (payload: CloudflareGroupPayload) =>
      CloudflareService.createGroup(payload),
    onSuccess: async () => {
      toast.success(t('created'));
      setCreateOpen(false);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const syncMutation = useMutation({
    mutationFn: (id: number) => CloudflareService.syncGroup(id),
    onSuccess: async () => {
      toast.success(t('syncQueued'), {
        action: { label: t('tasks.title'), onClick: onShowHistory },
      });
      await queryClient.invalidateQueries({
        queryKey: [...cloudflareQueryKey, 'sync-executions'],
      });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const deleteMutation = useMutation({
    mutationFn: (id: number) => CloudflareService.deleteGroup(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const overview = overviewQuery.data;
  const groups = groupsQuery.data ?? [];
  const keyword = search.trim().toLowerCase();
  const visibleGroups = groups.filter((group) =>
    [
      group.name,
      group.active_node?.name,
      group.active_node?.ip,
      group.record_content,
      group.smartdns_policy_name,
      group.smartdns_target_host,
    ].some((value) => value?.toLowerCase().includes(keyword)),
  );

  const dnsReady = overview?.dns_ready ?? overview?.connection.ready;

  return (
    <div className='flex flex-col gap-4'>
      {overviewQuery.isLoading ? (
        <LoadingStateWithBorder
          icon={Cloud}
          description={t('loadingOverview')}
        />
      ) : overviewQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(overviewQuery.error)}
          onRetry={() => void overviewQuery.refetch()}
        />
      ) : !dnsReady ? (
        <Alert>
          <AlertTriangle />
          <AlertTitle>{t('notReadyTitle')}</AlertTitle>
          <AlertDescription className='flex flex-wrap items-center gap-x-3 gap-y-1'>
            <span>{t('notReadyDesc')}</span>
            <Button asChild variant='link' className='h-auto px-0 py-0'>
              <Link href='/dns-accounts'>{t('configure')}</Link>
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {groups.length > 0 && (
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <div className='relative w-full sm:w-72'>
            <Search className='pointer-events-none absolute top-2.5 left-3 size-4 text-muted-foreground' />
            <Input
              type='search'
              className='pl-9'
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              aria-label={t('searchGroups')}
              placeholder={t('searchGroups')}
            />
          </div>
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            {t('addGroup')}
          </Button>
        </div>
      )}
      {groupsQuery.isPending ? (
        <LoadingStateWithBorder icon={Cloud} description={t('loadingGroups')} />
      ) : groupsQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(groupsQuery.error)}
          onRetry={() => void groupsQuery.refetch()}
        />
      ) : groups.length === 0 ? (
        <EmptyStateWithBorder
          icon={Cloud}
          title={t('emptyTitle')}
          description={t('emptyDesc')}
          actionText={t('addGroup')}
          onAction={() => setCreateOpen(true)}
        />
      ) : visibleGroups.length === 0 ? (
        <EmptyStateWithBorder
          icon={Search}
          title={t('noMatchingGroups')}
          actionText={t('clearSearch')}
          onAction={() => setSearch('')}
        />
      ) : (
        <div className='rounded-lg border border-dashed'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='pl-4'>{t('groupsTitle')}</TableHead>
                <TableHead>{t('currentPoint')}</TableHead>
                <TableHead>{t('groupSyncStatus')}</TableHead>
                <TableHead className='pr-4 text-center'>
                  {t('columns.actions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visibleGroups.map((group) => (
                <TableRow key={group.id}>
                  <TableCell className='py-3 pl-4 align-top'>
                    <Link
                      href={`/cloudflare/groups/${group.id}`}
                      className='font-medium underline-offset-4 hover:underline'
                    >
                      {group.name}
                    </Link>
                    <p className='text-xs text-muted-foreground'>
                      {t('domainCount', { count: group.member_count })}
                    </p>
                  </TableCell>
                  <TableCell className='min-w-48 py-3 whitespace-normal align-top'>
                    <p className='text-sm wrap-anywhere'>
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
                    <p className='text-xs text-muted-foreground'>
                      {group.target_mode === 'smartdns'
                        ? t('dnsOnly')
                        : t('proxyDefault', {
                            proxy: group.default_proxied
                              ? t('proxyOn')
                              : t('proxyOff'),
                          })}
                    </p>
                  </TableCell>
                  <TableCell>
                    <Badge variant={group.enabled ? 'default' : 'secondary'}>
                      {group.enabled ? t('enabled') : t('disabled')}
                    </Badge>
                  </TableCell>
                  <TableCell className='pr-4 text-center'>
                    <ListActions>
                      <ListActionButton
                        icon={Settings}
                        label={t('manage')}
                        href={`/cloudflare/groups/${group.id}`}
                      />
                      <ListActionButton
                        icon={RefreshCw}
                        label={t('sync')}
                        loading={syncMutation.isPending}
                        disabled={
                          !dnsReady || !group.enabled || syncMutation.isPending
                        }
                        onClick={() => syncMutation.mutate(group.id)}
                      />
                      <ListActionButton
                        icon={Trash2}
                        label={t('delete')}
                        tone='destructive'
                        onClick={() => setDeleteTarget(group)}
                      />
                    </ListActions>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <GroupDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        nodes={nodesQuery.data ?? []}
        smartdnsPolicies={smartdnsPoliciesQuery.data ?? emptySmartDNSPolicies}
        pending={createMutation.isPending}
        onSubmit={(payload) => createMutation.mutate(payload)}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) =>
          !open && !deleteMutation.isPending && setDeleteTarget(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('confirmDelete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
