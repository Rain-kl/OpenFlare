import { afterEach, describe, expect, it, vi } from 'vitest';
import english from '@/messages/fragments/languages.en.json';
import russian from '@/messages/fragments/languages.ru.json';
import { createLocaleBootstrapScript } from './bootstrap';
import { resolveClientLocale } from './client';

afterEach(async () => {
  document.cookie = 'NEXT_LOCALE=; Max-Age=0; Path=/';
  document.documentElement.removeAttribute('data-locale-pending');
  document.getElementById('locale-load-fallback')?.remove();
  await Promise.resolve();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('static locale first paint', () => {
  function setupFallback() {
    const fallback = document.createElement('div');
    fallback.id = 'locale-load-fallback';
    fallback.hidden = true;
    const message = document.createElement('p');
    message.setAttribute('data-locale-error-message', '');
    message.textContent = english.languageLoading.failed;
    const retry = document.createElement('button');
    retry.textContent = english.languageLoading.reload;
    fallback.append(message, retry);
    document.body.append(fallback);
    document.cookie = 'NEXT_LOCALE=ru; Path=/';
    return fallback;
  }

  it('shows an independent Russian recovery action if hydration never completes', () => {
    vi.useFakeTimers();
    const fallback = setupFallback();
    new Function(createLocaleBootstrapScript('en'))();
    vi.advanceTimersByTime(10000);
    expect(fallback.hidden).toBe(false);
    expect(fallback.textContent).toContain(russian.languageLoading.failed);
    expect(fallback.querySelector('button')?.textContent).toBe(
      russian.languageLoading.reload,
    );
    expect(document.documentElement.hasAttribute('data-locale-pending')).toBe(
      true,
    );
  });

  it('shows recovery immediately when an application script fails', () => {
    vi.useFakeTimers();
    const fallback = setupFallback();
    new Function(createLocaleBootstrapScript('en'))();
    const script = document.createElement('script');
    document.body.append(script);
    script.dispatchEvent(new Event('error'));
    script.remove();
    expect(fallback.hidden).toBe(false);
  });

  it('cancels the deadline when the provider reveals the localized page', async () => {
    vi.useFakeTimers();
    const fallback = setupFallback();
    new Function(createLocaleBootstrapScript('en'))();
    document.documentElement.removeAttribute('data-locale-pending');
    await Promise.resolve();
    vi.advanceTimersByTime(60000);
    expect(fallback.hidden).toBe(true);
  });

  it('waits for the body when a script fails during HTML parsing', () => {
    vi.useFakeTimers();
    vi.spyOn(document, 'readyState', 'get').mockReturnValue('loading');
    document.cookie = 'NEXT_LOCALE=ru; Path=/';
    new Function(createLocaleBootstrapScript('en'))();
    const script = document.createElement('script');
    document.head.append(script);
    script.dispatchEvent(new Event('error'));
    script.remove();
    expect(document.documentElement.hasAttribute('data-locale-pending')).toBe(
      true,
    );
    const fallback = setupFallback();
    document.dispatchEvent(new Event('DOMContentLoaded'));
    expect(fallback.hidden).toBe(false);
    expect(fallback.textContent).toContain(russian.languageLoading.failed);
  });

  it.each([
    ['ru', ['en-US'], 'ru', true],
    ['en', ['ru-RU'], 'en', false],
    ['', ['de-DE', 'ru-RU'], 'ru', true],
    ['', ['de-DE', 'en-US'], 'en', false],
    ['zh-CN', ['en-US'], 'zh-CN', true],
    ['unsupported', ['ru-RU'], 'en', false],
    ['%broken', ['ru-RU'], 'ru', true],
  ] as const)(
    'matches provider resolution for cookie %s and browser %s',
    (cookie, languages, locale, pending) => {
      document.cookie = `NEXT_LOCALE=${cookie}; Path=/`;
      Object.defineProperty(navigator, 'languages', {
        value: languages,
        configurable: true,
      });
      new Function(createLocaleBootstrapScript('en'))();
      expect(resolveClientLocale()).toBe(locale);
      expect(document.documentElement.hasAttribute('data-locale-pending')).toBe(
        pending,
      );
    },
  );
});
