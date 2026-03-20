'use client';

import { useEffect } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { dashboardNavigation } from '@/lib/constants/navigation';
import { cn } from '@/lib/utils/cn';
import { isNavigationItemActive } from '@/lib/utils/navigation';
import { useAppShellStore } from '@/store/app-shell';
import type { NavigationIconKey, NavigationItem } from '@/types/navigation';

function SidebarIcon({ icon }: { icon: NavigationIconKey }) {
  const commonProps = {
    className: 'h-[18px] w-[18px]',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.8,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    viewBox: '0 0 24 24',
  };

  switch (icon) {
    case 'home':
      return (
        <svg {...commonProps}>
          <path d="M3 10.5 12 3l9 7.5" />
          <path d="M5.5 9.5V21h13V9.5" />
          <path d="M9.5 21v-6h5v6" />
        </svg>
      );
    case 'node':
      return (
        <svg {...commonProps}>
          <rect x="4" y="4" width="6" height="6" rx="1.5" />
          <rect x="14" y="4" width="6" height="6" rx="1.5" />
          <rect x="9" y="14" width="6" height="6" rx="1.5" />
          <path d="M10 7h4M12 10v4" />
        </svg>
      );
    case 'website':
      return (
        <svg {...commonProps}>
          <circle cx="12" cy="12" r="8.5" />
          <path d="M3.5 12h17" />
          <path d="M12 3.5c2.4 2.2 3.8 5.2 3.8 8.5S14.4 18.3 12 20.5C9.6 18.3 8.2 15.3 8.2 12S9.6 5.7 12 3.5Z" />
        </svg>
      );
    case 'origin':
      return (
        <svg {...commonProps}>
          <path d="M12 4.5c3.3 0 6 2.5 6 5.6 0 4.4-6 9.4-6 9.4s-6-5-6-9.4c0-3.1 2.7-5.6 6-5.6Z" />
          <circle cx="12" cy="10.1" r="1.9" />
        </svg>
      );
    case 'domain':
      return (
        <svg {...commonProps}>
          <path d="M4 8.5h16" />
          <path d="M4 15.5h16" />
          <rect x="3.5" y="5" width="17" height="14" rx="3" />
        </svg>
      );
    case 'certificate':
      return (
        <svg {...commonProps}>
          <path d="M7 4.5h10l2 2V14a7 7 0 1 1-14 0V4.5Z" />
          <path d="M9 10h6" />
          <path d="M10 14h4" />
        </svg>
      );
    case 'proxy':
      return (
        <svg {...commonProps}>
          <path d="M7 7h10" />
          <path d="M7 12h10" />
          <path d="M7 17h6" />
          <path d="m14 15 3 2-3 2" />
        </svg>
      );
    case 'release':
      return (
        <svg {...commonProps}>
          <path d="M12 4v10" />
          <path d="m8.5 7.5 3.5-3.5 3.5 3.5" />
          <path d="M5 19.5h14" />
        </svg>
      );
    case 'log':
      return (
        <svg {...commonProps}>
          <rect x="5" y="4" width="14" height="16" rx="2.5" />
          <path d="M8 8h8" />
          <path d="M8 12h8" />
          <path d="M8 16h5" />
        </svg>
      );
    case 'performance':
      return (
        <svg {...commonProps}>
          <path d="M4.5 19.5h15" />
          <path d="M7 16l3-4 3 2 4-6" />
          <path d="m15.5 8 1.5 0 0 1.5" />
        </svg>
      );
    case 'user':
      return (
        <svg {...commonProps}>
          <circle cx="12" cy="8" r="3.25" />
          <path d="M5 19.5c1.7-3 4.1-4.5 7-4.5s5.3 1.5 7 4.5" />
        </svg>
      );
    case 'setting':
      return (
        <svg {...commonProps}>
          <circle cx="12" cy="12" r="3.5" />
          <path d="M12 3.5v2.2M12 18.3v2.2M20.5 12h-2.2M5.7 12H3.5M18.1 5.9l-1.6 1.6M7.5 16.5l-1.6 1.6M18.1 18.1l-1.6-1.6M7.5 7.5 5.9 5.9" />
        </svg>
      );
  }
}

