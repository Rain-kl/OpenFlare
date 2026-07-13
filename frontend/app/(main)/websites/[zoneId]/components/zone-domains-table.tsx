'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Eye, Plus, Trash2 } from 'lucide-react';
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
import { Button } from '@/components/ui/button';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  ZoneDomainService,
  type ProxyRouteItem,
  type TlsCertificateItem,
  type ZoneDomainItem,
} from '@/lib/services/openflare';

import { getUpstreamLabels } from '../../../proxy-routes/components/helpers';
import { ZoneDomainDialog } from './zone-domain-dialog';

export function ZoneDomainsTable({
  zoneId,
  zoneRoot,
  domains,
  certificates,
  routes,
  routesLoading = false,
  onChanged,
}: {
  zoneId: number;
  zoneRoot: string;
  domains: ZoneDomainItem[];
  certificates: TlsCertificateItem[];
  routes: ProxyRouteItem[];
  routesLoading?: boolean;
  onChanged(): Promise<unknown> | void;
}) {
  const [createOpen, setCreateOpen] = useState(false);
  const [deleting, setDeleting] = useState<ZoneDomainItem | null>(null);

  const remove = useMutation({
    mutationFn: (id: number) => ZoneDomainService.deleteById(zoneId, id),
    onSuccess: async () => {
      toast.success('域名已删除');
      setDeleting(null);
      await onChanged();
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : '删除失败'),
  });

  const certificateMap = useMemo(
    () =>
      new Map(certificates.map((certificate) => [certificate.id, certificate])),
    [certificates],
  );
  const routeMap = useMemo(
    () => new Map(routes.map((route) => [route.id, route])),
    [routes],
  );

  return (
    <>
      <div className='mb-3 flex justify-end'>
        <Button
          variant='secondary'
          size='sm'
          className='h-7 text-xs'
          onClick={() => setCreateOpen(true)}
        >
          <Plus className='mr-1 size-3.5' />
          添加域名
        </Button>
      </div>

      {domains.length === 0 ? (
        <EmptyStateWithBorder description='暂无已添加域名' />
      ) : (
        <div className='overflow-hidden rounded-lg border border-dashed shadow-none'>
          <TooltipProvider delayDuration={0}>
            <Table className='w-full min-w-full caption-bottom text-sm'>
              <TableHeader className='sticky top-0 z-20 bg-background'>
                <TableRow className='border-b border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 whitespace-nowrap py-2 min-w-[160px]'>
                    FQDN
                  </TableHead>
                  <TableHead className='h-8 whitespace-nowrap py-2 min-w-[120px]'>
                    证书
                  </TableHead>
                  <TableHead className='h-8 whitespace-nowrap py-2 min-w-[200px]'>
                    上游
                  </TableHead>
                  <TableHead className='sticky right-0 z-10 h-8 w-[90px] bg-background py-2 text-center'>
                    操作
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {domains.map((domain) => {
                  const route =
                    domain.proxy_route_id != null
                      ? routeMap.get(domain.proxy_route_id)
                      : undefined;
                  const upstreams = route ? getUpstreamLabels(route) : [];
                  const certLabel = domain.cert_id
                    ? (certificateMap.get(domain.cert_id)?.name ??
                      `证书 #${domain.cert_id}`)
                    : '未绑定';

                  return (
                    <TableRow
                      key={domain.id}
                      className='group border-dashed hover:bg-muted/30'
                    >
                      <TableCell className='py-1'>
                        <span
                          className='max-w-[220px] truncate text-[11px] font-medium leading-tight'
                          title={domain.domain}
                        >
                          {domain.domain}
                        </span>
                      </TableCell>
                      <TableCell className='py-1 font-mono text-[10px] whitespace-nowrap text-muted-foreground'>
                        {certLabel}
                      </TableCell>
                      <TableCell className='max-w-[280px] py-1'>
                        {!domain.proxy_route_id ? (
                          <span className='font-mono text-[10px] text-muted-foreground'>
                            未关联路由
                          </span>
                        ) : routesLoading && !route ? (
                          <span className='font-mono text-[10px] text-muted-foreground'>
                            加载中…
                          </span>
                        ) : upstreams.length === 0 ? (
                          <span className='font-mono text-[10px] text-muted-foreground'>
                            未配置上游
                          </span>
                        ) : (
                          <div className='flex flex-col gap-0'>
                            {upstreams.map((upstream) => (
                              <span
                                key={upstream}
                                className='truncate font-mono text-[10px] leading-tight text-muted-foreground'
                                title={upstream}
                              >
                                {upstream}
                              </span>
                            ))}
                          </div>
                        )}
                      </TableCell>
                      <TableCell
                        className='sticky right-0 z-10 bg-background py-1 text-center'
                        onClick={(event) => event.stopPropagation()}
                      >
                        <div className='flex items-center justify-center gap-0.5'>
                          {domain.proxy_route_id ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant='ghost'
                                  size='icon'
                                  className='h-6 w-6 text-muted-foreground hover:text-foreground'
                                  asChild
                                >
                                  <Link
                                    href={`/proxy-routes/detail?id=${domain.proxy_route_id}`}
                                  >
                                    <Eye className='size-3' />
                                  </Link>
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent side='top' className='text-xs'>
                                查看路由详情
                              </TooltipContent>
                            </Tooltip>
                          ) : null}

                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant='ghost'
                                size='icon'
                                className='h-6 w-6 text-muted-foreground hover:text-destructive'
                                onClick={() => setDeleting(domain)}
                              >
                                <Trash2 className='size-3' />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent side='top' className='text-xs'>
                              删除域名
                            </TooltipContent>
                          </Tooltip>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TooltipProvider>
        </div>
      )}

      <ZoneDomainDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        zoneId={zoneId}
        zoneRoot={zoneRoot}
        onSaved={onChanged}
      />

      <AlertDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleting(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除域名</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除 {deleting?.domain} 吗？此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={remove.isPending}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={remove.isPending || !deleting}
              onClick={(event) => {
                event.preventDefault();
                if (deleting) {
                  remove.mutate(deleting.id);
                }
              }}
            >
              {remove.isPending ? '删除中…' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
