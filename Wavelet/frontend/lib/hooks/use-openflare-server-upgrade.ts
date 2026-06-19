'use client';

import {useMutation, useQuery} from '@tanstack/react-query';
import {useCallback, useEffect, useRef, useState} from 'react';

import {AdminStatusService} from '@/lib/services/admin';
import type {AppUpdateStatus} from '@/lib/services/admin/types';
import {StatusService} from '@/lib/services/openflare';

export const openflarePublicStatusQueryKey = ['openflare', 'public-status'] as const;

export const adminUpdateStatusQueryKey = ['admin', 'update'] as const;

export function useOpenFlareServerUpgrade({
  open,
  canUpgrade,
}: {
  open: boolean;
  canUpgrade: boolean;
}) {
  const [feedback, setFeedback] = useState<string | null>(null);

  const upgradeReloadStartedRef = useRef(false);
  const upgradeReloadTimerRef = useRef<number | null>(null);

  const statusQuery = useQuery({
    queryKey: openflarePublicStatusQueryKey,
    queryFn: () => StatusService.getPublicStatus(),
    enabled: open,
  });

  const updateQuery = useQuery({
    queryKey: adminUpdateStatusQueryKey,
    queryFn: () => AdminStatusService.getUpdateStatus(),
    enabled: open && canUpgrade,
    staleTime: 5 * 60 * 1000,
  });

  const scheduleUpgradePageReload = useCallback(() => {
    if (upgradeReloadStartedRef.current) {
      return;
    }

    upgradeReloadStartedRef.current = true;
    setFeedback('服务升级已进入重启阶段，页面将在服务恢复后自动刷新。');

    const reloadWhenServerReady = async () => {
      try {
        await StatusService.getPublicStatus();
        window.location.reload();
      } catch {
        upgradeReloadTimerRef.current = window.setTimeout(reloadWhenServerReady, 1500);
      }
    };

    upgradeReloadTimerRef.current = window.setTimeout(reloadWhenServerReady, 1200);
  }, []);

  useEffect(() => {
    return () => {
      if (upgradeReloadTimerRef.current !== null) {
        window.clearTimeout(upgradeReloadTimerRef.current);
      }
    };
  }, []);

  const upgradeMutation = useMutation({
    mutationFn: () => AdminStatusService.applyUpdate(),
    onSuccess: () => {
      scheduleUpgradePageReload();
      setFeedback('升级包已校验完成，服务正在重启。');
    },
    onError: (error) => {
      setFeedback(error instanceof Error ? error.message : '升级失败，请稍后重试。');
    },
  });

  const resetTransientState = useCallback(() => {
    setFeedback(null);
    upgradeReloadStartedRef.current = false;
  }, []);

  const handleOpen = useCallback(() => {
    resetTransientState();
    if (canUpgrade) {
      void updateQuery.refetch();
    }
  }, [canUpgrade, resetTransientState, updateQuery]);

  const handleCheckRelease = useCallback(() => {
    setFeedback(null);
    if (!canUpgrade) {
      return;
    }
    void updateQuery.refetch();
  }, [canUpgrade, updateQuery]);

  const handleUpgrade = useCallback(() => {
    setFeedback(null);
    upgradeMutation.mutate();
  }, [upgradeMutation]);

  const update = updateQuery.data;
  const releaseErrorMessage =
    feedback ||
    (updateQuery.isError
      ? updateQuery.error instanceof Error
        ? updateQuery.error.message
        : '版本检查失败，请稍后重试。'
      : undefined);

  const currentVersion = statusQuery.data?.version || update?.current_version || 'unknown';

  return {
    currentVersion,
    update,
    releaseErrorMessage,
    isInitialLoading: updateQuery.isLoading && !updateQuery.data && canUpgrade,
    isChecking: updateQuery.isFetching,
    isUpgrading: upgradeMutation.isPending,
    handleOpen,
    handleCheckRelease,
    handleUpgrade,
  };
}

export type {AppUpdateStatus};