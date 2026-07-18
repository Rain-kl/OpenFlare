'use client';

import { useState } from 'react';
import { Eye } from 'lucide-react';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import type { AccessLogItem, AccessLogList } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { AccessLogDetailDialog } from './access-log-detail-dialog';
import {
  cacheOutcomeLabel,
  DETAIL_SORT_OPTIONS,
  resolveCacheOutcome,
  type CacheOutcome,
} from './access-log-utils';

function cacheOutcomeVariant(
  outcome: CacheOutcome,
): 'default' | 'secondary' | 'outline' | 'destructive' {
  switch (outcome) {
    case 'hit':
      return 'default';
    case 'origin':
      return 'secondary';
    default:
      return 'outline';
  }
}

function PaginationBar({
  page,
  hasMore,
  loading,
  onPrev,
  onNext,
}: {
  page: number;
  hasMore: boolean;
  loading: boolean;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className='flex items-center justify-between px-4 py-3 border-t border-dashed'>
      <p className='text-xs text-muted-foreground'>当前第 {page + 1} 页</p>
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

export function DetailTab({
  data,
  loading,
  error,
  page,
  detailSort,
  onDetailSortChange,
  onRetry,
  onPrevPage,
  onNextPage,
  isFetching,
}: {
  data?: AccessLogList;
  loading: boolean;
  error: Error | null;
  page: number;
  detailSort: string;
  onDetailSortChange: (value: string) => void;
  onRetry: () => void;
  onPrevPage: () => void;
  onNextPage: () => void;
  isFetching: boolean;
}) {
  const [selected, setSelected] = useState<AccessLogItem | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  return (
    <>
      <div className='rounded-lg border border-dashed overflow-hidden bg-background'>
        <div className='flex items-center justify-between px-4 py-3 border-b border-dashed'>
          <p className='text-sm font-medium'>日志明细</p>
          <Select value={detailSort} onValueChange={onDetailSortChange}>
            <SelectTrigger className='h-8 w-44 text-xs'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {DETAIL_SORT_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
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
          <EmptyStateWithBorder title='暂无访问日志' />
        ) : (
          <Table>
            <TableHeader className='bg-muted/40'>
              <TableRow className='border-dashed hover:bg-transparent'>
                <TableHead className='text-xs'>时间</TableHead>
                <TableHead className='text-xs'>节点</TableHead>
                <TableHead className='text-xs'>IP</TableHead>
                <TableHead className='text-xs'>域名</TableHead>
                <TableHead className='text-xs'>路径</TableHead>
                <TableHead className='text-xs'>缓存</TableHead>
                <TableHead className='text-xs'>状态码</TableHead>
                <TableHead className='w-12 text-center text-xs' />
              </TableRow>
            </TableHeader>
            <TableBody>
              {(data?.items ?? []).map((item) => {
                const outcome = resolveCacheOutcome(item.cache_status);
                return (
                  <TableRow key={item.id} className='border-dashed'>
                    <TableCell className='text-xs'>
                      {formatDateTime(item.logged_at)}
                    </TableCell>
                    <TableCell className='text-xs'>
                      {item.node_name || item.node_id}
                    </TableCell>
                    <TableCell className='text-xs font-mono'>
                      {item.remote_addr}
                    </TableCell>
                    <TableCell className='text-xs'>{item.host}</TableCell>
                    <TableCell className='text-xs max-w-48 truncate'>
                      {item.path}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={cacheOutcomeVariant(outcome)}
                        className='text-[10px]'
                        title={item.cache_status || undefined}
                      >
                        {cacheOutcomeLabel(outcome)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline' className='text-[10px]'>
                        {item.status_code}
                      </Badge>
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
                          查看详情
                        </TooltipContent>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        )}
        <PaginationBar
          page={page}
          hasMore={data?.has_more ?? false}
          loading={isFetching}
          onPrev={onPrevPage}
          onNext={onNextPage}
        />
      </div>

      <AccessLogDetailDialog
        open={detailOpen}
        item={selected}
        onOpenChange={setDetailOpen}
      />
    </>
  );
}
