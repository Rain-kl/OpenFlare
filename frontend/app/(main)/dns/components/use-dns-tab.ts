'use client';

import { useSyncExternalStore } from 'react';

function subscribe(onChange: () => void) {
  window.addEventListener('hashchange', onChange);
  return () => window.removeEventListener('hashchange', onChange);
}

function getHash() {
  return window.location.hash.slice(1);
}

export function useDnsTab<T extends string>(tabs: readonly T[], fallback: T) {
  const hash = useSyncExternalStore(subscribe, getHash, () => '');
  const activeTab = tabs.find((tab) => tab === hash) ?? fallback;
  return {
    activeTab,
    setActiveTab: (value: string) => {
      window.location.hash = value;
    },
  };
}
