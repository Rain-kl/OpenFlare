'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';

import { useAuth } from '@/components/providers/auth-provider';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { publicEnv } from '@/lib/env/public-env';
import { useAppShellStore } from '@/store/app-shell';

export function DashboardTopbar() {
  const router = useRouter();
  const { logout, user } = useAuth();
  const toggleSidebar = useAppShellStore((state) => state.toggleSidebar);
  const isMobileSidebarOpen = useAppShellStore((state) => state.isMobileSidebarOpen);
  const setMobileSidebarOpen = useAppShellStore((state) => state.setMobileSidebarOpen);
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!isUserMenuOpen) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setIsUserMenuOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsUserMenuOpen(false);
      }
    };

    window.addEventListener('mousedown', handlePointerDown);
    window.addEventListener('keydown', handleEscape);

    return () => {
      window.removeEventListener('mousedown', handlePointerDown);
      window.removeEventListener('keydown', handleEscape);
    };
  }, [isUserMenuOpen]);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    setIsUserMenuOpen(false);
    await logout();
    router.replace('/login');
  };

  const handleSidebarToggle = () => {
    if (window.innerWidth < 1000) {
      setMobileSidebarOpen(!isMobileSidebarOpen);
      return;
    }

    toggleSidebar();
  };

  return (
    <header className='sticky top-0 z-20 border-b border-[var(--border-default)] bg-[var(--surface-panel)]/88 px-4 py-4 backdrop-blur md:px-8'>
      <div className='flex items-center justify-between gap-3'>
        <button
          type='button'
          onClick={handleSidebarToggle}
          className='inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] text-lg text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]'
          aria-label='切换侧边栏'
        >
          ☰
        </button>

        <div className='flex items-center gap-3 text-sm text-[var(--foreground-secondary)]'>
          <span className='hidden rounded-full border border-[var(--border-default)] px-3 py-1.5 sm:inline-flex'>
            版本 {publicEnv.appVersion}
          </span>
          <ThemeToggle />
          <div className='relative' ref={menuRef}>
            <button
              type='button'
              onClick={() => setIsUserMenuOpen((value) => !value)}
              className='inline-flex h-11 items-center gap-2 rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-3 text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]'
              aria-expanded={isUserMenuOpen}
              aria-haspopup='menu'
            >
              <span className='inline-flex h-7 w-7 items-center justify-center rounded-full bg-[var(--accent-soft)] text-xs font-semibold'>
                {(user?.display_name || user?.username || 'U').slice(0, 1).toUpperCase()}
              </span>
              <span className='hidden sm:inline'>{user?.display_name || user?.username || '用户'}</span>
            </button>

            {isUserMenuOpen ? (
              <div className='absolute right-0 top-[calc(100%+0.5rem)] w-52 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] p-2 shadow-[var(--shadow-lg)]'>
                <div className='rounded-xl px-3 py-2'>
                  <p className='text-sm font-semibold text-[var(--foreground-primary)]'>
                    {user?.display_name || user?.username || '用户'}
                  </p>
                  {user?.username ? (
                    <p className='mt-1 text-xs text-[var(--foreground-secondary)]'>@{user.username}</p>
                  ) : null}
                </div>
                <button
                  type='button'
                  onClick={() => void handleLogout()}
                  disabled={isLoggingOut}
                  className='flex w-full items-center rounded-xl px-3 py-2 text-left text-sm text-[var(--status-danger-foreground)] transition hover:bg-[var(--status-danger-soft)] disabled:cursor-not-allowed disabled:opacity-60'
                >
                  {isLoggingOut ? '退出中...' : '退出登录'}
                </button>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </header>
  );
}
