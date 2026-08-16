'use client';

import { useTranslations } from 'next-intl';
import { useEffect, useRef, useState } from 'react';

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';

import { getHistoryTransition } from './editor-behavior';

const historyIndexKey = '__wafEditorIndex';

type PendingLeave =
  | { kind: 'href'; href: string }
  | { kind: 'history'; restoreDelta: number };

export function UnsavedChanges({ dirty }: { dirty: boolean }) {
  const t = useTranslations('waf.editor');
  const tCommon = useTranslations('common');
  const [pendingLeave, setPendingLeave] = useState<PendingLeave | null>(null);
  const dirtyRef = useRef(dirty);
  const allowLeaveRef = useRef(false);

  dirtyRef.current = dirty;

  useEffect(() => {
    if (!dirty) {
      setPendingLeave(null);
      return;
    }

    const initialState =
      history.state && typeof history.state === 'object' ? history.state : {};
    let currentIndex = Number.isInteger(initialState[historyIndexKey])
      ? (initialState[historyIndexKey] as number)
      : 0;
    const currentUrl =
      window.location.pathname + window.location.search + window.location.hash;

    history.replaceState(
      { ...initialState, [historyIndexKey]: currentIndex },
      '',
    );

    const originalPushState = history.pushState.bind(history);
    const originalReplaceState = history.replaceState.bind(history);

    history.pushState = (data, unused, url) => {
      currentIndex++;
      originalPushState(
        { ...(data as object), [historyIndexKey]: currentIndex },
        unused,
        url,
      );
    };
    history.replaceState = (data, unused, url) =>
      originalReplaceState(
        { ...(data as object), [historyIndexKey]: currentIndex },
        unused,
        url,
      );

    let restoring = false;

    const handler = (event: BeforeUnloadEvent) => {
      if (dirtyRef.current && !allowLeaveRef.current) {
        event.preventDefault();
      }
    };

    const clickHandler = (event: MouseEvent) => {
      if (
        !dirtyRef.current ||
        allowLeaveRef.current ||
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      ) {
        return;
      }
      const link = (event.target as Element | null)?.closest(
        'a[href]',
      ) as HTMLAnchorElement | null;
      if (
        !link ||
        link.target === '_blank' ||
        new URL(link.href, window.location.href).origin !==
          window.location.origin
      ) {
        return;
      }
      event.preventDefault();
      setPendingLeave({ kind: 'href', href: link.href });
    };

    const popstateHandler = (event: PopStateEvent) => {
      const hasTargetIndex = Number.isInteger(event.state?.[historyIndexKey]);
      const targetIndex = hasTargetIndex
        ? (event.state[historyIndexKey] as number)
        : currentIndex;

      if (restoring) {
        restoring = false;
        currentIndex = targetIndex;
        return;
      }

      if (allowLeaveRef.current || !dirtyRef.current) {
        currentIndex = targetIndex;
        return;
      }

      // Browser already navigated; restore immediately and ask for confirmation.
      if (!hasTargetIndex) {
        const destination =
          window.location.pathname +
          window.location.search +
          window.location.hash;
        originalPushState(
          { ...initialState, [historyIndexKey]: currentIndex },
          '',
          currentUrl,
        );
        setPendingLeave({ kind: 'href', href: destination });
        return;
      }

      const { restoreDelta } = getHistoryTransition(currentIndex, targetIndex);
      restoring = true;
      history.go(restoreDelta);
      setPendingLeave({ kind: 'history', restoreDelta });
    };

    window.addEventListener('beforeunload', handler);
    document.addEventListener('click', clickHandler, true);
    window.addEventListener('popstate', popstateHandler);

    return () => {
      history.pushState = originalPushState;
      history.replaceState = originalReplaceState;
      window.removeEventListener('beforeunload', handler);
      document.removeEventListener('click', clickHandler, true);
      window.removeEventListener('popstate', popstateHandler);
    };
  }, [dirty]);

  const handleConfirmLeave = () => {
    if (!pendingLeave) return;

    allowLeaveRef.current = true;
    const leave = pendingLeave;
    setPendingLeave(null);

    if (leave.kind === 'href') {
      window.location.assign(leave.href);
      return;
    }

    // Re-apply the history navigation the user originally attempted.
    const delta = -leave.restoreDelta;
    if (delta !== 0) {
      history.go(delta);
    }
  };

  return (
    <AlertDialog
      open={Boolean(pendingLeave)}
      onOpenChange={(open) => {
        if (!open) setPendingLeave(null);
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('leaveTitle')}</AlertDialogTitle>
          <AlertDialogDescription>{t('leaveDesc')}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{tCommon('cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirmLeave}>
            {t('leaveConfirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
