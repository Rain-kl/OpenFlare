import { describe, expect, it } from 'vitest';
import { formatSystemMetric } from './system-metrics';

describe('system metric presentation', () => {
  it('formats live values and legacy concatenated English durations', () => {
    expect(formatSystemMetric(50425, 'ru')).toBe('50\u00a0425');
    expect(formatSystemMetric('28 minutes41 seconds', 'ru')).toBe(
      '28 мин 41 с',
    );
    expect(formatSystemMetric('1 day 2 hours', 'ru')).toBe('1 дн. 2 ч');
    expect(formatSystemMetric('28 minutes41 seconds', 'en')).toBe('28m 41s');
    expect(formatSystemMetric('43 seconds', 'zh-CN')).toBe('43秒');
    expect(formatSystemMetric('5.8 MiB', 'ru')).toBe('5,8 MiB');
    expect(formatSystemMetric('0.001s', 'ru')).toBe('0,001 с');
    expect(formatSystemMetric('None', 'ru')).toBe('Нет');
    expect(formatSystemMetric('custom metric 42', 'ru')).toBe(
      'custom metric 42',
    );
  });
});
