import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';
import { NextRequest } from 'next/server';

let proxy: typeof import('../proxy').proxy;
beforeAll(async () => {
  vi.useFakeTimers();
  vi.stubEnv('WAVELET_RATE_LIMIT_ENABLED', 'true');
  ({ proxy } = await import('../proxy'));
});
afterAll(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.unstubAllEnvs();
});

describe('localized proxy rate limits', () => {
  it.each([
    ['ru', 'en', 'Слишком много запросов. Повторите через 5 секунд.'],
    ['en', 'ru', 'Too many requests; retry in 5 seconds'],
    [
      '',
      'de,ru-RU;q=0.9,en;q=0.8',
      'Слишком много запросов. Повторите через 5 секунд.',
    ],
  ])(
    'prefers cookie %s over Accept-Language %s',
    async (cookie, language, message) => {
      const request = new NextRequest('http://localhost/api/v1/oauth/login', {
        headers: {
          cookie: `NEXT_LOCALE=${cookie}`,
          'accept-language': language,
          'x-real-ip': `review-${cookie}-${language}`,
        },
      });
      expect(proxy(request).status).toBe(200);
      const limited = proxy(request);
      expect(limited.status).toBe(429);
      expect(limited.headers.get('retry-after')).toBe('5');
      expect(await limited.json()).toEqual({
        error_code: 'RATE_LIMITED',
        error_msg: message,
      });
    },
  );
});
