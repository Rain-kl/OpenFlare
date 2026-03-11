'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { dashboardNavigation } from '@/lib/constants/navigation';
import { cn } from '@/lib/utils/cn';
import { isPathActive } from '@/lib/utils/navigation';
import { useAppShellStore } from '@/store/app-shell';

export function DashboardSidebar() {
  const pathname = usePathname();
  const currentPath = pathname ?? '/';
  const isSidebarCollapsed = useAppShellStore((state) => state.isSidebarCollapsed);

  return (
    <aside
      className={cn(
        'sticky top-0 hidden h-screen shrink-0 border-r border-[var(--border-default)] bg-[var(--surface-panel)]/95 px-3 py-6 backdrop-blur lg:block',
        isSidebarCollapsed ? 'w-24' : 'w-72',
      )}
    >
      <div className='flex h-full flex-col gap-6'>
        <div className='flex items-center gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] px-3 py-3'>
          <div className='flex h-11 w-11 items-center justify-center rounded-2xl bg-[var(--brand-primary-soft)] text-lg font-semibold text-[var(--brand-primary)]'>
            AF
          </div>
          {!isSidebarCollapsed ? (
            <div>
              <p className='text-sm font-semibold text-[var(--foreground-primary)]'>ATSFlare</p>
              <p className='text-xs text-[var(--foreground-secondary)]'>控制面新版工程</p>
            </div>
          ) : null}
        </div>

        <nav className='flex-1 space-y-2'>
          {dashboardNavigation.map((item) => {
          const active = isPathActive(currentPath, item.href);

            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  'flex items-start gap-3 rounded-2xl border px-3 py-3 transition-colors',
                  active
                    ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                    : 'border-transparent text-[var(--foreground-secondary)] hover:border-[var(--border-default)] hover:bg-[var(--surface-muted)] hover:text-[var(--foreground-primary)]',
                )}
              >
                <span className='mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-[var(--control-background)] text-xs font-semibold'>
                  {item.shortLabel.slice(0, 1)}
                </span>
                {!isSidebarCollapsed ? (
                  <span className='min-w-0'>
                    <span className='block text-sm font-medium'>{item.label}</span>
                    <span className='mt-1 block text-xs leading-5 text-inherit/80'>
                      {item.description}
                    </span>
                  </span>
                ) : null}
              </Link>
            );
          })}
        </nav>

        {!isSidebarCollapsed ? (
          <div className='rounded-2xl border border-[var(--status-success-border)] bg-[var(--status-success-soft)] px-4 py-4 text-xs leading-6 text-[var(--status-success-foreground)]'>
            阶段 4 已接入设置、用户、文件等边缘模块，旧前端仅保留少量兼容兜底价值。
          </div>
        ) : null}
      </div>
    </aside>
  );
}
