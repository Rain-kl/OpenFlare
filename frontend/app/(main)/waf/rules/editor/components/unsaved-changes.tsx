'use client';

import { useEffect } from 'react';
import { getHistoryTransition } from './editor-behavior';

const historyIndexKey = '__wafEditorIndex';

export function UnsavedChanges({ dirty }: { dirty: boolean }) {
  useEffect(() => {
    if (!dirty) return;
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
        { ...data, [historyIndexKey]: currentIndex },
        unused,
        url,
      );
    };
    history.replaceState = (data, unused, url) =>
      originalReplaceState(
        { ...data, [historyIndexKey]: currentIndex },
        unused,
        url,
      );
    let restoring = false;
    const handler = (event: BeforeUnloadEvent) => {
      if (dirty) event.preventDefault();
    };
    const clickHandler = (event: MouseEvent) => {
      if (
        !dirty ||
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      )
        return;
      const link = (event.target as Element | null)?.closest(
        'a[href]',
      ) as HTMLAnchorElement | null;
      if (
        !link ||
        link.target === '_blank' ||
        new URL(link.href, window.location.href).origin !==
          window.location.origin
      )
        return;
      if (!window.confirm('存在未保存的更改，确定离开吗？'))
        event.preventDefault();
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
      if (window.confirm('存在未保存的更改，确定离开吗？')) {
        currentIndex = targetIndex;
        return;
      }
      if (!hasTargetIndex) {
        originalPushState(
          { ...initialState, [historyIndexKey]: currentIndex },
          '',
          currentUrl,
        );
        return;
      }
      restoring = true;
      history.go(getHistoryTransition(currentIndex, targetIndex).restoreDelta);
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
  return null;
}
