'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Cloud, Plus, RefreshCw, Settings, Trash2 } from 'lucide-react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useState } from 'react';
import { toast } from 'sonner';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
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

export default function CloudflareGroupDetailPage() {
  const params = useParams<{ id: string }>();
  const groupID = Number(params.id);
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const detailQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'groups', groupID],
    queryFn: () => CloudflareService.getGroup(groupID),
    enabled: Number.isInteger(groupID) && groupID > 0,
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
      toast.success('分组配置已更新');
      setEditOpen(false);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const addMutation = useMutation({
    mutationFn: ({
      domainID,
      proxied,
    }: {
      domainID: number;
      proxied: boolean;
    }) =>
      CloudflareService.createMember(groupID, {
        zone_domain_id: domainID,
        proxied,
      }),
    onSuccess: async () => {
      toast.success('域名已加入并排队同步');
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
      toast.success('橙云设置已更新并排队同步');
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const syncMutation = useMutation({
    mutationFn: (memberID: number) =>
      CloudflareService.syncMember(groupID, memberID),
    onSuccess: () => toast.success('成员同步任务已入队'),
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const removeMutation = useMutation({
    mutationFn: (memberID: number) =>
      CloudflareService.removeMember(groupID, memberID),
    onSuccess: async () => {
      toast.success('成员及远端 A 记录已删除');
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  if (detailQuery.isLoading)
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder icon={Cloud} description='加载分组详情中...' />
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
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {group.name}
          </h1>
        </div>
        <div className='flex items-center gap-2'>
          <Button asChild variant='outline' size='sm'>
            <Link href='/cloudflare/groups'>返回分组</Link>
          </Button>
          <Button variant='outline' size='sm' onClick={() => setEditOpen(true)}>
            <Settings data-icon='inline-start' />
            编辑
          </Button>
          <Button size='sm' onClick={() => setAddOpen(true)}>
            <Plus data-icon='inline-start' />
            添加域名
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>当前指向</CardTitle>
          <CardDescription>
            生效节点 {group.active_node.name} · {group.active_node.ip}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-wrap gap-2'>
          <Badge variant={group.enabled ? 'default' : 'secondary'}>
            {group.enabled ? '同步已启用' : '同步已停用'}
          </Badge>
          <Badge variant='outline'>主节点 {group.primary_node.name}</Badge>
          <Badge variant='outline'>
            备用节点 {group.backup_node?.name ?? '未设置'}
          </Badge>
        </CardContent>
      </Card>

      <Alert>
        <Cloud />
        <AlertTitle>远端记录所有权</AlertTitle>
        <AlertDescription>
          移出成员会立即删除本模块缓存或唯一同名 A 记录；存在多条同名 A
          时会拒绝操作并要求先手动清理。
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle>域名成员</CardTitle>
          <CardDescription>成员级橙云是同步时的唯一依据。</CardDescription>
        </CardHeader>
        <CardContent>
          {members.length === 0 ? (
            <p className='py-8 text-center text-sm text-muted-foreground'>
              暂无域名成员。
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>域名</TableHead>
                  <TableHead>期望 IP</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>橙云</TableHead>
                  <TableHead className='text-right'>操作</TableHead>
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
                    <TableCell>{member.desired_ip || '待同步'}</TableCell>
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
                          同步
                        </Button>
                        <Button
                          variant='destructive'
                          size='sm'
                          onClick={() => removeMutation.mutate(member.id)}
                        >
                          <Trash2 data-icon='inline-start' />
                          移出
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
        onSubmit={(domainID, proxied) =>
          addMutation.mutate({ domainID, proxied })
        }
      />
    </div>
  );
}
