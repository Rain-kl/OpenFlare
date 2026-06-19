'use client';

import Link from 'next/link';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useMemo, useState} from 'react';
import {Globe, Plus, Trash2} from 'lucide-react';
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
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import type {ManagedDomainItem} from '@/lib/services/openflare';
import {TlsCertificateService, WebsiteService} from '@/lib/services/openflare';

import {CertificateImportDialog} from './components/certificate-import-dialog';
import {WebsiteStatusBadge} from './components/status-badge';
import {WebsiteEditorDialog} from './components/website-editor-dialog';
import {buildCertificateLabel, getErrorMessage, getMatchTypeMeta,} from './components/website-utils';

const domainsQueryKey = ['openflare', 'managed-domains'];
const certificatesQueryKey = ['openflare', 'tls-certificates'];

export default function WebsitesPage() {
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [preferredCertificateId, setPreferredCertificateId] = useState<number | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ManagedDomainItem | null>(null);

  const domainsQuery = useQuery({
    queryKey: domainsQueryKey,
    queryFn: () => WebsiteService.list(),
  });

  const certificatesQuery = useQuery({
    queryKey: certificatesQueryKey,
    queryFn: () => TlsCertificateService.list(),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => WebsiteService.deleteById(id),
    onSuccess: async () => {
      toast.success('网站已删除');
      setDeleteTarget(null);
      await queryClient.invalidateQueries({queryKey: domainsQueryKey});
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const domains = useMemo(() => domainsQuery.data ?? [], [domainsQuery.data]);
  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );
  const certificateMap = useMemo(
    () => new Map(certificates.map((item) => [item.id, item])),
    [certificates],
  );

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Globe className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">网站</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" className="h-7 text-xs" onClick={() => setEditorOpen(true)}>
            <Plus className="size-3.5 mr-1" />
            新增网站
          </Button>
        </div>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-semibold">网站列表</CardTitle>
          <CardDescription>查看网站绑定的证书、启用状态和更新时间。</CardDescription>
        </CardHeader>
        <CardContent>
          {domainsQuery.isLoading ? (
            <LoadingStateWithBorder icon={Globe} description="加载网站列表中..." />
          ) : domainsQuery.isError ? (
            <div className="p-8 border border-dashed rounded-lg">
              <ErrorInline
                message={getErrorMessage(domainsQuery.error)}
                onRetry={() => void domainsQuery.refetch()}
                className="justify-center"
              />
            </div>
          ) : domains.length === 0 ? (
            <EmptyStateWithBorder
              icon={Globe}
              description="暂无网站，点击右上角「新增网站」开始录入。"
            />
          ) : (
            <div className="grid gap-3 lg:grid-cols-2">
              {domains.map((domain) => {
                const certificate = domain.cert_id
                  ? (certificateMap.get(domain.cert_id) ?? null)
                  : null;
                const matchType = getMatchTypeMeta(domain.domain);

                return (
                  <div
                    key={domain.id}
                    className="rounded-lg border bg-card p-4 space-y-3"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="space-y-2 min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <h2 className="text-sm font-semibold truncate">{domain.domain}</h2>
                          <WebsiteStatusBadge label={matchType.label} tone={matchType.tone} />
                          <WebsiteStatusBadge
                            label={domain.enabled ? '启用' : '停用'}
                            tone={domain.enabled ? 'success' : 'warning'}
                          />
                        </div>
                        <p className="text-xs text-muted-foreground">
                          {domain.remark || '暂无备注'}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          绑定证书：
                          {certificate
                            ? buildCertificateLabel(certificate)
                            : '未绑定证书'}
                        </p>
                      </div>
                      <div className="flex shrink-0 gap-1">
                        <Button variant="outline" size="sm" className="h-7 text-xs" asChild>
                          <Link href={`/websites/detail?id=${domain.id}`}>
                            详情
                          </Link>
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs text-destructive"
                          onClick={() => setDeleteTarget(domain)}
                        >
                          <Trash2 className="size-3" />
                        </Button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <WebsiteEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        certificates={certificates}
        certificatesLoading={certificatesQuery.isLoading}
        preferredCertificateId={preferredCertificateId}
        onRequestImportCertificate={() => setImportOpen(true)}
        onSaved={(_, mode) => {
          setPreferredCertificateId(null);
          toast.success(mode === 'create' ? '网站已创建' : '网站已更新');
        }}
      />

      <CertificateImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImported={(certificate) => {
          setPreferredCertificateId(certificate.id);
          toast.success(`证书 ${certificate.name} 已导入，可直接用于当前网站`);
        }}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除网站</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除网站 {deleteTarget?.domain} 吗？此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
            >
              删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
