'use client';

import { useEffect, useRef, useState } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  LayoutGrid,
  RefreshCw,
  Search,
  Table2,
  X,
} from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import { Field, FieldError } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import type { NodeStatus } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import type { NodeListView } from './nodes-table';

export function NodeListControls({
  view,
  onViewChange,
  search,
  searchTooLong,
  onSearchChange,
  status,
  onStatusChange,
  refreshing,
  onRefresh,
}: {
  view: NodeListView;
  onViewChange: (value: NodeListView) => void;
  search: string;
  searchTooLong: boolean;
  onSearchChange: (value: string) => void;
  status: NodeStatus | '';
  onStatusChange: (value: NodeStatus | '') => void;
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const t = useTranslations('nodes');
  const [searchOpen, setSearchOpen] = useState(false);
  const searchInput = useRef<HTMLInputElement>(null);
  const searchButton = useRef<HTMLButtonElement>(null);
  const restoreFocus = useRef(false);

  useEffect(() => {
    if (searchOpen) {
      searchInput.current?.focus();
    } else if (restoreFocus.current) {
      searchButton.current?.focus();
      restoreFocus.current = false;
    }
  }, [searchOpen]);

  const closeSearch = () => {
    onSearchChange('');
    restoreFocus.current = true;
    setSearchOpen(false);
  };

  return (
    <div className='flex flex-wrap items-start justify-end gap-2'>
      <Select
        value={status || 'all'}
        onValueChange={(value) =>
          onStatusChange(value === 'all' ? '' : (value as NodeStatus))
        }
      >
        <SelectTrigger size='sm' aria-label={t('table.status')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectItem value='all'>{t('pagination.allStatuses')}</SelectItem>
            {(['pending', 'online', 'offline', 'quarantined'] as const).map(
              (value) => (
                <SelectItem key={value} value={value}>
                  {t(`status.${value}`)}
                </SelectItem>
              ),
            )}
          </SelectGroup>
        </SelectContent>
      </Select>
      <ToggleGroup
        type='single'
        variant='outline'
        size='sm'
        spacing={1}
        value={view}
        aria-label={t('monitoring.view')}
        onValueChange={(value) => {
          if (value === 'grid' || value === 'list') onViewChange(value);
        }}
      >
        <ToggleGroupItem
          value='grid'
          aria-label={t('monitoring.gridView')}
          title={t('monitoring.gridView')}
          className='size-8 p-0 data-[state=on]:text-primary'
        >
          <LayoutGrid />
        </ToggleGroupItem>
        <ToggleGroupItem
          value='list'
          aria-label={t('monitoring.listView')}
          title={t('monitoring.listView')}
          className='size-8 p-0 data-[state=on]:text-primary'
        >
          <Table2 />
        </ToggleGroupItem>
      </ToggleGroup>
      {searchOpen ? (
        <Field
          className='w-64 max-w-full gap-1'
          data-invalid={searchTooLong || undefined}
        >
          <div className='relative'>
            <Search
              className='pointer-events-none absolute top-2 left-2.5 size-4 text-muted-foreground'
              aria-hidden
            />
            <Input
              ref={searchInput}
              type='text'
              inputMode='search'
              className='h-8 pr-8 pl-8'
              value={search}
              onChange={(event) => onSearchChange(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Escape') closeSearch();
              }}
              aria-label={t('pagination.search')}
              placeholder={t('pagination.searchPlaceholder')}
              maxLength={256}
              aria-invalid={searchTooLong || undefined}
              aria-describedby={searchTooLong ? 'node-search-error' : undefined}
            />
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              className='absolute top-0 right-0'
              aria-label={t('monitoring.closeSearch')}
              title={t('monitoring.closeSearch')}
              onClick={closeSearch}
            >
              <X />
            </Button>
          </div>
          {searchTooLong && (
            <FieldError id='node-search-error'>
              {t('pagination.searchTooLong')}
            </FieldError>
          )}
        </Field>
      ) : (
        <Button
          ref={searchButton}
          type='button'
          variant='outline'
          size='icon-sm'
          aria-label={t('pagination.search')}
          title={t('pagination.search')}
          onClick={() => setSearchOpen(true)}
        >
          <Search />
        </Button>
      )}
      <Button
        type='button'
        variant='outline'
        size='icon-sm'
        aria-label={t('refreshNow')}
        title={t('refreshNow')}
        onClick={onRefresh}
        disabled={refreshing}
      >
        <RefreshCw className={cn(refreshing && 'motion-safe:animate-spin')} />
      </Button>
    </div>
  );
}

export function NodePagination({
  page,
  total,
  totalPages,
  loading,
  onPageChange,
}: {
  page: number;
  total: number;
  totalPages: number;
  loading: boolean;
  onPageChange: (value: number) => void;
}) {
  const t = useTranslations('nodes');
  return (
    <div className='flex flex-wrap items-center justify-between gap-3'>
      <p className='text-xs text-muted-foreground' aria-live='polite'>
        {t('pagination.summary', { page, totalPages, total })}
      </p>
      {totalPages > 1 && (
        <nav aria-label={t('pagination.navigation')} className='flex gap-2'>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            aria-label={t('pagination.previous')}
            title={t('pagination.previous')}
            disabled={loading || page <= 1}
            onClick={() => onPageChange(page - 1)}
          >
            <ChevronLeft />
          </Button>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            aria-label={t('pagination.next')}
            title={t('pagination.next')}
            disabled={loading || page >= totalPages}
            onClick={() => onPageChange(page + 1)}
          >
            <ChevronRight />
          </Button>
        </nav>
      )}
    </div>
  );
}
