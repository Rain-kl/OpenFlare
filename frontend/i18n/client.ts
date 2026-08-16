'use client';

import {
  defaultLocale,
  isAppLocale,
  localeCookieName,
  normalizeLocale,
  type AppLocale,
} from './config';

const COOKIE_MAX_AGE_SECONDS = 60 * 60 * 24 * 365; // 1 year

function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null;
  const match = document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${name}=`));
  if (!match) return null;
  return decodeURIComponent(match.slice(name.length + 1));
}

export function getClientLocalePreference(): AppLocale | null {
  const raw = readCookie(localeCookieName);
  if (isAppLocale(raw)) return raw;
  if (raw) return normalizeLocale(raw);
  return null;
}

export function detectBrowserLocale(): AppLocale {
  if (typeof navigator === 'undefined') return defaultLocale;
  const languages =
    navigator.languages?.length > 0
      ? navigator.languages
      : [navigator.language];
  for (const lang of languages) {
    const lower = (lang ?? '').toLowerCase();
    if (lower.startsWith('zh') || lower.startsWith('en')) {
      return normalizeLocale(lang);
    }
  }
  return defaultLocale;
}

export function resolveClientLocale(): AppLocale {
  return getClientLocalePreference() ?? detectBrowserLocale();
}

export function setLocaleCookie(locale: AppLocale): void {
  if (typeof document === 'undefined') return;
  const secure =
    typeof window !== 'undefined' && window.location.protocol === 'https:'
      ? '; Secure'
      : '';
  document.cookie = `${localeCookieName}=${encodeURIComponent(locale)}; Path=/; Max-Age=${COOKIE_MAX_AGE_SECONDS}; SameSite=Lax${secure}`;
  try {
    localStorage.setItem(localeCookieName, locale);
  } catch {
    // ignore quota / private mode
  }
}

/** Persist locale and reload so server + client messages stay in sync. */
export function switchLocale(locale: AppLocale): void {
  setLocaleCookie(locale);
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale;
  }
  if (typeof window !== 'undefined') {
    window.location.reload();
  }
}
