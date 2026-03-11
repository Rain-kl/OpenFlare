'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';

import { useAuth } from '@/components/providers/auth-provider';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { getCurrentNavigationItem } from '@/lib/utils/navigation';
import { publicEnv } from '@/lib/env/public-env';
import { useAppShellStore } from '@/store/app-shell';
import { usePathname } from 'next/navigation';

export function DashboardTopbar() {
  const router = useRouter();
  const pathname = usePathname();
  const currentPath = pathname ?? '/';
  const { logout, user } = useAuth();
  const toggleSidebar = useAppShellStore((state) => state.toggleSidebar);
  const currentItem = getCurrentNavigationItem(currentPath);
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    await logout();
    router.replace('/login');
  };

  return (
    <header className='sticky top-0 z-10 border-b border-[var(--border-default)] bg-[var(--surface-panel)]/75 px-4 py-4 backdrop-blur md:px-8'>
      <div className='flex flex-col gap-4 md:flex-row md:items-center md:justify-between'>
        <div className='flex items-center gap-3'>
          <button
            type='button'
            onClick={toggleSidebar}
            className='inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] text-lg text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]'
            aria-label='切换侧边栏'
          >
            ≡
          </button>
          <div>
            <p className='text-xs uppercase tracking-[0.24em] text-[var(--foreground-secondary)]'>当前模块</p>
            <h2 className='text-lg font-semibold text-[var(--foreground-primary)]'>
              {currentItem?.label ?? 'ATSFlare 控制台'}
            </h2>
          </div>
        </div>

        <div className='flex items-center gap-3 text-sm text-[var(--foreground-secondary)]'>
          {user ? (
            <span className='hidden rounded-full border border-[var(--border-default)] px-3 py-1.5 md:inline-flex'>
              {user.display_name || user.username}
            </span>
          ) : null}
          <ThemeToggle />
          <span className='rounded-full border border-[var(--border-default)] px-3 py-1.5'>
            静态导出模式
          </span>
          <span className='rounded-full border border-[var(--border-default)] px-3 py-1.5'>
            版本 {publicEnv.appVersion}
          </span>
          <button
            type='button'
            onClick={() => void handleLogout()}
            disabled={isLoggingOut}
            className='inline-flex rounded-full border border-[var(--border-default)] bg-[var(--control-background)] px-3 py-1.5 text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)] disabled:cursor-not-allowed disabled:opacity-60'
          >
            {isLoggingOut ? '退出中...' : '退出登录'}
          </button>
        </div>
      </div>
    </header>
  );
}
