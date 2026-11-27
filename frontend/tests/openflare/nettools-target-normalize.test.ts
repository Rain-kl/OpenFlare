import { describe, expect, it } from 'vitest';

import { extractNetToolsTargetHost } from '@/app/(main)/network-tools/shared/utils';

describe('NetTools probe target host extraction', () => {
  it('strips scheme, path, query, and fragment from pasted URLs', () => {
    expect(
      extractNetToolsTargetHost('https://flare.arctel.net/network-tools/ping'),
    ).toEqual({ host: 'flare.arctel.net', port: null });
    expect(
      extractNetToolsTargetHost('http://example.com/health?x=1#frag'),
    ).toEqual({ host: 'example.com', port: null });
    expect(extractNetToolsTargetHost('HTTPS://Example.COM/Path')).toEqual({
      host: 'example.com',
      port: null,
    });
  });

  it('reduces scheme-less URLs that only carry a path', () => {
    expect(
      extractNetToolsTargetHost('flare.arctel.net/network-tools/ping'),
    ).toEqual({ host: 'flare.arctel.net', port: null });
    expect(extractNetToolsTargetHost('example.com?query=1')).toEqual({
      host: 'example.com',
      port: null,
    });
  });

  it('captures an explicit port from URLs and host:port forms', () => {
    expect(
      extractNetToolsTargetHost('https://example.com:8443/health'),
    ).toEqual({ host: 'example.com', port: 8443 });
    expect(extractNetToolsTargetHost('example.com:53')).toEqual({
      host: 'example.com',
      port: 53,
    });
    expect(extractNetToolsTargetHost('1.1.1.1:443')).toEqual({
      host: '1.1.1.1',
      port: 443,
    });
  });

  it('drops credentials from authority-style input', () => {
    expect(
      extractNetToolsTargetHost('https://user:pass@example.com/x'),
    ).toEqual({ host: 'example.com', port: null });
  });

  it('keeps bare hosts, IPs, and IPv6 literals intact', () => {
    expect(extractNetToolsTargetHost('  example.com  ')).toEqual({
      host: 'example.com',
      port: null,
    });
    expect(extractNetToolsTargetHost('1.1.1.1')).toEqual({
      host: '1.1.1.1',
      port: null,
    });
    expect(extractNetToolsTargetHost('2001:db8::1')).toEqual({
      host: '2001:db8::1',
      port: null,
    });
  });

  it('unwrackets bracketed IPv6 literals with an optional port', () => {
    expect(extractNetToolsTargetHost('[2001:db8::1]')).toEqual({
      host: '2001:db8::1',
      port: null,
    });
    expect(extractNetToolsTargetHost('https://[2001:db8::1]:443/')).toEqual({
      host: '2001:db8::1',
      port: 443,
    });
  });

  it('returns null for input without a usable host', () => {
    expect(extractNetToolsTargetHost('')).toBeNull();
    expect(extractNetToolsTargetHost('   ')).toBeNull();
    expect(extractNetToolsTargetHost('https://')).toBeNull();
    expect(extractNetToolsTargetHost('https:///path')).toBeNull();
  });

  it('leaves single-colon garbage to server-side validation', () => {
    expect(extractNetToolsTargetHost('example.com:notaport')).toEqual({
      host: 'example.com:notaport',
      port: null,
    });
    expect(extractNetToolsTargetHost('example.com:99999')).toEqual({
      host: 'example.com:99999',
      port: null,
    });
  });
});
