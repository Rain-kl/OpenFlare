'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { ClipboardList, Eye, RefreshCw, Search, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
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
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { type ApplyLogItem, ApplyLogService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { LogDetailSheet } from './components/log-detail-sheet';

const PAGE_SIZE_OPTIONS = [20, 50, 100];

function truncateHash(value: string) {
  if (!value) return '—';
  return value.length > 12 ? `${value.slice(0, 12)}...` : value;
}

function ResultBadge({ result }: { result: string }) {
  const t = useTranslations('applyLogs');
  if (result === 'success') {
    return (
      <Badge
        variant='outline'
        className='text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2'
      >
        <span className='size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0' />
        {t('success')}
      </Badge>
    );
  }
  if (result === 'warning') {
    return (
      <Badge
        variant='outline'
        className='text-[10px] bg-amber-500/10 border-amber-500/20 text-amber-600 rounded-full py-0 px-2'
      >
        <span className='size-1 bg-amber-500 rounded-full mr-1.5 shrink-0' />
        {t('warning')}
      </Badge>
    );
  }
  return (
    <Badge
      variant='outline'
      className='text-[10px] bg-destructive/10 border-destructive/20 text-destructive rounded-full py-0 px-2'
    >
      <span className='size-1 bg-destructive rounded-full mr-1.5 shrink-0' />
      {t('failed')}
    </Badge>
  );
}

export function ApplyLogsPageClient() {
  const t = useTranslations('applyLogs');
  const tc = useTranslations('common');
  const searchParams = useSearchParams();
  const initialNodeId = searchParams.get('node_id')?.trim() ?? '';

  const [nodeFilterInput, setNodeFilterInput] = useState(initialNodeId);
  const [nodeFilter, setNodeFilter] = useState(initialNodeId);
  const [pageNo, setPageNo] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const [rows, setRows] = useState<ApplyLogItem[]>([]);
  const [current, setCurrent] = useState(1);
  const [total, setTotal] = useState(0);
  const [totalPage, setTotalPage] = useState(0);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [selectedLog, setSelectedLog] = useState<ApplyLogItem | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [cleaning, setCleaning] = useState(false);

  const summary = useMemo(() => {
    const nodeIds = new Set(rows.map((item) => item.node_id));
    return [
      { label: t('totalRecords'), value: total },
      { label: t('currentPage'), value: current },
      { label: t('totalPages'), value: totalPage },
      { label: t('pageNodes'), value: nodeIds.size },
    ];
  }, [rows, total, current, totalPage, t]);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await ApplyLogService.list({
        node_id: nodeFilter || undefined,
        pageNo,
        pageSize,
      });
      setRows(data.rows);
      setCurrent(data.current);
      setTotal(data.total);
      setTotalPage(data.totalPage);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [nodeFilter, pageNo, pageSize, t]);

  useEffect(() => {
    void fetchLogs();
  }, [fetchLogs]);

  useEffect(() => {
    const nodeId = searchParams.get('node_id')?.trim();
    if (!nodeId) return;
    setNodeFilterInput(nodeId);
    setNodeFilter(nodeId);
    setPageNo(1);
  }, [searchParams]);

  const handleSearch = () => {
    setPageNo(1);
    setNodeFilter(nodeFilterInput.trim());
  };

  const handleResetFilters = () => {
    setNodeFilterInput('');
    setNodeFilter('');
    setPageNo(1);
  };

  const handleCleanupLogs = async () => {
    setCleaning(true);
    try {
      const result = await ApplyLogService.cleanup({ delete_all: true });
      toast.success(t('cleared'), {
        description: t('clearedDesc', { count: result.deleted_count }),
      });
      setCleanupOpen(false);
      setPageNo(1);
      await fetchLogs();
    } catch (err) {
      toast.error(t('clearFailed'), {
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
          <ClipboardList className='size-5 text-primary' />
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
            onClick={() => void fetchLogs()}
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
            disabled={loading || cleaning || total === 0}
          >
            <Trash2 className='size-3.5 mr-1' />
            {t('clear')}
          </Button>
          <Button variant='outline' size='sm' asChild>
            <Link href='/nodes'>{t('backToNodes')}</Link>
          </Button>
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {summary.map((item) => (
          <div
            key={item.label}
            className='rounded-lg border border-dashed px-4 py-3 bg-background'
          >
            <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
              {item.label}
            </p>
            <p className='mt-1 text-lg font-semibold'>{item.value}</p>
          </div>
        ))}
      </div>

      <div className='flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between'>
        <div className='grid flex-1 gap-3 md:grid-cols-[minmax(0,1fr)_160px]'>
          <div className='space-y-1.5'>
            <p className='text-xs font-medium text-muted-foreground'>
              {t('nodeId')}
            </p>
            <div className='relative'>
              <Search className='absolute left-2.5 top-2.5 size-3.5 text-muted-foreground' />
              <Input
                value={nodeFilterInput}
                onChange={(e) => setNodeFilterInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleSearch();
                }}
                placeholder={t('nodePlaceholder')}
                className='pl-8 h-9 text-xs'
              />
            </div>
          </div>
          <div className='space-y-1.5'>
            <p className='text-xs font-medium text-muted-foreground'>
              {t('pageSize')}
            </p>
            <Select
              value={String(pageSize)}
              onValueChange={(value) => {
                setPageSize(Number.parseInt(value, 10));
                setPageNo(1);
              }}
            >
              <SelectTrigger className='h-9 text-xs'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PAGE_SIZE_OPTIONS.map((option) => (
                  <SelectItem key={option} value={String(option)}>
                    {t('pageSizeOption', { count: option })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button size='sm' onClick={handleSearch}>
            {t('filter')}
          </Button>
          <Button variant='outline' size='sm' onClick={handleResetFilters}>
            {t('reset')}
          </Button>
        </div>
      </div>

      {error ? (
        <ErrorInline message={error} onRetry={() => void fetchLogs()} />
      ) : null}

      <div className='border border-dashed shadow-none rounded-lg overflow-hidden bg-background'>
        {loading ? (
          <LoadingStateWithBorder />
        ) : rows.length === 0 ? (
          <EmptyStateWithBorder
            title={t('emptyTitle')}
            description={t('emptyDesc')}
          />
        ) : (
          <Table>
            <TableHeader className='bg-muted/40'>
              <TableRow className='border-dashed hover:bg-transparent'>
                <TableHead className='text-xs font-semibold'>
                  {t('colNodeId')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colVersion')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colResult')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colChecksum')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colTime')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('colMessage')}
                </TableHead>
                <TableHead className='text-xs font-semibold text-right'>
                  {t('colActions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((log) => (
                <TableRow
                  key={log.id}
                  className='border-dashed hover:bg-muted/10 transition-colors align-top'
                >
                  <TableCell className='text-xs font-medium'>
                    {log.node_id}
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground'>
                    {log.version}
                  </TableCell>
                  <TableCell>
                    <ResultBadge result={log.result} />
                  </TableCell>
                  <TableCell
                    className='text-xs font-mono text-muted-foreground'
                    title={log.checksum}
                  >
                    {truncateHash(log.checksum)}
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground'>
                    {formatDateTime(log.created_at)}
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground max-w-56'>
                    <div className='line-clamp-2 break-words'>
                      {log.message || '—'}
                    </div>
                  </TableCell>
                  <TableCell className='text-right'>
                    <Button
                      variant='ghost'
                      size='sm'
                      className='h-7 text-xs'
                      onClick={() => {
                        setSelectedLog(log);
                        setDetailOpen(true);
                      }}
                    >
                      <Eye className='size-3 mr-1' />
                      {t('detail')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      {!loading && rows.length > 0 ? (
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-xs text-muted-foreground'>
            {t('pageInfo', {
              current,
              total: Math.max(totalPage, 1),
              count: total,
            })}
          </p>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={current <= 1}
              onClick={() => setPageNo((prev) => Math.max(1, prev - 1))}
            >
              {t('prev')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              disabled={totalPage === 0 || current >= totalPage}
              onClick={() =>
                setPageNo((prev) =>
                  totalPage > 0 ? Math.min(totalPage, prev + 1) : prev,
                )
              }
            >
              {t('next')}
            </Button>
          </div>
        </div>
      ) : null}

      <LogDetailSheet
        log={selectedLog}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />

      <AlertDialog open={cleanupOpen} onOpenChange={setCleanupOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('clearTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('clearDesc', { count: total })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cleaning}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              disabled={cleaning}
              onClick={(event) => {
                event.preventDefault();
                void handleCleanupLogs();
              }}
            >
              {cleaning ? t('clearing') : t('confirmClear')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
