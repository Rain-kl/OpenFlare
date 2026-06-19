'use client';

import Link from 'next/link';
import {useRouter, useSearchParams} from 'next/navigation';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useMemo, useState} from 'react';
import {ArrowLeft, Globe, Plus, Trash2} from 'lucide-react';
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
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
import type {TlsCertificateItem} from '@/lib/services/openflare';
import {ProxyRouteService, TlsCertificateService, WebsiteService,} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';
import {getUpstreamSummary} from '@/app/(main)/proxy-routes/components/helpers';

import {CertificateApplyDialog} from '../components/certificate-apply-dialog';
import {CertificateDetailDialog} from '../components/certificate-detail-dialog';
import {CertificateEditorDialog} from '../components/certificate-editor-dialog';
import {CertificateImportDialog} from '../components/certificate-import-dialog';
import {WebsiteStatusBadge} from '../components/status-badge';
import {WebsiteEditorDialog} from '../components/website-editor-dialog';
import {
  buildCertificateLabel,
  getCertificateStatus,
  getErrorMessage,
  getMatchTypeMeta,
  isRouteRelatedToManagedDomain,
} from '../components/website-utils';

const domainsQueryKey = ['openflare', 'managed-domains'];
const certificatesQueryKey = ['openflare', 'tls-certificates'];

