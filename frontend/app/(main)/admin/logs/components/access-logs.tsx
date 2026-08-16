/*
Copyright 2026 Arctel.net

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { format } from 'date-fns';
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Copy,
  Eye,
  RotateCcw,
  Search,
  XCircle,
} from 'lucide-react';

import services from '@/lib/services';
import { useTranslations } from 'next-intl';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

interface AccessLog {
  id: string;
  user_id: string;
  username: string;
  nickname: string;
  path: string;
  method: string;
  ip: string;
  user_agent: string;
  headers: string;
  status: number;
  latency: number;
  created_at: string;
}

const PAGE_SIZE = 15;

function formatDateTime(value?: string | null) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return format(date, 'yyyy-MM-dd HH:mm:ss');
}

function formatLatency(ms: number) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function statusVariant(status: number) {
  if (status >= 500) return 'destructive';
  if (status >= 400) return 'destructive';
  if (status >= 300) return 'outline';
  return 'secondary';
}

function isClickhouseDisabledError(error: Error) {
  const errMsg = error.message || '';
  return errMsg.includes('ClickHouse') || errMsg.includes('未启用');
}

export function AccessLogs() {
  const t = useTranslations('admin.logs.accessLogs');
  const [page, setPage] = useState(1);

  // 搜索过滤条件
  const [usernameFilter, setUsernameFilter] = useState('');
  const [pathFilter, setPathFilter] = useState('');
  const [startTimeFilter, setStartTimeFilter] = useState('');
  const [endTimeFilter, setEndTimeFilter] = useState('');

  // 实际提交的搜索条件
  const [searchParams, setSearchParams] = useState({
    username: '',
    path: '',
    start_time: '',
    end_time: '',
  });

  const [selectedLog, setSelectedLog] = useState<AccessLog | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const logsQuery = useQuery({
    queryKey: ['admin', 'access-logs', page, searchParams],
    queryFn: async () => {
      let startISO = '';
      if (searchParams.start_time) {
        startISO = new Date(searchParams.start_time).toISOString();
      }
      let endISO = '';
      if (searchParams.end_time) {
        endISO = new Date(searchParams.end_time).toISOString();
      }

      return services.adminLog.getAccessLogs({
        page,
        page_size: PAGE_SIZE,
        username: searchParams.username || undefined,
        path: searchParams.path || undefined,
        start_time: startISO || undefined,
        end_time: endISO || undefined,
      });
    },
    retry: false,
  });

  const logs = logsQuery.data?.list ?? [];
  const total = logsQuery.data?.total ?? 0;
  const loading = logsQuery.isPending || logsQuery.isFetching;
  const clickhouseDisabled =
    logsQuery.isError && isClickhouseDisabledError(logsQuery.error);
  const error =
    logsQuery.isError && !clickhouseDisabled ? logsQuery.error : null;

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(total / PAGE_SIZE)),
    [total],
  );

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    setSearchParams({
      username: usernameFilter.trim(),
      path: pathFilter.trim(),
      start_time: startTimeFilter,
      end_time: endTimeFilter,
    });
  };

  const handleReset = () => {
    setUsernameFilter('');
    setPathFilter('');
    setStartTimeFilter('');
    setEndTimeFilter('');
    setPage(1);
    setSearchParams({
      username: '',
      path: '',
      start_time: '',
      end_time: '',
    });
  };

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
  };

  const copyToClipboard = (text: string, subject: string) => {
    navigator.clipboard.writeText(text);
    toast.success(t('copiedToClipboard', { subject }));
  };

  // Prettify Headers JSON string
  const getPrettyHeaders = (headersRaw?: string) => {
    if (!headersRaw) return t('noHeaderData');
    try {
      const parsed = JSON.parse(headersRaw);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return headersRaw;
    }
  };

  if (clickhouseDisabled) {
    return (
      <div className='flex flex-col items-center justify-center p-8 border border-dashed rounded-lg bg-card text-center my-6 min-h-[300px]'>
        <XCircle className='size-10 text-muted-foreground mb-3' />
        <h3 className='text-base font-semibold'>{t('clickhouseDisabled')}</h3>
        <p className='text-sm text-muted-foreground mt-1 max-w-[400px] mb-4'>
          {t('clickhouseDisabledDesc')}
        </p>
      </div>
    );
  }

  if (error) {
    return (
      <ErrorInline error={error} onRetry={() => void logsQuery.refetch()} />
    );
  }

  return (
    <div className='space-y-4'>
      {/* Filters */}
      <form
        onSubmit={handleSearch}
        className='grid gap-3 p-4 border rounded-lg bg-card/30 md:grid-cols-5 items-end'
      >
        <div className='grid gap-1.5'>
          <Label htmlFor='search-user' className='text-xs'>
            {t('username')}
          </Label>
          <Input
            id='search-user'
            placeholder={t('usernamePlaceholder')}
            value={usernameFilter}
            onChange={(e) => setUsernameFilter(e.target.value)}
            className='h-8 text-xs'
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='search-path' className='text-xs'>
            {t('apiPath')}
          </Label>
          <Input
            id='search-path'
            placeholder={t('pathPlaceholder')}
            value={pathFilter}
            onChange={(e) => setPathFilter(e.target.value)}
            className='h-8 text-xs'
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='search-start' className='text-xs'>
            {t('startTime')}
          </Label>
          <Input
            id='search-start'
            type='datetime-local'
            value={startTimeFilter}
            onChange={(e) => setStartTimeFilter(e.target.value)}
            className='h-8 text-xs'
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='search-end' className='text-xs'>
            {t('endTime')}
          </Label>
          <Input
            id='search-end'
            type='datetime-local'
            value={endTimeFilter}
            onChange={(e) => setEndTimeFilter(e.target.value)}
            className='h-8 text-xs'
          />
        </div>
        <div className='flex gap-2'>
          <Button type='submit' size='sm' className='h-8 flex-1'>
            <Search className='size-3.5 mr-1' />
            {t('search')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleReset}
            className='h-8 flex-1'
          >
            <RotateCcw className='size-3.5 mr-1' />
            {t('reset')}
          </Button>
        </div>
      </form>

      {/* Loading Table */}
      {loading && logs.length === 0 ? (
        <LoadingStateWithBorder
          icon={Activity}
          description={t('loadingAccessLogs')}
        />
      ) : logs.length === 0 ? (
        <EmptyStateWithBorder icon={Activity} description={t('noAccessLogs')} />
      ) : (
        <div className='rounded-lg border bg-card overflow-hidden'>
          <Table className='min-w-[900px]'>
            <TableHeader>
              <TableRow className='hover:bg-transparent'>
                <TableHead className='w-[100px]'>{t('colMethod')}</TableHead>
                <TableHead className='min-w-[200px]'>{t('colPath')}</TableHead>
                <TableHead className='w-[150px]'>{t('colUser')}</TableHead>
                <TableHead className='w-[120px]'>{t('colIp')}</TableHead>
                <TableHead className='w-[90px]'>{t('colStatus')}</TableHead>
                <TableHead className='w-[100px]'>{t('colLatency')}</TableHead>
                <TableHead className='w-[170px]'>
                  {t('colRequestTime')}
                </TableHead>
                <TableHead className='w-[80px] text-center'>
                  {t('colDetail')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((log) => (
                <TableRow key={log.id} className='hover:bg-muted/30'>
                  <TableCell>
                    <Badge
                      variant='outline'
                      className='font-mono font-semibold uppercase text-xs'
                    >
                      {log.method}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-mono text-xs break-all max-w-[280px]'>
                    {log.path}
                  </TableCell>
                  <TableCell>
                    <div className='flex flex-col'>
                      <span className='text-xs font-medium'>
                        {log.username || t('unknownUser')}
                      </span>
                      {log.nickname && (
                        <span className='text-[10px] text-muted-foreground'>
                          ({log.nickname})
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className='font-mono text-xs text-muted-foreground'>
                    {log.ip}
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(log.status)}>
                      {log.status}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-mono text-xs text-muted-foreground'>
                    {formatLatency(log.latency)}
                  </TableCell>
                  <TableCell className='font-mono text-[11px] text-muted-foreground'>
                    {formatDateTime(log.created_at)}
                  </TableCell>
                  <TableCell className='text-center'>
                    <Button
                      variant='ghost'
                      size='icon'
                      className='size-7 hover:bg-muted'
                      onClick={() => {
                        setSelectedLog(log);
                        setDetailOpen(true);
                      }}
                    >
                      <Eye className='size-3.5' />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Pagination */}
      {logs.length > 0 && (
        <div className='flex items-center justify-between py-1'>
          <div className='text-xs text-muted-foreground'>
            {t('paginationInfo', { total, page, totalPages })}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => handlePageChange(Math.max(1, page - 1))}
              disabled={page <= 1 || loading}
              className='h-8 text-xs'
            >
              <ChevronLeft className='size-3.5 mr-1' />
              {t('prevPage')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => handlePageChange(Math.min(totalPages, page + 1))}
              disabled={page >= totalPages || loading}
              className='h-8 text-xs'
            >
              {t('nextPage')}
              <ChevronRight className='size-3.5 ml-1' />
            </Button>
          </div>
        </div>
      )}

      {/* Detail Drawer */}
      <Sheet open={detailOpen} onOpenChange={setDetailOpen}>
        <SheetContent className='w-full p-0 sm:max-w-[640px] flex flex-col h-full bg-background border-l'>
          <SheetHeader className='border-b px-5 py-4 shrink-0'>
            <SheetTitle className='text-lg font-bold'>
              {t('accessLogDetail')}
            </SheetTitle>
            <SheetDescription className='text-xs'>
              {t('accessId', { id: selectedLog?.id ?? '' })}
            </SheetDescription>
          </SheetHeader>

          {selectedLog ? (
            <div className='flex-1 overflow-y-auto px-5 py-4 space-y-5'>
              {/* Summary Details */}
              <div className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
                <div className='rounded-lg border p-3 bg-muted/10'>
                  <div className='text-[10px] text-muted-foreground uppercase font-bold tracking-wider'>
                    {t('method')}
                  </div>
                  <div className='mt-1 font-mono text-sm font-bold uppercase'>
                    {selectedLog.method}
                  </div>
                </div>
                <div className='rounded-lg border p-3 bg-muted/10'>
                  <div className='text-[10px] text-muted-foreground uppercase font-bold tracking-wider'>
                    {t('responseStatus')}
                  </div>
                  <div className='mt-1'>
                    <Badge variant={statusVariant(selectedLog.status)}>
                      {selectedLog.status}
                    </Badge>
                  </div>
                </div>
                <div className='rounded-lg border p-3 bg-muted/10'>
                  <div className='text-[10px] text-muted-foreground uppercase font-bold tracking-wider'>
                    {t('latency')}
                  </div>
                  <div className='mt-1 font-mono text-sm'>
                    {formatLatency(selectedLog.latency)}
                  </div>
                </div>
                <div className='rounded-lg border p-3 bg-muted/10 col-span-2 sm:col-span-1'>
                  <div className='text-[10px] text-muted-foreground uppercase font-bold tracking-wider'>
                    {t('ipAddress')}
                  </div>
                  <div className='mt-1 font-mono text-sm'>{selectedLog.ip}</div>
                </div>
                <div className='rounded-lg border p-3 bg-muted/10 col-span-2'>
                  <div className='text-[10px] text-muted-foreground uppercase font-bold tracking-wider'>
                    {t('user')}
                  </div>
                  <div className='mt-1 text-sm font-medium'>
                    {selectedLog.username
                      ? `${selectedLog.username} (${selectedLog.nickname || t('noNickname')})`
                      : t('unknownGuest')}
                    <span className='block font-mono text-[10px] text-muted-foreground mt-0.5'>
                      ID: {selectedLog.user_id}
                    </span>
                  </div>
                </div>
              </div>

              {/* Path */}
              <div className='grid gap-2'>
                <Label className='text-xs font-bold text-muted-foreground'>
                  {t('requestPath')}
                </Label>
                <div className='rounded-md border bg-muted/30 px-3 py-2 font-mono text-xs break-all'>
                  {selectedLog.path}
                </div>
              </div>

              {/* Request Time */}
              <div className='grid gap-2'>
                <Label className='text-xs font-bold text-muted-foreground'>
                  {t('requestTime')}
                </Label>
                <div className='font-mono text-xs'>
                  {formatDateTime(selectedLog.created_at)}
                </div>
              </div>

              {/* User Agent */}
              <div className='grid gap-2'>
                <div className='flex items-center justify-between'>
                  <Label className='text-xs font-bold text-muted-foreground'>
                    User-Agent
                  </Label>
                  <Button
                    variant='ghost'
                    size='icon'
                    className='size-6 text-muted-foreground hover:text-foreground'
                    onClick={() =>
                      copyToClipboard(selectedLog.user_agent, 'User-Agent')
                    }
                    title={t('copyUserAgent')}
                  >
                    <Copy className='size-3' />
                  </Button>
                </div>
                <div className='rounded-md border bg-muted/20 px-3 py-2 text-xs font-mono break-all max-h-24 overflow-y-auto leading-relaxed'>
                  {selectedLog.user_agent}
                </div>
              </div>

              {/* Headers */}
              <div className='grid gap-2 flex-1'>
                <div className='flex items-center justify-between'>
                  <Label className='text-xs font-bold text-muted-foreground'>
                    Request Headers
                  </Label>
                  <Button
                    variant='ghost'
                    size='icon'
                    className='size-6 text-muted-foreground hover:text-foreground'
                    onClick={() =>
                      copyToClipboard(
                        getPrettyHeaders(selectedLog.headers),
                        'Headers',
                      )
                    }
                    title={t('copyHeaders')}
                  >
                    <Copy className='size-3' />
                  </Button>
                </div>
                <pre className='overflow-auto rounded-md border bg-[#0d1117] p-3 text-xs leading-relaxed text-gray-300 font-mono max-h-[300px]'>
                  {getPrettyHeaders(selectedLog.headers)}
                </pre>
              </div>
            </div>
          ) : (
            <div className='flex-grow flex items-center justify-center'>
              <EmptyStateWithBorder
                icon={Activity}
                description={t('noLogSelected')}
              />
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
