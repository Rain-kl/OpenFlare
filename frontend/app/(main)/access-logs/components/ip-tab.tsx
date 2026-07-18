'use client';

import { useEffect, useMemo, useState } from 'react';
import { Eye } from 'lucide-react';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import type {
  AccessLogIPSummaryItem,
  AccessLogIPSummaryList,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { formatBytes, formatCompactNumber } from '@/lib/utils/metrics';

import {
  formatOverviewRangeHint,
  IP_SORT_OPTIONS,
  OVERVIEW_RANGE_OPTIONS,
  PAGE_SIZE_OPTIONS,
  type OverviewRangeHours,
} from './access-log-utils';
import { IpDetailDialog } from './ip-detail-dialog';

export function toLocalInputValue(date: Date) {
  const pad = (n: number) => `${n}`.padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** Default local range ending now, spanning `hours`. */
export function defaultLocalRangeForHours(hours: number) {
  const until = new Date();
  const since = new Date(until.getTime() - hours * 3_600_000);
  return {
    since: toLocalInputValue(since),
    until: toLocalInputValue(until),
  };
}

function localInputToRFC3339(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toISOString();
}

const MAX_CUSTOM_RANGE_MS = 30 * 24 * 3_600_000;

/** Validate custom local datetime-local range; returns error message or null. */
export function validateCustomTimeRange(
  sinceLocal: string,
  untilLocal: string,
): string | null {
  if (!sinceLocal.trim() || !untilLocal.trim()) {
    return '请填写开始与结束时间';
  }
  const sinceMs = new Date(sinceLocal).getTime();
  const untilMs = new Date(untilLocal).getTime();
  if (Number.isNaN(sinceMs) || Number.isNaN(untilMs)) {
    return '时间格式无效';
  }
  if (untilMs <= sinceMs) {
    return '结束时间必须晚于开始时间';
  }
  if (untilMs - sinceMs > MAX_CUSTOM_RANGE_MS) {
    return '时间范围不能超过 30 天';
  }
  return null;
}

function formatRatio(ratio: number) {
  if (!Number.isFinite(ratio) || ratio < 0) return '0%';
  return `${(ratio * 100).toFixed(1)}%`;
}

/** Exact hours for analysis API (1–720), matching list window duration. */
export function resolveAnalysisHours(input: {
  timeMode: IpTabTimeMode;
  hours: OverviewRangeHours;
  customSince: string;
  customUntil: string;
}): number {
  if (input.timeMode === 'custom' && input.customSince && input.customUntil) {
    const ms =
      new Date(input.customUntil).getTime() -
      new Date(input.customSince).getTime();
    if (Number.isFinite(ms) && ms > 0) {
      return Math.min(720, Math.max(1, Math.ceil(ms / 3_600_000)));
    }
  }
  return input.hours;
}

function PaginationBar({
  page,
  hasMore,
  loading,
  totalIp,
  onPrev,
  onNext,
}: {
  page: number;
  hasMore: boolean;
  loading: boolean;
  totalIp?: number;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className='flex items-center justify-between px-4 py-3 border-t border-dashed'>
      <p className='text-xs text-muted-foreground'>
        当前第 {page + 1} 页
        {typeof totalIp === 'number' ? ` · 共 ${totalIp} 个 IP` : ''}
      </p>
      <div className='flex gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={loading || page <= 0}
          onClick={onPrev}
        >
          上一页
        </Button>
        <Button
          variant='outline'
          size='sm'
          disabled={loading || !hasMore}
          onClick={onNext}
        >
          下一页
        </Button>
      </div>
    </div>
  );
}

export type IpTabTimeMode = 'preset' | 'custom';

export function IpTab({
  data,
  loading,
  error,
  page,
  pageSize,
  sort,
  hours,
  timeMode,
  customSince,
  customUntil,
  onHoursChange,
  onTimeModeChange,
  onApplyCustomRange,
  onPageSizeChange,
  onSortChange,
  onRetry,
  onPrevPage,
  onNextPage,
  isFetching,
}: {
  data?: AccessLogIPSummaryList;
  loading: boolean;
  error: Error | null;
  page: number;
  pageSize: number;
  sort: string;
  hours: OverviewRangeHours;
  timeMode: IpTabTimeMode;
  /** Applied custom range (local datetime-local strings). */
  customSince: string;
  customUntil: string;
  onHoursChange: (hours: OverviewRangeHours) => void;
  onTimeModeChange: (mode: IpTabTimeMode) => void;
  /** Commit validated custom range for list query. */
  onApplyCustomRange: (since: string, until: string) => void;
  onPageSizeChange: (size: number) => void;
  onSortChange: (value: string) => void;
  onRetry: () => void;
  onPrevPage: () => void;
  onNextPage: () => void;
  isFetching: boolean;
}) {
  const [selected, setSelected] = useState<AccessLogIPSummaryItem | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [draftSince, setDraftSince] = useState(customSince);
  const [draftUntil, setDraftUntil] = useState(customUntil);
  const [customError, setCustomError] = useState<string | null>(null);

  const defaultCustomRange = useMemo(
    () => defaultLocalRangeForHours(hours),
    [hours],
  );

  useEffect(() => {
    if (timeMode !== 'custom') {
      setCustomError(null);
      return;
    }
    // Sync draft from applied range when entering custom or after apply.
    setDraftSince(customSince || defaultCustomRange.since);
    setDraftUntil(customUntil || defaultCustomRange.until);
    setCustomError(null);
  }, [timeMode, customSince, customUntil, defaultCustomRange]);

  const analysisHours = resolveAnalysisHours({
    timeMode,
    hours,
    customSince,
    customUntil,
  });

  const rangeHint =
    timeMode === 'custom' && customSince && customUntil
      ? '自定义区间'
      : timeMode === 'custom'
        ? '自定义（未应用）'
        : formatOverviewRangeHint(hours);

  const handleApplyCustom = () => {
    const message = validateCustomTimeRange(draftSince, draftUntil);
    if (message) {
      setCustomError(message);
      return;
    }
    setCustomError(null);
    onApplyCustomRange(draftSince, draftUntil);
  };

  return (
    <>
      <div className='space-y-4'>
        <div className='rounded-lg border border-dashed bg-background p-4 space-y-3'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='space-y-1'>
              <p className='text-sm font-medium'>时间范围</p>
              <p className='text-xs text-muted-foreground'>
                当前：{rangeHint}
                {data?.total_ip != null
                  ? ` · ${formatCompactNumber(data.total_ip)} 个 IP`
                  : ''}
              </p>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <ToggleGroup
                type='single'
                value={timeMode === 'preset' ? String(hours) : ''}
                onValueChange={(value) => {
                  if (!value) return;
                  onTimeModeChange('preset');
                  onHoursChange(
                    Number.parseInt(value, 10) as OverviewRangeHours,
                  );
                }}
                variant='outline'
                size='sm'
              >
                {OVERVIEW_RANGE_OPTIONS.map((option) => (
                  <ToggleGroupItem
                    key={option.value}
                    value={String(option.value)}
                    className='px-2.5 text-xs'
                  >
                    {option.label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
              <Button
                size='sm'
                variant={timeMode === 'custom' ? 'default' : 'outline'}
                className='h-8 text-xs'
                onClick={() => onTimeModeChange('custom')}
              >
                自定义
              </Button>
            </div>
          </div>

          {timeMode === 'custom' ? (
            <div className='space-y-2'>
              <div className='flex flex-wrap items-end gap-3'>
                <div className='space-y-1'>
                  <p className='text-xs text-muted-foreground'>开始</p>
                  <Input
                    type='datetime-local'
                    className='h-9 w-52 text-xs'
                    value={draftSince || defaultCustomRange.since}
                    onChange={(e) => {
                      setDraftSince(e.target.value);
                      setCustomError(null);
                    }}
                  />
                </div>
                <div className='space-y-1'>
                  <p className='text-xs text-muted-foreground'>结束</p>
                  <Input
                    type='datetime-local'
                    className='h-9 w-52 text-xs'
                    value={draftUntil || defaultCustomRange.until}
                    onChange={(e) => {
                      setDraftUntil(e.target.value);
                      setCustomError(null);
                    }}
                  />
                </div>
                <Button
                  size='sm'
                  className='h-9 text-xs'
                  onClick={handleApplyCustom}
                >
                  应用
                </Button>
              </div>
              {customError ? (
                <p className='text-xs text-destructive'>{customError}</p>
              ) : (
                <p className='text-xs text-muted-foreground'>
                  修改时间后点击「应用」再查询；详情分析窗口与列表时长对齐。
                </p>
              )}
            </div>
          ) : null}

          <div className='flex flex-wrap items-center gap-3'>
            <div className='space-y-1'>
              <p className='text-xs text-muted-foreground'>排序</p>
              <Select value={sort} onValueChange={onSortChange}>
                <SelectTrigger className='h-8 w-48 text-xs'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {IP_SORT_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1'>
              <p className='text-xs text-muted-foreground'>每页</p>
              <Select
                value={String(pageSize)}
                onValueChange={(value) =>
                  onPageSizeChange(Number.parseInt(value, 10))
                }
              >
                <SelectTrigger className='h-8 w-24 text-xs'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PAGE_SIZE_OPTIONS.map((size) => (
                    <SelectItem key={size} value={String(size)}>
                      {size}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>

        <div className='rounded-lg border border-dashed overflow-hidden bg-background'>
          <div className='flex items-center justify-between px-4 py-3 border-b border-dashed'>
            <p className='text-sm font-medium'>IP 明细</p>
          </div>
          {error ? (
            <div className='p-4'>
              <ErrorInline
                message={error.message || '加载失败'}
                onRetry={onRetry}
              />
            </div>
          ) : loading ? (
            <LoadingStateWithBorder />
          ) : (data?.items ?? []).length === 0 ? (
            <EmptyStateWithBorder title='暂无 IP 数据' />
          ) : (
            <Table>
              <TableHeader className='bg-muted/40'>
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableHead className='text-xs'>IP</TableHead>
                  <TableHead className='text-xs'>地区</TableHead>
                  <TableHead className='text-xs text-right'>请求数</TableHead>
                  <TableHead className='text-xs text-right'>2xx 比例</TableHead>
                  <TableHead className='text-xs text-right'>入站</TableHead>
                  <TableHead className='text-xs text-right'>出站</TableHead>
                  <TableHead className='text-xs'>最后访问</TableHead>
                  <TableHead className='w-12 text-center text-xs' />
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.items ?? []).map((item) => (
                  <TableRow key={item.remote_addr} className='border-dashed'>
                    <TableCell className='font-mono text-xs'>
                      {item.remote_addr}
                    </TableCell>
                    <TableCell className='text-xs text-muted-foreground'>
                      {item.region || '—'}
                    </TableCell>
                    <TableCell className='text-xs text-right tabular-nums'>
                      {formatCompactNumber(item.total_requests)}
                    </TableCell>
                    <TableCell className='text-xs text-right tabular-nums'>
                      <span
                        title={`${item.success_2xx_count}/${item.total_requests}`}
                      >
                        {formatRatio(item.success_ratio)}
                      </span>
                    </TableCell>
                    <TableCell className='text-xs text-right tabular-nums'>
                      {formatBytes(item.bytes_received)}
                    </TableCell>
                    <TableCell className='text-xs text-right tabular-nums'>
                      {formatBytes(item.bytes_sent)}
                    </TableCell>
                    <TableCell className='text-xs'>
                      {formatDateTime(item.last_seen_at)}
                    </TableCell>
                    <TableCell className='text-center'>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            variant='ghost'
                            size='icon'
                            className='h-6 w-6 text-muted-foreground hover:text-foreground'
                            onClick={() => {
                              setSelected(item);
                              setDetailOpen(true);
                            }}
                          >
                            <Eye className='size-3' />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side='top' className='text-xs'>
                          查看 IP 详情
                        </TooltipContent>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          <PaginationBar
            page={page}
            hasMore={data?.has_more ?? false}
            loading={isFetching}
            totalIp={data?.total_ip}
            onPrev={onPrevPage}
            onNext={onNextPage}
          />
        </div>
      </div>

      <IpDetailDialog
        open={detailOpen}
        remoteAddr={selected?.remote_addr ?? null}
        region={selected?.region}
        initialHours={analysisHours}
        onOpenChange={setDetailOpen}
      />
    </>
  );
}

/** Build list API time params from IP tab state (applied values only). */
export function buildIpSummaryTimeParams(input: {
  timeMode: IpTabTimeMode;
  hours: OverviewRangeHours;
  customSince: string;
  customUntil: string;
}): { hours?: number; since?: string; until?: string } | null {
  if (input.timeMode === 'custom') {
    if (validateCustomTimeRange(input.customSince, input.customUntil)) {
      return null;
    }
    const since = localInputToRFC3339(input.customSince);
    const until = localInputToRFC3339(input.customUntil);
    if (!since || !until) {
      return null;
    }
    return { since, until };
  }
  return { hours: input.hours };
}
