'use client';

import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useState } from 'react';
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
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import type { CloudflareAvailableDomain } from '@/lib/services/openflare';

type DomainZoneGroup = {
  zoneId: number;
  zoneDomain: string;
  domains: CloudflareAvailableDomain[];
};

function groupDomainsByZone(
  domains: CloudflareAvailableDomain[],
): DomainZoneGroup[] {
  const map = new Map<number, DomainZoneGroup>();
  for (const domain of domains) {
    const existing = map.get(domain.zone_id);
    if (existing) {
      existing.domains.push(domain);
      continue;
    }
    map.set(domain.zone_id, {
      zoneId: domain.zone_id,
      zoneDomain: domain.zone_domain || `Zone #${domain.zone_id}`,
      domains: [domain],
    });
  }
  return [...map.values()]
    .map((group) => ({
      ...group,
      domains: [...group.domains].sort((a, b) =>
        a.domain.localeCompare(b.domain),
      ),
    }))
    .sort((a, b) => a.zoneDomain.localeCompare(b.zoneDomain));
}

export function MemberAddDialog({
  open,
  onOpenChange,
  domains,
  defaultProxied,
  pending,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  domains: CloudflareAvailableDomain[];
  defaultProxied: boolean;
  pending: boolean;
  onSubmit: (zoneDomainIDs: number[], proxied: boolean) => void;
}) {
  const t = useTranslations('cloudflare.memberDialog');
  const tCommon = useTranslations('common');
  const [keyword, setKeyword] = useState('');
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const [proxied, setProxied] = useState(defaultProxied);
  const [collapsedZones, setCollapsedZones] = useState<Set<number>>(
    () => new Set(),
  );

  useEffect(() => {
    if (!open) return;
    setKeyword('');
    setSelectedIDs([]);
    setProxied(defaultProxied);
    setCollapsedZones(new Set());
  }, [defaultProxied, open]);

  const selected = useMemo(() => new Set(selectedIDs), [selectedIDs]);

  const filteredGroups = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    const list = normalized
      ? domains.filter((domain) => {
          const zoneRoot = domain.zone_domain ?? '';
          return (
            domain.domain.toLowerCase().includes(normalized) ||
            zoneRoot.toLowerCase().includes(normalized) ||
            String(domain.id).includes(normalized)
          );
        })
      : domains;
    return groupDomainsByZone(list);
  }, [domains, keyword]);

  const visibleIDs = useMemo(
    () => filteredGroups.flatMap((group) => group.domains.map((d) => d.id)),
    [filteredGroups],
  );

  const allVisibleSelected =
    visibleIDs.length > 0 && visibleIDs.every((id) => selected.has(id));
  const someVisibleSelected =
    visibleIDs.some((id) => selected.has(id)) && !allVisibleSelected;

  const toggleOne = (id: number) => {
    setSelectedIDs((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    );
  };

  const toggleGroup = (group: DomainZoneGroup) => {
    const ids = group.domains.map((d) => d.id);
    const allSelected = ids.every((id) => selected.has(id));
    setSelectedIDs((prev) => {
      if (allSelected) {
        return prev.filter((id) => !ids.includes(id));
      }
      const next = new Set(prev);
      for (const id of ids) next.add(id);
      return [...next];
    });
  };

  const toggleAllVisible = () => {
    setSelectedIDs((prev) => {
      if (allVisibleSelected) {
        return prev.filter((id) => !visibleIDs.includes(id));
      }
      const next = new Set(prev);
      for (const id of visibleIDs) next.add(id);
      return [...next];
    });
  };

  const toggleCollapsed = (zoneId: number) => {
    setCollapsedZones((prev) => {
      const next = new Set(prev);
      if (next.has(zoneId)) next.delete(zoneId);
      else next.add(zoneId);
      return next;
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('title')}</DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='cf-domain-search'>
              {t('zoneDomain')}
            </FieldLabel>
            <div className='space-y-2'>
              <div className='relative'>
                <Search className='pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground' />
                <Input
                  id='cf-domain-search'
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder={t('searchPlaceholder')}
                  className='pl-8'
                  disabled={pending}
                />
              </div>

              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                  <Badge variant='secondary' className='font-normal'>
                    {t('selected', { count: selectedIDs.length })}
                  </Badge>
                  <span>{t('visible', { count: visibleIDs.length })}</span>
                </div>
                <div className='flex items-center gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-7 text-xs'
                    disabled={pending || visibleIDs.length === 0}
                    onClick={toggleAllVisible}
                  >
                    {allVisibleSelected ? t('unselectAll') : t('selectVisible')}
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-7 text-xs'
                    disabled={pending || selectedIDs.length === 0}
                    onClick={() => setSelectedIDs([])}
                  >
                    {t('clear')}
                  </Button>
                </div>
              </div>

              {domains.length === 0 ? (
                <div className='rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground'>
                  {t('noAvailable')}
                </div>
              ) : filteredGroups.length === 0 ? (
                <div className='rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground'>
                  {t('noMatch')}
                </div>
              ) : (
                <div className='max-h-72 space-y-1 overflow-y-auto rounded-lg border p-2'>
                  {/* Header row: select all visible */}
                  <label className='flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-xs text-muted-foreground hover:bg-muted/50'>
                    <Checkbox
                      checked={
                        allVisibleSelected
                          ? true
                          : someVisibleSelected
                            ? 'indeterminate'
                            : false
                      }
                      disabled={pending}
                      onCheckedChange={toggleAllVisible}
                    />
                    <span>{t('groupByZone')}</span>
                  </label>

                  {filteredGroups.map((group) => {
                    const groupIDs = group.domains.map((d) => d.id);
                    const groupAllSelected = groupIDs.every((id) =>
                      selected.has(id),
                    );
                    const groupSomeSelected =
                      groupIDs.some((id) => selected.has(id)) &&
                      !groupAllSelected;
                    const open = !collapsedZones.has(group.zoneId);

                    return (
                      <Collapsible
                        key={group.zoneId}
                        open={open}
                        onOpenChange={() => toggleCollapsed(group.zoneId)}
                      >
                        <div
                          className={cn(
                            'rounded-md',
                            (groupAllSelected || groupSomeSelected) &&
                              'bg-muted/30',
                          )}
                        >
                          <div className='flex items-center gap-1 px-1 py-0.5'>
                            <Checkbox
                              checked={
                                groupAllSelected
                                  ? true
                                  : groupSomeSelected
                                    ? 'indeterminate'
                                    : false
                              }
                              disabled={pending}
                              onCheckedChange={() => toggleGroup(group)}
                              aria-label={t('selectZone', {
                                name: group.zoneDomain,
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
                                  {
                                    groupIDs.filter((id) => selected.has(id))
                                      .length
                                  }
                                  /{group.domains.length}
                                </Badge>
                              </button>
                            </CollapsibleTrigger>
                          </div>

                          <CollapsibleContent>
                            <div className='ml-4 space-y-0.5 border-l border-border/70 py-0.5 pl-2'>
                              {group.domains.map((domain) => {
                                const checked = selected.has(domain.id);
                                const isApex =
                                  domain.domain === group.zoneDomain;
                                return (
                                  <label
                                    key={domain.id}
                                    className={cn(
                                      'flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 transition-colors hover:bg-muted/60',
                                      checked && 'bg-muted/40',
                                    )}
                                  >
                                    <Checkbox
                                      checked={checked}
                                      disabled={pending}
                                      onCheckedChange={() =>
                                        toggleOne(domain.id)
                                      }
                                    />
                                    <span className='min-w-0 flex-1 truncate text-sm'>
                                      {domain.domain}
                                    </span>
                                    {isApex ? (
                                      <Badge
                                        variant='secondary'
                                        className='shrink-0 text-[10px] font-normal'
                                      >
                                        {t('zone')}
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
          <Field orientation='horizontal'>
            <FieldLabel htmlFor='cf-member-proxied'>{t('proxied')}</FieldLabel>
            <Switch
              id='cf-member-proxied'
              checked={proxied}
              disabled={pending}
              onCheckedChange={setProxied}
            />
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            {tCommon('cancel')}
          </Button>
          <Button
            disabled={pending || selectedIDs.length === 0}
            onClick={() =>
              onSubmit(
                [...selectedIDs].sort((a, b) => a - b),
                proxied,
              )
            }
          >
            {selectedIDs.length > 1
              ? t('submitCount', { count: selectedIDs.length })
              : t('submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
