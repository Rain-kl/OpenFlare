import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { NodeStatus } from '@/lib/services/openflare';

export function NodeListControls({
  search,
  searchTooLong,
  onSearchChange,
  status,
  onStatusChange,
  page,
  pageSize,
  onPageSizeChange,
  total,
  totalPages,
  loading,
  hasData,
  onPageChange,
}: {
  search: string;
  searchTooLong: boolean;
  onSearchChange: (value: string) => void;
  status: NodeStatus | '';
  onStatusChange: (value: NodeStatus | '') => void;
  page: number;
  pageSize: number;
  onPageSizeChange: (value: number) => void;
  total: number;
  totalPages: number;
  loading: boolean;
  hasData: boolean;
  onPageChange: (value: number) => void;
}) {
  const t = useTranslations('nodes');
  return (
    <div className='flex flex-col gap-4'>
      <FieldGroup className='flex flex-wrap gap-3 sm:flex-row'>
        <Field className='flex-1' data-invalid={searchTooLong || undefined}>
          <FieldLabel htmlFor='node-search'>
            {t('pagination.search')}
          </FieldLabel>
          <Input
            id='node-search'
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
            placeholder={t('pagination.searchPlaceholder')}
            maxLength={256}
            aria-invalid={searchTooLong || undefined}
            aria-describedby={searchTooLong ? 'node-search-error' : undefined}
          />
          {searchTooLong && (
            <FieldError id='node-search-error'>
              {t('pagination.searchTooLong')}
            </FieldError>
          )}
        </Field>
        <Field className='sm:w-44'>
          <FieldLabel htmlFor='node-status'>{t('table.status')}</FieldLabel>
          <Select
            value={status || 'all'}
            onValueChange={(value) =>
              onStatusChange(value === 'all' ? '' : (value as NodeStatus))
            }
          >
            <SelectTrigger id='node-status'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value='all'>
                  {t('pagination.allStatuses')}
                </SelectItem>
                {(['pending', 'online', 'offline'] as const).map((value) => (
                  <SelectItem key={value} value={value}>
                    {t(`status.${value}`)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <Field className='sm:w-32'>
          <FieldLabel htmlFor='node-page-size'>
            {t('pagination.pageSize')}
          </FieldLabel>
          <Select
            value={String(pageSize)}
            onValueChange={(value) => onPageSizeChange(Number(value))}
          >
            <SelectTrigger id='node-page-size'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {[20, 50, 100].map((value) => (
                  <SelectItem key={value} value={String(value)}>
                    {value}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
      </FieldGroup>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        {hasData && (
          <p className='text-sm text-muted-foreground' aria-live='polite'>
            {t('pagination.summary', { page, totalPages, total })}
          </p>
        )}
        <nav aria-label={t('pagination.navigation')} className='flex gap-2'>
          <Button
            variant='outline'
            size='sm'
            disabled={!hasData || loading || page <= 1}
            onClick={() => onPageChange(page - 1)}
          >
            {t('pagination.previous')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            disabled={!hasData || loading || page >= totalPages}
            onClick={() => onPageChange(page + 1)}
          >
            {t('pagination.next')}
          </Button>
        </nav>
      </div>
    </div>
  );
}
