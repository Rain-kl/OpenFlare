'use client';

import {useQuery} from '@tanstack/react-query';
import {Copy, Loader2} from 'lucide-react';
import {toast} from 'sonner';

import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import {TlsCertificateService} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';

import {WebsiteStatusBadge} from './status-badge';
import {getCertificateStatus, getErrorMessage} from './website-utils';

interface CertificateDetailDialogProps {
  certificateId: number | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onEdit: () => void;
  onDelete: () => void;
  deleting?: boolean;
}

export function CertificateDetailDialog({
  certificateId,
  open,
  onOpenChange,
  onEdit,
  onDelete,
  deleting = false,
}: CertificateDetailDialogProps) {
  const certificateQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates', 'detail', certificateId],
    queryFn: () => TlsCertificateService.getById(certificateId as number),
    enabled: open && certificateId !== null,
  });

  const contentQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates', 'content', certificateId],
    queryFn: () => TlsCertificateService.getContent(certificateId as number),
    enabled: open && certificateId !== null,
  });

  const certificate = certificateQuery.data;
  const content = contentQuery.data;
  const status = certificate ? getCertificateStatus(certificate) : null;
  const loading = certificateQuery.isLoading || contentQuery.isLoading;
  const hasError = certificateQuery.isError || contentQuery.isError;

  const handleCopy = async (value: string, message: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(message);
    } catch (error) {
      toast.error(getErrorMessage(error));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>证书详情</DialogTitle>
          <DialogDescription>
            查看证书元信息、备注以及当前保存的 PEM 内容。
          </DialogDescription>
        </DialogHeader>

        {loading ? (
          <LoadingStateWithBorder description="加载证书详情中..." />
        ) : hasError ? (
          <ErrorInline
            message={getErrorMessage(certificateQuery.error ?? contentQuery.error)}
            className="justify-center"
          />
        ) : !certificate || !content ? (
          <EmptyStateWithBorder description="证书不存在，可能已被删除。" />
        ) : (
          <div className="space-y-4">
            <div className="grid gap-3 md:grid-cols-2">
              <div className="rounded-lg border p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  证书名称
                </p>
                <p className="mt-1 text-sm">{certificate.name}</p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">状态</p>
                <div className="mt-1">
                  {status ? <WebsiteStatusBadge label={status.label} tone={status.tone} /> : null}
                </div>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  生效时间
                </p>
                <p className="mt-1 text-sm">{formatDateTime(certificate.not_before)}</p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  到期时间
                </p>
                <p className="mt-1 text-sm">{formatDateTime(certificate.not_after)}</p>
              </div>
            </div>

            <div className="rounded-lg border p-3">
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground">备注</p>
              <p className="mt-1 text-sm">{certificate.remark || '暂无备注'}</p>
            </div>

            <div className="space-y-3">
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <p className="text-sm font-medium">证书 PEM</p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={() => void handleCopy(content.cert_pem, '证书 PEM 已复制')}
                  >
                    <Copy className="mr-1 size-3" />
                    复制
                  </Button>
                </div>
                <pre className="max-h-48 overflow-auto rounded-lg border bg-muted/40 p-3 text-xs break-all whitespace-pre-wrap">
                  {content.cert_pem}
                </pre>
              </div>
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <p className="text-sm font-medium">私钥 PEM</p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={() => void handleCopy(content.key_pem, '私钥 PEM 已复制')}
                  >
                    <Copy className="mr-1 size-3" />
                    复制
                  </Button>
                </div>
                <pre className="max-h-48 overflow-auto rounded-lg border bg-muted/40 p-3 text-xs break-all whitespace-pre-wrap">
                  {content.key_pem}
                </pre>
              </div>
            </div>
          </div>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
          <Button type="button" onClick={onEdit} disabled={!certificate}>
            编辑证书
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={onDelete}
            disabled={!certificate || deleting}
          >
            {deleting ? (
              <>
                <Loader2 className="mr-1 size-3.5 animate-spin" />
                删除中...
              </>
            ) : (
              '删除证书'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}