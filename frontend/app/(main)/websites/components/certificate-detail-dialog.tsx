'use client';

import { useQuery } from '@tanstack/react-query';
import { Copy, Loader2 } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { TlsCertificateService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { useTranslations } from 'next-intl';

import { WebsiteStatusBadge } from './status-badge';
import { getCertificateStatus, getErrorMessage } from './website-utils';

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
  const t = useTranslations('certificates');
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
  const status = certificate ? getCertificateStatus(certificate, t) : null;
  const loading = certificateQuery.isLoading || contentQuery.isLoading;
  const hasError = certificateQuery.isError || contentQuery.isError;

  const handleCopy = async (value: string, message: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(message);
    } catch (error) {
      toast.error(getErrorMessage(error, t('requestFailed')));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-3xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{t('detailTitle')}</DialogTitle>
          <DialogDescription>{t('detailDesc')}</DialogDescription>
        </DialogHeader>

        {loading ? (
          <LoadingStateWithBorder description={t('loadingDetail')} />
        ) : hasError ? (
          <ErrorInline
            message={getErrorMessage(
              certificateQuery.error ?? contentQuery.error,
              t('requestFailed'),
            )}
            className='justify-center'
          />
        ) : !certificate || !content ? (
          <EmptyStateWithBorder description={t('notFound')} />
        ) : (
          <div className='space-y-4'>
            <div className='grid gap-3 md:grid-cols-2'>
              <div className='rounded-lg border p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('name')}
                </p>
                <p className='mt-1 text-sm'>{certificate.name}</p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('status')}
                </p>
                <div className='mt-1'>
                  {status ? (
                    <WebsiteStatusBadge
                      label={status.label}
                      tone={status.tone}
                    />
                  ) : null}
                </div>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('notBefore')}
                </p>
                <p className='mt-1 text-sm'>
                  {formatDateTime(certificate.not_before)}
                </p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                  {t('notAfter')}
                </p>
                <p className='mt-1 text-sm'>
                  {formatDateTime(certificate.not_after)}
                </p>
              </div>
            </div>

            <div className='rounded-lg border p-3'>
              <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
                {t('remark')}
              </p>
              <p className='mt-1 text-sm'>
                {certificate.remark || t('noRemark')}
              </p>
            </div>

            <div className='space-y-3'>
              <div>
                <div className='mb-2 flex items-center justify-between'>
                  <p className='text-sm font-medium'>{t('certPem')}</p>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='h-7 text-xs'
                    onClick={() =>
                      void handleCopy(content.cert_pem, t('certPemCopied'))
                    }
                  >
                    <Copy className='mr-1 size-3' />
                    {t('copy')}
                  </Button>
                </div>
                <pre className='max-h-48 overflow-auto rounded-lg border bg-muted/40 p-3 text-xs break-all whitespace-pre-wrap'>
                  {content.cert_pem}
                </pre>
              </div>
              <div>
                <div className='mb-2 flex items-center justify-between'>
                  <p className='text-sm font-medium'>{t('keyPem')}</p>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='h-7 text-xs'
                    onClick={() =>
                      void handleCopy(content.key_pem, t('keyPemCopied'))
                    }
                  >
                    <Copy className='mr-1 size-3' />
                    {t('copy')}
                  </Button>
                </div>
                <pre className='max-h-48 overflow-auto rounded-lg border bg-muted/40 p-3 text-xs break-all whitespace-pre-wrap'>
                  {content.key_pem}
                </pre>
              </div>
            </div>
          </div>
        )}

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('close')}
          </Button>
          <Button type='button' onClick={onEdit} disabled={!certificate}>
            {t('editCert')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            onClick={onDelete}
            disabled={!certificate || deleting}
          >
            {deleting ? (
              <>
                <Loader2 className='mr-1 size-3.5 animate-spin' />
                {t('deleting')}
              </>
            ) : (
              t('deleteCert')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
