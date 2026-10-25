import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { getLocale, getMessages, getTranslations } from 'next-intl/server';
import { Toaster } from '@/components/ui/sonner';
import { ThemeProvider } from '@/components/layout/theme-provider';
import { CustomThemeProvider } from '@/lib/theme';
import { BellRingProvider } from '@/contexts/bell-ring-context';
import { NotificationSettingsProvider } from '@/contexts/notification-settings-context';
import { UserProvider } from '@/contexts/user-context';
import { AppQueryProvider } from '@/components/providers/query-provider';
import { AppIntlProvider } from '@/components/providers/intl-provider';
import { SiteTitleUpdater } from '@/components/providers/title-updater';
import { RobotsMeta } from '@/components/layout/robots-meta';
import type { AppLocale } from '@/i18n/config';
import { createLocaleBootstrapScript } from '@/i18n/bootstrap';
import { Button } from '@/components/ui/button';
import './globals.css';

const inter = Inter({
  subsets: ['latin', 'cyrillic'],
  variable: '--font-inter',
  display: 'swap',
});

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations('metadata');
  return {
    title: t('title'),
    description: t('description'),
  };
}

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const locale = (await getLocale()) as AppLocale;
  const messages = await getMessages();
  const loading = await getTranslations('languageLoading');

  return (
    <html
      lang={locale}
      className={`hide-scrollbar font-sans ${inter.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: createLocaleBootstrapScript(locale),
          }}
        />
      </head>
      <body
        className='hide-scrollbar font-sans antialiased'
        suppressHydrationWarning
      >
        <div
          id='locale-load-fallback'
          role='alert'
          hidden
          className='visible flex min-h-svh flex-col items-center justify-center gap-4 p-6 text-center'
        >
          <p data-locale-error-message suppressHydrationWarning>
            {loading('failed')}
          </p>
          <Button type='button' suppressHydrationWarning>
            {loading('reload')}
          </Button>
        </div>
        <ThemeProvider
          attribute='class'
          defaultTheme='system'
          enableSystem
          disableTransitionOnChange
        >
          <CustomThemeProvider>
            <AppQueryProvider>
              <AppIntlProvider locale={locale} messages={messages}>
                <SiteTitleUpdater />
                <RobotsMeta />
                <UserProvider>
                  <NotificationSettingsProvider>
                    <BellRingProvider>
                      {children}
                      <Toaster position='top-center' />
                    </BellRingProvider>
                  </NotificationSettingsProvider>
                </UserProvider>
              </AppIntlProvider>
            </AppQueryProvider>
          </CustomThemeProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
