'use client';

import { useState } from 'react';
import { ChevronDown, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { SearchDraft } from './access-log-utils';
import { PAGE_SIZE_OPTIONS, STATUS_CODE_OPTIONS } from './access-log-utils';

interface AccessLogFiltersProps {
  draft: SearchDraft;
  pageSize: number;
  onDraftChange: (draft: SearchDraft) => void;
  onPageSizeChange: (pageSize: number) => void;
  onSearch: () => void;
  onReset: () => void;
}

function FilterField({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className='space-y-1.5'>
      <p className='text-xs font-medium text-muted-foreground'>{label}</p>
      {children}
    </div>
  );
}

export function AccessLogFilters({
  draft,
  pageSize,
  onDraftChange,
  onPageSizeChange,
  onSearch,
  onReset,
}: AccessLogFiltersProps) {
  const [moreOpen, setMoreOpen] = useState(false);

  return (
    <div className='space-y-3'>
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
        <FilterField label='来源 IP'>
          <Input
            value={draft.remoteAddr}
            onChange={(e) =>
              onDraftChange({ ...draft, remoteAddr: e.target.value })
            }
            onKeyDown={(e) => {
              if (e.key === 'Enter') onSearch();
            }}
            placeholder='按 IP 搜索'
            className='h-9 text-xs'
          />
        </FilterField>
        <FilterField label='状态码'>
          <Select
            value={draft.statusCode}
            onValueChange={(value) =>
              onDraftChange({ ...draft, statusCode: value })
            }
          >
            <SelectTrigger className='h-9 text-xs'>
              <SelectValue placeholder='全部状态码' />
            </SelectTrigger>
            <SelectContent>
              {STATUS_CODE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FilterField>
      </div>

      <Collapsible open={moreOpen} onOpenChange={setMoreOpen}>
        <CollapsibleTrigger asChild>
          <Button
            variant='ghost'
            size='sm'
            className='-ml-1 h-8 gap-1 px-1 text-xs text-muted-foreground'
          >
            更多筛选
            <ChevronDown
              className={`size-3.5 transition-transform ${
                moreOpen ? 'rotate-180' : ''
              }`}
            />
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className='grid gap-3 pt-3 md:grid-cols-2 xl:grid-cols-3'>
            <FilterField label='节点 ID'>
              <div className='relative'>
                <Search className='absolute left-2.5 top-2.5 size-3.5 text-muted-foreground' />
                <Input
                  value={draft.nodeId}
                  onChange={(e) =>
                    onDraftChange({ ...draft, nodeId: e.target.value })
                  }
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') onSearch();
                  }}
                  placeholder='按 node_id 搜索'
                  className='pl-8 h-9 text-xs'
                />
              </div>
            </FilterField>
            <FilterField label='访问域名'>
              <Input
                value={draft.host}
                onChange={(e) =>
                  onDraftChange({ ...draft, host: e.target.value })
                }
                onKeyDown={(e) => {
                  if (e.key === 'Enter') onSearch();
                }}
                placeholder='按域名搜索'
                className='h-9 text-xs'
              />
            </FilterField>
            <FilterField label='请求路径'>
              <Input
                value={draft.path}
                onChange={(e) =>
                  onDraftChange({ ...draft, path: e.target.value })
                }
                onKeyDown={(e) => {
                  if (e.key === 'Enter') onSearch();
                }}
                placeholder='按路径搜索'
                className='h-9 text-xs'
              />
            </FilterField>
          </div>
        </CollapsibleContent>
      </Collapsible>

      <div className='flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between'>
        <div className='space-y-1.5 w-full sm:max-w-[180px]'>
          <p className='text-xs font-medium text-muted-foreground'>每页条数</p>
          <Select
            value={String(pageSize)}
            onValueChange={(value) =>
              onPageSizeChange(Number.parseInt(value, 10))
            }
          >
            <SelectTrigger className='h-9 text-xs'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZE_OPTIONS.map((option) => (
                <SelectItem key={option} value={String(option)}>
                  {option} 条
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='flex gap-2'>
          <Button size='sm' onClick={onSearch}>
            筛选
          </Button>
          <Button variant='outline' size='sm' onClick={onReset}>
            清空
          </Button>
        </div>
      </div>
    </div>
  );
}
