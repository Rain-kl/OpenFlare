import { render, screen } from '@testing-library/react';
import { useLocale, useTranslations } from 'next-intl';
import { formatDateTime, formatNumber } from '@/i18n/format';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import enMessages from '@/messages/en.json';
import zhCNMessages from '@/messages/zh-CN.json';
import ruMessages from '@/messages/ru.json';

import { AppIntlProvider } from './intl-provider';
import { createLocaleBootstrapScript } from '@/i18n/bootstrap';

function Probe() {
  const t = useTranslations('common');
  return <button>{t('save')}</button>;
}

function FormattingProbe() {
  const locale = useLocale();
  return (
    <span data-testid='formats'>
      {locale}|{formatNumber(1234.5)}|{formatDateTime('2026-01-15T00:00:00Z')}
    </span>
  );
}

const zhSave = (zhCNMessages.common as Record<string, string>).save;
const enSave = (enMessages.common as Record<string, string>).save;

describe('AppIntlProvider (static export locale resolution)', () => {
  beforeEach(() => {
    // Deterministic browser language (jsdom defaults to en-US).
    Object.defineProperty(navigator, 'languages', {
      value: ['zh-CN'],
      configurable: true,
    });
    Object.defineProperty(navigator, 'language', {
      value: 'zh-CN',
      configurable: true,
    });
    document.documentElement.lang = 'zh-CN'; // as pre-rendered by SSR
  });

  afterEach(() => {
    document.cookie = 'NEXT_LOCALE=; Max-Age=0; Path=/';
    document.documentElement.removeAttribute('data-locale-pending');
  });

  it('flips to the NEXT_LOCALE cookie locale after mount when SSR locale is stale', async () => {
    // Static export pre-renders with the build-time default (zh-CN) even when
    // the user previously chose en — the exact reported bug.
    document.cookie = 'NEXT_LOCALE=en; Path=/';

    render(
      <AppIntlProvider locale='zh-CN' messages={zhCNMessages}>
        <Probe />
      </AppIntlProvider>,
    );

    expect(await screen.findByText(enSave)).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('en');
  });

  it('keeps the SSR locale when the cookie matches it', () => {
    document.cookie = 'NEXT_LOCALE=zh-CN; Path=/';

    render(
      <AppIntlProvider locale='zh-CN' messages={zhCNMessages}>
        <Probe />
      </AppIntlProvider>,
    );

    expect(screen.getByText(zhSave)).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('zh-CN');
  });

  it('applies Russian cookies before showing the hydrated English page', () => {
    document.cookie = 'NEXT_LOCALE=ru; Path=/';
    new Function(createLocaleBootstrapScript('en'))();
    expect(document.documentElement.hasAttribute('data-locale-pending')).toBe(
      true,
    );
    render(
      <AppIntlProvider locale='en' messages={enMessages}>
        <Probe />
      </AppIntlProvider>,
    );
    expect(screen.getByText(ruMessages.common.save)).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('ru');
    expect(document.documentElement.hasAttribute('data-locale-pending')).toBe(
      false,
    );
  });

  it('recognizes Russian browser variants when there is no preference', () => {
    Object.defineProperty(navigator, 'languages', {
      value: ['ru-RU', 'en-US'],
      configurable: true,
    });
    render(
      <AppIntlProvider locale='en' messages={enMessages}>
        <Probe />
      </AppIntlProvider>,
    );
    expect(screen.getByText(ruMessages.common.save)).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('ru');
  });

  it('formats existing data in Russian on the first localized render', () => {
    document.documentElement.lang = 'en';
    document.cookie = 'NEXT_LOCALE=ru; Path=/';
    render(
      <AppIntlProvider locale='en' messages={enMessages}>
        <FormattingProbe />
      </AppIntlProvider>,
    );
    expect(screen.getByTestId('formats').textContent).toBe(
      'ru|1\u00a0234,5|15.01.2026, 08:00',
    );
  });

  it('does not flip without a cookie preference', () => {
    render(
      <AppIntlProvider locale='zh-CN' messages={zhCNMessages}>
        <Probe />
      </AppIntlProvider>,
    );

    expect(screen.getByText(zhSave)).toBeInTheDocument();
  });
});
