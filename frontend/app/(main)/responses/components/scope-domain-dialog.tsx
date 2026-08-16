'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { Check, ChevronDown, Search } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useTranslations } from 'next-intl';
import { cn } from '@/lib/utils';

type ScopeZoneGroup = {
  zoneDomain: string;
  domains: string[];
};

export function ScopeDomainDialog({
  open,
  onOpenChange,
  zones,
  selected,
  pending,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  zones: ScopeZoneGroup[];
  selected: string[];
  pending: boolean;
  onSubmit: (domains: string[]) => void;
}) {
  const t = useTranslations('responses');
  const tc = useTranslations('common');
  const [keyword, setKeyword] = useState('');
  const [selectedSet, setSelectedSet] = useState<Set<string>>(() => new Set());
  const [collapsedZones, setCollapsedZones] = useState<Set<string>>(
    () => new Set(),
  );

  const selectedRef = useRef(selected);
  selectedRef.current = selected;
  const prevOpenRef = useRef(open);

  useEffect(() => {
    if (open && !prevOpenRef.current) {
      setKeyword('');
      setSelectedSet(new Set(selectedRef.current));
      setCollapsedZones(new Set());
    }
    prevOpenRef.current = open;
  }, [open]);

  const filtered = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return zones;
    return zones
      .map((group) => ({
        zoneDomain: group.zoneDomain,
        domains: group.domains.filter(
          (d) =>
            d.toLowerCase().includes(normalized) ||
            group.zoneDomain.toLowerCase().includes(normalized),
        ),
      }))
      .filter((group) => group.domains.length > 0);
  }, [zones, keyword]);

  const visibleDomains = useMemo(
    () => filtered.flatMap((group) => group.domains),
    [filtered],
  );
  const allVisibleSelected =
    visibleDomains.length > 0 &&
    visibleDomains.every((d) => selectedSet.has(d));

  const toggleOne = (domain: string) => {
    setSelectedSet((prev) => {
      const next = new Set(prev);
      if (next.has(domain)) next.delete(domain);
      else next.add(domain);
      return next;
    });
  };

  const toggleGroup = (domains: string[]) => {
    setSelectedSet((prev) => {
      const next = new Set(prev);
      const allSelected = domains.every((d) => next.has(d));
      for (const d of domains) {
        if (allSelected) next.delete(d);
        else next.add(d);
      }
      return next;
    });
  };

  const toggleAllVisible = () => {
    setSelectedSet((prev) => {
      const next = new Set(prev);
      if (allVisibleSelected) {
        for (const d of visibleDomains) next.delete(d);
      } else {
        for (const d of visibleDomains) next.add(d);
      }
      return next;
    });
  };

  const toggleCollapsed = (zoneDomain: string) => {
    setCollapsedZones((prev) => {
      const next = new Set(prev);
      if (next.has(zoneDomain)) next.delete(zoneDomain);
      else next.add(zoneDomain);
      return next;
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('selectScopeTitle')}</DialogTitle>
          <DialogDescription>{t('selectScopeDesc')}</DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='sw-scope-search'>{t('domain')}</FieldLabel>
            <div className='space-y-2'>
              <div className='relative'>
                <Search className='pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground' />
                <Input
                  id='sw-scope-search'
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder={t('searchDomainOrZone')}
                  className='pl-8'
                  disabled={pending}
                />
              </div>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                  <Badge variant='secondary' className='font-normal'>
                    {t('selectedCount', { count: selectedSet.size })}
                  </Badge>
                  <span>
                    {t('visibleCount', { count: visibleDomains.length })}
                  </span>
                </div>
                <div className='flex items-center gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-7 text-xs'
                    disabled={pending || visibleDomains.length === 0}
                    onClick={toggleAllVisible}
                  >
                    {allVisibleSelected
                      ? t('unselectVisible')
                      : t('selectVisible')}
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-7 text-xs'
                    disabled={pending || selectedSet.size === 0}
                    onClick={() => setSelectedSet(new Set())}
                  >
                    {t('clear')}
                  </Button>
                </div>
              </div>
              {zones.length === 0 ? (
                <div className='rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground'>
                  {t('noAvailableDomains')}
                </div>
              ) : filtered.length === 0 ? (
                <div className='rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground'>
                  {t('noMatchingDomains')}
                </div>
              ) : (
                <div className='max-h-72 space-y-1 overflow-y-auto rounded-lg border p-2'>
                  {filtered.map((group) => {
                    const groupSelected = group.domains.filter((d) =>
                      selectedSet.has(d),
                    );
                    const allSelected =
                      groupSelected.length === group.domains.length;
                    const someSelected =
                      groupSelected.length > 0 && !allSelected;
                    const open = !collapsedZones.has(group.zoneDomain);
                    return (
                      <Collapsible
                        key={group.zoneDomain}
                        open={open}
                        onOpenChange={() => toggleCollapsed(group.zoneDomain)}
                      >
                        <div
                          className={cn(
                            'rounded-md',
                            (allSelected || someSelected) && 'bg-muted/30',
                          )}
                        >
                          <div className='flex items-center gap-1 px-1 py-0.5'>
                            <Checkbox
                              checked={
                                allSelected
                                  ? true
                                  : someSelected
                                    ? 'indeterminate'
                                    : false
                              }
                              disabled={pending}
                              onCheckedChange={() => toggleGroup(group.domains)}
                              aria-label={t('selectZone', {
                                domain: group.zoneDomain,
                              })}
                              className='ml-1'
                            />
                            <CollapsibleTrigger asChild>
                              <button
                                type='button'
                                className='flex min-w-0 flex-1 items-center gap-2 rounded-md px-1.5 py-1.5 text-left text-sm font-medium hover:bg-muted/60'
                              >
                                <ChevronDown
                                  className={cn(
                                    'size-4 shrink-0 text-muted-foreground transition-transform',
                                    !open && '-rotate-90',
                                  )}
                                />
                                <span className='truncate'>
                                  {group.zoneDomain}
                                </span>
                                <Badge
                                  variant='outline'
                                  className='ml-auto shrink-0 font-normal text-[10px]'
                                >
                                  {groupSelected.length}/{group.domains.length}
                                </Badge>
                              </button>
                            </CollapsibleTrigger>
                          </div>
                          <CollapsibleContent>
                            <div className='ml-4 space-y-0.5 border-l border-border/70 py-0.5 pl-2'>
                              {group.domains.map((domain) => {
                                const checked = selectedSet.has(domain);
                                const isApex = domain === group.zoneDomain;
                                return (
                                  <label
                                    key={domain}
                                    className={cn(
                                      'flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 transition-colors hover:bg-muted/60',
                                      checked && 'bg-muted/40',
                                    )}
                                  >
                                    <Checkbox
                                      checked={checked}
                                      disabled={pending}
                                      onCheckedChange={() => toggleOne(domain)}
                                    />
                                    <span className='min-w-0 flex-1 truncate text-sm'>
                                      {domain}
                                    </span>
                                    {isApex ? (
                                      <Badge
                                        variant='secondary'
                                        className='shrink-0 text-[10px] font-normal'
                                      >
                                        {t('apex')}
                                      </Badge>
                                    ) : null}
                                    {checked ? (
                                      <Check className='size-3.5 shrink-0 text-primary' />
                                    ) : null}
                                  </label>
                                );
                              })}
                            </div>
                          </CollapsibleContent>
                        </div>
                      </Collapsible>
                    );
                  })}
                </div>
              )}
            </div>
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            {tc('cancel')}
          </Button>
          <Button
            disabled={pending}
            onClick={() => onSubmit([...selectedSet].sort())}
          >
            {t('confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
