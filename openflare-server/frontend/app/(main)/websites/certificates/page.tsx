'use client';

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useMemo, useState} from 'react';
import {FileKey, Plus, RefreshCw, Trash2} from 'lucide-react';
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
import type {TlsCertificateItem} from '@/lib/services/openflare';
import {TlsCertificateService} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';

import {CertificateApplyDialog} from '../components/certificate-apply-dialog';
import {CertificateDetailDialog} from '../components/certificate-detail-dialog';
import {CertificateEditorDialog} from '../components/certificate-editor-dialog';
import {CertificateImportDialog} from '../components/certificate-import-dialog';
import {WebsiteStatusBadge} from '../components/status-badge';
import {getCertificateStatus, getErrorMessage} from '../components/website-utils';

const certificatesQueryKey = ['openflare', 'tls-certificates'];

type CertificateApplyMode = 'edit-acme' | 'convert-upload';

export default function CertificatesPage() {
  const queryClient = useQueryClient();
  const [importOpen, setImportOpen] = useState(false);
  const [applyOpen, setApplyOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<TlsCertificateItem | null>(null);
  const [selectedCertificateId, setSelectedCertificateId] = useState<number | null>(null);
  const [applyCertificate, setApplyCertificate] = useState<TlsCertificateItem | null>(null);
  const [applyMode, setApplyMode] = useState<CertificateApplyMode>('edit-acme');

  const certificatesQuery = useQuery({
    queryKey: certificatesQueryKey,
    queryFn: () => TlsCertificateService.list(),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => TlsCertificateService.deleteById(id),
    onSuccess: async () => {
      toast.success('证书已删除');
      setDeleteTarget(null);
      await queryClient.invalidateQueries({queryKey: certificatesQueryKey});
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const renewMutation = useMutation({
    mutationFn: (id: number) => TlsCertificateService.renew(id),
    onSuccess: async (cert) => {
      toast.success(`证书 ${cert.name} 续期任务已提交`);
      await queryClient.invalidateQueries({queryKey: certificatesQueryKey});
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );

  const handleOpenEditor = (certificate: TlsCertificateItem) => {
    if (certificate.provider === 'acme') {
      setApplyMode('edit-acme');
      setApplyCertificate(certificate);
      setApplyOpen(true);
    } else {
      setSelectedCertificateId(certificate.id);
      setEditorOpen(true);
    }
  };

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <FileKey className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">证书</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            onClick={() => void queryClient.invalidateQueries({queryKey: certificatesQueryKey})}
          >
            <RefreshCw className="size-3.5 mr-1" />
            刷新证书
          </Button>
          <Button
            variant="secondary"
            size="sm"
            className="h-7 text-xs"
            onClick={() => setImportOpen(true)}
          >
            导入证书
          </Button>
          <Button size="sm" className="h-7 text-xs" onClick={() => setApplyOpen(true)}>
            <Plus className="size-3.5 mr-1" />
            申请证书
          </Button>
        </div>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-semibold">证书列表</CardTitle>
          <CardDescription>
            展示证书有效期、备注和状态，支持查看详情、编辑内容或删除证书。
          </CardDescription>
        </CardHeader>
        <CardContent>
          {certificatesQuery.isLoading ? (
            <LoadingStateWithBorder icon={FileKey} description="加载证书列表中..." />
          ) : certificatesQuery.isError ? (
            <div className="p-8 border border-dashed rounded-lg">
              <ErrorInline
                message={getErrorMessage(certificatesQuery.error)}
                onRetry={() => void certificatesQuery.refetch()}
                className="justify-center"
              />
            </div>
          ) : certificates.length === 0 ? (
            <EmptyStateWithBorder
              icon={FileKey}
              description="暂无证书，点击右上角「导入证书」或「申请证书」开始录入。"
            />
          ) : (
            <div className="space-y-3">
              {certificates.map((certificate) => {
                const status = getCertificateStatus(certificate);

                return (
                  <div
                    key={certificate.id}
                    className="rounded-lg border bg-card px-4 py-3"
                  >
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                      <div className="space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="text-sm font-semibold">{certificate.name}</p>
                          <WebsiteStatusBadge label={status.label} tone={status.tone} />
                        </div>
                        <div className="text-xs leading-5 text-muted-foreground space-y-0.5">
                          <p>生效：{formatDateTime(certificate.not_before)}</p>
                          <p>到期：{formatDateTime(certificate.not_after)}</p>
                          <p>
                            来源：
                            {certificate.provider === 'acme' ? 'ACME 申请' : '手动上传'}
                          </p>
                          {certificate.apply_status === 'applying' ? (
                            <p className="text-blue-600">
                              状态：
                              {certificate.provider === 'upload'
                                ? '转换申请中...'
                                : '申请中...'}
                            </p>
                          ) : null}
                          {certificate.apply_status === 'error' ? (
                            <p className="text-destructive">
                              状态：
                              {certificate.provider === 'upload' ? '转换失败' : '申请失败'}
                              （{certificate.apply_message}）
                            </p>
                          ) : null}
                          <p>备注：{certificate.remark || '暂无备注'}</p>
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-1">
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => {
                            setSelectedCertificateId(certificate.id);
                            setDetailOpen(true);
                          }}
                        >
                          查看
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => handleOpenEditor(certificate)}
                        >
                          编辑
                        </Button>
                        {certificate.provider === 'acme' ? (
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-7 text-xs"
                            disabled={renewMutation.isPending}
                            onClick={() => renewMutation.mutate(certificate.id)}
                          >
                            续期
                          </Button>
                        ) : null}
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs text-destructive"
                          onClick={() => setDeleteTarget(certificate)}
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

      <CertificateImportDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImported={(certificate) =>
          toast.success(`证书 ${certificate.name} 已导入`)
        }
      />

      <CertificateApplyDialog
        open={applyOpen && !applyCertificate}
        onOpenChange={setApplyOpen}
        onApplied={(certificate) =>
          toast.success(`证书 ${certificate.name} 申请任务已提交`)
        }
      />

      {applyCertificate ? (
        <CertificateApplyDialog
          open={applyOpen}
          onOpenChange={(open) => {
            setApplyOpen(open);
            if (!open) setApplyCertificate(null);
          }}
          mode={applyMode}
          certificate={applyCertificate}
          onApplied={(certificate) => {
            setApplyCertificate(null);
            toast.success(
              applyMode === 'convert-upload'
                ? `证书 ${certificate.name} 转换申请已提交`
                : `证书 ${certificate.name} 配置已更新，重新申请中...`,
            );
          }}
        />
      ) : null}

      <CertificateDetailDialog
        certificateId={selectedCertificateId}
        open={detailOpen}
        onOpenChange={setDetailOpen}
        onEdit={() => {
          setDetailOpen(false);
          const item = certificates.find((c) => c.id === selectedCertificateId);
          if (item) handleOpenEditor(item);
        }}
        onDelete={() => {
          const item = certificates.find((c) => c.id === selectedCertificateId);
          if (item) {
            setDetailOpen(false);
            setDeleteTarget(item);
          }
        }}
        deleting={deleteMutation.isPending}
      />

      <CertificateEditorDialog
        certificateId={selectedCertificateId}
        open={editorOpen}
        onOpenChange={setEditorOpen}
        onSaved={(certificate) => toast.success(`证书 ${certificate.name} 已更新`)}
        onConvert={(certificate) => {
          setEditorOpen(false);
          setApplyMode('convert-upload');
          setApplyCertificate(certificate);
          setApplyOpen(true);
        }}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除证书</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除证书 {deleteTarget?.name} 吗？
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