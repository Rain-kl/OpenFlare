'use client';

import { useState } from 'react';
import { format } from 'date-fns';
import { CalendarIcon, ChevronDown, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { SearchDraft } from './access-log-utils';
import { PAGE_SIZE_OPTIONS } from './access-log-utils';

interface AccessLogFiltersProps {
  draft: SearchDraft;
  pageSize: number;
  onDraftChange: (draft: SearchDraft) => void;
  onPageSizeChange: (pageSize: number) => void;
  onSearch: () => void;
  onReset: () => void;
}

const HOUR_OPTIONS = Array.from({ length: 24 }, (_, i) =>
  String(i).padStart(2, '0'),
);
const MINUTE_OPTIONS = Array.from({ length: 60 }, (_, i) =>
  String(i).padStart(2, '0'),
);

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

function TimeSelect({
  value,
  options,
  onValueChange,
}: {
  value: string;
  options: string[];
  onValueChange: (value: string) => void;
}) {
  return (
    <Select value={value} onValueChange={onValueChange}>
      <SelectTrigger className='h-8 w-18 text-xs'>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option} value={option}>
            {option}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** shadcn 日期 + 时间选择器，value 为 ISO 字符串。 */
function DateTimePicker({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const current = value ? new Date(value) : undefined;

  const applyDate = (date: Date | undefined) => {
    if (!date) return;
    const next = value ? new Date(value) : new Date();
    next.setFullYear(date.getFullYear(), date.getMonth(), date.getDate());
    onChange(next.toISOString());
  };

  const applyTime = (hh: string, mm: string) => {
    const next = value ? new Date(value) : new Date();
    next.setHours(Number(hh), Number(mm), 0, 0);
    onChange(next.toISOString());
  };

  const hour = current ? String(current.getHours()).padStart(2, '0') : '00';
  const minute = current ? String(current.getMinutes()).padStart(2, '0') : '00';

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant='outline'
          className='h-9 w-full justify-start gap-2 px-3 text-xs font-normal'
        >
          <CalendarIcon className='size-3.5 text-muted-foreground' />
          {current ? (
            format(current, 'yyyy-MM-dd HH:mm')
          ) : (
            <span className='text-muted-foreground'>选择时间</span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className='w-auto p-0' align='start'>
        <Calendar mode='single' selected={current} onSelect={applyDate} />
        <div className='flex items-center gap-1.5 border-t p-2'>
          <TimeSelect
            value={hour}
            options={HOUR_OPTIONS}
            onValueChange={(h) => applyTime(h, minute)}
          />
          <span className='text-xs text-muted-foreground'>:</span>
          <TimeSelect
            value={minute}
            options={MINUTE_OPTIONS}
            onValueChange={(m) => applyTime(hour, m)}
          />
        </div>
      </PopoverContent>
    </Popover>
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
        <FilterField label='访问域名'>
          <Input
            value={draft.host}
            onChange={(e) => onDraftChange({ ...draft, host: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === 'Enter') onSearch();
            }}
            placeholder='按域名搜索'
            className='h-9 text-xs'
          />
        </FilterField>
        <FilterField label='状态码'>
          <Input
            value={draft.statusCode}
            onChange={(e) =>
              onDraftChange({
                ...draft,
                statusCode: e.target.value.replace(/\D/g, '').slice(0, 3),
              })
            }
            onKeyDown={(e) => {
              if (e.key === 'Enter') onSearch();
            }}
            placeholder='输入状态码，如 404'
            className='h-9 text-xs'
          />
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
            <FilterField label='时间范围'>
              <div className='grid grid-cols-2 gap-2'>
                <DateTimePicker
                  value={draft.since}
                  onChange={(value) =>
                    onDraftChange({ ...draft, since: value })
                  }
                />
                <DateTimePicker
                  value={draft.until}
                  onChange={(value) =>
                    onDraftChange({ ...draft, until: value })
                  }
                />
              </div>
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
