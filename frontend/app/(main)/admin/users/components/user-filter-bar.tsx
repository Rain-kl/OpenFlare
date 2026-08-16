'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Filter,
  Loader2,
  Search,
  X,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useAdminUsers } from '@/contexts/admin-users-context';
import { cn } from '@/lib/utils';

export function UserFilterBar() {
  const t = useTranslations('admin.users');
  const {
    total,
    loading,
    page,
    pageSize,
    searchUserId,
    searchUsername,
    searchEmail,
    statusFilter,
    setPage,
    setPageSize,
    setSearchUserId,
    setSearchUsername,
    setSearchEmail,
    setStatusFilter,
    fetchUsers,
  } = useAdminUsers();

  const totalPages = Math.ceil(total / pageSize);
  const hasSearchFilter = Boolean(
    searchUserId || searchUsername || searchEmail,
  );

  return (
    <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between gap-3'>
      <div className='flex items-center gap-2 flex-wrap'>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant='outline'
              size='sm'
              className={cn(
                'h-5 border-dashed text-[10px] font-medium shadow-none focus-visible:ring-0',
                hasSearchFilter && 'bg-primary/5 border-primary/20',
              )}
            >
              <Search className='size-3 mr-1' />
              {t('search')}
              {hasSearchFilter && (
                <>
                  <Separator orientation='vertical' className='mx-1' />
                  <Badge
                    variant='default'
                    className='text-[10px] h-3 px-1 rounded-full'
                  >
                    !
                  </Badge>
                </>
              )}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent className='w-56 p-3' align='start'>
            <div className='space-y-2.5'>
              <input
                className='w-full h-7 px-2 text-xs border border-dashed rounded-md outline-none focus:border-primary bg-background'
                placeholder={t('searchUserIdPlaceholder')}
                value={searchUserId}
                onChange={(e) => setSearchUserId(e.target.value)}
              />
              <input
                className='w-full h-7 px-2 text-xs border border-dashed rounded-md outline-none focus:border-primary bg-background'
                placeholder={t('searchUsernamePlaceholder')}
                value={searchUsername}
                onChange={(e) => setSearchUsername(e.target.value)}
              />
              <input
                className='w-full h-7 px-2 text-xs border border-dashed rounded-md outline-none focus:border-primary bg-background'
                placeholder={t('searchEmailPlaceholder')}
                value={searchEmail}
                onChange={(e) => setSearchEmail(e.target.value)}
              />
              {hasSearchFilter && (
                <Button
                  variant='ghost'
                  size='sm'
                  className='w-full h-6 text-xs'
                  onClick={() => {
                    setSearchUserId('');
                    setSearchUsername('');
                    setSearchEmail('');
                  }}
                >
                  {t('clear')}
                </Button>
              )}
            </div>
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant='outline'
              size='sm'
              className={cn(
                'h-5 border-dashed text-[10px] font-medium shadow-none focus-visible:ring-0',
                statusFilter !== 'all' && 'bg-primary/5 border-primary/20',
              )}
            >
              <Filter className='size-3' />
              {t('status')}
              {statusFilter !== 'all' && (
                <>
                  <Separator orientation='vertical' className='mx-1' />
                  <Badge
                    variant='default'
                    className='text-[10px] h-3 px-1 rounded-full'
                  >
                    1
                  </Badge>
                </>
              )}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent className='w-[120px]' align='start'>
            <DropdownMenuItem
              onSelect={(e) => {
                e.preventDefault();
                setStatusFilter('all');
              }}
            >
              <div
                className={cn(
                  'mr-2 flex size-3 items-center justify-center rounded-sm border border-primary',
                  statusFilter === 'all'
                    ? 'bg-primary text-primary-foreground'
                    : 'opacity-50',
                )}
              />
              <span className='text-xs'>{t('statusAll')}</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={(e) => {
                e.preventDefault();
                setStatusFilter('active');
              }}
            >
              <div
                className={cn(
                  'mr-2 flex size-3 items-center justify-center rounded-sm border border-primary',
                  statusFilter === 'active'
                    ? 'bg-primary text-primary-foreground'
                    : 'opacity-50',
                )}
              />
              <span className='text-xs'>{t('statusActive')}</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={(e) => {
                e.preventDefault();
                setStatusFilter('inactive');
              }}
            >
              <div
                className={cn(
                  'mr-2 flex size-3 items-center justify-center rounded-sm border border-primary',
                  statusFilter === 'inactive'
                    ? 'bg-primary text-primary-foreground'
                    : 'opacity-50',
                )}
              />
              <span className='text-xs'>{t('statusDisabled')}</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        {(hasSearchFilter || statusFilter !== 'all') && (
          <>
            <Separator orientation='vertical' className='h-6 hidden sm:block' />
            <Button
              variant='ghost'
              size='sm'
              onClick={() => {
                setSearchUserId('');
                setSearchUsername('');
                setSearchEmail('');
                setStatusFilter('all');
              }}
              className='h-5 px-2 lg:px-3 text-[11px] font-medium text-muted-foreground hover:text-foreground'
            >
              <X className='size-3' />
              {t('clearFilters')}
            </Button>
          </>
        )}
      </div>

      <Separator className='lg:hidden' />

      <div className='flex items-center gap-1.5 self-end lg:self-auto'>
        <span className='text-[10px] text-muted-foreground whitespace-nowrap'>
          {t('totalRecords', { total })}
        </span>
        <div className='flex items-center border border-dashed rounded-md shadow-none'>
          <Button
            variant='ghost'
            size='icon'
            className='h-5.5 w-6 rounded-none rounded-l-md disabled:opacity-30'
            onClick={() => setPage(Math.max(1, page - 1))}
            disabled={page <= 1 || loading}
          >
            <ChevronLeft className='size-3' />
          </Button>
          <span className='text-[10px] font-mono text-muted-foreground px-2 border-x border-dashed'>
            {page}/{totalPages}
          </span>
          <Button
            variant='ghost'
            size='icon'
            className='h-5.5 w-6 rounded-none rounded-r-md disabled:opacity-30'
            onClick={() => setPage(Math.min(totalPages, page + 1))}
            disabled={page >= totalPages || loading}
          >
            <ChevronRight className='size-3' />
          </Button>
        </div>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant='outline'
              size='sm'
              className='h-6 border-dashed text-[10px] px-2 font-mono shadow-none'
              disabled={loading}
            >
              {t('perPage', { count: pageSize })}
              <ChevronDown className='size-3 opacity-50' />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end'>
            {[20, 50, 100].map((size) => (
              <DropdownMenuItem
                key={size}
                onClick={() => setPageSize(size)}
                className={cn(
                  'font-mono text-xs',
                  pageSize === size && 'bg-accent',
                )}
              >
                {t('perPage', { count: size })}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        <Button
          variant='outline'
          size='icon'
          className='h-6 w-6 border-dashed shadow-none'
          onClick={() => fetchUsers(true)}
          disabled={loading}
          title={t('refreshData')}
        >
          <Loader2 className={cn('size-3', loading && 'animate-spin')} />
        </Button>
      </div>
    </div>
  );
}
