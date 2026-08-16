import { render, screen } from '@testing-library/react';
import { useTranslations } from 'next-intl';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import enMessages from '@/messages/en.json';
import zhCNMessages from '@/messages/zh-CN.json';

import { AppIntlProvider } from './intl-provider';

function Probe() {
  const t = useTranslations('common');
  return <button>{t('save')}</button>;
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

  it('does not flip without a cookie preference', () => {
    render(
      <AppIntlProvider locale='zh-CN' messages={zhCNMessages}>
        <Probe />
      </AppIntlProvider>,
    );

    expect(screen.getByText(zhSave)).toBeInTheDocument();
  });
});
