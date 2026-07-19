'use client';

import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ChevronDown, Filter, Loader2, X } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { ZoneService, zoneQueryKey } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import {
  OVERVIEW_RANGE_OPTIONS,
  type OverviewRangeHours,
} from './access-log-utils';

type ManagedZoneDomains = {
  zoneId: number;
  zoneName: string;
  domains: string[];
};

function useManagedZoneDomains(enabled: boolean) {
  return useQuery({
    queryKey: [...zoneQueryKey, 'zone-domain-tree'],
    enabled,
    staleTime: 60_000,
    queryFn: async (): Promise<ManagedZoneDomains[]> => {
      const zones = await ZoneService.list();
      const overviews = await Promise.all(
        zones.map((zone) => ZoneService.getOverview(zone.id)),
      );
      return overviews
        .map((overview) => {
          const domainSet = new Set<string>();
          for (const item of overview.domains ?? []) {
            const domain = item.domain?.trim();
            if (domain) domainSet.add(domain);
          }
          return {
            zoneId: overview.zone.id,
            zoneName: overview.zone.domain || `Zone #${overview.zone.id}`,
            domains: Array.from(domainSet).sort((a, b) => a.localeCompare(b)),
          };
        })
        .filter((zone) => zone.domains.length > 0)
        .sort((a, b) => a.zoneName.localeCompare(b.zoneName));
    },
  });
}

