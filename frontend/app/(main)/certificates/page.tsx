'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { FileKey, Plus, RefreshCw, Trash2 } from 'lucide-react';
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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import type { TlsCertificateItem } from '@/lib/services/openflare';
import { TlsCertificateService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { CertificateApplyDialog } from '../websites/components/certificate-apply-dialog';
import { CertificateDetailDialog } from '../websites/components/certificate-detail-dialog';
import { CertificateEditorDialog } from '../websites/components/certificate-editor-dialog';
import { CertificateImportDialog } from '../websites/components/certificate-import-dialog';
import { WebsiteStatusBadge } from '../websites/components/status-badge';
import { useTranslations } from 'next-intl';

import {
  getCertificateStatus,
  getErrorMessage,
} from '../websites/components/website-utils';

const certificatesQueryKey = ['openflare', 'tls-certificates'];

type CertificateApplyMode = 'edit-acme' | 'convert-upload';

export default function CertificatesPage() {
  const t = useTranslations('certificates');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [importOpen, setImportOpen] = useState(false);
  const [applyOpen, setApplyOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<TlsCertificateItem | null>(
    null,
  );
  const [selectedCertificateId, setSelectedCertificateId] = useState<
    number | null
  >(null);
  const [applyCertificate, setApplyCertificate] =
    useState<TlsCertificateItem | null>(null);
  const [applyMode, setApplyMode] = useState<CertificateApplyMode>('edit-acme');

  const certificatesQuery = useQuery({
    queryKey: certificatesQueryKey,
    queryFn: () => TlsCertificateService.list(),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => TlsCertificateService.deleteById(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: certificatesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const renewMutation = useMutation({
    mutationFn: (id: number) => TlsCertificateService.renew(id),
    onSuccess: async (cert) => {
      toast.success(t('renewSubmitted', { name: cert.name }));
      await queryClient.invalidateQueries({ queryKey: certificatesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
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
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-2'>
          <FileKey className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() =>
              void queryClient.invalidateQueries({
                queryKey: certificatesQueryKey,
              })
            }
          >
            <RefreshCw className='size-3.5 mr-1' />
            {t('refresh')}
          </Button>
          <Button
            variant='secondary'
            size='sm'
            className='h-7 text-xs'
            onClick={() => setImportOpen(true)}
          >
            {t('importCert')}
          </Button>
          <Button
            size='sm'
            className='h-7 text-xs'
            onClick={() => setApplyOpen(true)}
          >
            <Plus className='size-3.5 mr-1' />
            {t('applyTitle')}
          </Button>
        </div>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('listTitle')}
          </CardTitle>
          <CardDescription>{t('listDesc')}</CardDescription>
        </CardHeader>
        <CardContent>
          {certificatesQuery.isLoading ? (
            <LoadingStateWithBorder
              icon={FileKey}
              description={t('loadingList')}
            />
          ) : certificatesQuery.isError ? (
            <div className='p-8 border border-dashed rounded-lg'>
              <ErrorInline
                message={getErrorMessage(
                  certificatesQuery.error,
                  t('requestFailed'),
                )}
                onRetry={() => void certificatesQuery.refetch()}
                className='justify-center'
              />
            </div>
          ) : certificates.length === 0 ? (
            <EmptyStateWithBorder icon={FileKey} description={t('emptyList')} />
          ) : (
            <div className='space-y-3'>
              {certificates.map((certificate) => {
                const status = getCertificateStatus(certificate, t);

                return (
                  <div
                    key={certificate.id}
                    className='rounded-lg border bg-card px-4 py-3'
                  >
                    <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
                      <div className='space-y-2'>
                        <div className='flex flex-wrap items-center gap-2'>
                          <p className='text-sm font-semibold'>
                            {certificate.name}
                          </p>
                          <WebsiteStatusBadge
                            label={status.label}
                            tone={status.tone}
                          />
                        </div>
                        <div className='text-xs leading-5 text-muted-foreground space-y-0.5'>
                          <p>
                            {t('notBeforeLabel', {
                              value: formatDateTime(certificate.not_before),
                            })}
                          </p>
                          <p>
                            {t('notAfterLabel', {
                              value: formatDateTime(certificate.not_after),
                            })}
                          </p>
                          <p>
                            {t('sourceLabel', {
                              value:
                                certificate.provider === 'acme'
                                  ? t('sourceAcme')
                                  : t('sourceUpload'),
                            })}
                          </p>
                          {certificate.apply_status === 'applying' ? (
                            <p className='text-blue-600'>
                              {t('statusLabel', {
                                value:
                                  certificate.provider === 'upload'
                                    ? t('converting')
                                    : t('applying'),
                              })}
                            </p>
                          ) : null}
                          {certificate.apply_status === 'error' ? (
                            <p className='text-destructive'>
                              {t('statusError', {
                                value:
                                  certificate.provider === 'upload'
                                    ? t('convertFailed')
                                    : t('applyFailed'),
                                message: certificate.apply_message,
                              })}
                            </p>
                          ) : null}
                          <p>
                            {t('remarkLabel', {
                              value: certificate.remark || t('noRemark'),
                            })}
                          </p>
                        </div>
                      </div>

                      <div className='flex flex-wrap gap-1'>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() => {
                            setSelectedCertificateId(certificate.id);
                            setDetailOpen(true);
                          }}
                        >
                          {t('view')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() => handleOpenEditor(certificate)}
                        >
                          {t('edit')}
                        </Button>
                        {certificate.provider === 'acme' ? (
                          <Button
                            variant='outline'
                            size='sm'
                            className='h-7 text-xs'
                            disabled={renewMutation.isPending}
                            onClick={() => renewMutation.mutate(certificate.id)}
                          >
                            {t('renew')}
                          </Button>
                        ) : null}
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs text-destructive'
                          onClick={() => setDeleteTarget(certificate)}
                        >
                          <Trash2 className='size-3' />
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
          toast.success(t('imported', { name: certificate.name }))
        }
      />

      <CertificateApplyDialog
        open={applyOpen && !applyCertificate}
        onOpenChange={setApplyOpen}
        onApplied={(certificate) =>
          toast.success(t('applySubmitted', { name: certificate.name }))
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
                ? t('convertSubmitted', { name: certificate.name })
                : t('reapplySubmitted', { name: certificate.name }),
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
        onSaved={(certificate) =>
          toast.success(t('updated', { name: certificate.name }))
        }
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
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {tc('delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