export function WebsiteDetailPageClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const websiteId = searchParams.get('id')?.trim() ?? '';

  const [editorOpen, setEditorOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [certEditorOpen, setCertEditorOpen] = useState(false);
  const [deleteWebsiteOpen, setDeleteWebsiteOpen] = useState(false);
  const [deleteCertOpen, setDeleteCertOpen] = useState(false);
  const [preferredCertificateId, setPreferredCertificateId] = useState<number | null>(null);
  const [convertCertificate, setConvertCertificate] = useState<TlsCertificateItem | null>(null);

  const domainsQuery = useQuery({
    queryKey: domainsQueryKey,
    queryFn: () => WebsiteService.list(),
  });

  const certificatesQuery = useQuery({
    queryKey: certificatesQueryKey,
    queryFn: () => TlsCertificateService.list(),
  });

  const routesQuery = useQuery({
    queryKey: ['openflare', 'proxy-routes'],
    queryFn: () => ProxyRouteService.list(),
  });

  const website = useMemo(
    () =>
      (domainsQuery.data ?? []).find((item) => String(item.id) === websiteId) ?? null,
    [domainsQuery.data, websiteId],
  );

  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );
  const certificateMap = useMemo(
    () => new Map(certificates.map((item) => [item.id, item])),
    [certificates],
  );

  const certificate = website?.cert_id
    ? (certificateMap.get(website.cert_id) ?? null)
    : null;

  const relatedRoutes = useMemo(() => {
    if (!website) return [];
    return (routesQuery.data ?? []).filter((route) =>
      isRouteRelatedToManagedDomain(website.domain, route),
    );
  }, [routesQuery.data, website]);

  const deleteDomainMutation = useMutation({
    mutationFn: (id: number) => WebsiteService.deleteById(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({queryKey: domainsQueryKey});
      router.push('/websites');
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const deleteCertificateMutation = useMutation({
    mutationFn: (id: number) => TlsCertificateService.deleteById(id),
    onSuccess: async () => {
      toast.success('证书已删除');
      setDeleteCertOpen(false);
      await Promise.all([
        queryClient.invalidateQueries({queryKey: certificatesQueryKey}),
        queryClient.invalidateQueries({queryKey: domainsQueryKey}),
      ]);
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  if (!websiteId) {
    return (
      <div className="py-6 px-1">
        <EmptyStateWithBorder
          icon={Globe}
          description="缺少网站 ID，请从网站列表进入详情页。"
        />
      </div>
    );
  }

  if (
    domainsQuery.isLoading ||
    certificatesQuery.isLoading ||
    routesQuery.isLoading
  ) {
    return (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={Globe} description="加载网站详情中..." />
      </div>
    );
  }

  if (domainsQuery.isError) {
    return (
      <div className="py-6 px-1">
        <ErrorInline
          message={getErrorMessage(domainsQuery.error)}
          onRetry={() => void domainsQuery.refetch()}
          className="justify-center"
        />
      </div>
    );
  }

  if (!website) {
    return (
      <div className="py-6 px-1 space-y-4">
        <Button variant="ghost" size="sm" className="h-8 px-2" asChild>
          <Link href="/websites">
            <ArrowLeft className="size-4 mr-1" />
            返回网站列表
          </Link>
        </Button>
        <EmptyStateWithBorder
          icon={Globe}
          description="网站不存在，可能已被删除或 ID 无效。"
        />
      </div>
    );
  }

  const matchType = getMatchTypeMeta(website.domain);
  const certificateStatus = certificate ? getCertificateStatus(certificate) : null;
  const enabledRoutesCount = relatedRoutes.filter((route) => route.enabled).length;

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <Globe className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{website.domain}</h1>
            <p className="text-sm text-muted-foreground">网站详情</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" className="h-7 text-xs" asChild>
            <Link href="/websites">
              <ArrowLeft className="size-3.5 mr-1" />
              返回
            </Link>
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            onClick={() => setEditorOpen(true)}
          >
            编辑网站
          </Button>
          <Button size="sm" className="h-7 text-xs" onClick={() => setImportOpen(true)}>
            <Plus className="size-3.5 mr-1" />
            添加证书
          </Button>
          <Button
            variant="destructive"
            size="sm"
            className="h-7 text-xs"
            onClick={() => setDeleteWebsiteOpen(true)}
          >
            <Trash2 className="size-3.5 mr-1" />
            删除网站
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card className="shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">匹配类型</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <WebsiteStatusBadge label={matchType.label} tone={matchType.tone} />
            <p className="text-xs text-muted-foreground">
              {website.domain.startsWith('*.')
                ? '当前网站会覆盖该后缀下的子域名规则。'
                : '当前网站只匹配同名精确域名。'}
            </p>
          </CardContent>
        </Card>

        <Card className="shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">网站状态</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <WebsiteStatusBadge
              label={website.enabled ? '启用' : '停用'}
              tone={website.enabled ? 'success' : 'warning'}
            />
            <p className="text-xs text-muted-foreground">
              更新时间：{formatDateTime(website.updated_at)}
            </p>
          </CardContent>
        </Card>

        <Card className="shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">关联规则</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-xs text-muted-foreground">
            <p>规则总数：{relatedRoutes.length}</p>
            <p>已启用规则：{enabledRoutesCount}</p>
          </CardContent>
        </Card>

        <Card className="shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">绑定证书</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <WebsiteStatusBadge
              label={certificate ? certificate.name : '未绑定证书'}
              tone={certificate ? 'success' : 'warning'}
            />
            <p className="text-xs text-muted-foreground">
              {certificateStatus
                ? `证书状态：${certificateStatus.label}`
                : '当前网站未设置默认证书。'}
            </p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card className="shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">网站信息</CardTitle>
            <CardDescription>当前网站的基础配置与托管信息。</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 md:grid-cols-2">
            <div className="rounded-lg border p-3">
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground">域名</p>
              <p className="mt-1 text-sm">{website.domain}</p>
            </div>
            <div className="rounded-lg border p-3">
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
                创建时间
              </p>
              <p className="mt-1 text-sm">{formatDateTime(website.created_at)}</p>
            </div>
            <div className="rounded-lg border p-3 md:col-span-2">
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground">备注</p>
              <p className="mt-1 text-sm">{website.remark || '暂无备注'}</p>
            </div>
          </CardContent>
        </Card>

        <Card className="shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">证书信息</CardTitle>
            <CardDescription>
              当前网站绑定的默认证书信息。若还没有证书，可直接添加后回填。
            </CardDescription>
          </CardHeader>
          <CardContent>
            {certificate ? (
              <div className="space-y-3">
                {certificateStatus ? (
                  <WebsiteStatusBadge
                    label={certificateStatus.label}
                    tone={certificateStatus.tone}
                  />
                ) : null}
                <div className="space-y-1 text-xs text-muted-foreground">
                  <p>证书名称：{buildCertificateLabel(certificate)}</p>
                  <p>生效时间：{formatDateTime(certificate.not_before)}</p>
                  <p>到期时间：{formatDateTime(certificate.not_after)}</p>
                  <p>备注：{certificate.remark || '暂无备注'}</p>
                </div>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={() => setDetailOpen(true)}
                  >
                    查看
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs text-destructive"
                    onClick={() => setDeleteCertOpen(true)}
                  >
                    删除
                  </Button>
                </div>
              </div>
            ) : (
              <EmptyStateWithBorder description="未绑定证书，点击右上角「添加证书」后可在编辑网站时直接选择。" />
            )}
          </CardContent>
        </Card>
      </div>

      <Card className="shadow-none">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm">关联规则</CardTitle>
          <CardDescription>命中当前网站域名的代理规则。</CardDescription>
        </CardHeader>
        <CardContent>
          {relatedRoutes.length === 0 ? (
            <EmptyStateWithBorder description="暂无关联规则。" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>规则域名</TableHead>
                  <TableHead>源站</TableHead>
                  <TableHead>HTTPS</TableHead>
                  <TableHead>证书</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>备注</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {relatedRoutes.map((route) => {
                  const routeCertificate = route.cert_id
                    ? (certificateMap.get(route.cert_id) ?? null)
                    : null;
                  const matchedDomain =
                    route.domains.find((d) => d === website.domain) ?? route.primary_domain;

                  return (
                    <TableRow key={route.id}>
                      <TableCell>
                        <div className="space-y-1">
                          <p className="text-sm">{matchedDomain}</p>
                          <WebsiteStatusBadge
                            label={
                              matchedDomain === website.domain
                                ? '直接关联'
                                : '被网站覆盖'
                            }
                            tone={matchedDomain === website.domain ? 'info' : 'warning'}
                          />
                        </div>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground max-w-56 break-all">
                        {getUpstreamSummary(route)}
                      </TableCell>
                      <TableCell>
                        <WebsiteStatusBadge
                          label={route.enable_https ? '启用 HTTPS' : 'HTTP'}
                          tone={route.enable_https ? 'success' : 'warning'}
                        />
                      </TableCell>
                      <TableCell className="text-xs">
                        {routeCertificate ? routeCertificate.name : '未绑定证书'}
                      </TableCell>
                      <TableCell>
                        <WebsiteStatusBadge
                          label={route.enabled ? '启用' : '停用'}
                          tone={route.enabled ? 'success' : 'warning'}
                        />
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {route.remark || '暂无备注'}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <WebsiteEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        initialDomain={website}
        certificates={certificates}
        certificatesLoading={certificatesQuery.isLoading}
        preferredCertificateId={preferredCertificateId}
        onRequestImportCertificate={() => setImportOpen(true)}
        onSaved={() => toast.success('网站已更新')}
      />

      <CertificateImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImported={(imported) => {
          setPreferredCertificateId(imported.id);
          toast.success(`证书 ${imported.name} 已导入，可在编辑网站时直接应用`);
          setEditorOpen(true);
        }}
      />

      <CertificateDetailDialog
        certificateId={certificate?.id ?? null}
        open={detailOpen}
        onOpenChange={setDetailOpen}
        onEdit={() => {
          setDetailOpen(false);
          setCertEditorOpen(true);
        }}
        onDelete={() => {
          setDetailOpen(false);
          setDeleteCertOpen(true);
        }}
        deleting={deleteCertificateMutation.isPending}
      />

      <CertificateEditorDialog
        certificateId={certificate?.id ?? null}
        open={certEditorOpen}
        onOpenChange={setCertEditorOpen}
        onSaved={(updated) => toast.success(`证书 ${updated.name} 已更新`)}
        onConvert={(manualCertificate) => {
          setCertEditorOpen(false);
          setConvertCertificate(manualCertificate);
        }}
      />

      {convertCertificate ? (
        <CertificateApplyDialog
          open
          onOpenChange={(open) => !open && setConvertCertificate(null)}
          mode="convert-upload"
          certificate={convertCertificate}
          onApplied={(converted) => {
            setConvertCertificate(null);
            toast.success(`证书 ${converted.name} 转换申请已提交`);
          }}
        />
      ) : null}

      <AlertDialog open={deleteWebsiteOpen} onOpenChange={setDeleteWebsiteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除网站</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除网站 {website.domain} 吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteDomainMutation.mutate(website.id)}
            >
              删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteCertOpen} onOpenChange={setDeleteCertOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除证书</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除证书 {certificate?.name} 吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => certificate && deleteCertificateMutation.mutate(certificate.id)}
            >
              删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
