'use client';

import { NextIntlClientProvider } from 'next-intl';
import type { AbstractIntlMessages } from 'next-intl';
import type { ReactNode } from 'react';
import type { AppLocale } from '@/i18n/config';

type Props = {
  locale: AppLocale;
  messages: AbstractIntlMessages;
  children: ReactNode;
};

export function AppIntlProvider({ locale, messages, children }: Props) {
  return (
    <NextIntlClientProvider
      locale={locale}
      messages={messages}
      timeZone='Asia/Shanghai'
    >
      {children}
    </NextIntlClientProvider>
  );
}
