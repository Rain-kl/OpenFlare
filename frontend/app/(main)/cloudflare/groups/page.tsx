'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Cloud, Plus, RefreshCw, Settings, Trash2 } from 'lucide-react';
import Link from 'next/link';
import { useState } from 'react';
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
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  CloudflareService,
  cloudflareQueryKey,
  NodeService,
  type CloudflareGroup,
  type CloudflareGroupPayload,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';
import { GroupDialog } from '../components/group-dialog';

export default function CloudflareGroupsPage() {
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CloudflareGroup | null>(
    null,
  );
  const groupsQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'groups'],
    queryFn: () => CloudflareService.listGroups(),
  });
  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
  });

  const invalidate = async () =>
    queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });
  const createMutation = useMutation({
    mutationFn: (payload: CloudflareGroupPayload) =>
      CloudflareService.createGroup(payload),
    onSuccess: async () => {
      toast.success('指向分组已创建');
      setCreateOpen(false);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const syncMutation = useMutation({
    mutationFn: (id: number) => CloudflareService.syncGroup(id),
    onSuccess: () => toast.success('分组同步任务已入队'),
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const deleteMutation = useMutation({
    mutationFn: (id: number) => CloudflareService.deleteGroup(id),
    onSuccess: async () => {
      toast.success('指向分组及远端记录已删除');
      setDeleteTarget(null);
      await invalidate();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            Cloudflare 指向分组
          </h1>
        </div>
        <div className='flex items-center gap-2'>
          <Button asChild variant='outline' size='sm'>
            <Link href='/cloudflare'>返回总览</Link>
          </Button>
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            新增分组
          </Button>
        </div>
      </div>

      {groupsQuery.isLoading ? (
        <LoadingStateWithBorder icon={Cloud} description='加载指向分组中...' />
      ) : groupsQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(groupsQuery.error)}
          onRetry={() => void groupsQuery.refetch()}
        />
      ) : (groupsQuery.data ?? []).length === 0 ? (
        <Card>
          <CardContent className='py-12 text-center text-sm text-muted-foreground'>
            暂无指向分组。
          </CardContent>
        </Card>
      ) : (
        <div className='grid gap-4 lg:grid-cols-2'>
          {(groupsQuery.data ?? []).map((group) => (
            <Card key={group.id}>
              <CardHeader>
                <div className='flex items-start justify-between gap-3'>
                  <div>
                    <CardTitle>{group.name}</CardTitle>
                    <CardDescription>
                      {group.active_node.name} · {group.active_node.ip}
                    </CardDescription>
                  </div>
                  <Badge variant={group.enabled ? 'default' : 'secondary'}>
                    {group.enabled ? '已启用' : '已停用'}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className='flex flex-col gap-4'>
                <p className='text-sm text-muted-foreground'>
                  成员 {group.member_count} 个 · 新成员默认
                  {group.default_proxied ? '开启' : '关闭'}橙云
                </p>
                <div className='flex flex-wrap gap-2'>
                  <Button asChild size='sm'>
                    <Link href={`/cloudflare/groups/${group.id}`}>
                      <Settings data-icon='inline-start' />
                      管理
                    </Link>
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => syncMutation.mutate(group.id)}
                  >
                    <RefreshCw data-icon='inline-start' />
                    同步
                  </Button>
                  <Button
                    variant='destructive'
                    size='sm'
                    onClick={() => setDeleteTarget(group)}
                  >
                    <Trash2 data-icon='inline-start' />
                    删除
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <GroupDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        nodes={nodesQuery.data ?? []}
        pending={createMutation.isPending}
        onSubmit={(payload) => createMutation.mutate(payload)}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除指向分组</AlertDialogTitle>
            <AlertDialogDescription>
              将删除 {deleteTarget?.name} 的全部成员及本模块管理的远端 A
              记录。此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
