'use client';

import { useLayoutEffect, useState } from 'react';
import { NextIntlClientProvider } from 'next-intl';
import type { AbstractIntlMessages } from 'next-intl';
import type { ReactNode } from 'react';
import { resolveClientLocale } from '@/i18n/client';
import type { AppLocale } from '@/i18n/config';
import enMessages from '@/messages/en.json';
import ruMessages from '@/messages/ru.json';
import zhCNMessages from '@/messages/zh-CN.json';

const messagesByLocale: Record<AppLocale, AbstractIntlMessages> = {
  'zh-CN': zhCNMessages,
  en: enMessages,
  ru: ruMessages,
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
  // Static exports start in the default English locale. Keep hydration aligned
  // with that HTML, then apply an explicit preference before the hydrated UI
  // paints instead of switching languages in a passive effect after paint.
  const [clientLocale, setClientLocale] = useState<AppLocale | null>(null);

  useLayoutEffect(() => {
    const resolved = resolveClientLocale();
    // Formatting utilities run during the context update's render. Publish
    // its locale first, while the prepaint gate still hides stale markup.
    document.documentElement.lang = resolved;
    if (resolved !== ssrLocale) {
      setClientLocale(resolved);
    }
  }, [ssrLocale]);

  const locale = clientLocale ?? ssrLocale;

  useLayoutEffect(() => {
    if (locale === resolveClientLocale()) {
      document.documentElement.lang = locale;
      const fallback = document.getElementById('locale-load-fallback');
      if (fallback) fallback.hidden = true;
      document.documentElement.removeAttribute('data-locale-pending');
    }
  }, [locale]);

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
