import {
  defaultLocale,
  localeCookieName,
  locales,
  type AppLocale,
} from './config';
import english from '@/messages/fragments/languages.en.json';
import chinese from '@/messages/fragments/languages.zh-CN.json';
import russian from '@/messages/fragments/languages.ru.json';

// Runs in <head>, before static English markup can paint. Keep this script
// self-contained: it must not depend on a loaded application bundle.
export function createLocaleBootstrapScript(serverLocale: AppLocale): string {
  const settings = JSON.stringify({
    serverLocale,
    defaultLocale,
    localeCookieName,
    locales,
    fallbacks: {
      en: english.languageLoading,
      'zh-CN': chinese.languageLoading,
      ru: russian.languageLoading,
    },
  }).replaceAll('<', '\\u003c');
  return String.raw`(() => {
    const settings = ${settings};
    const normalize = (input) => {
      const family = input.trim().replaceAll('_', '-').toLowerCase().split('-')[0];
      return settings.locales.find((locale) => locale.toLowerCase().split('-')[0] === family);
    };
    const cookie = document.cookie.split(';').map((part) => part.trim()).find((part) => part.startsWith(settings.localeCookieName + '='));
    let preferred;
    if (cookie) {
      try {
        const value = decodeURIComponent(cookie.slice(settings.localeCookieName.length + 1));
        if (value) preferred = normalize(value) || settings.defaultLocale;
      } catch { /* Invalid cookie: use browser preference, as the provider does. */ }
    }
    if (!preferred) {
      for (const language of navigator.languages?.length ? navigator.languages : [navigator.language]) {
        preferred = normalize(language || '');
        if (preferred) break;
      }
    }
    if ((preferred || settings.defaultLocale) !== settings.serverLocale) {
      const root = document.documentElement;
      root.setAttribute('data-locale-pending', '');
      const showFailure = () => {
        if (!root.hasAttribute('data-locale-pending')) return;
        const fallback = document.getElementById('locale-load-fallback');
        if (!fallback) {
          if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', showFailure, {once: true});
          } else {
            root.removeAttribute('data-locale-pending');
          }
          return;
        }
        if (!fallback.hidden) return;
        const copy = settings.fallbacks[preferred || settings.defaultLocale];
        const message = fallback.querySelector('[data-locale-error-message]');
        const retry = fallback.querySelector('button');
        if (message) message.textContent = copy.failed;
        if (retry) {
          retry.textContent = copy.reload;
          retry.addEventListener('click', () => window.location.reload(), {once: true});
        }
        fallback.hidden = false;
      };
      const timeout = setTimeout(showFailure, 10000);
      const onError = (event) => {
        if (event.target?.tagName === 'SCRIPT') showFailure();
      };
      window.addEventListener('error', onError, true);
      const observer = new MutationObserver(() => {
        if (!root.hasAttribute('data-locale-pending')) {
          clearTimeout(timeout);
          window.removeEventListener('error', onError, true);
          observer.disconnect();
        }
      });
      observer.observe(root, {attributes: true, attributeFilter: ['data-locale-pending']});
    }
  })();`;
}
