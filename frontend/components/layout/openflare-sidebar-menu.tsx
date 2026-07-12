'use client';

import Link from 'next/link';
import {usePathname} from 'next/navigation';
import {useEffect, useMemo, useState} from 'react';
import {ChevronRight} from 'lucide-react';

import {Collapsible, CollapsibleContent, CollapsibleTrigger} from '@/components/ui/collapsible';
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from '@/components/ui/sidebar';
import type {OpenFlareNavGroup} from '@/lib/navigation/openflare-nav';
import {
  matchesNavPath,
  openflareSidebarNav,
  isNavGroupActive,
} from '@/lib/navigation/openflare-nav';
import {usePublicConfig} from '@/hooks/use-public-config';

function parseMenuDisplayConfig(raw: string | undefined): Record<string, boolean> {
  if (!raw) return {};
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};

    return Object.entries(parsed).reduce<Record<string, boolean>>((result, [key, value]) => {
      if (typeof value === 'boolean') {
        result[key] = value;
      }
      return result;
    }, {});
  } catch {
    return {};
  }
}

function SidebarNavGroupMenuItem({
  group,
  pathname,
  onNavigate,
}: {
  group: OpenFlareNavGroup;
  pathname: string;
  onNavigate?: () => void;
}) {
  const groupActive = isNavGroupActive(pathname, group);
  const [open, setOpen] = useState(groupActive);

  useEffect(() => {
    if (groupActive) {
      setOpen(true);
    }
  }, [groupActive]);

  return (
    <Collapsible
      asChild
      open={open}
      onOpenChange={setOpen}
      className="group/collapsible"
    >
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton tooltip={group.title} isActive={groupActive}>
            <group.icon />
            <span>{group.title}</span>
            <ChevronRight className="ml-auto size-4 transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            {group.items.map((item) => (
              <SidebarMenuSubItem key={item.title}>
                <SidebarMenuSubButton
                  asChild
                  isActive={matchesNavPath(pathname, item.url, item.childUrls)}
                >
                  <Link href={item.url} onClick={onNavigate}>
                    <span>{item.title}</span>
                  </Link>
                </SidebarMenuSubButton>
              </SidebarMenuSubItem>
            ))}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
}

export function OpenFlareSidebarMenu({
  onNavigate,
}: {
  onNavigate?: () => void;
}) {
  const pathname = usePathname();
  const { config } = usePublicConfig();
  const displayConfig = useMemo(() => parseMenuDisplayConfig(config?.menu_display_config), [config]);

  return (
    <SidebarMenu className="gap-1">
      {openflareSidebarNav.map((entry) => {
        if (entry.kind === 'group') {
          const filteredItems = entry.items.filter((item) => displayConfig[item.url] !== false);
          if (filteredItems.length === 0) return null;

          return (
            <SidebarNavGroupMenuItem
              key={entry.title}
              group={{ ...entry, items: filteredItems }}
              pathname={pathname}
              onNavigate={onNavigate}
            />
          );
        }

        if (displayConfig[entry.url] === false) {
          return null;
        }

        return (
          <SidebarMenuItem key={entry.title}>
            <SidebarMenuButton
              tooltip={entry.title}
              isActive={matchesNavPath(pathname, entry.url, entry.childUrls)}
              asChild
            >
              <Link href={entry.url} onClick={onNavigate}>
                <entry.icon />
                <span>{entry.title}</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        );
      })}
    </SidebarMenu>
  );
}