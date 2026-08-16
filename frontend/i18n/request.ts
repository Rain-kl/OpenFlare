import { cookies, headers } from 'next/headers';
import { getRequestConfig } from 'next-intl/server';
import {
  defaultLocale,
  isAppLocale,
  localeCookieName,
  normalizeLocale,
  pickLocaleFromAcceptLanguage,
  type AppLocale,
} from './config';

async function resolveRequestLocale(): Promise<AppLocale> {
  try {
    const cookieStore = await cookies();
    const cookieLocale = cookieStore.get(localeCookieName)?.value;
    if (isAppLocale(cookieLocale)) {
      return cookieLocale;
    }
    if (cookieLocale) {
      return normalizeLocale(cookieLocale);
    }
  } catch {
    // Static export / non-request contexts cannot read cookies.
  }

  try {
    const headerStore = await headers();
    const acceptLanguage = headerStore.get('accept-language');
    return pickLocaleFromAcceptLanguage(acceptLanguage);
  } catch {
    // Static export build time.
  }

  return defaultLocale;
}

export default getRequestConfig(async () => {
  const locale = await resolveRequestLocale();
  const messages = (await import(`../messages/${locale}.json`)).default;
  return {
    locale,
    messages,
  };
});
