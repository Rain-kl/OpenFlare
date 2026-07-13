'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { Eye, Globe, Plus, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { Input } from '@/components/ui/input';
import { LoadingStateWithBorder } from '@/components/layout/loading';
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
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { ZoneService, zoneQueryKey } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { ZoneEditorDialog } from './[zoneId]/components/zone-editor-dialog';
import { getErrorMessage } from './components/website-utils';

export default function WebsitesPage() {
  const router = useRouter();
  const [search, setSearch] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const zonesQuery = useQuery({
    queryKey: zoneQueryKey,
    queryFn: () => ZoneService.list(),
  });

  const zones = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    const list = zonesQuery.data ?? [];
    if (!keyword) {
      return list;
    }
    return list.filter((zone) => zone.domain.toLowerCase().includes(keyword));
  }, [search, zonesQuery.data]);

  return (
    <div className='space-y-4 py-6 px-1'>
      <div className='flex items-center justify-between gap-3 pb-2'>
        <div className='flex items-center gap-2'>
          <Globe className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>网站</h1>
        </div>
        <Button
          variant='secondary'
          size='sm'
          className='h-7 text-xs'
          onClick={() => setEditorOpen(true)}
        >
          <Plus className='mr-1 size-3.5' />
          新增 Zone
        </Button>
      </div>

      <div className='relative w-full sm:w-64'>
        <Search className='pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-muted-foreground' />
        <Input
          aria-label='搜索 Zone 根域'
          placeholder='搜索 Zone 根域'
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          className='h-8 pl-8 text-xs'
        />
      </div>

      {zonesQuery.isLoading ? (
        <LoadingStateWithBorder
          icon={Globe}
          description='加载 Zone 列表中...'
        />
      ) : zonesQuery.isError ? (
        <div className='rounded-lg border border-dashed p-8'>
          <ErrorInline
            className='justify-center'
            message={getErrorMessage(zonesQuery.error)}
            onRetry={() => void zonesQuery.refetch()}
          />
        </div>
      ) : zones.length === 0 ? (
        <EmptyStateWithBorder
          icon={Globe}
          description={
            search
              ? '未找到匹配的 Zone。'
              : '暂无 Zone，点击右上角「新增 Zone」开始录入。'
          }
        />
      ) : (
        <div className='overflow-hidden rounded-lg border border-dashed shadow-none'>
          <TooltipProvider delayDuration={0}>
            <Table className='w-full min-w-full caption-bottom text-sm'>
              <TableHeader className='sticky top-0 z-20 bg-background'>
                <TableRow className='border-b border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 w-[90px] whitespace-nowrap py-2'>
                    ID
                  </TableHead>
                  <TableHead className='h-8 min-w-[180px] whitespace-nowrap py-2'>
                    根域
                  </TableHead>
                  <TableHead className='h-8 min-w-[100px] whitespace-nowrap py-2'>
                    域名数
                  </TableHead>
                  <TableHead className='h-8 min-w-[140px] whitespace-nowrap py-2'>
                    创建时间
                  </TableHead>
                  <TableHead className='h-8 min-w-[140px] whitespace-nowrap py-2'>
                    更新时间
                  </TableHead>
                  <TableHead className='sticky right-0 z-10 h-8 w-[90px] bg-background py-2 text-center'>
                    操作
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {zones.map((zone) => (
                  <TableRow
                    key={zone.id}
                    className='group cursor-pointer border-dashed hover:bg-muted/30'
                    onClick={() => router.push(`/websites/${zone.id}`)}
                  >
                    <TableCell className='py-1 font-mono text-[11px] text-muted-foreground'>
                      {zone.id}
                    </TableCell>
                    <TableCell className='py-1 font-mono text-[11px] font-medium'>
                      {zone.domain}
                    </TableCell>
                    <TableCell className='py-1 font-mono text-[11px] text-muted-foreground'>
                      {zone.domain_count ?? 0}
                    </TableCell>
                    <TableCell className='py-1 font-mono text-[10px] whitespace-nowrap text-muted-foreground'>
                      {formatDateTime(zone.created_at)}
                    </TableCell>
                    <TableCell className='py-1 font-mono text-[10px] whitespace-nowrap text-muted-foreground'>
                      {formatDateTime(zone.updated_at)}
                    </TableCell>
                    <TableCell
                      className='sticky right-0 z-10 bg-background py-1 text-center'
                      onClick={(event) => event.stopPropagation()}
                    >
                      <div className='flex items-center justify-center gap-0.5'>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant='ghost'
                              size='icon'
                              className='h-6 w-6 text-muted-foreground hover:text-foreground'
                              asChild
                            >
                              <Link href={`/websites/${zone.id}`}>
                                <Eye className='size-3' />
                                <span className='sr-only'>管理</span>
                              </Link>
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side='top' className='text-xs'>
                            管理 Zone
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TooltipProvider>
        </div>
      )}

      <ZoneEditorDialog open={editorOpen} onOpenChange={setEditorOpen} />
    </div>
  );
}
