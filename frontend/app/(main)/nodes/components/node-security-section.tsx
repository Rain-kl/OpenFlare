'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  KeyRound,
  RefreshCw,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
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
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type {
  NodeCertificateDelivery,
  NodeItem,
  NodeQuarantineResult,
} from '@/lib/services/openflare';
import { NodeService, TlsCertificateService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { getErrorMessage } from './node-utils';

interface NodeSecuritySectionProps {
  node: NodeItem;
  onNodeUpdated?: () => void;
}

export function NodeSecuritySection({
  node,
  onNodeUpdated,
}: NodeSecuritySectionProps) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [unquarantineOpen, setUnquarantineOpen] = useState(false);
  const [rotateTarget, setRotateTarget] =
    useState<NodeCertificateDelivery | null>(null);

  const deliveriesQuery = useQuery({
    queryKey: ['openflare', 'nodes', node.id, 'certificate-deliveries'],
    queryFn: () => NodeService.getCertificateDeliveries(node.id),
    enabled: node.id > 0,
  });

  const unquarantineMutation = useMutation({
    mutationFn: (nodeId: number) => NodeService.unquarantine(nodeId),
    onSuccess: async () => {
      toast.success(t('security.unquarantineSuccess'));
      setUnquarantineOpen(false);
      await queryClient.invalidateQueries({ queryKey: ['openflare', 'nodes'] });
      onNodeUpdated?.();
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const rotateMutation = useMutation({
    mutationFn: (certificateId: number) =>
      TlsCertificateService.rotateRevoke(certificateId),
    onSuccess: async () => {
      const targetName =
        rotateTarget?.certificate_name ||
        rotateTarget?.primary_domain ||
        String(rotateTarget?.certificate_id ?? '');
      toast.success(t('security.rotateDispatched', { name: targetName }));
      setRotateTarget(null);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'nodes', node.id, 'certificate-deliveries'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'tls-certificates'],
        }),
      ]);
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const deliveries = deliveriesQuery.data ?? [];

  const renderProviderBadge = (provider: string) => {
    if (provider === 'acme') {
      return <Badge variant='outline'>{t('security.providerAcme')}</Badge>;
    }
    if (provider === 'deleted') {
      return <Badge variant='secondary'>{t('security.providerDeleted')}</Badge>;
    }
    return <Badge variant='outline'>{t('security.providerUpload')}</Badge>;
  };

  return (
    <div className='space-y-4'>
      {node.status === 'quarantined' && (
        <Alert variant='destructive' className='border-dashed'>
          <ShieldAlert className='size-4' />
          <AlertTitle>{t('security.quarantinedBannerTitle')}</AlertTitle>
          <AlertDescription className='space-y-2'>
            <p className='text-xs'>
              {t('security.quarantinedBannerMeta', {
                time: node.quarantined_at
                  ? formatDateTime(node.quarantined_at)
                  : t('na'),
                reason: node.quarantine_reason || t('security.noReason'),
              })}
            </p>
            <p>{t('security.quarantinedBannerDesc')}</p>
            <div className='pt-1'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => setUnquarantineOpen(true)}
                disabled={unquarantineMutation.isPending}
              >
                <ShieldCheck className='mr-1.5 size-4' />
                {t('security.unquarantineAction')}
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
          <div className='space-y-1'>
            <CardTitle className='flex items-center gap-2 text-base'>
              <KeyRound className='size-4 text-primary' />
              {t('security.deliveriesTitle')}
            </CardTitle>
            <CardDescription>{t('security.deliveriesDesc')}</CardDescription>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void deliveriesQuery.refetch()}
            disabled={deliveriesQuery.isFetching}
          >
            <RefreshCw
              className={`mr-1.5 size-3.5 ${
                deliveriesQuery.isFetching ? 'animate-spin' : ''
              }`}
            />
            {t('refresh')}
          </Button>
        </CardHeader>
        <CardContent>
          {deliveriesQuery.isLoading ? (
            <LoadingStateWithBorder description={t('loadingDetail')} />
          ) : deliveriesQuery.isError ? (
            <ErrorInline
              message={getErrorMessage(
                deliveriesQuery.error,
                t('requestFailed'),
              )}
              onRetry={() => void deliveriesQuery.refetch()}
            />
          ) : deliveries.length === 0 ? (
            <div className='rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground'>
              {t('security.deliveriesEmpty')}
            </div>
          ) : (
            <div className='overflow-x-auto rounded-md border border-dashed'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('security.colCertificate')}</TableHead>
                    <TableHead>{t('security.colProvider')}</TableHead>
                    <TableHead>{t('security.colFingerprint')}</TableHead>
                    <TableHead>{t('security.colExposure')}</TableHead>
                    <TableHead>{t('security.colDeliveries')}</TableHead>
                    <TableHead>{t('security.colLastDelivered')}</TableHead>
                    <TableHead className='text-right'>
                      {t('security.colActions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {deliveries.map((item) => {
                    const isApplying = item.apply_status === 'applying';
                    const canRotate =
                      item.provider === 'acme' && item.matches_current;
                    return (
                      <TableRow
                        key={`${item.certificate_id}-${item.cert_sha256}`}
                      >
                        <TableCell>
                          <div className='font-medium'>
                            {item.certificate_name || `#${item.certificate_id}`}
                          </div>
                          {item.primary_domain ? (
                            <div className='text-xs text-muted-foreground'>
                              {item.primary_domain}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell>
                          {renderProviderBadge(item.provider)}
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {item.cert_sha256
                            ? item.cert_sha256.slice(0, 12)
                            : t('na')}
                        </TableCell>
                        <TableCell>
                          {item.matches_current ? (
                            <Badge variant='destructive'>
                              {t('security.exposureCurrent')}
                            </Badge>
                          ) : (
                            <Badge variant='secondary'>
                              {t('security.exposureRotated')}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell>{item.delivery_count}</TableCell>
                        <TableCell className='whitespace-nowrap text-xs text-muted-foreground'>
                          {formatDateTime(item.last_delivered_at)}
                        </TableCell>
                        <TableCell className='text-right'>
                          {canRotate ? (
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              disabled={isApplying || rotateMutation.isPending}
                              onClick={() => setRotateTarget(item)}
                            >
                              <RotateCcw className='mr-1.5 size-3.5' />
                              {isApplying
                                ? t('security.rotating')
                                : t('security.rotateRevoke')}
                            </Button>
                          ) : (
                            <span className='text-xs text-muted-foreground'>
                              —
                            </span>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <AlertDialog open={unquarantineOpen} onOpenChange={setUnquarantineOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('security.unquarantineTitle')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t('security.unquarantineDesc', { name: node.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => unquarantineMutation.mutate(node.id)}
              disabled={unquarantineMutation.isPending}
            >
              {t('security.unquarantineConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={rotateTarget !== null}
        onOpenChange={(open) => {
          if (!open) setRotateTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('security.rotateTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('security.rotateDesc', {
                name:
                  rotateTarget?.certificate_name ||
                  rotateTarget?.primary_domain ||
                  '',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (rotateTarget) {
                  rotateMutation.mutate(rotateTarget.certificate_id);
                }
              }}
              disabled={rotateMutation.isPending}
            >
              {t('security.rotateConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

interface NodeQuarantineDialogProps {
  node: NodeItem;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onQuarantined?: (result: NodeQuarantineResult) => void;
}

export function NodeQuarantineDialog({
  node,
  open,
  onOpenChange,
  onQuarantined,
}: NodeQuarantineDialogProps) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [reason, setReason] = useState('');
  const [rotateAcmeCerts, setRotateAcmeCerts] = useState(true);
  const [result, setResult] = useState<NodeQuarantineResult | null>(null);

  const quarantineMutation = useMutation({
    mutationFn: (payload: { reason: string; rotate_acme_certs: boolean }) =>
      NodeService.quarantine(node.id, payload),
    onSuccess: async (data) => {
      setResult(data);
      toast.success(t('security.quarantineSuccess', { name: node.name }));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['openflare', 'nodes'] }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'nodes', node.id, 'certificate-deliveries'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'tls-certificates'],
        }),
      ]);
      onQuarantined?.(data);
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && !quarantineMutation.isPending) {
      setResult(null);
      setReason('');
      setRotateAcmeCerts(true);
    }
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <AlertTriangle className='size-4 text-destructive' />
            {result
              ? t('security.resultTitle')
              : t('security.dialogTitle', { name: node.name })}
          </DialogTitle>
          <DialogDescription>
            {result
              ? t('security.quarantinedBannerDesc')
              : t('security.dialogDesc')}
          </DialogDescription>
        </DialogHeader>

        {result ? (
          <div className='space-y-3 rounded-md border border-dashed p-4 text-sm'>
            <div>
              <div className='font-medium'>
                {t('security.resultDnsSwitched')}
              </div>
              <div className='text-xs text-muted-foreground'>
                {result.dns_switched_groups.length > 0
                  ? result.dns_switched_groups.join(', ')
                  : t('security.none')}
              </div>
            </div>
            <div>
              <div className='font-medium'>
                {t('security.resultDnsUnbacked')}
              </div>
              <div className='text-xs text-muted-foreground'>
                {result.dns_unbacked_groups.length > 0
                  ? result.dns_unbacked_groups.join(', ')
                  : t('security.none')}
              </div>
            </div>
            <div>
              <div className='font-medium'>{t('security.resultRotated')}</div>
              <div className='text-xs text-muted-foreground'>
                {result.rotated_certificate_ids.length > 0
                  ? result.rotated_certificate_ids
                      .map((id) => `#${id}`)
                      .join(', ')
                  : t('security.none')}
              </div>
            </div>
            {result.manual_action_certificate_ids.length > 0 && (
              <div>
                <div className='font-medium text-destructive'>
                  {t('security.resultManual')}
                </div>
                <div className='text-xs text-muted-foreground'>
                  {result.manual_action_certificate_ids
                    .map((id) => `#${id}`)
                    .join(', ')}
                </div>
              </div>
            )}
            {Object.keys(result.rotation_errors).length > 0 && (
              <div>
                <div className='font-medium text-destructive'>
                  {t('security.resultErrors')}
                </div>
                <div className='space-y-1 text-xs text-muted-foreground'>
                  {Object.entries(result.rotation_errors).map(([id, msg]) => (
                    <div key={id}>
                      #{id}: {msg}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='quarantine-reason'>
                {t('security.reasonLabel')}
              </Label>
              <Input
                id='quarantine-reason'
                maxLength={255}
                value={reason}
                onChange={(event) => setReason(event.target.value)}
                placeholder={t('security.reasonPlaceholder')}
              />
            </div>
            <div className='flex items-start gap-2.5 rounded-md border border-dashed p-3'>
              <Checkbox
                id='quarantine-rotate-acme'
                checked={rotateAcmeCerts}
                onCheckedChange={(checked) =>
                  setRotateAcmeCerts(checked === true)
                }
                className='mt-0.5'
              />
              <div className='space-y-1'>
                <Label
                  htmlFor='quarantine-rotate-acme'
                  className='cursor-pointer font-medium'
                >
                  {t('security.rotateAcmeLabel')}
                </Label>
                <p className='text-xs text-muted-foreground'>
                  {t('security.rotateAcmeHint')}
                </p>
              </div>
            </div>
          </div>
        )}

        <DialogFooter>
          {result ? (
            <Button type='button' onClick={() => handleOpenChange(false)}>
              {t('security.close')}
            </Button>
          ) : (
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => handleOpenChange(false)}
                disabled={quarantineMutation.isPending}
              >
                {tc('cancel')}
              </Button>
              <Button
                type='button'
                variant='destructive'
                disabled={quarantineMutation.isPending}
                onClick={() =>
                  quarantineMutation.mutate({
                    reason: reason.trim(),
                    rotate_acme_certs: rotateAcmeCerts,
                  })
                }
              >
                {quarantineMutation.isPending
                  ? t('security.quarantining')
                  : t('security.quarantineConfirm')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
