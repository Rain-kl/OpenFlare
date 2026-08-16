'use client';

import { useEffect, useState } from 'react';
import { NextIntlClientProvider } from 'next-intl';
import type { AbstractIntlMessages } from 'next-intl';
import type { ReactNode } from 'react';
import { resolveClientLocale } from '@/i18n/client';
import type { AppLocale } from '@/i18n/config';
import enMessages from '@/messages/en.json';
import zhCNMessages from '@/messages/zh-CN.json';

const messagesByLocale: Record<AppLocale, AbstractIntlMessages> = {
  'zh-CN': zhCNMessages,
  en: enMessages,
};

type Props = {
  locale: AppLocale;
  messages: AbstractIntlMessages;
  children: ReactNode;
};

export function AppIntlProvider({
  locale: ssrLocale,
  messages: ssrMessages,
  children,
}: Props) {
  // Static export (NEXT_STANDALONE_EXPORT) pre-renders every page at build time
  // with the default locale, so the SSR locale is stale at runtime. Re-resolve
  // from the NEXT_LOCALE cookie / browser language on the client. The first
  // render stays identical to SSR to avoid a hydration mismatch, then flips —
  // the brief flash matches the theme-hydration pattern (suppressHydrationWarning
  // is already set on <html>/<body>).
  const [clientLocale, setClientLocale] = useState<AppLocale | null>(null);

  useEffect(() => {
    const resolved = resolveClientLocale();
    if (resolved !== ssrLocale) {
      setClientLocale(resolved);
      document.documentElement.lang = resolved;
    }
  }, [ssrLocale]);

  const locale = clientLocale ?? ssrLocale;

  return (
    <NextIntlClientProvider
      locale={locale}
      messages={clientLocale ? messagesByLocale[clientLocale] : ssrMessages}
      timeZone='Asia/Shanghai'
    >
      {children}
    </NextIntlClientProvider>
  );
}
