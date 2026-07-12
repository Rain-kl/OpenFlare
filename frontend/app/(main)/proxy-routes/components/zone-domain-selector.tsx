'use client';

import {useMemo, useState} from 'react';
import Link from 'next/link';
import {ExternalLink} from 'lucide-react';

import {Badge} from '@/components/ui/badge';
import {Checkbox} from '@/components/ui/checkbox';
import {Input} from '@/components/ui/input';
import {cn} from '@/lib/utils';
import type {ZoneDomainItem, ZoneItem} from '@/lib/services/openflare';

export interface ZoneDomainSelectorProps {
  value: number[];
  onChange: (ids: number[]) => void;
  domains: ZoneDomainItem[];
  zones?: ZoneItem[];
  /** Current route ID: domains bound to this route remain selectable. */
  currentRouteId?: number | null;
  disabled?: boolean;
  className?: string;
}

export function ZoneDomainSelector({
  value,
  onChange,
  domains,
  zones = [],
  currentRouteId = null,
  disabled = false,
  className,
}: ZoneDomainSelectorProps) {
  const [keyword, setKeyword] = useState('');
  const selected = useMemo(() => new Set(value), [value]);
  const zoneMap = useMemo(
    () => new Map(zones.map((zone) => [zone.id, zone.domain])),
    [zones],
  );

  const filtered = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    const list = [...domains].sort((a, b) => a.domain.localeCompare(b.domain));
    if (!normalized) {
      return list;
    }
    return list.filter((domain) => {
      const zoneRoot = zoneMap.get(domain.zone_id) ?? '';
      return (
        domain.domain.toLowerCase().includes(normalized) ||
        zoneRoot.toLowerCase().includes(normalized) ||
        String(domain.id).includes(normalized)
      );
    });
  }, [domains, keyword, zoneMap]);

  const toggle = (domain: ZoneDomainItem) => {
    if (disabled) {
      return;
    }
    const boundElsewhere =
      domain.proxy_route_id != null &&
      domain.proxy_route_id !== currentRouteId;
    if (boundElsewhere) {
      return;
    }

    if (selected.has(domain.id)) {
      onChange(value.filter((id) => id !== domain.id));
      return;
    }
    onChange([...value, domain.id].sort((a, b) => a - b));
  };

  return (
    <div className={cn('space-y-3', className)}>
      <Input
        value={keyword}
        onChange={(event) => setKeyword(event.target.value)}
        placeholder="搜索域名或 Zone…"
        disabled={disabled}
      />

      {filtered.length === 0 ? (
        <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          {domains.length === 0 ? (
            <>
              暂无可用 Zone 域名。请先在{' '}
              <Link href="/websites" className="text-primary underline-offset-4 hover:underline">
                网站管理
              </Link>{' '}
              中添加 FQDN。
            </>
          ) : (
            '没有匹配的域名'
          )}
        </div>
      ) : (
        <div className="max-h-72 space-y-1 overflow-y-auto rounded-lg border p-2">
          {filtered.map((domain) => {
            const boundElsewhere =
              domain.proxy_route_id != null &&
              domain.proxy_route_id !== currentRouteId;
            const checked = selected.has(domain.id);
            const zoneRoot = zoneMap.get(domain.zone_id);

            return (
              <label
                key={domain.id}
                className={cn(
                  'flex cursor-pointer items-start gap-3 rounded-md px-2 py-2 transition-colors hover:bg-muted/60',
                  boundElsewhere && 'cursor-not-allowed opacity-60',
                  checked && !boundElsewhere && 'bg-muted/40',
                )}
              >
                <Checkbox
                  checked={checked}
                  disabled={disabled || boundElsewhere}
                  onCheckedChange={() => toggle(domain)}
                  className="mt-0.5"
                />
                <span className="min-w-0 flex-1 space-y-1">
                  <span className="flex flex-wrap items-center gap-2">
                    <span className="truncate text-sm font-medium">{domain.domain}</span>
                    {domain.cert_id ? (
                      <Badge variant="secondary" className="text-[10px]">
                        证书 #{domain.cert_id}
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="text-[10px]">
                        无证书
                      </Badge>
                    )}
                    {boundElsewhere ? (
                      <Badge variant="outline" className="text-[10px]">
                        路由 #{domain.proxy_route_id}
                      </Badge>
                    ) : null}
                  </span>
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    {zoneRoot ? (
                      <Link
                        href={`/websites/${domain.zone_id}`}
                        className="inline-flex items-center gap-0.5 hover:text-foreground"
                        onClick={(event) => event.stopPropagation()}
                      >
                        Zone {zoneRoot}
                        <ExternalLink className="size-3" />
                      </Link>
                    ) : (
                      <span>Zone #{domain.zone_id}</span>
                    )}
                  </span>
                </span>
              </label>
            );
          })}
        </div>
      )}
    </div>
  );
}
