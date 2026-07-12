import {describe, expect, it} from 'vitest';

import {resolveZoneDomainInput} from '@/app/(main)/websites/components/resolve-zone-domain-input';

describe('resolveZoneDomainInput', () => {
  it('maps short label and apex @ under zone root', () => {
    expect(resolveZoneDomainInput('api', 'example.com')).toEqual({
      domain: 'api.example.com',
    });
    expect(resolveZoneDomainInput('@', 'example.com')).toEqual({
      domain: 'example.com',
    });
  });

  it('accepts full FQDN under the zone', () => {
    expect(resolveZoneDomainInput('www.api.example.com', 'example.com')).toEqual({
      domain: 'www.api.example.com',
    });
    expect(resolveZoneDomainInput('example.com', 'example.com')).toEqual({
      domain: 'example.com',
    });
  });

  it('rejects foreign domains and wildcards', () => {
    expect(resolveZoneDomainInput('evil.com', 'example.com').error).toBeTruthy();
    expect(resolveZoneDomainInput('*.example.com', 'example.com').error).toBeTruthy();
  });
});
