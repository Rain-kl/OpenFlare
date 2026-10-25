import { describe, expect, it } from 'vitest';
import { messageContractTokens } from '../scripts/message-contract.mjs';
import english from '@/messages/en.json';
import chinese from '@/messages/zh-CN.json';
import russian from '@/messages/ru.json';

describe('translation rendering contracts', () => {
  it('distinguishes translated renderer identifiers from translated prose', () => {
    const source = '$title <websites>Websites</websites> $content';
    expect(
      messageContractTokens('$title <websites>Сайты</websites> $content'),
    ).toEqual(messageContractTokens(source));
    expect(
      messageContractTokens('$заголовок <websites>Сайты</websites> $content'),
    ).not.toEqual(messageContractTokens(source));
    expect(
      messageContractTokens('$title <веб-сайты>Сайты</веб-сайты> $content'),
    ).not.toEqual(messageContractTokens(source));
  });

  it('keeps examples accepted by rate and duration validators copyable', () => {
    const contracts = {
      'proxyRoutes.rateLimitPlaceholder': ['512k', '1m'],
      'proxyRoutes.rateLimitHint': ['512k', '1m'],
      'proxyRoutes.reqLimitPlaceholder': ['10r/s', '100r/m'],
      'proxyRoutes.reqLimitHint': ['10r/s', '100r/m'],
      'proxyRoutes.validation.invalidLimitReq': ['10r/s', '100r/m'],
      'rateLimits.config.limitRateHint': ['512k', '1m'],
      'rateLimits.config.reqPerIpHint': ['10r/s', '100r/m'],
      'rateLimits.config.invalidReq': ['10r/s', '100r/m'],
      'builtinMetadata.configDescriptions.openresty_default_limit_req_per_ip': [
        '10r/s',
        '100r/m',
      ],
    };
    for (const [locale, messages] of Object.entries({
      en: english,
      'zh-CN': chinese,
      ru: russian,
    })) {
      for (const [key, tokens] of Object.entries(contracts)) {
        let value: unknown = messages;
        for (const part of key.split('.'))
          value =
            value && typeof value === 'object'
              ? (value as Record<string, unknown>)[part]
              : undefined;
        for (const token of tokens)
          expect(value, `${locale}:${key}`).toContain(token);
      }
    }
  });
});
