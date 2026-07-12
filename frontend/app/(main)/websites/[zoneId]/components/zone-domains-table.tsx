'use client';

import {useState} from 'react';
import {useMutation} from '@tanstack/react-query';
import {MoreHorizontal, Pencil, Plus, Trash2} from 'lucide-react';
import {toast} from 'sonner';

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
import {Button} from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from '@/components/ui/table';
import {ZoneDomainService, type TlsCertificateItem, type ZoneDomainItem} from '@/lib/services/openflare';

import {ZoneDomainDialog} from './zone-domain-dialog';

export function ZoneDomainsTable({
  zoneId,
  domains,
  certificates,
  onChanged,
}: {
  zoneId: number;
  domains: ZoneDomainItem[];
  certificates: TlsCertificateItem[];
  onChanged(): Promise<unknown> | void;
}) {
  const [editing, setEditing] = useState<ZoneDomainItem | null | undefined>(undefined);
  const [deleting, setDeleting] = useState<ZoneDomainItem | null>(null);

  const remove = useMutation({
    mutationFn: (id: number) => ZoneDomainService.deleteById(zoneId, id),
    onSuccess: async () => {
      toast.success('域名已删除');
      setDeleting(null);
      await onChanged();
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '删除失败'),
  });

  const certificateMap = new Map(certificates.map((certificate) => [certificate.id, certificate]));

  return (
    <>
      <div className="mb-4 flex justify-end">
        <Button size="sm" className="h-7 text-xs" onClick={() => setEditing(null)}>
          <Plus className="mr-1 size-3.5" />
          添加域名
        </Button>
      </div>

      {domains.length === 0 ? (
        <EmptyStateWithBorder description="暂无已添加域名" />
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>FQDN</TableHead>
                <TableHead>证书</TableHead>
                <TableHead>关联路由</TableHead>
                <TableHead>备注</TableHead>
                <TableHead className="w-12" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {domains.map((domain) => (
                <TableRow key={domain.id}>
                  <TableCell className="font-medium">{domain.domain}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {domain.cert_id
                      ? (certificateMap.get(domain.cert_id)?.name ?? `证书 #${domain.cert_id}`)
                      : '未绑定'}
                  </TableCell>
                  <TableCell>
                    {domain.proxy_route_id ? `#${domain.proxy_route_id}` : '未关联'}
                  </TableCell>
                  <TableCell className="max-w-48 truncate text-muted-foreground">
                    {domain.remark || '—'}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`操作 ${domain.domain}`}
                        >
                          <MoreHorizontal />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditing(domain)}>
                          <Pencil />
                          编辑
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => setDeleting(domain)}
                        >
                          <Trash2 />
                          删除
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <ZoneDomainDialog
        open={editing !== undefined}
        onOpenChange={(open) => {
          if (!open) {
            setEditing(undefined);
          }
        }}
        zoneId={zoneId}
        domain={editing ?? null}
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
            <AlertDialogCancel disabled={remove.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
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
