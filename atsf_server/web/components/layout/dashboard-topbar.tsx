'use client';

import { useMutation, useQuery } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';

import { useAuth } from '@/components/providers/auth-provider';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { getPublicStatus } from '@/features/auth/api/public';
import { getLatestRelease, upgradeServer } from '@/features/update/api/update';
import { VersionUpgradeModal } from '@/features/update/components/version-upgrade-modal';
import { publicEnv } from '@/lib/env/public-env';
import { useAppShellStore } from '@/store/app-shell';

export function DashboardTopbar() {
  const router = useRouter();
  const { logout, user } = useAuth();
  const toggleSidebar = useAppShellStore((state) => state.toggleSidebar);
  const isMobileSidebarOpen = useAppShellStore(
    (state) => state.isMobileSidebarOpen,
  );
  const setMobileSidebarOpen = useAppShellStore(
    (state) => state.setMobileSidebarOpen,
  );
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const [isVersionModalOpen, setIsVersionModalOpen] = useState(false);
  const [versionFeedback, setVersionFeedback] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const isRoot = (user?.role ?? 0) >= 100;

  const publicStatusQuery = useQuery({
    queryKey: ['public-status'],
    queryFn: getPublicStatus,
  });

  const latestReleaseQuery = useQuery({
    queryKey: ['update', 'latest-release'],
    queryFn: getLatestRelease,
    enabled: isRoot,
    refetchInterval: 60 * 60 * 1000,
  });

  const upgradeMutation = useMutation({
    mutationFn: upgradeServer,
    onSuccess: (release) => {
      setVersionFeedback(
        `服务升级任务已启动，目标版本 ${release.tag_name}。页面可能短暂不可用。`,
      );
      void latestReleaseQuery.refetch();
    },
    onError: (error) => {
      setVersionFeedback(
        error instanceof Error ? error.message : '升级失败，请稍后重试。',
      );
    },
  });

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

  const handleOpenVersionModal = () => {
    setVersionFeedback(null);
    setIsVersionModalOpen(true);
    if (isRoot) {
      void latestReleaseQuery.refetch();
    }
  };

  const handleUpgrade = () => {
    setVersionFeedback(null);
    upgradeMutation.mutate();
  };

  const release = latestReleaseQuery.data;
  const hasUpdate = Boolean(isRoot && release?.has_update);
  const currentVersion = publicStatusQuery.data?.version || 'unknown';
  const versionLabel = hasUpdate
    ? `版本 ${publicEnv.appVersion} · 可升级`
    : `版本 ${publicEnv.appVersion}`;
  const versionButtonClassName = hasUpdate
    ? 'border-[var(--status-warning-border)] bg-[var(--status-warning-soft)] text-[var(--status-warning-foreground)]'
    : 'border-[var(--border-default)]';
  const versionErrorMessage =
    versionFeedback ||
    (latestReleaseQuery.isError
      ? latestReleaseQuery.error instanceof Error
        ? latestReleaseQuery.error.message
        : '版本检查失败，请稍后重试。'
      : undefined);

  return (
    <>
      <header className="sticky top-0 z-20 border-b border-[var(--border-default)] bg-[var(--surface-panel)]/88 px-4 py-4 backdrop-blur md:px-8">
        <div className="flex items-center justify-between gap-3">
          <button
            type="button"
            onClick={handleSidebarToggle}
            className="inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] text-lg text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
            aria-label="切换侧边栏"
          >
            ☰
          </button>

          <div className="flex items-center gap-3 text-sm text-[var(--foreground-secondary)]">
            <button
              type="button"
              onClick={handleOpenVersionModal}
              className={[
                'inline-flex rounded-full border px-3 py-1.5 transition',
                versionButtonClassName,
              ].join(' ')}
            >
              <span className="sm:hidden">版本</span>
              <span className="hidden sm:inline">{versionLabel}</span>
            </button>
            <ThemeToggle />
            <div className="relative" ref={menuRef}>
              <button
                type="button"
                onClick={() => setIsUserMenuOpen((value) => !value)}
                className="inline-flex h-11 items-center gap-2 rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-3 text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
                aria-expanded={isUserMenuOpen}
                aria-haspopup="menu"
              >
                <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-[var(--accent-soft)] text-xs font-semibold">
                  {(user?.display_name || user?.username || 'U')
                    .slice(0, 1)
                    .toUpperCase()}
                </span>
                <span className="hidden sm:inline">
                  {user?.display_name || user?.username || '用户'}
                </span>
              </button>

              {isUserMenuOpen ? (
                <div className="absolute top-[calc(100%+0.5rem)] right-0 w-52 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] p-2 shadow-[var(--shadow-lg)]">
                  <div className="rounded-xl px-3 py-2">
                    <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                      {user?.display_name || user?.username || '用户'}
                    </p>
                    {user?.username ? (
                      <p className="mt-1 text-xs text-[var(--foreground-secondary)]">
                        @{user.username}
                      </p>
                    ) : null}
                  </div>
                  <button
                    type="button"
                    onClick={() => void handleLogout()}
                    disabled={isLoggingOut}
                    className="flex w-full items-center rounded-xl px-3 py-2 text-left text-sm text-[var(--status-danger-foreground)] transition hover:bg-[var(--status-danger-soft)] disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {isLoggingOut ? '退出中...' : '退出登录'}
                  </button>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </header>

      <VersionUpgradeModal
        isOpen={isVersionModalOpen}
        onClose={() => setIsVersionModalOpen(false)}
        currentVersion={currentVersion}
        frontendVersion={publicEnv.appVersion}
        startTime={publicStatusQuery.data?.start_time}
        release={release}
        isLoading={latestReleaseQuery.isLoading && !release && isRoot}
        errorMessage={versionErrorMessage}
        canUpgrade={isRoot}
        isChecking={latestReleaseQuery.isFetching}
        isUpgrading={upgradeMutation.isPending}
        onRefresh={() => {
          setVersionFeedback(null);
          if (isRoot) {
            void latestReleaseQuery.refetch();
          }
        }}
        onUpgrade={handleUpgrade}
      />
    </>
  );
}
