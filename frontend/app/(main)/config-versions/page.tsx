'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Eye,
  GitCompare,
  History,
  Loader2,
  Play,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  AlertDialog,
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  type ConfigDiffResult,
  type ConfigPreviewResult,
  ConfigVersionService,
  type ConfigVersionSummary,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { CleanupDialog } from './components/cleanup-dialog';
import { DiffDialog } from './components/diff-dialog';
import { PreviewSheet } from './components/preview-sheet';
import { VersionSnapshotSheet } from './components/version-snapshot-sheet';

function truncateChecksum(checksum: string) {
  if (!checksum) return '—';
  return checksum.length > 16 ? `${checksum.slice(0, 16)}...` : checksum;
}

function hasConfigDiff(diff: ConfigDiffResult) {
  return (
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.added_sites.length > 0 ||
    diff.removed_sites.length > 0 ||
    diff.modified_sites.length > 0 ||
    diff.main_config_changed ||
    diff.waf_config_changed ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  );
}

export default function ConfigVersionsPage() {
  const t = useTranslations('configVersions');
  const tc = useTranslations('common');
  const [versions, setVersions] = useState<ConfigVersionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [previewOpen, setPreviewOpen] = useState(false);
  const [preview, setPreview] = useState<ConfigPreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const [diffOpen, setDiffOpen] = useState(false);
  const [diff, setDiff] = useState<ConfigDiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffError, setDiffError] = useState<string | null>(null);

  const [snapshotVersion, setSnapshotVersion] =
    useState<ConfigVersionSummary | null>(null);
  const [snapshotOpen, setSnapshotOpen] = useState(false);

  const [publishConfirmOpen, setPublishConfirmOpen] = useState(false);
  const [forcePublishConfirmOpen, setForcePublishConfirmOpen] = useState(false);
  const [activateTarget, setActivateTarget] =
    useState<ConfigVersionSummary | null>(null);
  const [cleanupOpen, setCleanupOpen] = useState(false);

  const [publishing, setPublishing] = useState(false);
  const [activating, setActivating] = useState(false);
  const [cleaning, setCleaning] = useState(false);

  const canPublish = useMemo(
    () =>
      Boolean(
        preview && preview.route_count > 0 && diff && hasConfigDiff(diff),
      ),
    [preview, diff],
  );

  const sortedVersions = useMemo(
    () =>
      [...versions].sort(
        (left, right) =>
          new Date(right.created_at).getTime() -
          new Date(left.created_at).getTime(),
      ),
    [versions],
  );

  const fetchVersions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await ConfigVersionService.list();
      setVersions(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void fetchVersions();
  }, [fetchVersions]);

  const loadPreviewData = async () => {
    setPreviewLoading(true);
    setPreviewError(null);
    setDiffLoading(true);
    setDiffError(null);

    try {
      const [previewData, diffData] = await Promise.all([
        ConfigVersionService.preview(),
        ConfigVersionService.diff(),
      ]);
      setPreview(previewData);
      setDiff(diffData);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : t('previewLoadFailed');
      setPreviewError(message);
      setDiffError(message);
    } finally {
      setPreviewLoading(false);
      setDiffLoading(false);
    }
  };

  const handleOpenPreview = async () => {
    setPreviewOpen(true);
    if (!preview) {
      await loadPreviewData();
    }
  };

  const handleOpenDiff = async () => {
    setDiffOpen(true);
    if (!diff) {
      setDiffLoading(true);
      setDiffError(null);
      try {
        const diffData = await ConfigVersionService.diff();
        setDiff(diffData);
      } catch (err) {
        setDiffError(err instanceof Error ? err.message : t('diffLoadFailed'));
      } finally {
        setDiffLoading(false);
      }
    }
  };

  const handlePublish = async (force = false) => {
    setPublishing(true);
    try {
      const version = await ConfigVersionService.publish(force);
      toast.success(t('publishSuccess'), {
        description: t('publishSuccessDesc', { version: version.version }),
      });
      setPreviewOpen(false);
      setPublishConfirmOpen(false);
      setForcePublishConfirmOpen(false);
      setPreview(null);
      setDiff(null);
      await fetchVersions();
    } catch (err) {
      toast.error(t('publishFailed'), {
        description: err instanceof Error ? err.message : t('unknownError'),
      });
    } finally {
      setPublishing(false);
    }
  };

  const handleActivate = async () => {
    if (!activateTarget) return;

    setActivating(true);
    try {
      const version = await ConfigVersionService.activate(activateTarget.id);
      toast.success(t('activateSuccess'), {
        description: t('activateSuccessDesc', { version: version.version }),
      });
      setActivateTarget(null);
      await fetchVersions();
    } catch (err) {
      toast.error(t('activateFailed'), {
        description: err instanceof Error ? err.message : t('unknownError'),
      });
    } finally {
      setActivating(false);
    }
  };

  const handleCleanup = async (keepCount: number) => {
    setCleaning(true);
    try {
      const result = await ConfigVersionService.cleanup({
        keep_count: keepCount,
      });
      toast.success(t('cleanupDone'), {
        description: t('cleanupDoneDesc', { count: result.deleted_count }),
      });
      setCleanupOpen(false);
      await fetchVersions();
    } catch (err) {
      toast.error(t('cleanupFailed'), {
        description: err instanceof Error ? err.message : t('unknownError'),
      });
    } finally {
      setCleaning(false);
    }
  };

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-2'>
          <History className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>
              {t('title')}
            </h1>
            <p className='text-sm text-muted-foreground'>{t('description')}</p>
          </div>
        </div>

        <div className='flex flex-wrap gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void fetchVersions()}
            disabled={loading}
          >
            <RefreshCw
              className={`size-3.5 mr-1 ${loading ? 'animate-spin' : ''}`}
            />
            {t('refresh')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setCleanupOpen(true)}
          >
            <Trash2 className='size-3.5 mr-1' />
            {t('cleanupOld')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void handleOpenDiff()}
          >
            <GitCompare className='size-3.5 mr-1' />
            {t('viewDiff')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setForcePublishConfirmOpen(true)}
          >
            {t('forcePublish')}
          </Button>
          <Button size='sm' onClick={() => void handleOpenPreview()}>
            <Eye className='size-3.5 mr-1' />
            {t('previewPublish')}
          </Button>
        </div>
      </div>

      {error ? (
        <ErrorInline message={error} onRetry={() => void fetchVersions()} />
      ) : null}

      <div className='border border-dashed shadow-none rounded-lg overflow-hidden bg-background'>
        {loading ? (
          <LoadingStateWithBorder />
        ) : sortedVersions.length === 0 ? (
          <EmptyStateWithBorder
            title={t('emptyTitle')}
            description={t('emptyDesc')}
          />
        ) : (
          <Table>
            <TableHeader className='bg-muted/40'>
              <TableRow className='border-dashed hover:bg-transparent'>
                <TableHead className='text-xs font-semibold'>
                  {t('colVersion')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colStatus')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colAuthor')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colChecksum')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colCreatedAt')}
                </TableHead>
                <TableHead className='text-xs font-semibold text-right'>
                  {t('colActions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedVersions.map((version) => (
                <TableRow
                  key={version.id}
                  className='border-dashed hover:bg-muted/10 transition-colors'
                >
                  <TableCell className='font-mono text-xs font-semibold'>
                    {version.version}
                  </TableCell>
                  <TableCell>
                    {version.is_active ? (
                      <Badge
                        variant='outline'
                        className='text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2'
                      >
                        <span className='size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0' />
                        {t('active')}
                      </Badge>
                    ) : (
                      <Badge
                        variant='outline'
                        className='text-[10px] rounded-full py-0 px-2'
                      >
                        {t('history')}
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground'>
                    {version.created_by || t('system')}
                  </TableCell>
                  <TableCell
                    className='text-xs font-mono text-muted-foreground'
                    title={version.checksum}
                  >
                    {truncateChecksum(version.checksum)}
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground'>
                    {formatDateTime(version.created_at)}
                  </TableCell>
                  <TableCell className='text-right'>
                    <div className='flex items-center justify-end gap-1.5'>
                      <Button
                        variant='ghost'
                        size='sm'
                        className='h-7 text-xs'
                        onClick={() => {
                          setSnapshotVersion(version);
                          setSnapshotOpen(true);
                        }}
                      >
                        <Eye className='size-3 mr-1' />
                        {t('snapshot')}
                      </Button>
                      {!version.is_active ? (
                        <Button
                          variant='ghost'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() => setActivateTarget(version)}
                        >
                          <Play className='size-3 mr-1' />
                          {t('activate')}
                        </Button>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <PreviewSheet
        open={previewOpen}
        onOpenChange={setPreviewOpen}
        preview={preview}
        loading={previewLoading}
        error={previewError}
        publishing={publishing}
        canPublish={canPublish}
        onPublish={() => setPublishConfirmOpen(true)}
      />

      <DiffDialog
        open={diffOpen}
        onOpenChange={setDiffOpen}
        diff={diff}
        loading={diffLoading}
        error={diffError}
      />

      <VersionSnapshotSheet
        version={snapshotVersion}
        open={snapshotOpen}
        onOpenChange={setSnapshotOpen}
      />

      <CleanupDialog
        open={cleanupOpen}
        onOpenChange={setCleanupOpen}
        onConfirm={(keepCount) => void handleCleanup(keepCount)}
        loading={cleaning}
      />

      <AlertDialog
        open={publishConfirmOpen}
        onOpenChange={setPublishConfirmOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('confirmPublishTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('confirmPublishDesc')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={publishing}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button
              onClick={() => void handlePublish(false)}
              disabled={publishing}
            >
              {publishing ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                t('confirmPublish')
              )}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={forcePublishConfirmOpen}
        onOpenChange={setForcePublishConfirmOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('confirmForceTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('confirmForceDesc')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={publishing}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              onClick={() => void handlePublish(true)}
              disabled={publishing}
            >
              {publishing ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                t('forcePublish')
              )}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={activateTarget !== null}
        onOpenChange={(open) => {
          if (!open) setActivateTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('confirmActivateTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {activateTarget
                ? t('confirmActivateDesc', { version: activateTarget.version })
                : ''}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={activating}>
              {tc('cancel')}
            </AlertDialogCancel>
            <Button onClick={() => void handleActivate()} disabled={activating}>
              {activating ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                t('confirmActivate')
              )}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
