import { describe, expect, it } from 'vitest';

import {
  previewZoneDomainInput,
  resolveZoneDomainInput,
} from '@/app/(main)/websites/components/resolve-zone-domain-input';

const root = 'example.com';

describe('resolveZoneDomainInput', () => {
  it.each([
    ['api', 'api.example.com'],
    ['@', root],
    [root, root],
    ['www.api.example.com', 'www.api.example.com'],
    ['API.Example.COM', 'api.example.com'],
    ['*', '*.example.com'],
    ['*.example.com', '*.example.com'],
    ['*.s3.example.com', '*.s3.example.com'],
    ['*.S3.Example.COM', '*.s3.example.com'],
    [' *.Example.COM ', '*.example.com'],
    [' api ', 'api.example.com'],
    ['*.example.com\n', '*.example.com'],
    ['\t*.example.com', '*.example.com'],
    [`${'a'.repeat(63)}.example.com`, `${'a'.repeat(63)}.example.com`],
  ])('resolves %s to %s', (input, domain) => {
    expect(resolveZoneDomainInput(input, root)).toEqual({ domain });
    expect(previewZoneDomainInput(input, root)).toBe(domain);
  });

  it.each([
    'evil.com',
    '*.evil.com',
    '*.notexample.com',
    '*.example.com.evil.com',
    '*.s3',
    'api.s3',
  ])(
    'rejects %s outside the zone without expanding dotted shorthand',
    (input) => {
      expect(resolveZoneDomainInput(input, root)).toEqual({
        domain: '',
        error: 'mustBelongToZone',
      });
    },
  );

  it.each([
    'foo*',
    'foo*.example.com',
    'a.*',
    'a.*.example.com',
    '**',
    '**.example.com',
    '*.*.example.com',
    '*.s*.example.com',
  ])('rejects wildcard outside the single leftmost label: %s', (input) => {
    expect(resolveZoneDomainInput(input, root)).toEqual({
      domain: '',
      error: 'noWildcard',
    });
  });

  it.each([
    '-api',
    'api-',
    '_api',
    '-api.example.com',
    'api-.example.com',
    '_api.example.com',
    '*.bad_.example.com',
    '*.-bad.example.com',
    '*.bad-.example.com',
    '..example.com',
    '*.example..com',
    '*.example.com.',
    '*.example.com:443',
    '*.example.com;other',
    '*.example.com/path',
    '*.example.com?query',
    '*.example.com#fragment',
    'https://*.example.com',
    'https://api.example.com',
    'user@api.example.com',
    'api.example.com:80',
    'api.example.com;other',
    'a b.example.com',
    '*.s3 .example.com',
    '*.exa\nmple.com',
    '*.s3\t.example.com',
    `${'a'.repeat(64)}.example.com`,
    `*.${'a'.repeat(64)}.example.com`,
    `${`${'a'.repeat(63)}.`.repeat(4)}example.com`,
  ])('rejects malformed DNS names and unsafe input: %s', (input) => {
    expect(resolveZoneDomainInput(input, root).error).toBeTruthy();
    expect(previewZoneDomainInput(input, root)).toBe('');
  });

  it('checks total length after expanding shorthand', () => {
    const longRoot = `${'a'.repeat(63)}.${'b'.repeat(63)}.${'c'.repeat(63)}.${'d'.repeat(61)}`;
    expect(longRoot).toHaveLength(253);
    expect(resolveZoneDomainInput('@', longRoot)).toEqual({ domain: longRoot });
    expect(resolveZoneDomainInput('*', longRoot).error).toBe('invalidFormat');
    expect(resolveZoneDomainInput('api', longRoot).error).toBe('invalidFormat');
  });

  it('requires input and a selected, non-wildcard zone root', () => {
    expect(resolveZoneDomainInput('*', '').error).toBe('selectZone');
    expect(resolveZoneDomainInput('', root).error).toBe('enterDomain');
    expect(resolveZoneDomainInput('  ', root).error).toBe('enterDomain');
    expect(resolveZoneDomainInput('*', '*.example.com').error).toBe(
      'invalidFormat',
    );
    expect(resolveZoneDomainInput('*', 'Example.COM')).toEqual({
      domain: '*.example.com',
    });
  });
});