function SidebarNavItem({
  item,
  currentPath,
  isSidebarCollapsed,
  forceExpanded,
  onNavigate,
  depth = 0,
}: {
  item: NavigationItem;
  currentPath: string;
  isSidebarCollapsed: boolean;
  forceExpanded?: boolean;
  onNavigate?: () => void;
  depth?: number;
}) {
  const active = isNavigationItemActive(currentPath, item);
  const hasChildren = Boolean(item.children?.length);
  const showLabel = forceExpanded || !isSidebarCollapsed;

  return (
    <div className="space-y-2">
      <Link
        href={item.href}
        onClick={onNavigate}
        className={cn(
          'flex min-h-[50px] items-center gap-3 rounded-2xl border px-3 py-2.5 transition-colors',
          depth > 0 && 'ml-3 rounded-xl',
          active
            ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
            : 'border-transparent text-[var(--foreground-secondary)] hover:border-[var(--border-default)] hover:bg-[var(--surface-muted)] hover:text-[var(--foreground-primary)]',
        )}
      >
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-[var(--control-background)] text-[var(--foreground-primary)]">
          <SidebarIcon icon={item.icon} />
        </span>
        {showLabel ? (
          <span className="min-w-0 flex-1 text-sm font-medium">
            {item.label}
          </span>
        ) : null}
      </Link>
      {showLabel && hasChildren ? (
        <div className="space-y-2">
          {item.children?.map((child) => (
            <SidebarNavItem
              key={child.href}
              item={child}
              currentPath={currentPath}
              isSidebarCollapsed={isSidebarCollapsed}
              forceExpanded={forceExpanded}
              onNavigate={onNavigate}
              depth={depth + 1}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function SidebarContent({
  currentPath,
  isSidebarCollapsed,
  forceExpanded = false,
  onNavigate,
}: {
  currentPath: string;
  isSidebarCollapsed: boolean;
  forceExpanded?: boolean;
  onNavigate?: () => void;
}) {
  const showLabel = forceExpanded || !isSidebarCollapsed;

  return (
    <div className="flex h-full flex-col gap-5">
      <div className="flex items-center gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] px-3 py-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-[var(--brand-primary-soft)] text-sm font-semibold text-[var(--brand-primary)]">
          AF
        </div>
        {showLabel ? (
          <div>
            <p className="text-sm font-semibold text-[var(--foreground-primary)]">
              OpenFlare
            </p>
            <p className="text-xs text-[var(--foreground-secondary)]">控制面</p>
          </div>
        ) : null}
      </div>

      <nav className="flex-1 space-y-2">
        <div className="flex max-h-full min-h-0 flex-col gap-2 overflow-y-auto pr-1">
          {dashboardNavigation.map((item) => (
            <SidebarNavItem
              key={item.href}
              item={item}
              currentPath={currentPath}
              isSidebarCollapsed={isSidebarCollapsed}
              forceExpanded={forceExpanded}
              onNavigate={onNavigate}
            />
          ))}
        </div>
      </nav>
    </div>
  );
}

export function DashboardSidebar() {
  const pathname = usePathname();
  const currentPath = pathname ?? '/';
  const isSidebarCollapsed = useAppShellStore(
    (state) => state.isSidebarCollapsed,
  );
  const isMobileSidebarOpen = useAppShellStore(
    (state) => state.isMobileSidebarOpen,
  );
  const setMobileSidebarOpen = useAppShellStore(
    (state) => state.setMobileSidebarOpen,
  );

  useEffect(() => {
    setMobileSidebarOpen(false);
  }, [currentPath, setMobileSidebarOpen]);

  return (
    <>
      <div
        className={cn(
          'fixed inset-0 z-30 bg-black/35 transition-opacity duration-200 min-[1000px]:hidden',
          isMobileSidebarOpen ? 'opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={() => setMobileSidebarOpen(false)}
        aria-hidden="true"
      />

      <aside
        className={cn(
          'fixed top-0 left-0 z-40 h-screen w-[200px] overflow-hidden border-r border-[var(--border-default)] bg-[var(--surface-panel)]/95 px-3 py-5 backdrop-blur transition-transform duration-200 min-[1000px]:hidden',
          isMobileSidebarOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <SidebarContent
          currentPath={currentPath}
          isSidebarCollapsed={false}
          forceExpanded
          onNavigate={() => setMobileSidebarOpen(false)}
        />
      </aside>

      <aside
        className={cn(
          'sticky top-0 z-10 hidden h-screen shrink-0 overflow-hidden border-r border-[var(--border-default)] bg-[var(--surface-panel)]/95 px-3 py-5 backdrop-blur min-[1000px]:block',
          isSidebarCollapsed ? 'w-[76px]' : 'w-[200px]',
        )}
      >
        <SidebarContent
          currentPath={currentPath}
          isSidebarCollapsed={isSidebarCollapsed}
        />
      </aside>
    </>
  );
}
