'use client';

import type { useTranslations } from 'next-intl';
import { resolveClientLocale } from './client';
import { getRuntimeTranslator, type RuntimeKey } from './runtime-translators';

export { getRuntimeTranslator } from './runtime-translators';

export type RuntimeTranslator = ReturnType<typeof useTranslations>;

// Imperative browser code (network errors, form utilities) has no React hook
// context. Resolve the current preference when creating a message, not at import.
export const runtimeText = Object.assign(
  (key: RuntimeKey, values?: Record<string, string | number>) =>
    getRuntimeTranslator(resolveClientLocale())(key, values),
  {
    raw: (key: RuntimeKey): string =>
      getRuntimeTranslator(resolveClientLocale()).raw(key),
  },
);
