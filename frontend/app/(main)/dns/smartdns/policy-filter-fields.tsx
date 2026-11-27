'use client';

import { useQuery } from '@tanstack/react-query';
import { Check, ChevronsUpDown } from 'lucide-react';
import { useLocale, useTranslations } from 'next-intl';
import { useMemo } from 'react';

import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  NodeService,
  NodeTagService,
  nodeTagQueryKey,
  type SmartDNSPolicyFilters,
} from '@/lib/services/openflare';
import { createCountryOptions, type GeoOption } from '@/lib/utils/geo-options';

export type PolicyFilterValues = Pick<
  SmartDNSPolicyFilters,
  'countries' | 'tags' | 'network_groups'
>;

function simpleOption(value: string): GeoOption {
  return { value, label: value, searchText: value };
}

function FilterPicker({
  id,
  label,
  value,
  options,
  pending,
  onRetry,
  onChange,
}: {
  id: string;
  label: string;
  value: string[];
  options: GeoOption[];
  pending?: boolean;
  onRetry?: () => void;
  onChange: (value: string[]) => void;
}) {
  const t = useTranslations('smartdns.filters');
  const available = new Map(options.map((option) => [option.value, option]));
  for (const selected of value) {
    if (!available.has(selected))
      available.set(selected, simpleOption(selected));
  }
  const selectedOptions = value.map(
    (selected) => available.get(selected)?.label ?? selected,
  );
  const summary = selectedOptions.join(', ');

  return (
    <Field className='min-w-0'>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Popover>
        <PopoverTrigger asChild>
          <Button
            id={id}
            type='button'
            variant='outline'
            className='w-full justify-between'
            title={summary || undefined}
          >
            <span className='min-w-0 truncate'>
              {selectedOptions.length > 2
                ? t('selectedCount', { count: selectedOptions.length })
                : summary || t('any')}
            </span>
            <ChevronsUpDown data-icon='inline-end' className='shrink-0' />
          </Button>
        </PopoverTrigger>
        <PopoverContent
          align='start'
          aria-label={label}
          className='w-[max(14rem,var(--radix-popover-trigger-width))] max-w-[var(--radix-popover-content-available-width)] p-0'
        >
          <Command>
            <CommandInput
              aria-label={t('searchField', { label })}
              placeholder={t('search')}
            />
            <CommandList className='max-h-56'>
              <CommandEmpty>
                {pending ? t('loading') : t('noMatches')}
              </CommandEmpty>
              <CommandGroup>
                {[...available.values()].map((option) => {
                  const selected = value.includes(option.value);
                  return (
                    <CommandItem
                      key={option.value}
                      value={option.value}
                      keywords={[option.searchText]}
                      onSelect={() =>
                        onChange(
                          selected
                            ? value.filter((item) => item !== option.value)
                            : [...value, option.value],
                        )
                      }
                    >
                      <span className='min-w-0 grow truncate'>
                        {option.label}
                      </span>
                      {option.label !== option.value && (
                        <span className='text-xs text-muted-foreground'>
                          {option.value}
                        </span>
                      )}
                      {selected && (
                        <>
                          <Check />
                          <span className='sr-only'>{t('selected')}</span>
                        </>
                      )}
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
          {(onRetry || value.length > 0) && (
            <div className='flex flex-wrap justify-end gap-1 p-1'>
              {onRetry && (
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={onRetry}
                >
                  {t('retry')}
                </Button>
              )}
              {value.length > 0 && (
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={() => onChange([])}
                >
                  {t('clear')}
                </Button>
              )}
            </div>
          )}
        </PopoverContent>
      </Popover>
    </Field>
  );
}

export function PolicyFilterFields({
  id,
  value,
  onChange,
}: {
  id: string;
  value: PolicyFilterValues;
  onChange: (patch: Partial<PolicyFilterValues>) => void;
}) {
  const locale = useLocale();
  const t = useTranslations('smartdns.flow');
  const countries = useMemo(() => createCountryOptions(locale), [locale]);
  const tags = useQuery({
    queryKey: nodeTagQueryKey,
    queryFn: () => NodeTagService.list(),
  });
  const nodes = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
  });
  const networkGroups = [
    ...new Set(
      (nodes.data ?? [])
        .map((node) => node.network_group?.trim())
        .filter((name): name is string => Boolean(name)),
    ),
  ].sort((a, b) => a.localeCompare(b, locale));

  return (
    <>
      <FilterPicker
        id={`${id}-countries`}
        label={t('filterCountries')}
        value={value.countries}
        options={countries}
        onChange={(countries) => onChange({ countries })}
      />
      <FilterPicker
        id={`${id}-tags`}
        label={t('filterTags')}
        value={value.tags}
        options={(tags.data ?? []).map((tag) => ({
          value: tag.slug,
          label: tag.name,
          searchText: `${tag.name} ${tag.slug}`,
        }))}
        pending={tags.isPending}
        onRetry={tags.isError ? () => void tags.refetch() : undefined}
        onChange={(tags) => onChange({ tags })}
      />
      <FilterPicker
        id={`${id}-network-groups`}
        label={t('filterGroups')}
        value={value.network_groups}
        options={networkGroups.map(simpleOption)}
        pending={nodes.isPending}
        onRetry={nodes.isError ? () => void nodes.refetch() : undefined}
        onChange={(network_groups) => onChange({ network_groups })}
      />
    </>
  );
}
