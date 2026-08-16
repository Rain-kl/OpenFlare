// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import { useQuery } from '@tanstack/react-query';
import {
  ChevronLeft,
  ChevronRight,
  History,
  RefreshCw,
  Search,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { cn } from '@/lib/utils';

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

import type { PushHistory } from '@/lib/services/push';
import { PushService } from '@/lib/services/push';

function getLevelBadgeVariant(
  level: string,
): 'outline' | 'secondary' | 'destructive' | 'default' {
  switch (level) {
    case 'CRITICAL':
      return 'destructive';
    case 'IMPORTANT':
      return 'default';
    default:
      return 'outline';
  }
}

export function HistoriesTab() {
  const t = useTranslations('admin.push.histories');
  const [historyPage, setHistoryPage] = React.useState(1);
  const [historySearch, setHistorySearch] = React.useState('');
  const [historyStatus, setHistoryStatus] = React.useState('all');

  const [detailOpen, setDetailOpen] = React.useState(false);
  const [selectedHistory, setSelectedHistory] =
    React.useState<PushHistory | null>(null);

  const handleRowClick = (hist: PushHistory) => {
    setSelectedHistory(hist);
    setDetailOpen(true);
  };

  const historiesQuery = useQuery({
    queryKey: [
      'admin',
      'push-histories',
      historyPage,
      historySearch,
      historyStatus,
    ],
    queryFn: () =>
      PushService.listHistories({
        page: historyPage,
        page_size: 10,
        event_key: historySearch || undefined,
        status: historyStatus === 'all' ? undefined : historyStatus,
      }),
  });

  return (
    <div className='pt-4 space-y-4'>
      <div className='flex flex-col sm:flex-row gap-2'>
        <div className='relative flex-1'>
          <Search className='absolute left-2.5 top-2.5 size-4 text-muted-foreground' />
          <Input
            type='text'
            placeholder={t('searchPlaceholder')}
            className='pl-8 text-xs h-9'
            value={historySearch}
            onChange={(e) => {
              setHistorySearch(e.target.value);
              setHistoryPage(1);
            }}
          />
        </div>
        <div className='w-[150px]'>
          <Select
            value={historyStatus}
            onValueChange={(val) => {
              setHistoryStatus(val);
              setHistoryPage(1);
            }}
          >
            <SelectTrigger className='text-xs h-9'>
              <SelectValue placeholder={t('sendStatus')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all' className='text-xs'>
                {t('allStatus')}
              </SelectItem>
              <SelectItem value='success' className='text-xs'>
                {t('sendSuccess')}
              </SelectItem>
              <SelectItem value='failed' className='text-xs'>
                {t('sendFailed')}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button
          variant='outline'
          size='icon'
          className='h-9 w-9 shrink-0'
          onClick={() => historiesQuery.refetch()}
        >
          <RefreshCw
            className={cn(
              'size-3.5',
              historiesQuery.isFetching && 'animate-spin',
            )}
          />
        </Button>
      </div>

      <div className='border rounded-lg overflow-hidden'>
        <Table>
          <TableHeader>
            <TableRow className='bg-muted/30'>
              <TableHead className='text-xs font-semibold'>
                {t('colEvent')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {t('colChannel')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {t('colTarget')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {t('colTitle')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {t('colLevel')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {t('colStatus')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {t('colTime')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {historiesQuery.isLoading ? (
              <TableRow>
                <TableCell colSpan={7} className='h-32'>
                  <LoadingStateWithBorder
                    icon={History}
                    description={t('loadingHistories')}
                    className='border-0 shadow-none'
                  />
                </TableCell>
              </TableRow>
            ) : historiesQuery.isError ? (
              <TableRow>
                <TableCell colSpan={7} className='h-32'>
                  <ErrorInline
                    error={historiesQuery.error}
                    onRetry={() => historiesQuery.refetch()}
                    className='justify-center'
                  />
                </TableCell>
              </TableRow>
            ) : (historiesQuery.data?.results ?? []).length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='h-32 text-center text-xs text-muted-foreground'
                >
                  {t('noHistoryData')}
                </TableCell>
              </TableRow>
            ) : (
              (historiesQuery.data?.results ?? []).map((hist) => (
                <TableRow
                  key={hist.id}
                  className='hover:bg-muted/10 cursor-pointer transition-colors'
                  onClick={() => handleRowClick(hist)}
                >
                  <TableCell className='text-xs font-mono font-medium'>
                    {hist.event_key}
                  </TableCell>
                  <TableCell className='text-xs uppercase font-semibold text-muted-foreground'>
                    {hist.channel === 'email'
                      ? t('emailChannel')
                      : hist.channel}
                  </TableCell>
                  <TableCell
                    className='text-xs font-mono text-muted-foreground max-w-[120px] truncate'
                    title={hist.target}
                  >
                    {hist.target}
                  </TableCell>
                  <TableCell
                    className='text-xs max-w-[180px] truncate'
                    title={hist.content}
                  >
                    {hist.title}
                  </TableCell>
                  <TableCell className='text-xs'>
                    <Badge
                      variant={getLevelBadgeVariant(hist.level)}
                      className='text-[10px] font-semibold'
                    >
                      {hist.level}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-xs'>
                    <Badge
                      variant={
                        hist.status === 'success' ? 'secondary' : 'destructive'
                      }
                      className='text-[10px] font-semibold'
                    >
                      {hist.status === 'success' ? t('success') : t('failed')}
                    </Badge>
                    {hist.status !== 'success' && hist.error_msg && (
                      <div
                        className='text-[10px] text-muted-foreground font-mono truncate max-w-[180px] mt-1'
                        title={hist.error_msg}
                      >
                        {hist.error_msg}
                      </div>
                    )}
                  </TableCell>
                  <TableCell className='text-[11px] text-muted-foreground whitespace-nowrap'>
                    {new Date(hist.created_at).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {historiesQuery.data && historiesQuery.data.total > 0 && (
        <div className='flex justify-between items-center text-xs text-muted-foreground'>
          <span>{t('totalRecords', { total: historiesQuery.data.total })}</span>
          <div className='flex items-center gap-1.5'>
            <Button
              variant='outline'
              size='icon'
              disabled={historyPage === 1}
              onClick={() => setHistoryPage((p) => Math.max(1, p - 1))}
              className='h-8 w-8'
            >
              <ChevronLeft className='size-4' />
            </Button>
            <span className='text-xs font-medium px-2'>{historyPage}</span>
            <Button
              variant='outline'
              size='icon'
              disabled={historyPage * 10 >= historiesQuery.data.total}
              onClick={() => setHistoryPage((p) => p + 1)}
              className='h-8 w-8'
            >
              <ChevronRight className='size-4' />
            </Button>
          </div>
        </div>
      )}

      {/* ==================== 对话框：推送详情 ==================== */}
      <Dialog open={detailOpen} onOpenChange={setDetailOpen}>
        <DialogContent className='sm:max-w-[550px] max-h-[85vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <History className='size-5 text-primary' />
              {t('pushNotificationDetail')}
            </DialogTitle>
            <DialogDescription>
              {t('pushNotificationDetailDesc')}
            </DialogDescription>
          </DialogHeader>

          {selectedHistory && (
            <div className='space-y-4 py-4 text-xs'>
              <div className='grid grid-cols-2 gap-4'>
                <div className='space-y-1.5'>
                  <span className='font-semibold text-muted-foreground block'>
                    {t('eventIdentifier')}
                  </span>
                  <div className='font-mono bg-muted/40 p-2 rounded border'>
                    {selectedHistory.event_key}
                  </div>
                </div>
                <div className='space-y-1.5'>
                  <span className='font-semibold text-muted-foreground block'>
                    {t('sendChannel')}
                  </span>
                  <div className='bg-muted/40 p-2 rounded border uppercase font-medium'>
                    {selectedHistory.channel === 'email'
                      ? t('emailPush')
                      : selectedHistory.channel}
                  </div>
                </div>
              </div>

              <div className='grid grid-cols-2 gap-4'>
                <div className='space-y-1.5'>
                  <span className='font-semibold text-muted-foreground block'>
                    {t('pushTarget')}
                  </span>
                  <div
                    className='font-mono bg-muted/40 p-2 rounded border truncate'
                    title={selectedHistory.target}
                  >
                    {selectedHistory.target}
                  </div>
                </div>
                <div className='space-y-1.5'>
                  <span className='font-semibold text-muted-foreground block'>
                    {t('sendTime')}
                  </span>
                  <div className='bg-muted/40 p-2 rounded border'>
                    {new Date(selectedHistory.created_at).toLocaleString()}
                  </div>
                </div>
              </div>

              <div className='grid grid-cols-2 gap-4'>
                <div className='space-y-1.5'>
                  <span className='font-semibold text-muted-foreground block'>
                    {t('notificationLevel')}
                  </span>
                  <div>
                    <Badge
                      variant={getLevelBadgeVariant(selectedHistory.level)}
                      className='text-[10px] font-semibold py-0.5 px-2'
                    >
                      {selectedHistory.level}
                    </Badge>
                  </div>
                </div>
                <div className='space-y-1.5'>
                  <span className='font-semibold text-muted-foreground block'>
                    {t('sendStatus')}
                  </span>
                  <div>
                    <Badge
                      variant={
                        selectedHistory.status === 'success'
                          ? 'secondary'
                          : 'destructive'
                      }
                      className='text-[10px] font-semibold py-0.5 px-2'
                    >
                      {selectedHistory.status === 'success'
                        ? t('success')
                        : t('failed')}
                    </Badge>
                  </div>
                </div>
              </div>

              {selectedHistory.status === 'failed' &&
                selectedHistory.error_msg && (
                  <div className='space-y-1.5'>
                    <span className='font-semibold text-destructive block'>
                      {t('failureReason')}
                    </span>
                    <div className='font-mono text-destructive bg-destructive/10 p-2.5 rounded border border-destructive/20 whitespace-pre-wrap break-all'>
                      {selectedHistory.error_msg}
                    </div>
                  </div>
                )}

              <div className='space-y-1.5'>
                <span className='font-semibold text-muted-foreground block'>
                  {t('notificationTitle')}
                </span>
                <div className='bg-muted/30 p-2.5 rounded border font-medium text-[13px]'>
                  {selectedHistory.title}
                </div>
              </div>

              <div className='space-y-1.5'>
                <span className='font-semibold text-muted-foreground block'>
                  {t('notificationContent')}
                </span>
                <div className='bg-muted/30 p-3 rounded border whitespace-pre-wrap break-all leading-relaxed font-sans text-xs'>
                  {selectedHistory.content}
                </div>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setDetailOpen(false)}
              className='h-9 text-xs'
            >
              {t('close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