export function OverviewHostFilter({
  hosts,
  onHostsChange,
}: {
  hosts: string[];
  onHostsChange: (hosts: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [expandedZones, setExpandedZones] = useState<Record<number, boolean>>(
    {},
  );
  const zonesQuery = useManagedZoneDomains(open);
  const zones = useMemo(() => zonesQuery.data ?? [], [zonesQuery.data]);
  const selectedSet = useMemo(() => new Set(hosts), [hosts]);
  const filteredZones = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return zones;
    return zones
      .map((zone) => {
        const zoneMatched = zone.zoneName.toLowerCase().includes(q);
        const domains = zoneMatched
          ? zone.domains
          : zone.domains.filter((domain) => domain.toLowerCase().includes(q));
        return { ...zone, domains };
      })
      .filter((zone) => zone.domains.length > 0);
  }, [query, zones]);

  useEffect(() => {
    if (!open || zones.length === 0) return;
    setExpandedZones((prev) => {
      const next = { ...prev };
      let changed = false;
      for (const zone of zones) {
        if (next[zone.zoneId] === undefined) {
          next[zone.zoneId] = true;
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [open, zones]);

  useEffect(() => {
    const q = query.trim();
    if (!q || filteredZones.length === 0) return;
    setExpandedZones((prev) => {
      const next = { ...prev };
      for (const zone of filteredZones) {
        next[zone.zoneId] = true;
      }
      return next;
    });
  }, [filteredZones, query]);

  const toggleHost = (domain: string, checked: boolean | 'indeterminate') => {
    if (checked === true) {
      if (selectedSet.has(domain)) return;
      onHostsChange([...hosts, domain]);
      return;
    }
    onHostsChange(hosts.filter((item) => item !== domain));
  };

  const toggleZone = (
    zoneDomains: string[],
    checked: boolean | 'indeterminate',
  ) => {
    if (checked === true) {
      const next = new Set(hosts);
      for (const domain of zoneDomains) next.add(domain);
      onHostsChange(Array.from(next));
      return;
    }
    onHostsChange(hosts.filter((item) => !zoneDomains.includes(item)));
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery('');
      }}
    >
      <PopoverTrigger asChild>
        <Button
          type='button'
          variant='outline'
          size='icon'
          className={cn(
            'size-8 shrink-0',
            hosts.length > 0 ? 'border-primary text-primary' : undefined,
          )}
          title={
            hosts.length > 0 ? `已筛选 ${hosts.length} 个域名` : '按域名筛选'
          }
          aria-label={
            hosts.length > 0 ? `已筛选 ${hosts.length} 个域名` : '按域名筛选'
          }
        >
          <Filter className='size-3.5' />
        </Button>
      </PopoverTrigger>
      <PopoverContent align='end' className='w-96 space-y-3 p-3'>
        <div className='flex items-center justify-between gap-2'>
          <p className='text-sm font-medium'>筛选域名</p>
          {hosts.length > 0 ? (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-7 px-2 text-xs'
              onClick={() => onHostsChange([])}
            >
              <X className='mr-1 size-3' />
              清除
            </Button>
          ) : null}
        </div>
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder='搜索 Zone 或域名'
          className='h-8 text-xs'
        />
        <div className='max-h-72 space-y-2 overflow-y-auto hide-scrollbar'>
          {zonesQuery.isLoading ? (
            <div className='flex items-center justify-center gap-2 py-6 text-xs text-muted-foreground'>
              <Loader2 className='size-3.5 animate-spin' />
              加载域名…
            </div>
          ) : zonesQuery.isError ? (
            <div className='space-y-2 py-2'>
              <p className='text-xs text-destructive'>加载域名失败</p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-7 text-xs'
                onClick={() => void zonesQuery.refetch()}
              >
                重试
              </Button>
            </div>
          ) : filteredZones.length === 0 ? (
            <p className='py-6 text-center text-xs text-muted-foreground'>
              {zones.length === 0 ? '暂无已登记域名' : '没有匹配的域名'}
            </p>
          ) : (
            filteredZones.map((zone) => {
              const selectedCount = zone.domains.filter((domain) =>
                selectedSet.has(domain),
              ).length;
              const allSelected = selectedCount === zone.domains.length;
              const partialSelected =
                selectedCount > 0 && selectedCount < zone.domains.length;
              const expanded = expandedZones[zone.zoneId] ?? true;
              return (
                <Collapsible
                  key={zone.zoneId}
                  open={expanded}
                  onOpenChange={(next) =>
                    setExpandedZones((prev) => ({
                      ...prev,
                      [zone.zoneId]: next,
                    }))
                  }
                  className='rounded-md border border-dashed'
                >
                  <div className='flex items-center gap-1 px-2 py-1.5'>
                    <Checkbox
                      checked={
                        allSelected
                          ? true
                          : partialSelected
                            ? 'indeterminate'
                            : false
                      }
                      onCheckedChange={(checked) =>
                        toggleZone(zone.domains, checked)
                      }
                      aria-label={`选择 Zone ${zone.zoneName}`}
                    />
                    <CollapsibleTrigger asChild>
                      <button
                        type='button'
                        className='flex min-w-0 flex-1 items-center gap-1 rounded-md px-1 py-0.5 text-left text-xs font-medium hover:bg-accent'
                      >
                        <ChevronDown
                          className={cn(
                            'size-3.5 shrink-0 text-muted-foreground transition-transform',
                            expanded ? 'rotate-0' : '-rotate-90',
                          )}
                        />
                        <span className='min-w-0 flex-1 truncate'>
                          {zone.zoneName}
                        </span>
                        <span className='text-[10px] text-muted-foreground'>
                          {selectedCount}/{zone.domains.length}
                        </span>
                      </button>
                    </CollapsibleTrigger>
                  </div>
                  <CollapsibleContent>
                    <div className='space-y-0.5 border-t border-dashed px-2 py-1.5'>
                      {zone.domains.map((domain) => {
                        const selected = selectedSet.has(domain);
                        return (
                          <label
                            key={domain}
                            className={cn(
                              'flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-accent',
                              selected ? 'bg-accent/50' : undefined,
                            )}
                          >
                            <Checkbox
                              checked={selected}
                              onCheckedChange={(checked) =>
                                toggleHost(domain, checked)
                              }
                              aria-label={`选择域名 ${domain}`}
                            />
                            <span className='min-w-0 flex-1 truncate font-mono'>
                              {domain}
                            </span>
                          </label>
                        );
                      })}
                    </div>
                  </CollapsibleContent>
                </Collapsible>
              );
            })
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function OverviewToolbar({
  hours,
  hosts,
  onHoursChange,
  onHostsChange,
}: {
  hours: OverviewRangeHours;
  hosts: string[];
  onHoursChange: (hours: OverviewRangeHours) => void;
  onHostsChange: (hosts: string[]) => void;
}) {
  return (
    <div className='flex flex-wrap items-center justify-end gap-2'>
      {hosts.length > 0 ? (
        <Badge variant='secondary' className='max-w-[260px] truncate'>
          {hosts.length === 1 ? hosts[0] : `已选 ${hosts.length} 个域名`}
        </Badge>
      ) : null}
      <OverviewHostFilter hosts={hosts} onHostsChange={onHostsChange} />
      <ToggleGroup
        type='single'
        value={String(hours)}
        onValueChange={(value) => {
          if (!value) return;
          onHoursChange(Number.parseInt(value, 10) as OverviewRangeHours);
        }}
        variant='outline'
        size='sm'
        className='justify-end'
      >
        {OVERVIEW_RANGE_OPTIONS.map((option) => (
          <ToggleGroupItem
            key={option.value}
            value={String(option.value)}
            className='px-2.5 text-xs'
          >
            {option.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  );
}
